import asyncio
import ipaddress
import logging
import re
import socket
import threading
import urllib.parse
from dataclasses import dataclass
from typing import Any

import httpx
from scrapy.downloadermiddlewares.redirect import RedirectMiddleware
from scrapy.exceptions import IgnoreRequest
from scrapy.http import Request, Response
from scrapy.settings import Settings

from nearhive_discovery.settings import settings

logger = logging.getLogger(__name__)


class SSRFError(ValueError):
    """Raised when a URL targets a private, loopback, or unsafe network address."""


class PlaywrightRenderError(RuntimeError):
    """Raised when rendering a page via Playwright fails."""


@dataclass(frozen=True)
class NormalizedURL:
    url: str
    scheme: str
    host: str
    port: int
    path: str
    query: str
    domain: str


def is_safe_ip(ip: ipaddress.IPv4Address | ipaddress.IPv6Address) -> bool:
    """Returns True if the IP address is a safe, publicly routable unicast address.

    Blocks private RFC 1918, loopback, link-local, multicast, reserved,
    carrier-grade NAT, unspecified, and IPv6 unique local addresses.
    """
    # RFC 6052 Well-Known Prefix for NAT64 (64:ff9b::/96)
    if isinstance(ip, ipaddress.IPv6Address):
        if ip in ipaddress.IPv6Network("64:ff9b::/96"):
            embedded = ipaddress.IPv4Address(ip.packed[-4:])
            return is_safe_ip(embedded)

    if ip.is_private or ip.is_loopback or ip.is_link_local:
        return False
    if ip.is_multicast or ip.is_reserved or ip.is_unspecified:
        return False
    if not ip.is_global:
        return False

    # IPv6 special embedded IPv4 checks (e.g. ::ffff:127.0.0.1 or 6to4 2002::)
    if isinstance(ip, ipaddress.IPv6Address):
        if ip.ipv4_mapped is not None and not is_safe_ip(ip.ipv4_mapped):
            return False
        if getattr(ip, "sixtofour", None) is not None and not is_safe_ip(ip.sixtofour):
            return False

    return True


def _check_ip_literal(host: str) -> None:
    """Validates host if it is an IPv4 or IPv6 literal or representation."""
    clean_host = host.strip("[]").lower()

    if clean_host == "0.0.0.0" or clean_host == "0":
        raise SSRFError(f"Unspecified IP address disallowed: {host}")

    # Check decimal integer IP (e.g. 2130706433 -> 127.0.0.1)
    if clean_host.isdigit():
        int_ip = None
        try:
            int_ip = ipaddress.IPv4Address(int(clean_host))
        except (ValueError, OverflowError):
            pass
        if int_ip is not None and not is_safe_ip(int_ip):
            raise SSRFError(f"Private or local integer IP address disallowed: {host}")

    # Check hex IP (e.g. 0x7f000001)
    if clean_host.startswith("0x") or clean_host.startswith("0X"):
        hex_ip = None
        try:
            hex_ip = ipaddress.IPv4Address(int(clean_host, 16))
        except (ValueError, OverflowError):
            pass
        if hex_ip is not None and not is_safe_ip(hex_ip):
            raise SSRFError(f"Private or local hex IP address disallowed: {host}")

    # Standard IP address parsing
    parsed_ip = None
    try:
        parsed_ip = ipaddress.ip_address(clean_host)
    except ValueError:
        pass

    if parsed_ip is not None and not is_safe_ip(parsed_ip):
        raise SSRFError(f"Private, local, or unsafe IP address disallowed: {host}")


def validate_public_url(
    url: str,
    resolve_dns: bool = True,
    allow_local: bool | None = None,
) -> NormalizedURL:
    """Validates that a URL is a public, safe HTTP(S) URL and returns its normalized form.

    Enforces strict SSRF defense:
    - Only http:// and https:// schemes allowed.
    - Credentials (user:pass) are rejected.
    - Localhost, link-local, RFC1918 private IPv4, and private IPv6 are rejected.
    - Resolves hostnames via DNS and verifies all resolved IPs against private ranges.
    - If allow_local is True or NEARHIVE_ALLOW_LOCAL_DISCOVERY=true, allows loopback/local fixtures.
    """
    if allow_local is None:
        import os
        allow_local = os.getenv("NEARHIVE_ALLOW_LOCAL_DISCOVERY", "").lower() in ("1", "true", "yes")

    if not url or not isinstance(url, str):
        raise SSRFError("URL must be a non-empty string")

    parsed = urllib.parse.urlsplit(url.strip())
    scheme = parsed.scheme.lower()
    if scheme not in ("http", "https"):
        raise SSRFError(f"Disallowed scheme '{parsed.scheme}'; only http and https are allowed")

    if parsed.username is not None or parsed.password is not None or "@" in parsed.netloc:
        raise SSRFError("Credentials in URL are disallowed")

    hostname = parsed.hostname
    if not hostname:
        raise SSRFError("Missing hostname in URL")

    hostname_lower = hostname.lower()

    # Reject localhost names unless allow_local
    if not allow_local and (hostname_lower == "localhost" or hostname_lower.endswith(".localhost")):
        raise SSRFError(f"Localhost address disallowed: {hostname}")

    # Check if host is an IP literal
    if not allow_local:
        _check_ip_literal(hostname_lower)

    port = parsed.port or (80 if scheme == "http" else 443)

    # DNS resolution check
    if resolve_dns and not allow_local:
        clean_host = hostname_lower.strip("[]")
        try:
            ipaddress.ip_address(clean_host)
            is_ip = True
        except ValueError:
            is_ip = False

        if not is_ip:
            try:
                addr_info = socket.getaddrinfo(hostname, port, type=socket.SOCK_STREAM)
                for item in addr_info:
                    sockaddr = item[4]
                    ip_str = sockaddr[0]
                    resolved_ip = ipaddress.ip_address(ip_str)
                    if not is_safe_ip(resolved_ip):
                        raise SSRFError(
                            f"Hostname '{hostname}' resolves to unsafe IP address: {ip_str}"
                        )
            except socket.gaierror as exc:
                raise SSRFError(f"Could not resolve hostname '{hostname}': {exc}") from exc

    # Domain extraction (strip leading www.)
    domain = hostname_lower
    if domain.startswith("www."):
        domain = domain[4:]

    # Reconstruct normalized URL without fragment and with explicit standard port handling
    path = parsed.path if parsed.path else "/"
    query = parsed.query

    if (scheme == "http" and port == 80) or (scheme == "https" and port == 443):
        netloc = hostname_lower
    else:
        netloc = f"{hostname_lower}:{port}"

    query_part = f"?{query}" if query else ""
    normalized_url_str = f"{scheme}://{netloc}{path}{query_part}"

    return NormalizedURL(
        url=normalized_url_str,
        scheme=scheme,
        host=hostname_lower,
        port=port,
        path=path,
        query=query,
        domain=domain,
    )


def validate_redirect(
    source_url: str,
    location_header: str,
    resolve_dns: bool = True,
) -> NormalizedURL:
    """Resolves and validates a redirect target URL against SSRF policy."""
    resolved_url = urllib.parse.urljoin(source_url, location_header.strip())
    return validate_public_url(resolved_url, resolve_dns=resolve_dns)


class SafeDownloadMiddleware:
    """Scrapy Downloader Middleware to intercept and block unsafe or internal requests."""

    def process_request(self, request: Request, spider: Any = None) -> None:
        try:
            validate_public_url(request.url)
        except SSRFError as exc:
            logger.warning("SafeDownloadMiddleware blocked request to %s: %s", request.url, exc)
            raise IgnoreRequest(f"SSRF policy blocked request to {request.url}: {exc}") from exc


class SafeRedirectMiddleware(RedirectMiddleware):
    """Scrapy Redirect Middleware that revalidates redirect targets before following them."""

    def __init__(self, settings: Any = None) -> None:
        if settings is None:
            settings = Settings()
        super().__init__(settings)
        if not hasattr(self, "crawler"):
            self.crawler = type("DummyCrawler", (), {"spider": None})()

    def process_response(
        self, request: Request, response: Response, spider: Any = None
    ) -> Any:
        if (
            response.status in (301, 302, 303, 307, 308)
            and "Location" in response.headers
        ):
            raw_location = response.headers["Location"].decode("utf-8", errors="replace")
            try:
                validate_redirect(request.url, raw_location)
            except SSRFError as exc:
                logger.warning(
                    "SafeRedirectMiddleware blocked redirect from %s to %s: %s",
                    request.url,
                    raw_location,
                    exc,
                )
                raise IgnoreRequest(
                    f"SSRF policy blocked redirect to {raw_location}: {exc}"
                ) from exc

        return super().process_response(request, response, spider)


SPA_SHELL_PATTERNS = re.compile(
    r'<div\b[^>]*\bid=["\'](?:root|app|__next|__nuxt)["\'][^>]*>|<app-root\b[^>]*>',
    re.IGNORECASE,
)


def should_render(response_or_html: Any) -> bool:
    """Evaluates if a page is a client-rendered JavaScript shell requiring browser execution.

    Returns True ONLY if:
    1. Explicitly requested via request/response metadata (e.g., meta["render_js"] or meta["playwright"]).
    2. Or page is a 200 OK HTML page containing a JavaScript SPA root container
       (e.g., id="root", id="app", id="__next", id="__nuxt", or <app-root>)
       WITH little or no static text content (< 200 characters of meaningful visible text).

    Returns False for:
    - Ordinary HTML with substantial content.
    - HTTP errors (non-200 responses) - do NOT retry failed HTTP pages in a browser.
    - Empty or non-HTML strings.
    - Pages without JS shell indicators.
    """
    if response_or_html is None:
        return False

    # Check request / response metadata for explicit rendering flags
    meta: dict[str, Any] = {}
    if hasattr(response_or_html, "meta") and isinstance(response_or_html.meta, dict):
        meta.update(response_or_html.meta)
    if hasattr(response_or_html, "request") and getattr(response_or_html.request, "meta", None):
        if isinstance(response_or_html.request.meta, dict):
            meta.update(response_or_html.request.meta)

    if meta.get("render_js") or meta.get("playwright"):
        return True

    # Check HTTP status code if present: do NOT retry failed HTTP pages in a browser
    status = getattr(response_or_html, "status", None)
    if status is None:
        status = getattr(response_or_html, "status_code", None)
    if status is not None and status != 200:
        return False

    # Extract HTML string
    if isinstance(response_or_html, str):
        html = response_or_html
    elif hasattr(response_or_html, "text") and isinstance(response_or_html.text, str):
        html = response_or_html.text
    elif hasattr(response_or_html, "body") and isinstance(
        response_or_html.body, (bytes, bytearray)
    ):
        html = response_or_html.body.decode("utf-8", errors="replace")
    else:
        return False

    trimmed = html.strip()
    if not trimmed:
        return False

    # Check for empty SPA root container or noscript prompt
    has_spa_shell = bool(SPA_SHELL_PATTERNS.search(trimmed))
    has_noscript_hint = bool(
        re.search(
            r'<noscript\b[^>]*>.*?(?:enable|require|javascript|js).*?</noscript>',
            trimmed,
            re.IGNORECASE | re.DOTALL,
        )
    )
    if not has_spa_shell and not (has_noscript_hint and "<div" in trimmed):
        return False

    # Strip scripts, styles, noscript, comments, and HTML tags to calculate visible text
    stripped = re.sub(
        r'<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>',
        "",
        trimmed,
        flags=re.IGNORECASE,
    )
    stripped = re.sub(
        r'<style\b[^<]*(?:(?!<\/style>)<[^<]*)*<\/style>',
        "",
        stripped,
        flags=re.IGNORECASE,
    )
    stripped = re.sub(
        r'<noscript\b[^<]*(?:(?!<\/noscript>)<[^<]*)*<\/noscript>',
        "",
        stripped,
        flags=re.IGNORECASE,
    )
    stripped = re.sub(r"<!--[\s\S]*?-->", "", stripped)
    stripped = re.sub(r"<[^>]+>", " ", stripped)
    visible_text = " ".join(stripped.split())

    # If visible text is short (< 200 chars) and SPA shell / noscript is present, it's an empty shell!
    return len(visible_text) < 200


class PlaywrightFallbackMiddleware:
    """Scrapy Downloader Middleware to transparently fallback to Playwright for JS shell pages."""

    def process_response(
        self, request: Request, response: Response, spider: Any = None
    ) -> Any:
        if request.meta.get("playwright"):
            return response

        if should_render(response):
            logger.info(
                "JavaScript shell detected for %s; dispatching Playwright render",
                request.url,
            )
            new_meta = dict(request.meta)
            new_meta["playwright"] = True
            new_meta["playwright_include_page"] = False
            return request.replace(meta=new_meta, dont_filter=True)

        return response


DEFAULT_USER_AGENT = "NearHiveBot/1.0 (+https://nearhive.com/bot; bot@nearhive.com)"


class PlaywrightPool:
    """Bounded pool of Playwright browser contexts for rendering JavaScript SPAs."""

    def __init__(
        self,
        max_contexts: int | None = None,
        headless: bool = True,
        timeout: float = 15.0,
        resolve_dns: bool = True,
        user_agent: str = DEFAULT_USER_AGENT,
    ) -> None:
        self.max_contexts = (
            max_contexts
            if max_contexts is not None
            else settings.playwright_contexts
        )
        self.headless = headless
        self.timeout = timeout
        self.resolve_dns = resolve_dns
        self.user_agent = user_agent

        # Persistent background thread with dedicated event loop to avoid event loop binding deadlocks
        self._loop = asyncio.new_event_loop()
        self._thread = threading.Thread(
            target=self._run_loop,
            daemon=True,
            name="PlaywrightPool-Thread",
        )
        self._thread.start()

        # State managed inside self._loop
        self._playwright: Any = None
        self._browser: Any = None
        self._contexts: set[Any] = set()
        self._semaphore: asyncio.Semaphore | None = None
        self._init_lock: asyncio.Lock | None = None
        self._is_closed = False

    def _run_loop(self) -> None:
        asyncio.set_event_loop(self._loop)
        self._loop.run_forever()

    async def _init_resources(self) -> None:
        if self._init_lock is None:
            self._init_lock = asyncio.Lock()
        async with self._init_lock:
            if self._is_closed:
                raise PlaywrightRenderError("PlaywrightPool is closed")
            if self._playwright is None:
                from playwright.async_api import async_playwright

                self._playwright = await async_playwright().start()
            if self._browser is None or not self._browser.is_connected():
                self._browser = await self._playwright.chromium.launch(
                    headless=self.headless,
                    args=["--no-sandbox", "--disable-dev-shm-usage"],
                )
            if self._semaphore is None:
                self._semaphore = asyncio.Semaphore(self.max_contexts)

    @property
    def active_contexts_count(self) -> int:
        return len(self._contexts)

    async def _acquire_context_internal(self) -> Any:
        await self._init_resources()
        assert self._semaphore is not None
        await self._semaphore.acquire()
        try:
            ctx = await self._browser.new_context(user_agent=self.user_agent)
            self._contexts.add(ctx)
            return ctx
        except Exception:
            self._semaphore.release()
            raise

    async def _release_context_internal(self, ctx: Any) -> None:
        if ctx in self._contexts:
            self._contexts.discard(ctx)
            try:
                await ctx.close()
            except Exception:
                pass
            if self._semaphore is not None:
                self._semaphore.release()

    async def acquire_context(self) -> Any:
        fut = asyncio.run_coroutine_threadsafe(
            self._acquire_context_internal(), self._loop
        )
        return await asyncio.wrap_future(fut)

    async def release_context(self, ctx: Any) -> None:
        fut = asyncio.run_coroutine_threadsafe(
            self._release_context_internal(ctx), self._loop
        )
        await asyncio.wrap_future(fut)

    async def _close_contexts_internal(self) -> None:
        for ctx in list(self._contexts):
            await self._release_context_internal(ctx)

    async def close_contexts(self) -> None:
        if self._loop.is_running():
            fut = asyncio.run_coroutine_threadsafe(
                self._close_contexts_internal(), self._loop
            )
            await asyncio.wrap_future(fut)

    async def _close_internal(self) -> None:
        self._is_closed = True
        await self._close_contexts_internal()
        if self._browser is not None:
            try:
                await self._browser.close()
            except Exception:
                pass
            self._browser = None
        if self._playwright is not None:
            try:
                await self._playwright.stop()
            except Exception:
                pass
            self._playwright = None

    async def close(self) -> None:
        if self._loop.is_running():
            fut = asyncio.run_coroutine_threadsafe(
                self._close_internal(), self._loop
            )
            try:
                await asyncio.wrap_future(fut)
            finally:
                self._loop.call_soon_threadsafe(self._loop.stop)
                self._thread.join(timeout=3.0)

    def close_sync(self) -> None:
        if self._loop.is_running():
            fut = asyncio.run_coroutine_threadsafe(
                self._close_internal(), self._loop
            )
            try:
                fut.result(timeout=5.0)
            except Exception:
                pass
            finally:
                self._loop.call_soon_threadsafe(self._loop.stop)
                self._thread.join(timeout=3.0)

    async def _render_html_internal(
        self,
        html: str,
        wait_until: str = "domcontentloaded",
        timeout: float | None = None,
    ) -> str:
        ctx = await self._acquire_context_internal()
        try:
            page = await ctx.new_page()
            t_ms = int((timeout if timeout is not None else self.timeout) * 1000)
            await page.set_content(html, wait_until=wait_until, timeout=t_ms)
            try:
                await page.wait_for_load_state("networkidle", timeout=min(2000, t_ms))
            except Exception:
                pass
            return await page.content()
        except Exception as exc:
            logger.warning("Error rendering HTML in Playwright: %s", exc)
            raise PlaywrightRenderError(f"Failed to render HTML: {exc}") from exc
        finally:
            await self._release_context_internal(ctx)

    async def render_html(
        self,
        html: str,
        wait_until: str = "domcontentloaded",
        timeout: float | None = None,
    ) -> str:
        fut = asyncio.run_coroutine_threadsafe(
            self._render_html_internal(html, wait_until=wait_until, timeout=timeout),
            self._loop,
        )
        return await asyncio.wrap_future(fut)

    async def _render_page_internal(
        self,
        url: str,
        wait_until: str = "domcontentloaded",
        timeout: float | None = None,
        resolve_dns: bool = True,
    ) -> str:
        ctx = await self._acquire_context_internal()

        async def handle_route(route: Any) -> None:
            req_url = route.request.url
            if req_url.startswith("data:") or req_url.startswith("blob:"):
                await route.continue_()
                return
            try:
                validate_public_url(req_url, resolve_dns=resolve_dns)
                await route.continue_()
            except SSRFError as s_exc:
                logger.warning(
                    "Playwright route blocked unsafe URL %s: %s", req_url, s_exc
                )
                await route.abort("blockedbyclient")

        await ctx.route("**/*", handle_route)

        try:
            page = await ctx.new_page()
            t_ms = int((timeout if timeout is not None else self.timeout) * 1000)
            try:
                await page.goto(url, wait_until=wait_until, timeout=t_ms)
            except Exception as exc:
                if self._browser is not None and not self._browser.is_connected():
                    self._browser = None
                raise PlaywrightRenderError(
                    f"Page navigation timed out or failed: {exc}"
                ) from exc

            validate_public_url(page.url, resolve_dns=resolve_dns)

            try:
                await page.wait_for_load_state("networkidle", timeout=min(3000, t_ms))
            except Exception:
                pass

            return await page.content()
        except SSRFError:
            raise
        except PlaywrightRenderError:
            raise
        except Exception as exc:
            if self._browser is not None and not self._browser.is_connected():
                self._browser = None
            raise PlaywrightRenderError(f"Rendering failed for {url}: {exc}") from exc
        finally:
            await self._release_context_internal(ctx)

    async def render_page(
        self,
        url: str,
        wait_until: str = "domcontentloaded",
        timeout: float | None = None,
        resolve_dns: bool | None = None,
    ) -> str:
        resolve = self.resolve_dns if resolve_dns is None else resolve_dns
        validate_public_url(url, resolve_dns=resolve)
        fut = asyncio.run_coroutine_threadsafe(
            self._render_page_internal(
                url,
                wait_until=wait_until,
                timeout=timeout,
                resolve_dns=resolve,
            ),
            self._loop,
        )
        return await asyncio.wrap_future(fut)


async def safe_fetch_text(
    url: str,
    client: httpx.AsyncClient | None = None,
    resolve_dns: bool = True,
    max_redirects: int = 5,
    user_agent: str = DEFAULT_USER_AGENT,
    timeout: float = 15.0,
    render_js: bool = False,
    playwright_pool: PlaywrightPool | None = None,
) -> str:
    """Fetches text from a public URL using httpx, revalidating every redirect target against SSRF policy.

    If render_js is True or playwright_pool is provided and the page is an empty JavaScript shell,
    falls back to rendering via Playwright.
    """
    validate_public_url(url, resolve_dns=resolve_dns)

    current_url = url
    owns_client = False
    if client is None:
        client = httpx.AsyncClient(timeout=timeout)
        owns_client = True

    try:
        for _ in range(max_redirects + 1):
            resp = await client.get(
                current_url,
                headers={"User-Agent": user_agent},
                follow_redirects=False,
            )
            if resp.status_code in (301, 302, 303, 307, 308) and "Location" in resp.headers:
                location = resp.headers["Location"]
                normalized = validate_redirect(
                    current_url, location, resolve_dns=resolve_dns
                )
                current_url = normalized.url
                continue

            resp.raise_for_status()
            text = resp.text

            # Check if Playwright fallback is requested or needed
            if render_js or (playwright_pool is not None and should_render(text)):
                owns_pool = False
                if playwright_pool is not None:
                    pool = playwright_pool
                else:
                    pool = PlaywrightPool(
                        timeout=timeout,
                        resolve_dns=resolve_dns,
                        user_agent=user_agent,
                    )
                    owns_pool = True

                try:
                    return await pool.render_page(
                        current_url,
                        timeout=timeout,
                        resolve_dns=resolve_dns,
                    )
                finally:
                    if owns_pool:
                        await pool.close()

            return text

        raise SSRFError(f"Too many redirects from {url} (exceeded {max_redirects})")
    finally:
        if owns_client:
            await client.aclose()


DEFAULT_CRAWL_SETTINGS: dict[str, Any] = {
    "ROBOTSTXT_OBEY": True,
    "USER_AGENT": DEFAULT_USER_AGENT,
    "DOWNLOAD_MAXSIZE": 10 * 1024 * 1024,  # 10MB bounded response size
    "DOWNLOAD_WARNSIZE": 5 * 1024 * 1024,  # 5MB warning
    "CONCURRENT_REQUESTS_PER_DOMAIN": 2,
    "CONCURRENT_REQUESTS": 8,
    "DOWNLOAD_DELAY": 1.0,
    "DOWNLOAD_TIMEOUT": 15,
    "AUTOTHROTTLE_ENABLED": True,
    "AUTOTHROTTLE_START_DELAY": 1.0,
    "AUTOTHROTTLE_MAX_DELAY": 10.0,
    "AUTOTHROTTLE_TARGET_CONCURRENCY": 1.0,
    "TWISTED_REACTOR": "twisted.internet.asyncioreactor.AsyncioSelectorReactor",
    "DOWNLOAD_HANDLERS": {
        "http": "scrapy_playwright.handler.ScrapyPlaywrightDownloadHandler",
        "https": "scrapy_playwright.handler.ScrapyPlaywrightDownloadHandler",
    },
    "PLAYWRIGHT_BROWSER_TYPE": "chromium",
    "PLAYWRIGHT_MAX_CONTEXTS": 2,
    "PLAYWRIGHT_LAUNCH_OPTIONS": {"headless": True},
    "DOWNLOADER_MIDDLEWARES": {
        "nearhive_discovery.http.SafeDownloadMiddleware": 50,
        "nearhive_discovery.http.SafeRedirectMiddleware": 600,
        "nearhive_discovery.http.PlaywrightFallbackMiddleware": 700,
        "scrapy.downloadermiddlewares.redirect.RedirectMiddleware": None,
    },
}


def get_crawl_settings(overrides: dict[str, Any] | None = None) -> dict[str, Any]:
    """Returns a dictionary of Scrapy crawl settings enforcing NearHive policy."""
    cfg = dict(DEFAULT_CRAWL_SETTINGS)
    if overrides:
        cfg.update(overrides)
    return cfg


def create_scrapy_settings(overrides: dict[str, Any] | None = None) -> Settings:
    """Returns a configured Scrapy Settings instance."""
    s = Settings()
    s.update(get_crawl_settings(overrides))
    return s


