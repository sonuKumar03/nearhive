from __future__ import annotations

import contextlib
import json
import logging
import threading
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any
import urllib.parse

logger = logging.getLogger(__name__)


class FakeSiteHandler(BaseHTTPRequestHandler):
    """HTTP Request Handler providing deterministic local fixtures for discovery."""

    def log_message(self, format: str, *args: Any) -> None:
        # Suppress noisy standard HTTP access logs in test output
        logger.debug(format, *args)

    def do_GET(self) -> None:
        parsed = urllib.parse.urlsplit(self.path)
        path = parsed.path
        port = self.server.server_address[1]  # type: ignore[attr-defined]
        now_iso = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
        now_ms = int(datetime.now(timezone.utc).timestamp() * 1000)

        # 1. Ordinary directory page
        if path == "/directory" or path == "/directory/search":
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            html = f"""<!DOCTYPE html>
<html>
<head><title>Tech Parks Directory</title></head>
<body>
  <h1>Verified Companies in Bangalore</h1>
  <div class="directory-list">
    <div class="directory-card" data-company-id="apex-innovations-01">
      <h2 class="company-title">Apex Innovations</h2>
      <span class="company-location">100 Innovation Way, Suite 100, Bangalore, KA 560001, India</span>
      <span class="company-phone">+91 80 5555 0100</span>
      <a class="detail-link" href="/directory/apex">View Profile</a>
      <a class="website-link" href="https://apexinnovations.example.com">Visit Website</a>
    </div>
  </div>
</body>
</html>"""
            self.wfile.write(html.encode("utf-8"))
            return

        # 2. Directory detail page
        if path == "/directory/apex":
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            html = f"""<!DOCTYPE html>
<html>
<head><title>Apex Innovations - Directory Detail</title></head>
<body>
  <h1>Apex Innovations</h1>
  <div class="detail-container">
    <span class="detail-address">100 Innovation Way, Suite 100, Bangalore, Karnataka 560001, India</span>
    <span class="geo-lat">12.9716</span>
    <span class="geo-lng">77.5946</span>
    <span class="detail-phone">+91 80 5555 0100</span>
    <a class="official-website" href="https://apexinnovations.example.com">Official Website</a>
    <p class="company-desc">Pioneering distributed cloud systems and robotics.</p>
  </div>
</body>
</html>"""
            self.wfile.write(html.encode("utf-8"))
            return

        # 3. Company website: homepage (ordinary HTML)
        if path == "/company/apex" or path == "/company/apex/":
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            html = f"""<!DOCTYPE html>
<html>
<head>
  <title>Apex Innovations - Official Website</title>
</head>
<body>
  <header>
    <h1>Apex Innovations</h1>
    <nav>
      <a href="http://127.0.0.1:{port}/company/apex/about">About</a>
      <a href="http://127.0.0.1:{port}/company/apex/careers">Careers</a>
      <a href="http://127.0.0.1:{port}/company/apex/jsonld">Structured Info</a>
    </nav>
  </header>
  <main>
    <address>100 Innovation Way, Suite 100, Bangalore, Karnataka 560001, India</address>
    <a href="tel:+918055550100">+91 80 5555 0100</a>
  </main>
</body>
</html>"""
            self.wfile.write(html.encode("utf-8"))
            return

        # 4. Company website: about page
        if path == "/company/apex/about":
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            html = f"""<!DOCTYPE html>
<html>
<head><title>About Us - Apex Innovations</title></head>
<body>
  <h1>About Apex Innovations</h1>
  <p>Building high-concurrency infrastructure in Bangalore.</p>
  <address>100 Innovation Way, Suite 100, Bangalore, Karnataka 560001</address>
</body>
</html>"""
            self.wfile.write(html.encode("utf-8"))
            return

        # 5. Company website: careers / JSON-LD page
        if path in ("/company/apex/careers", "/company/apex/jsonld"):
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            html = f"""<!DOCTYPE html>
<html>
<head>
  <title>Careers & Information - Apex Innovations</title>
  <script type="application/ld+json">
  {{
    "@context": "https://schema.org",
    "@graph": [
      {{
        "@type": "LocalBusiness",
        "@id": "https://apexinnovations.example.com/#org",
        "name": "Apex Innovations",
        "url": "https://apexinnovations.example.com",
        "telephone": "+91-80-5555-0100",
        "address": {{
          "@type": "PostalAddress",
          "streetAddress": "100 Innovation Way Suite 100",
          "addressLocality": "Bangalore",
          "addressRegion": "KA",
          "postalCode": "560001",
          "addressCountry": "IN"
        }},
        "geo": {{
          "@type": "GeoCoordinates",
          "latitude": 12.9716,
          "longitude": 77.5946
        }}
      }},
      {{
        "@type": "JobPosting",
        "title": "Senior Distributed Systems Engineer",
        "description": "Develop high-throughput Go backend services, distributed storage, and Kubernetes platforms.",
        "datePosted": "{now_iso[:10]}",
        "validThrough": "2030-01-01",
        "employmentType": "FULL_TIME",
        "hiringOrganization": {{
          "@type": "Organization",
          "name": "Apex Innovations",
          "sameAs": "https://apexinnovations.example.com"
        }},
        "jobLocation": {{
          "@type": "Place",
          "address": {{
            "@type": "PostalAddress",
            "streetAddress": "100 Innovation Way Suite 100",
            "addressLocality": "Bangalore",
            "addressRegion": "KA",
            "addressCountry": "IN"
          }}
        }}
      }}
    ]
  }}
  </script>
</head>
<body>
  <h1>Apex Innovations Careers</h1>
  <div class="job-list">
    <h2>Senior Distributed Systems Engineer</h2>
    <p>Bangalore, Karnataka, India</p>
  </div>
</body>
</html>"""
            self.wfile.write(html.encode("utf-8"))
            return

        # 6. Public ATS endpoint: Greenhouse
        if path.startswith("/ats/greenhouse/"):
            self.send_response(200)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.end_headers()
            data = {
                "jobs": [
                    {
                        "id": 8801,
                        "title": "Principal Infrastructure Architect",
                        "updated_at": now_iso,
                        "absolute_url": f"http://127.0.0.1:{port}/jobs/8801",
                        "location": {"name": "Bangalore, India"},
                        "departments": [{"name": "Platform Engineering"}],
                        "content": "<p>Architect next-generation distributed systems in Go and Rust.</p>",
                    }
                ]
            }
            self.wfile.write(json.dumps(data).encode("utf-8"))
            return

        # 7. Public ATS endpoint: Lever
        if path.startswith("/ats/lever/"):
            self.send_response(200)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.end_headers()
            data = [
                {
                    "id": "lever-7701",
                    "text": "Senior Site Reliability Engineer",
                    "createdAt": now_ms,
                    "hostedUrl": f"http://127.0.0.1:{port}/jobs/lever-7701",
                    "categories": {
                        "location": "Bangalore, India",
                        "team": "Infrastructure",
                        "commitment": "Full-time",
                    },
                    "descriptionPlain": "Build and scale high-reliability cloud systems.",
                }
            ]
            self.wfile.write(json.dumps(data).encode("utf-8"))
            return

        # 8. JavaScript-rendered page (SPA shell requiring DOM execution)
        if path == "/js/quantum" or path == "/js/quantum/":
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            html = f"""<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Quantum Robotics</title>
  <script>
    window.addEventListener("DOMContentLoaded", () => {{
      const root = document.getElementById("root");
      if (root) {{
        root.innerHTML = `
          <div class="company-profile">
            <h1 class="company-name">Quantum Robotics Inc.</h1>
            <p class="description">Autonomous computer vision and warehouse automation.</p>
            <address>200 Quantum Way, Bangalore, Karnataka 560001</address>
            <div class="contact-info">
              <a href="tel:+918055550299" class="phone">+91 80 5555 0299</a>
              <a href="https://quantumrobotics.example.com" class="website">quantumrobotics.example.com</a>
            </div>
            <div class="jobs">
              <h2>Open Positions</h2>
              <div class="job-card">
                <h3>Robotics Computer Vision Engineer</h3>
                <span class="location">Bangalore, India</span>
              </div>
            </div>
          </div>
        `;
        const s = document.createElement("script");
        s.type = "application/ld+json";
        s.textContent = JSON.stringify({{
          "@context": "https://schema.org",
          "@type": "LocalBusiness",
          "name": "Quantum Robotics Inc.",
          "url": "https://quantumrobotics.example.com",
          "telephone": "+91-80-5555-0299",
          "address": {{
            "@type": "PostalAddress",
            "streetAddress": "200 Quantum Way",
            "addressLocality": "Bangalore",
            "addressRegion": "KA",
            "postalCode": "560001",
            "addressCountry": "IN"
          }},
          "geo": {{
            "@type": "GeoCoordinates",
            "latitude": 12.9720,
            "longitude": 77.5950
          }}
        }});
        document.head.appendChild(s);
      }}
    }});
  </script>
</head>
<body>
  <div id="root"></div>
  <noscript>You need to enable JavaScript to view this application.</noscript>
</body>
</html>"""
            self.wfile.write(html.encode("utf-8"))
            return

        # 9. Failing endpoint to test partial-source handling
        if path == "/fail" or path.startswith("/fail/"):
            self.send_response(500)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.end_headers()
            self.wfile.write(b'{"error": "Simulated source failure", "code": 500}')
            return

        # Default 404
        self.send_response(404)
        self.send_header("Content-Type", "text/plain")
        self.end_headers()
        self.wfile.write(b"Not Found")


class FakeSiteServer:
    """Manages the background HTTP server for tests."""

    def __init__(self, host: str = "127.0.0.1", port: int = 0) -> None:
        self.host = host
        self.server = ThreadingHTTPServer((host, port), FakeSiteHandler)
        self.port = self.server.server_address[1]
        self._thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    @property
    def base_url(self) -> str:
        return f"http://{self.host}:{self.port}"

    def start(self) -> None:
        self._thread.start()

    def stop(self) -> None:
        self.server.shutdown()
        self.server.server_close()
        self._thread.join(timeout=2.0)


@contextlib.contextmanager
def run_fake_site_server(host: str = "127.0.0.1", port: int = 0):
    """Context manager for running the fake site fixture server."""
    server = FakeSiteServer(host=host, port=port)
    server.start()
    try:
        yield server
    finally:
        server.stop()
