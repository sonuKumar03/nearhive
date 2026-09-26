import uuid
from datetime import datetime, timezone
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock
import pytest

from nearhive_discovery.contracts import (
    CompanyEvidence,
    DiscoveryJob,
    DiscoveryStatus,
    EvidenceBatch,
)
from nearhive_discovery.sources.base import SourceAdapter
from nearhive_discovery.sources.configured_directory import ConfiguredDirectorySource

FIXTURES_DIR = Path(__file__).parent / "fixtures"


@pytest.fixture
def directory_html() -> str:
    return (FIXTURES_DIR / "directory.html").read_text(encoding="utf-8")


@pytest.fixture
def directory_detail_html() -> str:
    return (FIXTURES_DIR / "directory_detail.html").read_text(encoding="utf-8")


@pytest.fixture
def sample_job() -> DiscoveryJob:
    return DiscoveryJob(
        id=uuid.uuid4(),
        user_id=uuid.uuid4(),
        status=DiscoveryStatus.RUNNING,
        lat=12.9716,
        lng=77.5946,
        radius_km=15.0,
    )


class TestConfiguredDirectorySource:
    """Tests for YAML-configured directory parsing and pagination."""

    def test_implements_source_adapter_protocol(self) -> None:
        source = ConfiguredDirectorySource(
            config={
                "id": "test_directory",
                "name": "Test Directory",
                "url_template": "https://directory.example.com/search?lat={lat}&lng={lng}&radius={radius}&page={page}",
                "selectors": {"card": ".directory-card", "name": ".company-title::text"},
            }
        )
        assert isinstance(source, SourceAdapter)
        assert source.name == "test_directory"
        assert source.source_family == "public_directory"

    def test_url_formatting_with_location_and_radius(self, sample_job: DiscoveryJob) -> None:
        source = ConfiguredDirectorySource(
            config={
                "id": "geo_directory",
                "name": "Geo Directory",
                "url_template": "https://directory.example.com/items?lat={lat}&lng={lng}&radius={radius}&page={page}",
            }
        )
        url = source.format_url(source.url_template, sample_job, page=2)
        assert url == f"https://directory.example.com/items?lat={sample_job.lat}&lng={sample_job.lng}&radius={sample_job.radius_km}&page=2"

    @pytest.mark.asyncio
    async def test_parses_cards_and_skips_malformed(
        self, sample_job: DiscoveryJob, directory_html: str, directory_detail_html: str
    ) -> None:
        """Asserts valid cards are extracted, provenance URL is set, and malformed card is skipped."""
        config = {
            "id": "tech_parks_dir",
            "name": "Tech Parks Directory",
            "url_template": "https://directory.example.com/search?lat={lat}&lng={lng}&radius={radius}&page={page}",
            "pagination": {"page_ceiling": 1},
            "selectors": {
                "card": ".directory-card",
                "name": ".company-title::text",
                "source_record_id": "@data-company-id",
                "address": ".company-location::text",
                "phone": ".company-phone::text",
                "detail_url": "a.detail-link::attr(href)",
                "website": "a.website-link::attr(href)",
            },
            "detail_selectors": {
                "address": ".detail-address::text",
                "phone": ".detail-phone::text",
                "website": "a.official-website::attr(href)",
                "lat": ".geo-lat::text",
                "lng": ".geo-lng::text",
            },
        }

        # Mock fetcher to return directory HTML for list and detail HTML for detail pages
        async def mock_fetch(url: str) -> str:
            if "/directory/companies/dir-101" in url:
                return directory_detail_html
            return directory_html

        source = ConfiguredDirectorySource(config=config, fetcher=mock_fetch)

        batches = []
        async for batch in source.run(sample_job):
            batches.append(batch)

        assert len(batches) >= 1
        all_companies = [c for b in batches for c in b.companies]

        # In directory.html: 3 valid cards (dir-101, dir-102, dir-103) and 1 malformed card without title
        # The malformed card must be skipped gracefully without failing the run
        assert len(all_companies) == 3
        names = [c.name for c in all_companies]
        assert "Acme Robotics" in names
        assert "Quantum Data Systems" in names
        assert "Apex Cloud Services" in names

        # Check Acme Robotics (which had a detail page visited)
        acme = next(c for c in all_companies if c.name == "Acme Robotics")
        assert acme.source_record_id == "dir-101"
        assert acme.lat == 12.9854
        assert acme.lng == 77.7366
        # Detail address should override or enrich listing address
        assert "Building 3, Unit 402" in acme.address
        assert acme.domain == "acmerobotics.example.com"
        assert acme.evidence_url == "https://directory.example.com/directory/companies/dir-101"

        # Check Quantum Data Systems (no detail page response mocked or kept list page provenance)
        quantum = next(c for c in all_companies if c.name == "Quantum Data Systems")
        assert quantum.source_record_id == "dir-102"
        assert quantum.domain == "quantumdata.example.com"
        assert "Electronic City" in quantum.address

    @pytest.mark.asyncio
    async def test_enforces_pagination_page_ceiling(
        self, sample_job: DiscoveryJob, directory_html: str
    ) -> None:
        """Asserts pagination does not exceed configured page_ceiling."""
        config = {
            "id": "paginated_dir",
            "name": "Paginated Directory",
            "url_template": "https://directory.example.com/search?page={page}",
            "pagination": {"page_ceiling": 2, "start_page": 1},
            "selectors": {
                "card": ".directory-card",
                "name": ".company-title::text",
            },
        }

        requested_pages = []

        async def mock_fetch(url: str) -> str:
            requested_pages.append(url)
            return directory_html

        source = ConfiguredDirectorySource(config=config, fetcher=mock_fetch)

        batches = []
        async for batch in source.run(sample_job):
            batches.append(batch)

        # page_ceiling is 2, so only 2 pages requested even if next page link is present
        assert len(requested_pages) == 2
        assert "page=1" in requested_pages[0]
        assert "page=2" in requested_pages[1]

    @pytest.mark.asyncio
    async def test_loads_from_yaml_file(self, tmp_path: Path, sample_job: DiscoveryJob, directory_html: str) -> None:
        yaml_content = """
directories:
  - id: "yaml_directory"
    name: "YAML Directory"
    url_template: "https://directory.example.com/tenants?lat={lat}&lng={lng}&radius={radius}&page={page}"
    pagination:
      page_ceiling: 1
    selectors:
      card: ".directory-card"
      name: ".company-title::text"
      source_record_id: "@data-company-id"
"""
        yaml_file = tmp_path / "test_sources.yaml"
        yaml_file.write_text(yaml_content, encoding="utf-8")

        async def mock_fetch(url: str) -> str:
            return directory_html

        sources = ConfiguredDirectorySource.load_all(yaml_file, fetcher=mock_fetch)
        assert len(sources) == 1
        source = sources[0]
        assert source.name == "yaml_directory"
        assert source.source_family == "public_directory"

        batches = []
        async for batch in source.run(sample_job):
            batches.append(batch)

        assert len(batches) == 1
        assert len(batches[0].companies) == 3
