import hashlib
import logging
from collections.abc import AsyncIterator, Callable, Awaitable
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
import urllib.parse

import httpx
import yaml
from scrapy import Selector

from nearhive_discovery.contracts import (
    CONTRACT_VERSION,
    CompanyEvidence,
    DiscoveryJob,
    EvidenceBatch,
)
from nearhive_discovery.http import (
    DEFAULT_USER_AGENT,
    SSRFError,
    validate_public_url,
)
from nearhive_discovery.sources.base import BaseSourceAdapter

logger = logging.getLogger(__name__)


def _extract_domain(url: str | None) -> str | None:
    if not url:
        return None
    try:
        parsed = urllib.parse.urlsplit(url)
        host = parsed.hostname
        if host:
            host = host.lower()
            if host.startswith("www."):
                host = host[4:]
            return host
    except Exception:
        pass
    return None


def _extract_value(element: Any, selector_pattern: str | None) -> str | None:
    if not selector_pattern or not element:
        return None
    pattern = selector_pattern.strip()
    try:
        if pattern.startswith("/") or pattern.startswith(".//") or pattern.startswith("("):
            val = element.xpath(pattern).get()
        elif pattern.startswith("@"):
            val = element.xpath(pattern).get()
        elif pattern.startswith("xpath:"):
            val = element.xpath(pattern[6:].strip()).get()
        elif pattern.startswith("css:"):
            val = element.css(pattern[4:].strip()).get()
        else:
            val = element.css(pattern).get()

        if val is not None:
            val = val.strip()
            return val if val else None
    except Exception as exc:
        logger.debug("Failed evaluating selector '%s': %s", pattern, exc)
    return None


class ConfiguredDirectorySource(BaseSourceAdapter):
    """Source adapter for public directories configured via YAML."""

    source_family: str = "public_directory"

    def __init__(
        self,
        config: dict[str, Any] | None = None,
        config_path: str | Path | None = None,
        batch_size: int = 50,
        fetcher: Callable[[str], Awaitable[str]] | None = None,
        http_client: httpx.AsyncClient | None = None,
        resolve_dns: bool | None = None,
    ) -> None:
        if config is not None:
            self.config = dict(config)
        elif config_path is not None:
            with open(config_path, "r", encoding="utf-8") as f:
                loaded = yaml.safe_load(f)
                if isinstance(loaded, dict) and "directories" in loaded:
                    dirs = loaded["directories"]
                    self.config = dirs[0] if dirs else {}
                elif isinstance(loaded, dict):
                    self.config = loaded
                else:
                    self.config = {}
        else:
            self.config = {}

        self.name: str = self.config.get("id") or self.config.get("name") or "configured_directory"
        self.url_template: str = self.config.get("url_template", "")
        self.selectors: dict[str, str] = self.config.get("selectors", {})
        self.detail_selectors: dict[str, str] = self.config.get("detail_selectors", {})
        self.pagination: dict[str, Any] = self.config.get("pagination", {})
        self.batch_size: int = batch_size
        self.fetcher = fetcher
        self._http_client = http_client
        # If fetcher is provided (e.g. tests), default resolve_dns to False unless explicitly specified
        self.resolve_dns: bool = (
            resolve_dns if resolve_dns is not None else (fetcher is None)
        )

    @classmethod
    def load_all(
        cls,
        config_path: str | Path,
        batch_size: int = 50,
        fetcher: Callable[[str], Awaitable[str]] | None = None,
        http_client: httpx.AsyncClient | None = None,
        resolve_dns: bool | None = None,
    ) -> list["ConfiguredDirectorySource"]:
        """Loads all directories defined in a YAML configuration file."""
        with open(config_path, "r", encoding="utf-8") as f:
            data = yaml.safe_load(f)

        sources: list[ConfiguredDirectorySource] = []
        if isinstance(data, dict):
            dirs = data.get("directories", [])
            for d in dirs:
                sources.append(
                    cls(
                        config=d,
                        batch_size=batch_size,
                        fetcher=fetcher,
                        http_client=http_client,
                        resolve_dns=resolve_dns,
                    )
                )
        return sources

    def format_url(self, template: str, job: DiscoveryJob, page: int) -> str:
        """Formats the URL template with job lat/lng/radius and current page."""
        params = {
            "lat": job.lat,
            "lng": job.lng,
            "radius": job.radius_km,
            "radius_km": job.radius_km,
            "page": page,
        }
        formatted = template
        for k, v in params.items():
            formatted = formatted.replace(f"{{{k}}}", str(v))
        return formatted

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

    async def run(self, job: DiscoveryJob) -> AsyncIterator[EvidenceBatch]:
        """Crawls the directory, extracts companies, and streams EvidenceBatches."""
        start_page = int(self.pagination.get("start_page", 1))
        page_ceiling = int(self.pagination.get("page_ceiling", self.pagination.get("max_pages", 5)))
        current_page = start_page

        batch_buffer: list[CompanyEvidence] = []
        seen_hashes: set[str] = set()

        while current_page <= page_ceiling:
            page_url = self.format_url(self.url_template, job, current_page)
            logger.info("Fetching directory page %d: %s", current_page, page_url)

            try:
                html = await self._fetch(page_url)
            except Exception as exc:
                logger.warning("Failed to fetch directory page %s: %s", page_url, exc)
                break

            sel = Selector(text=html)
            card_sel = self.selectors.get("card", ".directory-card")
            cards = sel.css(card_sel) if not (card_sel.startswith("/") or card_sel.startswith(".//")) else sel.xpath(card_sel)

            if not cards:
                logger.info("No cards found on page %d; terminating pagination", current_page)
                break

            for card in cards:
                try:
                    name = _extract_value(card, self.selectors.get("name"))
                    if not name or not name.strip():
                        # Malformed card: missing company name
                        logger.warning("Skipping malformed card: missing or empty company name")
                        continue
                    name = name.strip()

                    source_record_id = _extract_value(card, self.selectors.get("source_record_id"))
                    address = _extract_value(card, self.selectors.get("address")) or ""
                    phone = _extract_value(card, self.selectors.get("phone"))
                    website = _extract_value(card, self.selectors.get("website"))
                    detail_url_val = _extract_value(card, self.selectors.get("detail_url"))

                    evidence_url = page_url
                    lat: float | None = None
                    lng: float | None = None
                    metadata: dict[str, Any] = {"directory_id": self.name}

                    # List/detail pattern: follow detail page if present
                    if detail_url_val:
                        resolved_detail = urllib.parse.urljoin(page_url, detail_url_val)
                        evidence_url = resolved_detail
                        try:
                            detail_html = await self._fetch(resolved_detail)
                            detail_sel = Selector(text=detail_html)

                            if "address" in self.detail_selectors:
                                det_addr = _extract_value(detail_sel, self.detail_selectors["address"])
                                if det_addr:
                                    address = det_addr

                            if "phone" in self.detail_selectors:
                                det_phone = _extract_value(detail_sel, self.detail_selectors["phone"])
                                if det_phone:
                                    phone = det_phone

                            if "website" in self.detail_selectors:
                                det_web = _extract_value(detail_sel, self.detail_selectors["website"])
                                if det_web:
                                    website = det_web

                            if "lat" in self.detail_selectors:
                                lat_str = _extract_value(detail_sel, self.detail_selectors["lat"])
                                if lat_str:
                                    try:
                                        parsed_lat = float(lat_str)
                                        if -90.0 <= parsed_lat <= 90.0:
                                            lat = parsed_lat
                                    except ValueError:
                                        pass

                            if "lng" in self.detail_selectors:
                                lng_str = _extract_value(detail_sel, self.detail_selectors["lng"])
                                if lng_str:
                                    try:
                                        parsed_lng = float(lng_str)
                                        if -180.0 <= parsed_lng <= 180.0:
                                            lng = parsed_lng
                                    except ValueError:
                                        pass

                            if "description" in self.detail_selectors:
                                desc = _extract_value(detail_sel, self.detail_selectors["description"])
                                if desc:
                                    metadata["description"] = desc
                        except Exception as det_exc:
                            logger.warning(
                                "Failed to fetch or parse detail page %s: %s",
                                resolved_detail,
                                det_exc,
                            )

                    domain = _extract_domain(website) or _extract_domain(evidence_url)

                    # Deterministic content hash
                    hash_input = f"{name.lower()}|{domain or ''}|{address.lower()}|{lat or ''}|{lng or ''}"
                    content_hash = hashlib.sha256(hash_input.encode("utf-8")).hexdigest()

                    if content_hash in seen_hashes:
                        continue
                    seen_hashes.add(content_hash)

                    company = CompanyEvidence(
                        name=name,
                        source_record_id=source_record_id,
                        evidence_url=evidence_url,
                        domain=domain,
                        address=address,
                        lat=lat,
                        lng=lng,
                        phone=phone,
                        content_hash=content_hash,
                        metadata=metadata,
                    )
                    batch_buffer.append(company)

                    if len(batch_buffer) >= self.batch_size:
                        yield EvidenceBatch(
                            contract_version=CONTRACT_VERSION,
                            discovery_job_id=job.id,
                            source=self.name,
                            source_family=self.source_family,
                            observed_at=datetime.now(timezone.utc),
                            companies=list(batch_buffer),
                            jobs=[],
                        )
                        batch_buffer.clear()

                except Exception as card_exc:
                    logger.warning("Error processing directory card: %s", card_exc)
                    continue

            # Stop if reached page ceiling
            if current_page >= page_ceiling:
                break
            current_page += 1

        if batch_buffer:
            yield EvidenceBatch(
                contract_version=CONTRACT_VERSION,
                discovery_job_id=job.id,
                source=self.name,
                source_family=self.source_family,
                observed_at=datetime.now(timezone.utc),
                companies=list(batch_buffer),
                jobs=[],
            )
            batch_buffer.clear()
