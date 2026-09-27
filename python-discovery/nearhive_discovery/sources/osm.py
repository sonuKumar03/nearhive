from collections.abc import AsyncIterator, Awaitable, Callable
from datetime import datetime, timezone
import json
import logging
from typing import Any
import urllib.parse

import httpx

from nearhive_discovery.classify import compute_company_content_hash
from nearhive_discovery.contracts import (
    CONTRACT_VERSION,
    CompanyEvidence,
    DiscoveryJob,
    EvidenceBatch,
)
from nearhive_discovery.sources.base import BaseSourceAdapter

logger = logging.getLogger(__name__)

DEFAULT_OVERPASS_ENDPOINTS = [
    "https://overpass-api.de/api/interpreter",
    "https://overpass.kumi.systems/api/interpreter",
    "https://overpass.private.coffee/api/interpreter",
    "https://maps.mail.ru/osm/tools/overpass/api/interpreter",
]


class OpenStreetMapSource(BaseSourceAdapter):
    """Source adapter for discovering real tech companies and offices via OpenStreetMap Overpass API."""

    name: str = "osm_dataset"
    source_family: str = "open_dataset"

    def __init__(
        self,
        endpoints: list[str] | None = None,
        fetcher: Callable[[str, str], Awaitable[str]] | None = None,
        http_client: httpx.AsyncClient | None = None,
        batch_size: int = 50,
        user_agent: str = "NearHive/1.0 (Discovery Pipeline; +https://github.com/sonuKumar03/nearhive)",
    ) -> None:
        self.endpoints = list(endpoints) if endpoints else list(DEFAULT_OVERPASS_ENDPOINTS)
        self.fetcher = fetcher
        self.http_client = http_client
        self.batch_size = max(1, batch_size)
        self.user_agent = user_agent

    async def _query_overpass(self, query: str) -> dict[str, Any] | None:
        headers = {
            "User-Agent": self.user_agent,
            "Content-Type": "application/x-www-form-urlencoded",
        }
        last_error = None

        for endpoint in self.endpoints:
            try:
                if self.fetcher is not None:
                    raw_text = await self.fetcher(endpoint, query)
                    return json.loads(raw_text)

                client = self.http_client or httpx.AsyncClient(timeout=30.0)
                try:
                    resp = await client.post(endpoint, data={"data": query}, headers=headers)
                    if resp.status_code == 200:
                        return resp.json()
                    last_error = f"Endpoint {endpoint} returned status {resp.status_code}"
                    logger.info("Overpass endpoint %s returned HTTP %d; trying next endpoint...", endpoint, resp.status_code)
                finally:
                    if self.http_client is None:
                        await client.aclose()
            except Exception as exc:
                last_error = str(exc)
                logger.info("Overpass endpoint %s failed (%s); trying next endpoint...", endpoint, exc)

        logger.warning("All Overpass endpoints failed for discovery: %s", last_error)
        return None

    async def run(
        self, job: DiscoveryJob, now: datetime | None = None
    ) -> AsyncIterator[EvidenceBatch]:
        """Queries OpenStreetMap Overpass for offices and tech companies within the job radius."""
        if job.lat == 0 and job.lng == 0:
            logger.warning("Skipping OSM discovery for 0,0 coordinates")
            return

        current_time = now or datetime.now(timezone.utc)
        radius_meters = int((job.radius_km or 15.0) * 1000)
        if radius_meters <= 0:
            radius_meters = 15000

        query = (
            f'[out:json][timeout:20];('
            f'nwr["office"~"company|it|software|telecommunication|coworking|research"](around:{radius_meters},{job.lat},{job.lng});'
            f'nwr["amenity"="coworking_space"](around:{radius_meters},{job.lat},{job.lng});'
            f');out center;'
        )

        data = await self._query_overpass(query)
        if not data or not isinstance(data.get("elements"), list):
            logger.info("No elements returned from OpenStreetMap for (%f, %f)", job.lat, job.lng)
            return

        elements = data["elements"]
        logger.info("Discovered %d raw OSM elements for job %s", len(elements), job.id)

        batch_companies: list[CompanyEvidence] = []

        for el in elements:
            tags = el.get("tags") or {}
            name = (tags.get("name") or tags.get("brand") or tags.get("operator") or "").strip()
            if not name or len(name) < 2:
                continue

            # Extract coordinates
            lat = el.get("lat")
            lng = el.get("lon")
            if (lat is None or lng is None) and "center" in el:
                center = el["center"]
                lat = center.get("lat")
                lng = center.get("lon")

            if lat is None or lng is None:
                continue

            try:
                lat_f = float(lat)
                lng_f = float(lng)
            except (ValueError, TypeError):
                continue

            # Extract and normalize domain
            raw_website = tags.get("website") or tags.get("contact:website") or tags.get("url")
            domain = None
            if raw_website:
                raw_website = raw_website.strip()
                if not raw_website.startswith(("http://", "https://")):
                    raw_website = "https://" + raw_website
                try:
                    parsed = urllib.parse.urlsplit(raw_website)
                    host = parsed.hostname or ""
                    if host.startswith("www."):
                        host = host[4:]
                    if "." in host and len(host) >= 4:
                        domain = host.lower()
                except Exception:
                    domain = None

            # Extract address
            addr_parts = []
            for k in ("addr:housenumber", "addr:street", "addr:suburb", "addr:city", "addr:postcode"):
                v = tags.get(k)
                if v:
                    addr_parts.append(v.strip())
            address = ", ".join(addr_parts)

            # Extract phone
            phone = tags.get("phone") or tags.get("contact:phone")

            el_type = el.get("type", "node")
            el_id = el.get("id", "")
            source_rec_id = f"osm:{el_type}:{el_id}"
            evidence_url = f"https://www.openstreetmap.org/{el_type}/{el_id}"

            c_hash = compute_company_content_hash(
                name=name,
                domain=domain,
                source_record_id=source_rec_id,
            )

            batch_companies.append(
                CompanyEvidence(
                    name=name,
                    source_record_id=source_rec_id,
                    evidence_url=evidence_url,
                    domain=domain,
                    address=address,
                    lat=lat_f,
                    lng=lng_f,
                    phone=phone,
                    content_hash=c_hash,
                    metadata={"osm_type": el_type, "osm_id": el_id, "tags": tags},
                )
            )

            if len(batch_companies) >= self.batch_size:
                yield EvidenceBatch(
                    contract_version=CONTRACT_VERSION,
                    discovery_job_id=job.id,
                    source=self.name,
                    source_family=self.source_family,
                    observed_at=current_time,
                    companies=list(batch_companies),
                    jobs=[],
                )
                batch_companies = []

        if batch_companies:
            yield EvidenceBatch(
                contract_version=CONTRACT_VERSION,
                discovery_job_id=job.id,
                source=self.name,
                source_family=self.source_family,
                observed_at=current_time,
                companies=list(batch_companies),
                jobs=[],
            )
