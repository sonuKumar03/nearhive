from datetime import datetime, timezone
import json
from pathlib import Path
import uuid
import pytest

from nearhive_discovery.contracts import (
    DiscoveryJob,
    DiscoveryStatus,
    PublicationState,
    WorkArrangement,
)
from nearhive_discovery.sources.base import SourceAdapter
from nearhive_discovery.sources.greenhouse import GreenhouseSource

FIXTURES_DIR = Path(__file__).parent / "fixtures"


@pytest.fixture
def greenhouse_json() -> str:
    return (FIXTURES_DIR / "greenhouse.json").read_text(encoding="utf-8")


@pytest.fixture
def sample_job() -> DiscoveryJob:
    return DiscoveryJob(
        id=uuid.uuid4(),
        user_id=uuid.uuid4(),
        status=DiscoveryStatus.RUNNING,
        lat=30.2672,
        lng=-97.7431,
        radius_km=15.0,
    )


class TestGreenhouseSource:
    """Step 3: ATS adapter tests for Greenhouse."""

    def test_implements_source_adapter_protocol(self) -> None:
        source = GreenhouseSource(board_token="testco")
        assert isinstance(source, SourceAdapter)
        assert source.source_family == "job_ats"
        assert source.name == "greenhouse"

    @pytest.mark.asyncio
    async def test_extracts_technical_jobs_and_company_from_fixture(
        self, greenhouse_json: str, sample_job: DiscoveryJob
    ) -> None:
        async def mock_fetch(url: str) -> str:
            return greenhouse_json

        source = GreenhouseSource(
            board_token="novarobotics",
            company_name="Nova Robotics",
            company_domain="novarobotics.io",
            fetcher=mock_fetch,
            reference_time=datetime(2026, 9, 27, 0, 0, tzinfo=timezone.utc),
        )

        batches = []
        async for batch in source.run(sample_job):
            batches.append(batch)

        assert len(batches) >= 1
        first_batch = batches[0]
        assert first_batch.source == "greenhouse"
        assert first_batch.source_family == "job_ats"
        assert str(first_batch.discovery_job_id) == str(sample_job.id)

        # Company evidence
        assert len(first_batch.companies) == 1
        comp = first_batch.companies[0]
        assert comp.name == "Nova Robotics"
        assert comp.domain == "novarobotics.io"
        assert comp.source_record_id == "novarobotics"
        assert comp.evidence_url == "https://boards.greenhouse.io/novarobotics"
        assert comp.metadata.get("ats") == "greenhouse"
        assert len(comp.content_hash) > 0

        # Technical jobs: Only technical roles retained
        # In fixture: 101 (Software), 102 (DevOps), 103 (Data Science) are technical
        # 104 (Technical Recruiter), 105 (Enterprise Account Executive), 106 (Sanitation Engineer) are excluded
        jobs = [j for b in batches for j in b.jobs]
        job_ids = [j.source_job_id for j in jobs]
        assert "101" in job_ids
        assert "102" in job_ids
        assert "103" in job_ids
        assert "104" not in job_ids
        assert "105" not in job_ids
        assert "106" not in job_ids
        assert len(jobs) == 3

        # Verify Job 101 details
        job_101 = next(j for j in jobs if j.source_job_id == "101")
        assert job_101.title == "Senior Software Engineer"
        assert job_101.company_name == "Nova Robotics"
        assert job_101.company_domain == "novarobotics.io"
        assert job_101.canonical_url == "https://boards.greenhouse.io/novarobotics/jobs/101"
        assert job_101.location_raw == "Austin, TX"
        assert job_101.work_arrangement == WorkArrangement.HYBRID
        assert job_101.publication_state == PublicationState.POSTED_RECENTLY
        assert job_101.technical_classification == "software"
        assert job_101.rule_version == "technical-title-v1"
        assert len(job_101.classification_reasons) > 0
        assert "<p>" not in (job_101.description_excerpt or "")
        assert "backend distributed systems" in (job_101.description_excerpt or "")
        assert len(job_101.content_hash) > 0

        # Verify Job 102 arrangement and classification
        job_102 = next(j for j in jobs if j.source_job_id == "102")
        assert job_102.work_arrangement == WorkArrangement.REMOTE
        assert job_102.technical_classification == "infrastructure"

        # Verify Job 103 stale date (> 14 days ago)
        job_103 = next(j for j in jobs if j.source_job_id == "103")
        assert job_103.work_arrangement == WorkArrangement.IN_OFFICE
        assert job_103.publication_state == PublicationState.STALE
        assert job_103.technical_classification == "data"

    @pytest.mark.asyncio
    async def test_stable_deduplication_fingerprint(
        self, greenhouse_json: str, sample_job: DiscoveryJob
    ) -> None:
        async def mock_fetch(url: str) -> str:
            return greenhouse_json

        source1 = GreenhouseSource(board_token="novarobotics", fetcher=mock_fetch)
        source2 = GreenhouseSource(board_token="novarobotics", fetcher=mock_fetch)

        batches1 = [b async for b in source1.run(sample_job)]
        batches2 = [b async for b in source2.run(sample_job)]

        jobs1 = batches1[0].jobs
        jobs2 = batches2[0].jobs

        for j1, j2 in zip(jobs1, jobs2):
            assert j1.source_job_id == j2.source_job_id
            assert j1.content_hash == j2.content_hash

    @pytest.mark.asyncio
    async def test_pagination_and_page_ceiling(
        self, sample_job: DiscoveryJob
    ) -> None:
        fetched_urls = []

        async def mock_fetch(url: str) -> str:
            fetched_urls.append(url)
            # Return empty jobs on page 3
            if "page=3" in url:
                return json.dumps({"jobs": []})
            return json.dumps({
                "jobs": [
                    {
                        "id": 500 + len(fetched_urls),
                        "title": f"Software Engineer {len(fetched_urls)}",
                        "absolute_url": f"https://boards.greenhouse.io/testco/jobs/{500 + len(fetched_urls)}",
                        "location": {"name": "Remote"},
                        "workplace_type": "remote",
                        "updated_at": "2026-09-26T10:00:00Z",
                    }
                ]
            })

        source = GreenhouseSource(
            board_token="testco",
            fetcher=mock_fetch,
            page_ceiling=2,
        )

        batches = [b async for b in source.run(sample_job)]
        # Page ceiling is 2, so at most 2 pages requested
        assert len(fetched_urls) == 2
        all_jobs = [j for b in batches for j in b.jobs]
        assert len(all_jobs) == 2

    @pytest.mark.asyncio
    async def test_accepts_dynamically_discovered_board_tokens(
        self, sample_job: DiscoveryJob
    ) -> None:
        fetched_urls = []

        async def mock_fetch(url: str) -> str:
            fetched_urls.append(url)
            return json.dumps({"jobs": []})

        source = GreenhouseSource(board_token="initial", fetcher=mock_fetch)
        source.add_board(board_token="dynamic1", company_name="Dynamic 1", company_domain="dyn1.com")
        # Duplicate token should be ignored
        source.add_board(board_token="dynamic1", company_name="Dynamic 1 Again")
        source.add_board(board_token="initial")

        batches = [b async for b in source.run(sample_job)]
        # Should have fetched both 'initial' and 'dynamic1' once
        assert any("initial" in u for u in fetched_urls)
        assert any("dynamic1" in u for u in fetched_urls)
        assert len(fetched_urls) == 2

