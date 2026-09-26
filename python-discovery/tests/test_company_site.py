import uuid
from datetime import datetime, timezone
from pathlib import Path
from unittest.mock import AsyncMock
import pytest

from nearhive_discovery.contracts import (
    CompanyEvidence,
    DiscoveryJob,
    DiscoveryStatus,
    EvidenceBatch,
)
from nearhive_discovery.sources.base import SourceAdapter
from nearhive_discovery.sources.company_site import (
    CompanySiteSource,
    is_allowed_site_url,
    should_reject_url,
)

FIXTURES_DIR = Path(__file__).parent / "fixtures" / "company_site"


@pytest.fixture
def site_fixtures() -> dict[str, str]:
    return {
        "homepage": (FIXTURES_DIR / "homepage.html").read_text(encoding="utf-8"),
        "about": (FIXTURES_DIR / "about.html").read_text(encoding="utf-8"),
        "contact": (FIXTURES_DIR / "contact.html").read_text(encoding="utf-8"),
        "careers": (FIXTURES_DIR / "careers.html").read_text(encoding="utf-8"),
        "locations": (FIXTURES_DIR / "locations.html").read_text(encoding="utf-8"),
        "sitemap": (FIXTURES_DIR / "sitemap.xml").read_text(encoding="utf-8"),
    }


@pytest.fixture
def sample_job() -> DiscoveryJob:
    return DiscoveryJob(
        id=uuid.uuid4(),
        user_id=uuid.uuid4(),
        status=DiscoveryStatus.RUNNING,
        lat=37.7749,
        lng=-122.4194,
        radius_km=25.0,
    )


class TestCompanySiteBoundary:
    """Tests official-site crawling boundary, depth, rejection rules, and evidence extraction."""

    def test_implements_source_adapter_protocol(self) -> None:
        source = CompanySiteSource(target_url="https://novarobotics.example.com")
        assert isinstance(source, SourceAdapter)
        assert source.name == "company_site"
        assert source.source_family == "official_site"

    @pytest.mark.parametrize(
        "url,expected_rejected",
        [
            ("https://novarobotics.example.com/logout", True),
            ("https://novarobotics.example.com/auth/logout", True),
            ("https://novarobotics.example.com/login", True),
            ("https://novarobotics.example.com/user/account", True),
            ("https://novarobotics.example.com/my-account", True),
            ("https://novarobotics.example.com/cart", True),
            ("https://novarobotics.example.com/checkout", True),
            ("https://novarobotics.example.com/basket", True),
            ("https://novarobotics.example.com/forms/submit", True),
            ("https://novarobotics.example.com/wp-admin", True),
            ("https://novarobotics.example.com/about", False),
            ("https://novarobotics.example.com/contact-us", False),
            ("https://novarobotics.example.com/careers", False),
            ("https://novarobotics.example.com/locations", False),
        ],
    )
    def test_rejection_rules(self, url: str, expected_rejected: bool) -> None:
        assert should_reject_url(url) == expected_rejected

    @pytest.mark.parametrize(
        "url,allowed",
        [
            # Allowed categories (same domain)
            ("https://novarobotics.example.com/", True),
            ("https://novarobotics.example.com/about", True),
            ("https://novarobotics.example.com/about-us", True),
            ("https://novarobotics.example.com/company/team", True),
            ("https://novarobotics.example.com/contact", True),
            ("https://novarobotics.example.com/locations", True),
            ("https://novarobotics.example.com/offices", True),
            ("https://novarobotics.example.com/careers", True),
            ("https://novarobotics.example.com/jobs", True),
            # Disallowed categories (same domain, but not about/contact/careers/locations/homepage)
            ("https://novarobotics.example.com/products/robot-arm-v1", False),
            ("https://novarobotics.example.com/blog/2026/01/future-of-ai", False),
            ("https://novarobotics.example.com/pricing", False),
            # Cross-domain URLs (must be rejected)
            ("https://twitter.com/novarobotics", False),
            ("https://linkedin.com/company/novarobotics", False),
            ("https://partner-portal.otherdomain.com/login", False),
            ("https://otherdomain.com/about", False),
        ],
    )
    def test_allowed_categories_and_cross_domain_filtering(self, url: str, allowed: bool) -> None:
        target_domain = "novarobotics.example.com"
        assert is_allowed_site_url(url, target_domain=target_domain) == allowed

    @pytest.mark.asyncio
    async def test_bounded_crawl_follows_only_allowed_pages(
        self, sample_job: DiscoveryJob, site_fixtures: dict[str, str]
    ) -> None:
        """Asserts crawling follows homepage, about, contact, locations, careers, but not rejected or cross-domain links."""
        crawled_urls = []

        async def mock_fetch(url: str) -> str:
            crawled_urls.append(url)
            if url.endswith("/about"):
                return site_fixtures["about"]
            elif url.endswith("/contact"):
                return site_fixtures["contact"]
            elif url.endswith("/careers"):
                return site_fixtures["careers"]
            elif url.endswith("/locations"):
                return site_fixtures["locations"]
            elif "sitemap.xml" in url:
                return site_fixtures["sitemap"]
            return site_fixtures["homepage"]

        source = CompanySiteSource(
            target_url="https://novarobotics.example.com",
            max_pages=10,
            max_depth=2,
            fetcher=mock_fetch,
        )

        batches = []
        async for batch in source.run(sample_job):
            batches.append(batch)

        assert len(batches) >= 1
        assert len(crawled_urls) > 0

        # Assert only allowed pages were crawled
        allowed_paths = {"/", "/about", "/contact", "/careers", "/locations"}
        for url in crawled_urls:
            # Check domain
            assert "novarobotics.example.com" in url
            # Check rejected keywords are never crawled
            assert not any(rej in url for rej in ["login", "logout", "account", "cart", "forms", "twitter", "linkedin"])

        # Check evidence extracted
        all_companies = [c for b in batches for c in b.companies]
        assert len(all_companies) >= 1
        comp = all_companies[0]
        assert "Nova Robotics" in comp.name
        assert comp.domain == "novarobotics.example.com"
        assert comp.source_record_id == "https://novarobotics.example.com/#org"
        assert comp.lat == 37.7749
        assert comp.lng == -122.4194

    @pytest.mark.asyncio
    async def test_enforces_max_pages_ceiling(
        self, sample_job: DiscoveryJob, site_fixtures: dict[str, str]
    ) -> None:
        """Asserts crawl stops immediately when max_pages ceiling is reached."""
        crawled_urls = []

        async def mock_fetch(url: str) -> str:
            crawled_urls.append(url)
            return site_fixtures["homepage"]

        # Limit to max 2 pages
        source = CompanySiteSource(
            target_url="https://novarobotics.example.com",
            max_pages=2,
            max_depth=2,
            fetcher=mock_fetch,
        )

        batches = []
        async for batch in source.run(sample_job):
            batches.append(batch)

        assert len(crawled_urls) <= 2

    @pytest.mark.asyncio
    async def test_enforces_depth_ceiling(
        self, sample_job: DiscoveryJob
    ) -> None:
        """Asserts links beyond max_depth are not followed."""
        crawled_urls = []

        html_depth_0 = '<a href="/depth1">Level 1</a>'
        html_depth_1 = '<a href="/depth2">Level 2</a>'
        html_depth_2 = '<a href="/depth3">Level 3</a>'

        async def mock_fetch(url: str) -> str:
            crawled_urls.append(url)
            if "/depth1" in url:
                return html_depth_1
            elif "/depth2" in url:
                return html_depth_2
            return html_depth_0

        # max_depth=1: should crawl homepage (0) and /depth1 (1), but NOT /depth2 (2)
        source = CompanySiteSource(
            target_url="https://novarobotics.example.com",
            max_pages=10,
            max_depth=1,
            fetcher=mock_fetch,
        )

        batches = []
        async for batch in source.run(sample_job):
            batches.append(batch)

        assert "https://novarobotics.example.com" in crawled_urls[0]
        # Should not have followed to depth 2
        assert not any("/depth2" in u for u in crawled_urls)

    @pytest.mark.asyncio
    async def test_sitemap_integration(
        self, sample_job: DiscoveryJob, site_fixtures: dict[str, str]
    ) -> None:
        """Asserts sitemap URLs are filtered for allowed categories and same domain."""
        crawled_urls = []

        async def mock_fetch(url: str) -> str:
            crawled_urls.append(url)
            if "sitemap.xml" in url:
                return site_fixtures["sitemap"]
            return "<html><body>Hello</body></html>"

        source = CompanySiteSource(
            target_url="https://novarobotics.example.com",
            max_pages=10,
            sitemap_url="https://novarobotics.example.com/sitemap.xml",
            fetcher=mock_fetch,
        )

        batches = []
        async for batch in source.run(sample_job):
            batches.append(batch)

        # Confirm sitemap was fetched and its valid URLs enqueued
        assert any("sitemap.xml" in u for u in crawled_urls)
        # /products/robot-arm-v1 and /cart and /login and external domain in sitemap must not be crawled
        assert not any("/products/" in u for u in crawled_urls)
        assert not any("/cart" in u for u in crawled_urls)
        assert not any("/login" in u for u in crawled_urls)
        assert not any("external-domain" in u for u in crawled_urls)
