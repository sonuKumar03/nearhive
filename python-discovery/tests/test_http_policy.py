import socket
from unittest.mock import patch
import pytest
from scrapy.exceptions import IgnoreRequest
from scrapy.http import HtmlResponse, Request, Response

from nearhive_discovery.http import (
    DEFAULT_CRAWL_SETTINGS,
    NormalizedURL,
    SSRFError,
    SafeDownloadMiddleware,
    SafeRedirectMiddleware,
    get_crawl_settings,
    validate_public_url,
    validate_redirect,
)


class TestURLSafetyValidation:
    """Tests SSRF defense and URL validation."""

    @pytest.mark.parametrize(
        "disallowed_scheme_url",
        [
            "file:///etc/passwd",
            "file:///var/log/system.log",
            "ftp://ftp.example.com/resource",
            "javascript:alert(1)",
            "data:text/html,<h1>Hello</h1>",
            "gopher://example.com:70/1",
        ],
    )
    def test_rejects_non_http_schemes(self, disallowed_scheme_url: str) -> None:
        with pytest.raises(SSRFError, match="(?i)scheme"):
            validate_public_url(disallowed_scheme_url)

    @pytest.mark.parametrize(
        "credential_url",
        [
            "http://user:password@example.com",
            "https://admin:secret@example.com/api",
            "http://user@example.com/path",
            "https://:secret@example.com",
        ],
    )
    def test_rejects_credentials_in_url(self, credential_url: str) -> None:
        with pytest.raises(SSRFError, match="(?i)credential"):
            validate_public_url(credential_url)

    @pytest.mark.parametrize(
        "localhost_url",
        [
            "http://localhost",
            "http://localhost:8080",
            "https://localhost/admin",
            "http://subdomain.localhost",
            "http://127.0.0.1",
            "http://127.0.0.1:3000",
            "http://127.0.1.1",
            "http://127.255.255.254",
            "http://[::1]",
            "http://[::1]:8080",
            "http://0.0.0.0",
        ],
    )
    def test_rejects_localhost_and_loopback(self, localhost_url: str) -> None:
        with pytest.raises(SSRFError):
            validate_public_url(localhost_url)

    @pytest.mark.parametrize(
        "rfc1918_url",
        [
            "http://10.0.0.1",
            "http://10.254.1.99:8080/status",
            "http://172.16.0.1",
            "http://172.24.50.1",
            "http://172.31.255.254",
            "http://192.168.0.1",
            "http://192.168.1.100:9000",
        ],
    )
    def test_rejects_rfc1918_private_addresses(self, rfc1918_url: str) -> None:
        with pytest.raises(SSRFError):
            validate_public_url(rfc1918_url)

    @pytest.mark.parametrize(
        "link_local_url",
        [
            "http://169.254.169.254/latest/meta-data/",
            "http://169.254.1.1",
            "http://169.254.254.254",
            "http://[fe80::1]",
            "http://[fe80::dead:beef]",
        ],
    )
    def test_rejects_link_local_addresses(self, link_local_url: str) -> None:
        with pytest.raises(SSRFError):
            validate_public_url(link_local_url)

    @pytest.mark.parametrize(
        "ipv6_private_url",
        [
            "http://[fc00::1]",
            "http://[fd12:3456:789a:1::1]",
            "http://[::ffff:127.0.0.1]",
            "http://[::ffff:10.0.0.1]",
            "http://[::ffff:169.254.169.254]",
            "http://[2002:7f00:0001::]",  # 6to4 mapped 127.0.0.1
            "http://[2002:0a00:0001::]",  # 6to4 mapped 10.0.0.1
        ],
    )
    def test_rejects_ipv6_local_and_private(self, ipv6_private_url: str) -> None:
        with pytest.raises(SSRFError):
            validate_public_url(ipv6_private_url)

    def test_rejects_hostname_resolving_to_private_ip(self) -> None:
        def fake_getaddrinfo(host, port, *args, **kwargs):
            if host == "internal.corp.local":
                return [
                    (socket.AF_INET, socket.SOCK_STREAM, 6, "", ("10.0.0.42", port))
                ]
            if host == "metadata.cloud.provider":
                return [
                    (socket.AF_INET, socket.SOCK_STREAM, 6, "", ("169.254.169.254", port))
                ]
            if host == "ipv6-loopback.example":
                return [
                    (socket.AF_INET6, socket.SOCK_STREAM, 6, "", ("::1", port, 0, 0))
                ]
            return [
                (socket.AF_INET, socket.SOCK_STREAM, 6, "", ("93.184.216.34", port))
            ]

        with patch("socket.getaddrinfo", side_effect=fake_getaddrinfo):
            with pytest.raises(SSRFError, match="(?i)private|unsafe"):
                validate_public_url("http://internal.corp.local/api")

            with pytest.raises(SSRFError, match="(?i)private|unsafe"):
                validate_public_url("http://metadata.cloud.provider/meta")

            with pytest.raises(SSRFError, match="(?i)private|unsafe"):
                validate_public_url("http://ipv6-loopback.example/")

    def test_accepts_valid_public_urls(self) -> None:
        def fake_getaddrinfo(host, port, *args, **kwargs):
            return [(socket.AF_INET, socket.SOCK_STREAM, 6, "", ("93.184.216.34", port))]

        with patch("socket.getaddrinfo", side_effect=fake_getaddrinfo):
            res = validate_public_url("https://www.example.com:443/careers?dept=eng#team")
            assert isinstance(res, NormalizedURL)
            assert res.scheme == "https"
            assert res.host == "www.example.com"
            assert res.port == 443
            assert res.path == "/careers"
            assert res.query == "dept=eng"
            assert res.domain == "example.com"
            assert res.url == "https://www.example.com/careers?dept=eng"

            # Plain http with standard port
            res2 = validate_public_url("http://example.org/")
            assert res2.scheme == "http"
            assert res2.port == 80
            assert res2.url == "http://example.org/"


class TestRedirectRevalidation:
    """Tests redirect safety validation and SSRF defenses."""

    def test_revalidate_safe_redirect(self) -> None:
        def fake_getaddrinfo(host, port, *args, **kwargs):
            return [(socket.AF_INET, socket.SOCK_STREAM, 6, "", ("93.184.216.34", port))]

        with patch("socket.getaddrinfo", side_effect=fake_getaddrinfo):
            # Relative redirect
            target = validate_redirect("https://example.com/home", "/about-us")
            assert target.url == "https://example.com/about-us"

            # Absolute redirect to public URL
            target2 = validate_redirect("https://example.com", "https://careers.example.com")
            assert target2.host == "careers.example.com"

    def test_revalidate_blocks_redirect_to_private_address(self) -> None:
        # Redirect to cloud metadata
        with pytest.raises(SSRFError):
            validate_redirect("https://example.com/redirect", "http://169.254.169.254/latest/meta-data")

        # Redirect to localhost
        with pytest.raises(SSRFError):
            validate_redirect("https://example.com/login", "http://127.0.0.1:8080/admin")

        # Redirect to file://
        with pytest.raises(SSRFError):
            validate_redirect("https://example.com", "file:///etc/passwd")

    def test_safe_download_middleware_blocks_unsafe_request(self) -> None:
        middleware = SafeDownloadMiddleware()
        req = Request(url="http://127.0.0.1:8080/admin")
        with pytest.raises(IgnoreRequest):
            middleware.process_request(req, spider=None)

    def test_safe_redirect_middleware_blocks_unsafe_redirect_location(self) -> None:
        middleware = SafeRedirectMiddleware()
        req = Request(url="https://example.com/login")
        resp = Response(
            url="https://example.com/login",
            status=302,
            headers={"Location": b"http://169.254.169.254/latest/meta-data"},
            request=req,
        )
        with pytest.raises(IgnoreRequest):
            middleware.process_response(req, resp, spider=None)


class TestCrawlPolicy:
    """Tests Scrapy crawl policy defaults and settings."""

    def test_crawl_policy_settings_compliance(self) -> None:
        settings = get_crawl_settings()

        # 1. robots.txt must be obeyed
        assert settings.get("ROBOTSTXT_OBEY") is True

        # 2. Identifiable user agent with contact information
        ua = settings.get("USER_AGENT")
        assert ua is not None
        assert "NearHive" in ua
        assert "http" in ua or "@" in ua  # contact url or email

        # 3. Bounded response size (e.g. 5MB - 10MB)
        max_size = settings.get("DOWNLOAD_MAXSIZE")
        assert max_size is not None
        assert 1024 * 1024 <= max_size <= 20 * 1024 * 1024

        # 4. Per-domain concurrency limits
        domain_concurrency = settings.get("CONCURRENT_REQUESTS_PER_DOMAIN")
        assert domain_concurrency is not None
        assert 1 <= domain_concurrency <= 4

        # 5. Delay and timeout
        delay = settings.get("DOWNLOAD_DELAY")
        assert delay is not None
        assert delay >= 0.5

        timeout = settings.get("DOWNLOAD_TIMEOUT")
        assert timeout is not None
        assert 5 <= timeout <= 60

        # 6. Safety middlewares configured
        middlewares = settings.get("DOWNLOADER_MIDDLEWARES")
        assert isinstance(middlewares, dict)
        assert "nearhive_discovery.http.SafeDownloadMiddleware" in middlewares
        assert "nearhive_discovery.http.SafeRedirectMiddleware" in middlewares
