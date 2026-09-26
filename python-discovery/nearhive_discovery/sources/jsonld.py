import hashlib
from html import unescape
import json
import logging
import re
import urllib.parse
from datetime import datetime, timezone
from typing import Any

from scrapy import Selector

from nearhive_discovery.contracts import (
    CompanyEvidence,
    PublicationState,
    TechnicalJobEvidence,
    WorkArrangement,
)

logger = logging.getLogger(__name__)

COMPANY_TYPES = {
    "organization",
    "corporation",
    "company",
    "localbusiness",
    "store",
    "restaurant",
    "professionalservice",
    "financialservice",
    "automotivebusiness",
    "medicalorganization",
    "educationalorganization",
    "ngo",
    "governmentorganization",
}

JOB_TYPES = {
    "jobposting",
}


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


def _resolve_url(raw_url: Any, base_url: str) -> str | None:
    """Resolves relative URLs against base_url, or returns base_url if raw_url is missing."""
    if not raw_url or not isinstance(raw_url, str):
        return base_url if base_url else None
    cleaned = raw_url.strip()
    if not cleaned:
        return base_url if base_url else None
    if base_url:
        return urllib.parse.urljoin(base_url, cleaned)
    return cleaned


def _format_address(raw_addr: Any) -> str:
    if isinstance(raw_addr, str):
        return raw_addr.strip()
    if isinstance(raw_addr, dict):
        street = raw_addr.get("streetAddress", "")
        locality = raw_addr.get("addressLocality", "")
        region = raw_addr.get("addressRegion", "")
        postal_code = raw_addr.get("postalCode", "")
        country = raw_addr.get("addressCountry", "")
        if isinstance(country, dict):
            country = country.get("name", "")
        parts = [street, locality, region, postal_code, country]
        return ", ".join(str(p).strip() for p in parts if p and str(p).strip())
    if isinstance(raw_addr, list):
        formatted = [_format_address(item) for item in raw_addr]
        return "; ".join(f for f in formatted if f)
    return ""


def _extract_coordinates(entity: dict[str, Any]) -> tuple[float | None, float | None]:
    geo = entity.get("geo")
    if isinstance(geo, dict):
        lat_raw = geo.get("latitude")
        lng_raw = geo.get("longitude")
    else:
        lat_raw = entity.get("latitude")
        lng_raw = entity.get("longitude")

    lat: float | None = None
    lng: float | None = None

    if lat_raw is not None:
        try:
            lat = float(lat_raw)
        except (ValueError, TypeError):
            lat = None

    if lng_raw is not None:
        try:
            lng = float(lng_raw)
        except (ValueError, TypeError):
            lng = None

    # Both coordinates must be present and bounded within valid geographic limits
    if lat is None or lng is None:
        return None, None

    if not (-90.0 <= lat <= 90.0) or not (-180.0 <= lng <= 180.0):
        return None, None

    return lat, lng


def _clean_html_text(raw_html: str, max_length: int = 500) -> str:
    if not raw_html:
        return ""
    text = re.sub(r"<[^>]+>", " ", raw_html)
    text = unescape(text)
    text = " ".join(text.split())
    return text[:max_length]


def _flatten_entities(node: Any) -> list[dict[str, Any]]:
    if not node:
        return []
    if isinstance(node, list):
        out: list[dict[str, Any]] = []
        for item in node:
            out.extend(_flatten_entities(item))
        return out
    if isinstance(node, dict):
        out = []
        if "@graph" in node and isinstance(node["@graph"], (list, dict)):
            out.extend(_flatten_entities(node["@graph"]))
        if "@type" in node:
            out.append(node)
        return out
    return []


def _normalize_types(raw_type: Any) -> set[str]:
    if isinstance(raw_type, list):
        types = [str(t) for t in raw_type]
    elif raw_type is not None:
        types = [str(raw_type)]
    else:
        types = []

    normalized = set()
    for t in types:
        # Strip URL prefix or namespace e.g. https://schema.org/Organization -> Organization
        clean = t.split("/")[-1].split(":")[-1].strip().lower()
        if clean:
            normalized.add(clean)
    return normalized


def _parse_iso_datetime(date_val: Any) -> datetime | None:
    if not date_val:
        return None
    try:
        dt = datetime.fromisoformat(str(date_val).strip())
        if dt.tzinfo is None:
            return dt.replace(tzinfo=timezone.utc)
        return dt.astimezone(timezone.utc)
    except (ValueError, TypeError):
        return None


def extract_jsonld(
    response: Any,
    base_url: str = "",
) -> tuple[list[CompanyEvidence], list[TechnicalJobEvidence]]:
    """Extracts CompanyEvidence and TechnicalJobEvidence from JSON-LD script blocks.

    Handles Organization, LocalBusiness, PostalAddress, GeoCoordinates, and JobPosting.
    Skips malformed or corrupt JSON-LD blocks without losing valid blocks.
    Pure extraction function that works with Scrapy Responses, strings, or bytes.
    """
    if hasattr(response, "text"):
        html_text = response.text
        if not base_url and hasattr(response, "url"):
            base_url = str(response.url)
    elif isinstance(response, bytes):
        html_text = response.decode("utf-8", errors="replace")
    elif isinstance(response, str):
        html_text = response
    else:
        html_text = str(response)

    try:
        sel = Selector(text=html_text)
        script_blocks = sel.xpath('//script[contains(@type, "application/ld+json")]/text()').getall()
    except Exception as exc:
        logger.warning("Failed to parse HTML for JSON-LD scripts: %s", exc)
        return [], []

    companies: list[CompanyEvidence] = []
    jobs: list[TechnicalJobEvidence] = []

    seen_company_hashes: set[str] = set()
    seen_job_hashes: set[str] = set()

    for script_raw in script_blocks:
        raw_text = script_raw.strip()
        if not raw_text:
            continue

        try:
            parsed_json = json.loads(raw_text)
        except (json.JSONDecodeError, UnicodeDecodeError, ValueError) as exc:
            logger.debug("Skipping invalid JSON-LD block: %s", exc)
            continue

        entities = _flatten_entities(parsed_json)
        for entity in entities:
            types = _normalize_types(entity.get("@type"))

            # 1. Company Extraction (Organization / LocalBusiness / etc.)
            if types & COMPANY_TYPES:
                name = entity.get("name") or entity.get("legalName")
                if not name or not isinstance(name, str) or not name.strip():
                    continue
                name = name.strip()

                source_id = entity.get("@id")
                if not source_id:
                    ident = entity.get("identifier")
                    if isinstance(ident, dict):
                        source_id = ident.get("value") or ident.get("@id")
                    elif isinstance(ident, str):
                        source_id = ident
                source_record_id = str(source_id) if source_id else None

                evidence_url = _resolve_url(entity.get("url"), base_url)
                domain = _extract_domain(evidence_url) or _extract_domain(base_url)

                phone = entity.get("telephone") or entity.get("phone")
                phone = str(phone).strip() if phone else None

                address = _format_address(entity.get("address"))
                lat, lng = _extract_coordinates(entity)

                # Deterministic content hash
                hash_input = (
                    f"{name.lower()}|{domain or ''}|{address.lower()}|{lat or ''}|{lng or ''}"
                )
                content_hash = hashlib.sha256(hash_input.encode("utf-8")).hexdigest()

                if content_hash in seen_company_hashes:
                    continue
                seen_company_hashes.add(content_hash)

                metadata: dict[str, Any] = {}
                raw_type = entity.get("@type")
                if raw_type:
                    metadata["raw_type"] = raw_type
                if entity.get("description"):
                    metadata["description"] = _clean_html_text(str(entity["description"]))

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
                companies.append(company)

            # 2. Technical Job Extraction (JobPosting)
            if types & JOB_TYPES:
                title = entity.get("title") or entity.get("jobTitle")
                if not title or not isinstance(title, str) or not title.strip():
                    continue
                title = title.strip()

                hiring = entity.get("hiringOrganization")
                if isinstance(hiring, dict):
                    company_name = hiring.get("name") or hiring.get("legalName") or ""
                    hiring_url = _resolve_url(hiring.get("url"), base_url)
                elif isinstance(hiring, str):
                    company_name = hiring
                    hiring_url = None
                else:
                    company_name = ""
                    hiring_url = None

                company_name = company_name.strip() if company_name else "Unknown"

                source_id = None
                ident = entity.get("identifier")
                if isinstance(ident, dict):
                    source_id = ident.get("value") or ident.get("@id")
                elif isinstance(ident, str):
                    source_id = ident
                if not source_id:
                    source_id = entity.get("@id")
                source_job_id = str(source_id) if source_id else None

                canonical_url = _resolve_url(entity.get("url"), base_url) or ""
                company_domain = (
                    _extract_domain(hiring_url)
                    or _extract_domain(canonical_url)
                    or _extract_domain(base_url)
                )

                description_excerpt = _clean_html_text(str(entity.get("description", "")))

                job_loc = entity.get("jobLocation")
                if isinstance(job_loc, dict):
                    location_raw = _format_address(job_loc.get("address"))
                    lat, lng = _extract_coordinates(job_loc)
                elif isinstance(job_loc, str):
                    location_raw = job_loc.strip()
                    lat, lng = None, None
                elif isinstance(job_loc, list) and job_loc:
                    first = job_loc[0]
                    if isinstance(first, dict):
                        location_raw = _format_address(first.get("address"))
                        lat, lng = _extract_coordinates(first)
                    else:
                        location_raw = str(first).strip()
                        lat, lng = None, None
                else:
                    location_raw = ""
                    lat, lng = None, None

                # Work arrangement classification
                job_loc_type = str(entity.get("jobLocationType", "")).upper()
                desc_lower = description_excerpt.lower()
                title_lower = title.lower()
                if (
                    "TELECOMMUTE" in job_loc_type
                    or "remote" in job_loc_type.lower()
                    or "remote" in title_lower
                ):
                    work_arrangement = WorkArrangement.REMOTE
                elif "hybrid" in job_loc_type.lower() or "hybrid" in title_lower:
                    work_arrangement = WorkArrangement.HYBRID
                else:
                    work_arrangement = WorkArrangement.UNKNOWN

                # Publication state and posted_at confidence
                date_posted_str = entity.get("datePosted")
                posted_at = _parse_iso_datetime(date_posted_str)
                now = datetime.now(timezone.utc)

                if posted_at is not None:
                    posted_at_confidence = 1.0
                    age_days = (now - posted_at).days
                    if age_days <= 14:
                        pub_state = PublicationState.POSTED_RECENTLY
                    else:
                        pub_state = PublicationState.STALE
                else:
                    posted_at_confidence = 0.0
                    pub_state = PublicationState.OBSERVED_RECENTLY

                # Deterministic content hash
                hash_input = (
                    f"{company_name.lower()}|{title.lower()}|{canonical_url}|{location_raw.lower()}"
                )
                content_hash = hashlib.sha256(hash_input.encode("utf-8")).hexdigest()

                if content_hash in seen_job_hashes:
                    continue
                seen_job_hashes.add(content_hash)

                metadata = {}
                raw_type = entity.get("@type")
                if raw_type:
                    metadata["raw_type"] = raw_type
                if entity.get("employmentType"):
                    metadata["employment_type"] = entity.get("employmentType")

                job = TechnicalJobEvidence(
                    company_name=company_name,
                    title=title,
                    source_job_id=source_job_id,
                    canonical_url=canonical_url,
                    company_domain=company_domain,
                    description_excerpt=description_excerpt or None,
                    content_hash=content_hash,
                    location_raw=location_raw,
                    lat=lat,
                    lng=lng,
                    work_arrangement=work_arrangement,
                    publication_state=pub_state,
                    posted_at=posted_at,
                    posted_at_confidence=posted_at_confidence,
                    metadata=metadata,
                )
                jobs.append(job)

    return companies, jobs
