from datetime import datetime, timezone
from pathlib import Path
import pytest
from scrapy.http import HtmlResponse

from nearhive_discovery.contracts import (
    CompanyEvidence,
    DiscoveryJob,
    DiscoveryStatus,
    EvidenceBatch,
    PublicationState,
    TechnicalJobEvidence,
    WorkArrangement,
)
from nearhive_discovery.sources.base import SourceAdapter
from nearhive_discovery.sources.jsonld import extract_jsonld

FIXTURES_DIR = Path(__file__).parent / "fixtures"


class TestSourceAdapterProtocol:
    """Tests the SourceAdapter protocol definition."""

    def test_protocol_runtime_checkable(self) -> None:
        class ValidAsyncAdapter:
            name = "test-adapter"
            source_family = "test-family"

            async def run(self, job: DiscoveryJob):
                yield EvidenceBatch(
                    contract_version=1,
                    discovery_job_id=job.id,
                    source=self.name,
                    source_family=self.source_family,
                    observed_at=datetime.now(timezone.utc),
                )

        adapter = ValidAsyncAdapter()
        assert isinstance(adapter, SourceAdapter)
        assert adapter.name == "test-adapter"
        assert adapter.source_family == "test-family"

    def test_protocol_rejects_missing_attributes(self) -> None:
        class IncompleteAdapter:
            name = "incomplete"

        assert not isinstance(IncompleteAdapter(), SourceAdapter)


class TestJSONLDExtraction:
    """Tests JSON-LD extraction from HTML pages and responses."""

    def test_extract_company_fixture(self) -> None:
        fixture_path = FIXTURES_DIR / "jsonld_company.html"
        html = fixture_path.read_text(encoding="utf-8")

        response = HtmlResponse(
            url="https://acme.example.com",
            body=html.encode("utf-8"),
            encoding="utf-8",
        )

        companies, jobs = extract_jsonld(response)

        # Asserts malformed block was skipped and both valid entities were parsed
        assert len(companies) == 2
        assert len(jobs) == 0

        # Entity 1: Organization
        org = next(c for c in companies if c.name == "Acme Corporation")
        assert org.source_record_id == "https://acme.example.com/#org"
        assert org.domain == "acme.example.com"
        assert org.evidence_url == "https://acme.example.com"
        assert org.phone == "+1-512-555-0100"
        assert "100 Tech Blvd" in org.address
        assert "Austin" in org.address
        assert "TX" in org.address
        assert "78701" in org.address
        assert org.content_hash != ""

        # Entity 2: LocalBusiness from @graph with coordinates
        lab = next(c for c in companies if c.name == "Acme Austin Lab")
        assert lab.source_record_id == "https://acme.example.com/labs/austin"
        assert lab.domain == "acme.example.com"
        assert lab.evidence_url == "https://acme.example.com/labs/austin"
        assert lab.phone == "+1-512-555-0199"
        assert "102 Tech Blvd Suite 400" in lab.address
        assert lab.lat == pytest.approx(30.2672)
        assert lab.lng == pytest.approx(-97.7431)
        assert lab.content_hash != ""

    def test_extract_jobs_fixture(self) -> None:
        fixture_path = FIXTURES_DIR / "jsonld_jobs.html"
        html = fixture_path.read_text(encoding="utf-8")

        response = HtmlResponse(
            url="https://cloudscale.example.com/careers",
            body=html.encode("utf-8"),
            encoding="utf-8",
        )

        companies, jobs = extract_jsonld(response)

        assert len(jobs) == 2

        # Job 1: Remote Senior Backend Engineer
        job1 = next(j for j in jobs if j.title == "Senior Backend Engineer")
        assert job1.company_name == "CloudScale Systems"
        assert job1.company_domain == "cloudscale.example.com"
        assert job1.source_job_id == "JOB-1024"
        assert job1.canonical_url == "https://cloudscale.example.com/jobs/senior-backend-engineer"
        assert job1.work_arrangement == WorkArrangement.REMOTE
        assert "500 Main St" in job1.location_raw
        assert job1.lat == pytest.approx(30.2680)
        assert job1.lng == pytest.approx(-97.7440)
        assert job1.posted_at is not None
        assert job1.posted_at.year == 2026
        assert job1.posted_at_confidence == 1.0
        assert "<p>" not in (job1.description_excerpt or "")
        assert "Senior Backend Engineer" in (job1.description_excerpt or "")
        assert job1.content_hash != ""

        # Job 2: Staff Infrastructure Engineer (older posting from @graph)
        job2 = next(j for j in jobs if j.title == "Staff Infrastructure Engineer")
        assert job2.company_name == "CloudScale Systems"
        assert job2.source_job_id == "https://cloudscale.example.com/jobs/staff-infra-2048"
        assert job2.canonical_url == "https://cloudscale.example.com/jobs/staff-infra-engineer"
        assert "Austin, TX" in job2.location_raw
        assert job2.posted_at is not None
        assert job2.posted_at.month == 1
        assert job2.publication_state == PublicationState.STALE
        assert job2.content_hash != ""

    def test_extract_pure_string_and_bytes_input(self) -> None:
        raw_html = """
        <html>
        <head>
          <script type="application/ld+json">
          {
            "@context": "https://schema.org",
            "@type": "Organization",
            "name": "Pure String Company",
            "url": "https://purestring.example.com",
            "address": "123 String Way, Denver, CO"
          }
          </script>
        </head>
        </html>
        """
        # Test passing string
        companies1, jobs1 = extract_jsonld(raw_html, base_url="https://purestring.example.com")
        assert len(companies1) == 1
        assert companies1[0].name == "Pure String Company"
        assert companies1[0].address == "123 String Way, Denver, CO"

        # Test passing bytes
        companies2, jobs2 = extract_jsonld(raw_html.encode("utf-8"), base_url="https://purestring.example.com")
        assert len(companies2) == 1
        assert companies2[0].name == "Pure String Company"

    def test_resilience_to_malformed_and_missing_data(self) -> None:
        html = """
        <html>
        <head>
          <script type="application/ld+json">not even json</script>
          <script type="application/ld+json">{"@type": "Organization"}</script>
          <script type="application/ld+json">{"@type": "JobPosting"}</script>
          <script type="application/ld+json">["random", "list", 123]</script>
          <script type="application/ld+json">
          {
            "@context": "https://schema.org",
            "@type": "Organization",
            "name": "Valid Survivor"
          }
          </script>
        </head>
        </html>
        """
        companies, jobs = extract_jsonld(html)
        assert len(companies) == 1
        assert companies[0].name == "Valid Survivor"
        assert len(jobs) == 0
