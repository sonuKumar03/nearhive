from __future__ import annotations

import asyncio
import logging
from dataclasses import replace
from typing import Any

import httpx

from nearhive_discovery.contracts import TechnicalJobEvidence, WorkArrangement
from nearhive_discovery.settings import settings

logger = logging.getLogger(__name__)


class LocationResolver:
    def __init__(
        self,
        http_client: httpx.AsyncClient | None = None,
        base_url: str | None = None,
        provider_name: str | None = None,
        max_lookups: int | None = None,
        timeout: float = 5.0,
    ) -> None:
        self.http_client = http_client
        self._owns_client = http_client is None
        self.base_url = base_url or getattr(
            settings, "geocoder_url", "https://nominatim.openstreetmap.org/search"
        )
        self.provider_name = provider_name or getattr(settings, "geocoder_provider", "nominatim")
        self.max_lookups = (
            max_lookups if max_lookups is not None else getattr(settings, "max_job_geocodes", 50)
        )
        self.timeout = timeout
        self.lookups_count: int = 0
        self._cache: dict[str, tuple[float, float] | None] = {}

    async def _get_client(self) -> httpx.AsyncClient:
        if self.http_client is None:
            self.http_client = httpx.AsyncClient(
                timeout=self.timeout,
                headers={"User-Agent": "NearHive-Discovery/1.0"},
            )
        return self.http_client

    async def resolve(self, location_raw: str) -> tuple[float, float] | None:
        query = location_raw.strip()
        if not query:
            return None

        normalized = query.lower()
        if normalized in self._cache:
            return self._cache[normalized]

        if self.lookups_count >= self.max_lookups:
            logger.debug(
                "Geocoder lookup ceiling reached (%d/%d), skipping: %s",
                self.lookups_count,
                self.max_lookups,
                location_raw,
            )
            return None

        self.lookups_count += 1
        client = await self._get_client()

        try:
            resp = await client.get(
                self.base_url,
                params={"q": query, "format": "json", "limit": 1},
                headers={"User-Agent": "NearHive-Discovery/1.0"},
            )
            if resp.status_code != 200:
                logger.debug("Geocoder returned status %d for query %s", resp.status_code, query)
                self._cache[normalized] = None
                return None

            data = resp.json()
            if not isinstance(data, list) or len(data) == 0:
                self._cache[normalized] = None
                return None

            first = data[0]
            lat = float(first.get("lat", 0.0))
            lng = float(first.get("lon", first.get("lng", 0.0)))

            if not (-90.0 <= lat <= 90.0 and -180.0 <= lng <= 180.0):
                logger.debug(
                    "Geocoder returned out of range coordinates (%f, %f) for %s",
                    lat,
                    lng,
                    query,
                )
                self._cache[normalized] = None
                return None

            res = (lat, lng)
            self._cache[normalized] = res
            return res
        except Exception as exc:
            logger.debug("Geocoder lookup failed for %s: %s", query, exc)
            self._cache[normalized] = None
            return None

    async def close(self) -> None:
        if self._owns_client and self.http_client is not None:
            await self.http_client.aclose()
            self.http_client = None


async def resolve_job_location(
    job: TechnicalJobEvidence,
    resolver: LocationResolver | None = None,
) -> TechnicalJobEvidence:
    meta = dict(job.metadata)

    # 1. Existing structured coordinates are preserved
    if job.lat is not None and job.lng is not None:
        meta["location_resolution"] = "structured"
        return replace(job, metadata=meta)

    # 2. Remote jobs are never geocoded
    if job.work_arrangement == WorkArrangement.REMOTE:
        meta["location_resolution"] = "unresolved"
        return replace(job, lat=None, lng=None, metadata=meta)

    # 3. Missing raw location or resolver
    raw = (job.location_raw or "").strip()
    if not raw or resolver is None:
        meta["location_resolution"] = "unresolved"
        return replace(job, lat=None, lng=None, metadata=meta)

    # 4. Resolve coordinates
    coords = await resolver.resolve(raw)
    if coords is not None:
        lat, lng = coords
        meta["location_resolution"] = "geocoded"
        if resolver.provider_name:
            meta["location_provider"] = resolver.provider_name
        return replace(job, lat=lat, lng=lng, metadata=meta)

    meta["location_resolution"] = "unresolved"
    return replace(job, lat=None, lng=None, metadata=meta)
