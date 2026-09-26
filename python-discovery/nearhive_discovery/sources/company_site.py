import collections
import hashlib
import logging
import re
from collections.abc import AsyncIterator, Callable, Awaitable
from datetime import datetime, timezone
from typing import Any
import urllib.parse
import xml.etree.ElementTree as ET

import httpx
from scrapy import Selector

from nearhive_discovery.contracts import (
    CONTRACT_VERSION,
    CompanyEvidence,
    DiscoveryJob,
    EvidenceBatch,
    TechnicalJobEvidence,
)
from nearhive_discovery.http import (
    DEFAULT_USER_AGENT,
    SSRFError,
    validate_public_url,
)
from nearhive_discovery.sources.base import BaseSourceAdapter
from nearhive_discovery.sources.jsonld import extract_jsonld

logger = logging.getLogger(__name__)

REJECT_TOKENS = {
    "logout", "signout", "login", "signin",
    "auth", "sso", "account", "profile", "user",
    "dashboard", "cart", "checkout", "basket", "order",
    "shop", "buy", "form", "forms", "submit", "admin",
}

ALLOWED_PATH_TOKENS = {
    "about", "company", "team", "story",
    "contact", "reach",
    "location", "locations", "office", "offices", "branch", "branches",
    "career", "careers", "job", "jobs", "opening", "openings",
    "sitemap",
}


def _clean_domain(host: str | None) -> str | None:
    if not host:
        return None
    cleaned = host.lower().strip()
    if cleaned.startswith("www."):
        cleaned = cleaned[4:]
    return cleaned


def should_reject_url(url: str) -> bool:
    """Returns True if the URL contains disallowed patterns like login, cart, logout, form."""
    if not url:
        return True
    try:
        parsed = urllib.parse.urlsplit(url.strip())
        if parsed.scheme.lower() not in ("http", "https"):
            return True

        path_query = f"{parsed.path}?{parsed.query}".lower()
        tokens = [t for t in re.split(r"[\s/._?=&~-]+", path_query) if t]
        for t in tokens:
            if t in REJECT_TOKENS:
                return True
        return False
    except Exception:
        return True


def is_allowed_site_url(url: str, target_domain: str) -> bool:
    """Validates if a URL is within the official site boundary (same domain, allowed category)."""
    if should_reject_url(url):
        return False

    try:
        parsed = urllib.parse.urlsplit(url.strip())
        host = _clean_domain(parsed.hostname)
        expected = _clean_domain(target_domain)

        # Cross-domain check
        if not host or host != expected:
            return False

        path = parsed.path.strip()
        # Homepage is always allowed
        if not path or path in ("/", "/index.html", "/index.htm", "/index.php"):
            return True

        # Check allowed category path tokens
        tokens = [t for t in re.split(r"[\s/._?=&~-]+", path.lower()) if t]
        for t in tokens:
            if t in ALLOWED_PATH_TOKENS:
                return True

        return False
    except Exception:
        return False


class CompanySiteSource(BaseSourceAdapter):
    """Source adapter for bounded enrichment of official company websites."""

    source_family: str = "official_site"

    def __init__(
        self,
        target_url: str | None = None,
        target_urls: list[str] | None = None,
        sitemap_url: str | None = None,
        max_pages: int = 10,
        max_depth: int = 2,
        batch_size: int = 50,
        fetcher: Callable[[str], Awaitable[str]] | None = None,
        http_client: httpx.AsyncClient | None = None,
        resolve_dns: bool | None = None,
    ) -> None:
        self.name: str = "company_site"
        self.target_urls: list[str] = []
        if target_urls:
            self.target_urls.extend(target_urls)
        elif target_url:
            self.target_urls.append(target_url)

        self.sitemap_url = sitemap_url
        self.max_pages = max_pages
        self.max_depth = max_depth
        self.batch_size = batch_size
        self.fetcher = fetcher
        self._http_client = http_client
        self.resolve_dns: bool = (
            resolve_dns if resolve_dns is not None else (fetcher is None)
        )

    async def _fetch(self, url: str) -> str:
        validate_public_url(url, resolve_dns=self.resolve_dns)
        if self.fetcher is not None:
            return await self.fetcher(url)

        if self._http_client is not None:
            resp = await self._http_client.get(
                url,
                headers={"User-Agent": DEFAULT_USER_AGENT},
                follow_redirects=True,
                timeout=15.0,
            )
            resp.raise_for_status()
            return resp.text

        async with httpx.AsyncClient(
            timeout=15.0,
            follow_redirects=True,
            headers={"User-Agent": DEFAULT_USER_AGENT},
        ) as client:
            resp = await client.get(url)
            resp.raise_for_status()
            return resp.text

    def _parse_sitemap_urls(self, xml_text: str, target_domain: str) -> list[str]:
        """Extracts allowed, non-rejected URLs from an XML sitemap."""
        valid_urls: list[str] = []
        try:
            root = ET.fromstring(xml_text)
            # Remove namespace if present
            for elem in root.iter():
                if "}" in elem.tag:
                    elem.tag = elem.tag.split("}", 1)[1]

            for loc in root.findall(".//loc"):
                if loc.text:
                    u = loc.text.strip()
                    if is_allowed_site_url(u, target_domain):
                        valid_urls.append(u)
        except Exception as exc:
            logger.debug("Failed parsing sitemap XML: %s", exc)
        return valid_urls

    async def run(self, job: DiscoveryJob) -> AsyncIterator[EvidenceBatch]:
        """Performs bounded BFS crawl over official site pages, streaming evidence batches."""
        if not self.target_urls:
            return

        company_buffer: list[CompanyEvidence] = []
        job_buffer: list[TechnicalJobEvidence] = []
        seen_company_hashes: set[str] = set()
        seen_job_hashes: set[str] = set()

        for seed_url in self.target_urls:
            parsed_seed = urllib.parse.urlsplit(seed_url)
            target_domain = _clean_domain(parsed_seed.hostname)
            if not target_domain:
                continue

            queue: collections.deque[tuple[str, int]] = collections.deque()
            visited_urls: set[str] = set()
            enqueued_urls: set[str] = {seed_url}

            # Enqueue seed URL at depth 0
            queue.append((seed_url, 0))

            # If sitemap is configured or available, fetch and enqueue matching URLs at depth 1
            sitemap_target = self.sitemap_url
            if sitemap_target:
                try:
                    sitemap_xml = await self._fetch(sitemap_target)
                    sitemap_links = self._parse_sitemap_urls(sitemap_xml, target_domain)
                    for sm_url in sitemap_links:
                        if sm_url not in enqueued_urls:
                            enqueued_urls.add(sm_url)
                            queue.append((sm_url, 1))
                except Exception as sm_exc:
                    logger.debug("Could not fetch or parse sitemap %s: %s", sitemap_target, sm_exc)

            while queue and len(visited_urls) < self.max_pages:
                current_url, depth = queue.popleft()
                if current_url in visited_urls:
                    continue
                if depth > self.max_depth:
                    continue

                visited_urls.add(current_url)
                logger.info("Crawling official site page (%d/%d): %s (depth %d)", len(visited_urls), self.max_pages, current_url, depth)

                try:
                    html = await self._fetch(current_url)
                except Exception as exc:
                    logger.warning("Failed to fetch official site page %s: %s", current_url, exc)
                    continue

                # 1. JSON-LD Extraction
                companies, jobs = extract_jsonld(html, base_url=current_url)
                for comp in companies:
                    if comp.content_hash not in seen_company_hashes:
                        seen_company_hashes.add(comp.content_hash)
                        company_buffer.append(comp)

                for j in jobs:
                    if j.content_hash not in seen_job_hashes:
                        seen_job_hashes.add(j.content_hash)
                        job_buffer.append(j)

                # 2. HTML parsing for links and fallback evidence
                sel = Selector(text=html)

                # If no company extracted from JSON-LD on homepage, extract from HTML
                if not companies and depth == 0:
                    title = sel.xpath("//title/text()").get() or ""
                    clean_name = title.split("-")[0].split("|")[0].strip()
                    if clean_name:
                        addr = sel.xpath("//address//text()").getall()
                        addr_str = " ".join(" ".join(addr).split()) if addr else ""
                        phone = sel.xpath('//a[starts-with(@href, "tel:")]/@href').get()
                        phone_str = phone.replace("tel:", "").strip() if phone else None

                        hash_input = f"{clean_name.lower()}|{target_domain}|{addr_str.lower()}||"
                        c_hash = hashlib.sha256(hash_input.encode("utf-8")).hexdigest()
                        if c_hash not in seen_company_hashes:
                            seen_company_hashes.add(c_hash)
                            company_buffer.append(
                                CompanyEvidence(
                                    name=clean_name,
                                    domain=target_domain,
                                    evidence_url=current_url,
                                    address=addr_str,
                                    phone=phone_str,
                                    content_hash=c_hash,
                                    metadata={"source": "html_enrichment"},
                                )
                            )

                # 3. Discover next links if below depth ceiling and page limit
                if depth < self.max_depth and len(visited_urls) < self.max_pages:
                    discovered_links = sel.xpath("//a/@href").getall()
                    for raw_link in discovered_links:
                        clean_link = raw_link.strip()
                        if not clean_link:
                            continue
                        resolved = urllib.parse.urljoin(current_url, clean_link)
                        # Remove fragment
                        resolved = urllib.parse.urldefrag(resolved).url

                        if resolved not in enqueued_urls and resolved not in visited_urls:
                            if is_allowed_site_url(resolved, target_domain):
                                enqueued_urls.add(resolved)
                                queue.append((resolved, depth + 1))

                # Emit batch if buffer is full
                if len(company_buffer) + len(job_buffer) >= self.batch_size:
                    yield EvidenceBatch(
                        contract_version=CONTRACT_VERSION,
                        discovery_job_id=job.id,
                        source=self.name,
                        source_family=self.source_family,
                        observed_at=datetime.now(timezone.utc),
                        companies=list(company_buffer),
                        jobs=list(job_buffer),
                    )
                    company_buffer.clear()
                    job_buffer.clear()

        # Emit remaining buffered evidence
        if company_buffer or job_buffer:
            yield EvidenceBatch(
                contract_version=CONTRACT_VERSION,
                discovery_job_id=job.id,
                source=self.name,
                source_family=self.source_family,
                observed_at=datetime.now(timezone.utc),
                companies=list(company_buffer),
                jobs=list(job_buffer),
            )
            company_buffer.clear()
            job_buffer.clear()
