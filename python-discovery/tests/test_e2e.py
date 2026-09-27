import os
import subprocess
import time
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Generator

import httpx
import psycopg
import pytest

from nearhive_discovery.client import IngestionClient
from nearhive_discovery.contracts import DiscoveryJob, DiscoveryStatus
from nearhive_discovery.http import PlaywrightPool
from nearhive_discovery.location import LocationResolver
from nearhive_discovery.sources.company_site import CompanySiteSource
from nearhive_discovery.sources.configured_directory import ConfiguredDirectorySource
from nearhive_discovery.sources.greenhouse import GreenhouseSource
from nearhive_discovery.worker import Worker
from tests.fake_site.server import run_fake_site_server

DEFAULT_DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgres://postgres:postgres@localhost:5432/nearhive?sslmode=disable",
)
DEFAULT_WORKER_TOKEN = os.getenv("NEARHIVE_WORKER_TOKEN", "dev-worker-token")


class FailingSource:
    """Simulated failing source adapter to test partial failure handling."""

    name: str = "failing_source"
    source_family: str = "test_fail"

    def run(self, job: DiscoveryJob):
        raise RuntimeError("Simulated source failure to verify partial status resilience")


@pytest.fixture(scope="session")
def api_base_url() -> Generator[str, None, None]:
    """Ensures a Go API server is reachable, launching a local test server if needed."""
    candidate_url = os.getenv("NEARHIVE_API_URL", "http://localhost:8080").rstrip("/")

    # Check if existing server is responsive and has the internal discovery batch route
    is_live = False
    try:
        resp = httpx.get(f"{candidate_url}/health", timeout=1.0)
        if resp.status_code == 200:
            batch_check = httpx.post(
                f"{candidate_url}/api/v1/internal/discovery/batches",
                headers={"X-NearHive-Worker-Token": DEFAULT_WORKER_TOKEN},
                json={"contract_version": 1, "discovery_job_id": str(uuid.uuid4()), "source": "t", "source_family": "t", "observed_at": "2026-09-27T00:00:00Z", "companies": [], "jobs": []},
                timeout=1.0,
            )
            jobs_check = httpx.get(f"{candidate_url}/api/v1/search/jobs", timeout=1.0)
            if batch_check.status_code == 200 and jobs_check.status_code == 401:
                is_live = True
    except Exception:
        is_live = False

    if is_live:
        yield candidate_url
        return

    # If no live server, start local nearhive serve on port 18080
    test_port = "18080"
    repo_root = Path(__file__).resolve().parent.parent.parent
    bin_path = repo_root / "bin" / "nearhive"
    if not bin_path.exists():
        subprocess.run(["go", "build", "-o", str(bin_path), "./cmd/nearhive"], cwd=str(repo_root), check=True)

    env = os.environ.copy()
    env["PORT"] = test_port
    env["DATABASE_URL"] = DEFAULT_DATABASE_URL
    env["DISCOVERY_WORKER_TOKEN"] = DEFAULT_WORKER_TOKEN
    env["JWT_SECRET"] = "nearhive-dev-local-secret-32-chars-min!"
    env["ENVIRONMENT"] = "development"

    proc = subprocess.Popen([str(bin_path), "serve"], cwd=str(repo_root), env=env)
    local_url = f"http://127.0.0.1:{test_port}"

    # Wait for server to boot
    for _ in range(50):
        try:
            r = httpx.get(f"{local_url}/health", timeout=0.5)
            if r.status_code == 200:
                break
        except Exception:
            time.sleep(0.1)

    try:
        yield local_url
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=3.0)
        except Exception:
            proc.kill()


@pytest.fixture
def db_conn():
    conn = psycopg.connect(DEFAULT_DATABASE_URL, autocommit=True)
    yield conn
    conn.close()


@pytest.fixture
def test_auth(api_base_url: str) -> dict[str, str]:
    """Registers a fresh test user and returns Authorization headers."""
    email = f"e2e-user-{uuid.uuid4().hex[:8]}@example.com"
    resp = httpx.post(
        f"{api_base_url}/api/v1/auth/register",
        json={"email": email, "password": "password123!"},
        timeout=5.0,
    )
    assert resp.status_code == 201, f"Failed to register user: {resp.text}"
    token = resp.json()["token"]
    return {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }


def test_end_to_end_discovery_stack(
    api_base_url: str,
    db_conn: psycopg.Connection,
    test_auth: dict[str, str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Executes a full discovery cycle and validates results through the Go REST API.

    Verifies:
    1. Local fake source serving ordinary HTML, JSON-LD, ATS fixtures, and a JS-rendered page.
    2. Discovery job creation in Postgres.
    3. Worker execution cycle with Playwright rendering and partial-source fault handling.
    4. Searchable companies and presence determination via GET /api/v1/search.
    5. Provenance audit trail via GET /api/v1/companies/{id}/sightings.
    6. Recent technical job extraction and classification via GET /api/v1/companies/{id}/technical-jobs.
    """
    monkeypatch.setenv("NEARHIVE_ALLOW_LOCAL_DISCOVERY", "true")

    with run_fake_site_server() as fake_site:
        port = fake_site.port

        # 1. Create discovery job via Go API
        bangalore_lat = 12.9716
        bangalore_lng = 77.5946
        job_create_resp = httpx.post(
            f"{api_base_url}/api/v1/discovery/jobs",
            headers=test_auth,
            json={
                "lat": bangalore_lat,
                "lng": bangalore_lng,
                "radius_km": 15.0,
            },
            timeout=5.0,
        )
        assert job_create_resp.status_code == 201, f"Create job failed: {job_create_resp.text}"
        job_payload = job_create_resp.json()
        job_id = job_payload["id"]
        assert job_payload["status"] == "pending"

        # 2. Configure source adapters
        # (a) Configured directory source (ordinary HTML + detail page)
        dir_config = {
            "id": "tech_parks_directory",
            "name": "Tech Parks Directory",
            "url_template": f"{fake_site.base_url}/directory?lat={{lat}}&lng={{lng}}&radius={{radius}}&page={{page}}",
            "pagination": {"page_param": "page", "start_page": 1, "page_ceiling": 1},
            "selectors": {
                "card": ".directory-card",
                "name": ".company-title::text",
                "source_record_id": "@data-company-id",
                "detail_url": "a.detail-link::attr(href)",
                "website": "a.website-link::attr(href)",
                "address": ".company-location::text",
                "phone": ".company-phone::text",
            },
            "detail_selectors": {
                "address": ".detail-address::text",
                "lat": ".geo-lat::text",
                "lng": ".geo-lng::text",
                "phone": ".detail-phone::text",
                "website": "a.official-website::attr(href)",
            },
        }
        dir_source = ConfiguredDirectorySource(config=dir_config, resolve_dns=False)

        # (b) Official company website source (ordinary HTML + JSON-LD)
        site_source = CompanySiteSource(
            target_url=f"{fake_site.base_url}/company/apex",
            max_pages=5,
            max_depth=2,
            resolve_dns=False,
        )

        # (c) ATS source (public Greenhouse jobs fixture)
        ats_source = GreenhouseSource(
            boards=[
                {
                    "board_token": "apexinnovations",
                    "company_name": "Apex Innovations",
                    "company_domain": "apexinnovations.example.com",
                }
            ],
            api_base_url=f"{fake_site.base_url}/ats/greenhouse",
        )

        # (d) JavaScript-rendered company source (Playwright SPA fallback)
        js_source = CompanySiteSource(
            target_url=f"{fake_site.base_url}/js/quantum",
            max_pages=2,
            max_depth=1,
            resolve_dns=False,
        )

        # (e) Failing source to test partial-source handling
        failing_source = FailingSource()

        sources = [dir_source, site_source, ats_source, js_source, failing_source]

        # 3. Execute one worker cycle
        playwright_pool = PlaywrightPool(max_contexts=2, headless=True)
        ingestion_client = IngestionClient(
            base_url=api_base_url,
            worker_token=DEFAULT_WORKER_TOKEN,
        )

        geo_client = httpx.AsyncClient()
        resolver = LocationResolver(
            http_client=geo_client,
            base_url=f"{fake_site.base_url}/geocoder",
        )

        worker = Worker(
            db_url=DEFAULT_DATABASE_URL,
            client=ingestion_client,
            sources=sources,
            worker_id="test-e2e-worker",
            playwright_pool=playwright_pool,
            location_resolver=resolver,
        )

        try:
            handled = worker.run_once()
            assert handled is True, "Worker did not claim or process the discovery job"
        finally:
            worker.close()

        # 4. Verify discovery job status through Go API
        job_get_resp = httpx.get(
            f"{api_base_url}/api/v1/discovery/jobs/{job_id}",
            headers=test_auth,
            timeout=5.0,
        )
        assert job_get_resp.status_code == 200, f"Get job failed: {job_get_resp.text}"
        finished_job = job_get_resp.json()

        # Partial status verified because 4 sources succeeded and 1 source failed
        assert finished_job["status"] == "partial", f"Expected partial status, got: {finished_job['status']}"
        assert finished_job["company_count"] >= 2, f"Expected >= 2 companies, got: {finished_job['company_count']}"
        assert finished_job["job_count"] >= 1, f"Expected >= 1 job, got: {finished_job['job_count']}"
        assert finished_job["evidence_count"] >= 3, f"Expected >= 3 evidence items, got: {finished_job['evidence_count']}"

        # 5. Verify searchable companies and presence via GET /api/v1/search
        search_resp = httpx.get(
            f"{api_base_url}/api/v1/search?lat={bangalore_lat}&lng={bangalore_lng}&radius=15",
            headers=test_auth,
            timeout=5.0,
        )
        assert search_resp.status_code == 200, f"Search failed: {search_resp.text}"
        search_data = search_resp.json()
        companies = search_data.get("companies", [])
        assert len(companies) >= 2, f"Expected at least 2 discovered companies in search: {companies}"

        company_names = [c["name"] for c in companies]
        assert any("Apex Innovations" in name for name in company_names), f"Apex Innovations missing from search: {company_names}"
        assert any("Quantum Robotics" in name for name in company_names), f"Quantum Robotics (JS-rendered) missing from search: {company_names}"

        # Find Apex Innovations in search results
        apex_comp = next(c for c in companies if "Apex Innovations" in c["name"])
        apex_id = apex_comp["id"]

        # Assert presence calculation on company / locations
        # Apex has evidence from public_directory, official_site, and job_ats -> confirmed_office
        assert apex_comp.get("presence_type") in ("confirmed_office", "probable_office"), (
            f"Unexpected presence type on search result: {apex_comp.get('presence_type')}"
        )

        # 6. Verify company profile via GET /api/v1/companies/{id}
        comp_resp = httpx.get(
            f"{api_base_url}/api/v1/companies/{apex_id}",
            headers=test_auth,
            timeout=5.0,
        )
        comp_data = comp_resp.json().get("company", {})
        assert comp_data.get("id") == apex_id
        assert "Apex Innovations" in comp_data.get("name", "")

        # 7. Verify provenance via GET /api/v1/companies/{id}/sightings
        sightings_resp = httpx.get(
            f"{api_base_url}/api/v1/companies/{apex_id}/sightings",
            headers=test_auth,
            timeout=5.0,
        )
        assert sightings_resp.status_code == 200, f"Get sightings failed: {sightings_resp.text}"
        sightings_data = sightings_resp.json().get("sightings", [])
        assert len(sightings_data) >= 1, "Expected sightings for discovered company"

        for s in sightings_data:
            assert s.get("source"), "Sighting missing source identifier"
            assert s.get("scraped_at") or s.get("first_seen_at"), "Sighting missing timestamp"
            assert s.get("content_hash"), "Sighting missing content_hash"

        # 8. Verify recent technical jobs via GET /api/v1/companies/{id}/technical-jobs
        tech_jobs_resp = httpx.get(
            f"{api_base_url}/api/v1/companies/{apex_id}/technical-jobs",
            headers=test_auth,
            timeout=5.0,
        )
        assert tech_jobs_resp.status_code == 200, f"Get technical jobs failed: {tech_jobs_resp.text}"
        jobs_list = tech_jobs_resp.json().get("technical_jobs", [])
        assert len(jobs_list) >= 1, f"Expected technical jobs for company, got: {jobs_list}"

        job = jobs_list[0]
        assert job.get("title"), "Job missing title"
        assert job.get("publication_state") in ("posted_recently", "observed_recently"), (
            f"Unexpected publication_state: {job.get('publication_state')}"
        )
        assert job.get("technical_classification") != "", "Job technical classification should not be empty"

        # 9. Verify spatial job search via GET /api/v1/search/jobs
        search_jobs_resp = httpx.get(
            f"{api_base_url}/api/v1/search/jobs?lat={bangalore_lat}&lng={bangalore_lng}&radius=15",
            headers=test_auth,
            timeout=5.0,
        )
        assert search_jobs_resp.status_code == 200, f"Search jobs failed: {search_jobs_resp.text}"
        search_jobs_data = search_jobs_resp.json()
        spatial_jobs = search_jobs_data.get("jobs", [])
        assert len(spatial_jobs) >= 1, f"Expected spatial jobs in search: {search_jobs_data}"

        job_titles = [j["title"] for j in spatial_jobs]
        assert "Principal Infrastructure Architect" in job_titles, (
            f"Recent local technical job missing from spatial search: {job_titles}"
        )
        assert "Legacy Systems Programmer" not in job_titles, (
            f"Stale job should NOT be in spatial search: {job_titles}"
        )
        assert "Remote Cloud Developer" not in job_titles, (
            f"Remote job should NOT be in spatial search: {job_titles}"
        )

        matched_job = next(j for j in spatial_jobs if j["title"] == "Principal Infrastructure Architect")
        assert matched_job["distance_meters"] <= 15000, "Job distance should be within 15km"
        assert matched_job["work_arrangement"] in ("in_office", "hybrid")
        assert matched_job["company_name"] == "Apex Innovations"
