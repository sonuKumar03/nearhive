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
from nearhive_discovery.sources.lever import LeverSource

FIXTURES_DIR = Path(__file__).parent / "fixtures"


@pytest.fixture
def lever_json() -> str:
    return (FIXTURES_DIR / "lever.json").read_text(encoding="utf-8")


@pytest.fixture
def sample_job() -> DiscoveryJob:
    return DiscoveryJob(
        id=uuid.uuid4(),
        user_id=uuid.uuid4(),
        status=DiscoveryStatus.RUNNING,
        lat=37.7749,
        lng=-122.4194,
        radius_km=15.0,
    )


class TestLeverSource:
    """Step 3: ATS adapter tests for Lever."""

    def test_implements_source_adapter_protocol(self) -> None:
        source = LeverSource(site="testco")
        assert isinstance(source, SourceAdapter)
        assert source.source_family == "job_ats"
        assert source.name == "lever"

    @pytest.mark.asyncio
    async def test_extracts_technical_jobs_and_company_from_fixture(
        self, lever_json: str, sample_job: DiscoveryJob
    ) -> None:
        async def mock_fetch(url: str) -> str:
            return lever_json

        source = LeverSource(
            site="apexcloud",
            company_name="Apex Cloud Services",
            company_domain="apexcloud.io",
            fetcher=mock_fetch,
        )

        batches = []
        async for batch in source.run(sample_job):
            batches.append(batch)

        assert len(batches) >= 1
        first_batch = batches[0]
        assert first_batch.source == "lever"
        assert first_batch.source_family == "job_ats"
        assert str(first_batch.discovery_job_id) == str(sample_job.id)

        # Company evidence
        assert len(first_batch.companies) == 1
        comp = first_batch.companies[0]
        assert comp.name == "Apex Cloud Services"
        assert comp.domain == "apexcloud.io"
        assert comp.source_record_id == "apexcloud"
        assert comp.evidence_url == "https://jobs.lever.co/apexcloud"
        assert comp.metadata.get("ats") == "lever"
        assert len(comp.content_hash) > 0

        # Technical jobs: Only technical roles retained
        # In fixture: lev-201 (Backend), lev-202 (SRE), lev-203 (QA) are technical
        # lev-204 (Locomotive Engineer), lev-205 (Staff Accountant) are excluded
        jobs = [j for b in batches for j in b.jobs]
        job_ids = [j.source_job_id for j in jobs]
        assert "lev-201" in job_ids
        assert "lev-202" in job_ids
        assert "lev-203" in job_ids
        assert "lev-204" not in job_ids
        assert "lev-205" not in job_ids
        assert len(jobs) == 3

        # Verify Job lev-201 details
        job_201 = next(j for j in jobs if j.source_job_id == "lev-201")
        assert job_201.title == "Principal Backend Engineer"
        assert job_201.company_name == "Apex Cloud Services"
        assert job_201.company_domain == "apexcloud.io"
        assert job_201.canonical_url == "https://jobs.lever.co/apexcloud/lev-201"
        assert job_201.location_raw == "San Francisco, CA"
        assert job_201.work_arrangement == WorkArrangement.HYBRID
        assert job_201.publication_state == PublicationState.POSTED_RECENTLY
        assert job_201.technical_classification == "software"
        assert job_201.rule_version == "technical-title-v1"
        assert len(job_201.classification_reasons) > 0
        assert "distributed systems" in (job_201.description_excerpt or "")
        assert len(job_201.content_hash) > 0

        # Verify Job lev-202 arrangement and classification
        job_202 = next(j for j in jobs if j.source_job_id == "lev-202")
        assert job_202.work_arrangement == WorkArrangement.REMOTE
        assert job_202.technical_classification == "infrastructure"

        # Verify Job lev-203 stale date and in_office
        job_203 = next(j for j in jobs if j.source_job_id == "lev-203")
        assert job_203.work_arrangement == WorkArrangement.IN_OFFICE
        assert job_203.publication_state == PublicationState.STALE
        assert job_203.technical_classification == "qa_automation"

    @pytest.mark.asyncio
    async def test_stable_deduplication_fingerprint(
        self, lever_json: str, sample_job: DiscoveryJob
    ) -> None:
        async def mock_fetch(url: str) -> str:
            return lever_json

        source1 = LeverSource(site="apexcloud", fetcher=mock_fetch)
        source2 = LeverSource(site="apexcloud", fetcher=mock_fetch)

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
            if "skip=40" in url:
                return "[]"
            return json.dumps([
                {
                    "id": f"lev-page-{len(fetched_urls)}",
                    "text": f"Software Engineer {len(fetched_urls)}",
                    "hostedUrl": f"https://jobs.lever.co/testco/lev-page-{len(fetched_urls)}",
                    "categories": {"location": "Remote", "workplaceType": "remote"},
                    "createdAt": 1790380800000,
                }
            ])

        source = LeverSource(
            site="testco",
            fetcher=mock_fetch,
            page_ceiling=2,
        )

        batches = [b async for b in source.run(sample_job)]
        assert len(fetched_urls) == 2
        all_jobs = [j for b in batches for j in b.jobs]
        assert len(all_jobs) == 2
