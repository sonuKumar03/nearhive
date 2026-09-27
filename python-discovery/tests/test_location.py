import asyncio
from dataclasses import replace
import httpx
import pytest

from nearhive_discovery.contracts import TechnicalJobEvidence, WorkArrangement
from nearhive_discovery.location import LocationResolver, resolve_job_location


@pytest.fixture
def sample_technical_job() -> TechnicalJobEvidence:
    return TechnicalJobEvidence(
        company_name="Nova Corp",
        title="Senior Go Engineer",
        location_raw="Austin, TX",
        work_arrangement=WorkArrangement.ON_SITE,
    )


class TestLocationResolver:
    @pytest.mark.asyncio
    async def test_structured_coordinates_are_untouched(self) -> None:
        called = False

        def handler(req: httpx.Request) -> httpx.Response:
            nonlocal called
            called = True
            return httpx.Response(200, json=[{"lat": "30.2672", "lon": "-97.7431"}])

        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        resolver = LocationResolver(http_client=client)

        job = TechnicalJobEvidence(
            company_name="Nova Corp",
            title="Senior Go Engineer",
            location_raw="Austin, TX",
            lat=30.2672,
            lng=-97.7431,
            work_arrangement=WorkArrangement.HYBRID,
        )

        resolved = await resolve_job_location(job, resolver)
        assert called is False
        assert resolved.lat == 30.2672
        assert resolved.lng == -97.7431
        assert resolved.metadata.get("location_resolution") == "structured"

    @pytest.mark.asyncio
    async def test_remote_jobs_are_never_geocoded(self) -> None:
        called = False

        def handler(req: httpx.Request) -> httpx.Response:
            nonlocal called
            called = True
            return httpx.Response(200, json=[{"lat": "30.2672", "lon": "-97.7431"}])

        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        resolver = LocationResolver(http_client=client)

        job = TechnicalJobEvidence(
            company_name="Nova Corp",
            title="Senior Go Engineer",
            location_raw="Austin, TX (Remote)",
            work_arrangement=WorkArrangement.REMOTE,
        )

        resolved = await resolve_job_location(job, resolver)
        assert called is False
        assert resolved.lat is None
        assert resolved.lng is None
        assert resolved.metadata.get("location_resolution") == "unresolved"

    @pytest.mark.asyncio
    async def test_identical_normalized_strings_call_geocoder_once(self) -> None:
        call_count = 0

        def handler(req: httpx.Request) -> httpx.Response:
            nonlocal call_count
            call_count += 1
            return httpx.Response(200, json=[{"lat": "30.2672", "lon": "-97.7431"}])

        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        resolver = LocationResolver(http_client=client)

        job1 = TechnicalJobEvidence(
            company_name="Nova Corp",
            title="Job 1",
            location_raw="Austin, TX",
            work_arrangement=WorkArrangement.ON_SITE,
        )
        job2 = TechnicalJobEvidence(
            company_name="Nova Corp",
            title="Job 2",
            location_raw="  austin, tx  ",
            work_arrangement=WorkArrangement.ON_SITE,
        )

        r1 = await resolve_job_location(job1, resolver)
        r2 = await resolve_job_location(job2, resolver)

        assert call_count == 1
        assert r1.lat == 30.2672
        assert r2.lat == 30.2672
        assert r1.metadata.get("location_resolution") == "geocoded"
        assert r2.metadata.get("location_resolution") == "geocoded"

    @pytest.mark.asyncio
    async def test_invalid_or_out_of_range_results_are_discarded(self) -> None:
        def handler(req: httpx.Request) -> httpx.Response:
            return httpx.Response(200, json=[{"lat": "999.0", "lon": "500.0"}])

        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        resolver = LocationResolver(http_client=client)

        job = TechnicalJobEvidence(
            company_name="Nova Corp",
            title="Job",
            location_raw="Invalid Land",
            work_arrangement=WorkArrangement.ON_SITE,
        )

        resolved = await resolve_job_location(job, resolver)
        assert resolved.lat is None
        assert resolved.lng is None
        assert resolved.metadata.get("location_resolution") == "unresolved"

    @pytest.mark.asyncio
    async def test_timeout_or_empty_result_leaves_job_unresolved(self) -> None:
        def handler(req: httpx.Request) -> httpx.Response:
            return httpx.Response(200, json=[])

        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        resolver = LocationResolver(http_client=client)

        job = TechnicalJobEvidence(
            company_name="Nova Corp",
            title="Job",
            location_raw="Nonexistent Nowhere",
            work_arrangement=WorkArrangement.ON_SITE,
        )

        resolved = await resolve_job_location(job, resolver)
        assert resolved.lat is None
        assert resolved.lng is None
        assert resolved.metadata.get("location_resolution") == "unresolved"

    @pytest.mark.asyncio
    async def test_valid_result_adds_coordinates_and_provenance(self) -> None:
        def handler(req: httpx.Request) -> httpx.Response:
            return httpx.Response(
                200,
                json=[{"lat": "12.9716", "lon": "77.5946", "display_name": "Bengaluru, India"}],
            )

        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        resolver = LocationResolver(http_client=client, provider_name="nominatim")

        job = TechnicalJobEvidence(
            company_name="Nova Corp",
            title="Go Backend Lead",
            location_raw="Bengaluru, Karnataka, India",
            work_arrangement=WorkArrangement.ON_SITE,
        )

        resolved = await resolve_job_location(job, resolver)
        assert resolved.lat == 12.9716
        assert resolved.lng == 77.5946
        assert resolved.metadata.get("location_resolution") == "geocoded"
        assert resolved.metadata.get("location_provider") == "nominatim"

    @pytest.mark.asyncio
    async def test_ceiling_caps_geocoding_lookups(self) -> None:
        call_count = 0

        def handler(req: httpx.Request) -> httpx.Response:
            nonlocal call_count
            call_count += 1
            return httpx.Response(200, json=[{"lat": "10.0", "lon": "20.0"}])

        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        resolver = LocationResolver(http_client=client, max_lookups=1)

        job1 = TechnicalJobEvidence(
            company_name="Nova Corp",
            title="Job 1",
            location_raw="City 1",
            work_arrangement=WorkArrangement.ON_SITE,
        )
        job2 = TechnicalJobEvidence(
            company_name="Nova Corp",
            title="Job 2",
            location_raw="City 2",
            work_arrangement=WorkArrangement.ON_SITE,
        )

        r1 = await resolve_job_location(job1, resolver)
        r2 = await resolve_job_location(job2, resolver)

        assert call_count == 1
        assert r1.lat == 10.0
        assert r1.metadata.get("location_resolution") == "geocoded"
        assert r2.lat is None
        assert r2.metadata.get("location_resolution") == "unresolved"
