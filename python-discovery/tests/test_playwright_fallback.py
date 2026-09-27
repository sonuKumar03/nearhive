import asyncio
from pathlib import Path
from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch
import pytest
from scrapy.http import HtmlResponse, Request, Response

from nearhive_discovery.contracts import (
    DiscoveryJob,
    DiscoveryStatus,
    EvidenceBatch,
)
from nearhive_discovery.http import (
    PlaywrightFallbackMiddleware,
    PlaywrightPool,
    PlaywrightRenderError,
    SSRFError,
    get_crawl_settings,
    should_render,
)
from nearhive_discovery.settings import settings
from nearhive_discovery.worker import Source, Worker

FIXTURES_DIR = Path(__file__).parent / "fixtures"


@pytest.fixture
def js_company_html() -> str:
    return (FIXTURES_DIR / "js_company.html").read_text(encoding="utf-8")


@pytest.fixture
def ordinary_html() -> str:
    return """<!DOCTYPE html>
<html>
<head><title>Standard Company</title></head>
<body>
    <h1>Standard Hardware Ltd</h1>
    <p>We manufacture industrial nuts and bolts. Established in 1985 with facilities across North America.</p>
    <address>456 Industrial Way, Cleveland, OH 44114</address>
    <p>Contact us at info@standardhardware.example.com or call 216-555-0123.</p>
</body>
</html>"""


class TestShouldRender:
    """Tests the should_render classifier for JavaScript shell detection."""

    def test_fixture_js_shell_triggers_rendering(self, js_company_html: str) -> None:
        assert should_render(js_company_html) is True

    def test_ordinary_html_never_triggers_rendering(self, ordinary_html: str) -> None:
        assert should_render(ordinary_html) is False

    @pytest.mark.parametrize(
        "shell_html",
        [
            '<!DOCTYPE html><html><body><div id="root"></div></body></html>',
            '<!DOCTYPE html><html><body><div id="app"></div></body></html>',
            '<!DOCTYPE html><html><body><div id="__next"></div></body></html>',
            '<!DOCTYPE html><html><body><div id="__nuxt"></div></body></html>',
            '<!DOCTYPE html><html><body><app-root></app-root></body></html>',
            '<!DOCTYPE html><html><body><div id="root"></div><noscript>Please enable JS</noscript></body></html>',
        ],
    )
    def test_spa_shells_trigger_rendering(self, shell_html: str) -> None:
        assert should_render(shell_html) is True

    def test_ordinary_short_page_without_spa_container_does_not_render(self) -> None:
        short_html = "<!DOCTYPE html><html><body><h1>Welcome</h1><p>Under construction.</p></body></html>"
        assert should_render(short_html) is False

    def test_empty_or_whitespace_html_does_not_render(self) -> None:
        assert should_render("") is False
        assert should_render("   \n\t  ") is False

    def test_http_errors_never_trigger_browser_rendering(self) -> None:
        req = Request(url="https://example.com/not-found")
        resp_404 = HtmlResponse(
            url="https://example.com/not-found",
            status=404,
            body=b'<!DOCTYPE html><html><body><div id="root"></div><h1>404 Not Found</h1></body></html>',
            request=req,
        )
        assert should_render(resp_404) is False

        resp_500 = Response(
            url="https://example.com/error",
            status=500,
            body=b"Internal Server Error",
            request=req,
        )
        assert should_render(resp_500) is False

    def test_explicit_request_metadata_triggers_rendering(self) -> None:
        req = Request(url="https://example.com/page", meta={"render_js": True})
        resp = HtmlResponse(
            url="https://example.com/page",
            status=200,
            body=b"<html><body><h1>Some static text</h1></body></html>",
            request=req,
        )
        assert should_render(resp) is True

        req2 = Request(url="https://example.com/page2", meta={"playwright": True})
        resp2 = HtmlResponse(
            url="https://example.com/page2",
            status=200,
            body=b"<html><body><h1>Some static text</h1></body></html>",
            request=req2,
        )
        assert should_render(resp2) is True


class TestPlaywrightFallbackMiddleware:
    """Tests Scrapy middleware fallback behavior."""

    def test_ordinary_html_passes_through_without_playwright(self, ordinary_html: str) -> None:
        middleware = PlaywrightFallbackMiddleware()
        req = Request(url="https://example.com/ordinary")
        resp = HtmlResponse(
            url="https://example.com/ordinary",
            status=200,
            body=ordinary_html.encode("utf-8"),
            request=req,
        )

        result = middleware.process_response(req, resp, spider=None)
        assert result is resp
        assert "playwright" not in req.meta

    def test_js_shell_returns_new_playwright_request(self, js_company_html: str) -> None:
        middleware = PlaywrightFallbackMiddleware()
        req = Request(url="https://example.com/spa")
        resp = HtmlResponse(
            url="https://example.com/spa",
            status=200,
            body=js_company_html.encode("utf-8"),
            request=req,
        )

        result = middleware.process_response(req, resp, spider=None)
        assert isinstance(result, Request)
        assert result.url == "https://example.com/spa"
        assert result.meta.get("playwright") is True
        assert result.dont_filter is True

    def test_does_not_loop_if_request_already_has_playwright(self, js_company_html: str) -> None:
        middleware = PlaywrightFallbackMiddleware()
        req = Request(url="https://example.com/spa", meta={"playwright": True})
        resp = HtmlResponse(
            url="https://example.com/spa",
            status=200,
            body=js_company_html.encode("utf-8"),
            request=req,
        )

        result = middleware.process_response(req, resp, spider=None)
        assert result is resp


class TestBoundedPlaywrightPool:
    """Tests bounded pool concurrency, lifecycle, fault isolation, and SSRF safety."""

    def test_settings_default_playwright_contexts(self) -> None:
        assert settings.playwright_contexts == 2
        crawl_settings = get_crawl_settings()
        assert crawl_settings.get("PLAYWRIGHT_MAX_CONTEXTS") == 2
        assert crawl_settings.get("PLAYWRIGHT_BROWSER_TYPE") == "chromium"

    def test_pool_capped_by_max_contexts(self) -> None:
        pool = PlaywrightPool(max_contexts=2)
        assert pool.max_contexts == 2

    @pytest.mark.asyncio
    async def test_fixture_js_shell_renders_in_browser(self, tmp_path: Path, js_company_html: str) -> None:
        # Serve fixture via local file or local route
        file_path = tmp_path / "index.html"
        file_path.write_text(js_company_html, encoding="utf-8")

        pool = PlaywrightPool(max_contexts=2, headless=True)
        try:
            # We can use file:// URL in testing if allowed or render via pool
            rendered = await pool.render_html(js_company_html)
            assert "Acme Robotics Inc." in rendered
            assert "100 Tech Blvd, Suite 400, Austin, TX 78701" in rendered
            assert "+1 512-555-0199" in rendered
        finally:
            await pool.close()

    @pytest.mark.asyncio
    async def test_redirect_and_ssrf_safety_blocks_private_urls(self) -> None:
        pool = PlaywrightPool(max_contexts=2)
        try:
            with pytest.raises(SSRFError):
                await pool.render_page("http://127.0.0.1:8080/admin")

            with pytest.raises(SSRFError):
                await pool.render_page("http://169.254.169.254/latest/meta-data")

            with pytest.raises(SSRFError):
                await pool.render_page("http://localhost:3000/")
        finally:
            await pool.close()

    @pytest.mark.asyncio
    async def test_fault_isolation_timeout_fails_only_that_page(self) -> None:
        from playwright.async_api import Page
        pool = PlaywrightPool(max_contexts=2, timeout=0.1)
        try:
            # Fake a slow/timeout page
            with patch.object(Page, "goto", side_effect=TimeoutError("Page navigation timed out")):
                with pytest.raises(PlaywrightRenderError, match="(?i)timed out"):
                    await pool.render_page("https://example.com/timeout", resolve_dns=False)

            # Pool must remain alive and able to render another page
            simple_html = "<!DOCTYPE html><html><body><h1>Alive</h1></body></html>"
            rendered = await pool.render_html(simple_html)
            assert "Alive" in rendered
        finally:
            await pool.close()

    @pytest.mark.asyncio
    async def test_cancellation_closes_contexts(self) -> None:
        pool = PlaywrightPool(max_contexts=2)
        try:
            # Open a context
            ctx = await pool.acquire_context()
            assert pool.active_contexts_count == 1
            # Cancellation closes contexts
            await pool.close_contexts()
            assert pool.active_contexts_count == 0
        finally:
            await pool.close()

    @pytest.mark.asyncio
    async def test_safe_fetch_text_ordinary_html_never_opens_browser(self, ordinary_html: str) -> None:
        from nearhive_discovery.http import safe_fetch_text
        import httpx

        mock_pool = MagicMock(spec=PlaywrightPool)
        mock_pool.render_page = AsyncMock()

        transport = httpx.MockTransport(lambda req: httpx.Response(200, text=ordinary_html))
        client = httpx.AsyncClient(transport=transport)

        res = await safe_fetch_text(
            "https://example.com/ordinary",
            client=client,
            resolve_dns=False,
            playwright_pool=mock_pool,
        )

        assert "Standard Hardware Ltd" in res
        # Pool must NEVER have been called for ordinary HTML!
        mock_pool.render_page.assert_not_called()

    @pytest.mark.asyncio
    async def test_safe_fetch_text_js_shell_triggers_playwright_fallback(self, js_company_html: str) -> None:
        from nearhive_discovery.http import safe_fetch_text
        import httpx

        mock_pool = MagicMock(spec=PlaywrightPool)
        mock_pool.render_page = AsyncMock(return_value="<html><body><h1>Rendered by Playwright</h1></body></html>")

        transport = httpx.MockTransport(lambda req: httpx.Response(200, text=js_company_html))
        client = httpx.AsyncClient(transport=transport)

        res = await safe_fetch_text(
            "https://example.com/spa",
            client=client,
            resolve_dns=False,
            playwright_pool=mock_pool,
        )

        assert "Rendered by Playwright" in res
        mock_pool.render_page.assert_called_once_with(
            "https://example.com/spa",
            timeout=15.0,
            resolve_dns=False,
        )

    def test_pool_across_multiple_sequential_asyncio_runs_does_not_deadlock(self) -> None:
        """Verifies PlaywrightPool survives across multiple independent asyncio.run() invocations without deadlock."""
        pool = PlaywrightPool(max_contexts=2, headless=True)
        try:
            # 1st independent asyncio.run loop
            res1 = asyncio.run(pool.render_html("<html><body><h1>Run One</h1></body></html>"))
            assert "Run One" in res1

            # 2nd independent asyncio.run loop
            res2 = asyncio.run(pool.render_html("<html><body><h1>Run Two</h1></body></html>"))
            assert "Run Two" in res2

            # 3rd independent asyncio.run loop
            res3 = asyncio.run(pool.render_html("<html><body><h1>Run Three</h1></body></html>"))
            assert "Run Three" in res3
        finally:
            # Close in a 4th independent loop
            asyncio.run(pool.close())

    @pytest.mark.asyncio
    async def test_safe_fetch_text_temporary_pool_closed_without_leak(self, js_company_html: str) -> None:
        """Verifies temporary PlaywrightPool created by safe_fetch_text is closed on exit."""
        from nearhive_discovery.http import safe_fetch_text
        import httpx

        transport = httpx.MockTransport(lambda req: httpx.Response(200, text=js_company_html))
        client = httpx.AsyncClient(transport=transport)

        close_called = []
        original_close = PlaywrightPool.close

        async def tracking_close(pool_self: Any) -> None:
            close_called.append(True)
            await original_close(pool_self)

        with patch.object(PlaywrightPool, "render_page", new_callable=AsyncMock) as mock_render:
            mock_render.return_value = "<html><body><h1>Acme Robotics</h1></body></html>"
            with patch.object(PlaywrightPool, "close", tracking_close):
                res = await safe_fetch_text(
                    "https://example.com/spa",
                    client=client,
                    resolve_dns=False,
                    render_js=True,
                    playwright_pool=None,
                )
                assert "Acme Robotics" in res
                assert len(close_called) == 1

    def test_crawl_settings_scrapy_playwright_handlers_and_reactor(self) -> None:
        crawl_settings = get_crawl_settings()
        assert (
            crawl_settings.get("TWISTED_REACTOR")
            == "twisted.internet.asyncioreactor.AsyncioSelectorReactor"
        )
        handlers = crawl_settings.get("DOWNLOAD_HANDLERS")
        assert isinstance(handlers, dict)
        assert (
            handlers.get("http")
            == "scrapy_playwright.handler.ScrapyPlaywrightDownloadHandler"
        )
        assert (
            handlers.get("https")
            == "scrapy_playwright.handler.ScrapyPlaywrightDownloadHandler"
        )


class TestWorkerPlaywrightIntegration:
    """Tests worker lifecycle and fault isolation with Playwright fallback."""

    def test_worker_cancellation_closes_playwright_contexts(self) -> None:
        mock_pool = MagicMock(spec=PlaywrightPool)
        worker = Worker(
            db_url="postgres://nearhive:password@localhost:5432/nearhive?sslmode=disable",
            playwright_pool=mock_pool,
        )
        worker.close()
        mock_pool.close.assert_called_once()

    def test_worker_propagates_playwright_pool_to_sources(self) -> None:
        from nearhive_discovery.sources.company_site import CompanySiteSource

        source = CompanySiteSource(target_url="https://example.com")
        assert source.playwright_pool is None

        mock_pool = MagicMock(spec=PlaywrightPool)
        worker = Worker(
            db_url="postgres://nearhive:password@localhost:5432/nearhive?sslmode=disable",
            sources=[source],
            playwright_pool=mock_pool,
        )

        assert source.playwright_pool is mock_pool
        worker.close()


