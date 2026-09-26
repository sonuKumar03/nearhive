import ipaddress
import logging
import socket
import urllib.parse
from dataclasses import dataclass
from typing import Any

import httpx
from scrapy.downloadermiddlewares.redirect import RedirectMiddleware
from scrapy.exceptions import IgnoreRequest
from scrapy.http import Request, Response
from scrapy.settings import Settings

logger = logging.getLogger(__name__)


class SSRFError(ValueError):
    """Raised when a URL targets a private, loopback, or unsafe network address."""


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


def validate_public_url(url: str, resolve_dns: bool = True) -> NormalizedURL:
    """Validates that a URL is a public, safe HTTP(S) URL and returns its normalized form.

    Enforces strict SSRF defense:
    - Only http:// and https:// schemes allowed.
    - Credentials (user:pass) are rejected.
    - Localhost, link-local, RFC1918 private IPv4, and private IPv6 are rejected.
    - Resolves hostnames via DNS and verifies all resolved IPs against private ranges.
    """
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

    # Reject localhost names
    if hostname_lower == "localhost" or hostname_lower.endswith(".localhost"):
        raise SSRFError(f"Localhost address disallowed: {hostname}")

    # Check if host is an IP literal
    _check_ip_literal(hostname_lower)

    port = parsed.port or (80 if scheme == "http" else 443)

    # DNS resolution check
    if resolve_dns:
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


DEFAULT_USER_AGENT = "NearHiveBot/1.0 (+https://nearhive.com/bot; bot@nearhive.com)"


async def safe_fetch_text(
    url: str,
    client: httpx.AsyncClient | None = None,
    resolve_dns: bool = True,
    max_redirects: int = 5,
    user_agent: str = DEFAULT_USER_AGENT,
    timeout: float = 15.0,
) -> str:
    """Fetches text from a public URL using httpx, revalidating every redirect target against SSRF policy."""
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
            return resp.text

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
    "DOWNLOADER_MIDDLEWARES": {
        "nearhive_discovery.http.SafeDownloadMiddleware": 50,
        "nearhive_discovery.http.SafeRedirectMiddleware": 600,
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
