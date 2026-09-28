"""Canonical PostgreSQL writes for discovered company and technical-job evidence."""

from __future__ import annotations

import hashlib
import json
import math
import re
from datetime import datetime, timedelta, timezone
from typing import Any

import psycopg

from nearhive_discovery.contracts import BatchRecordResult, BatchResult, EvidenceBatch
from nearhive_discovery.http import validate_public_url
from psycopg.types.json import Jsonb

MAX_BATCH_RECORDS = 500
MAX_DESCRIPTION_EXCERPT = 2_000
MAX_METADATA_BYTES = 64 * 1024
VALID_WORK_ARRANGEMENTS = {"in_office", "hybrid", "remote", "unknown"}
VALID_PUBLICATION_STATES = {"posted_recently", "observed_recently", "stale"}
JOB_STATE_BY_PUBLICATION = {
    "posted_recently": "open",
    "observed_recently": "open",
    "stale": "stale",
}


def _normalized_name(name: str) -> str:
    return re.sub(r"\s+", " ", re.sub(r"[^\w]+", " ", name.casefold())).strip()


def _content_hash(*parts: str) -> str:
    return hashlib.sha256("\0".join(parts).encode()).hexdigest()


def _validate_coordinates(lat: float | None, lng: float | None, label: str) -> None:
    if (lat is None) != (lng is None):
        raise ValueError(f"{label}: latitude and longitude must be provided together")
    if lat is not None and lng is not None and (
        not math.isfinite(lat)
        or not math.isfinite(lng)
        or not -90 <= lat <= 90
        or not -180 <= lng <= 180
    ):
        raise ValueError(f"{label}: invalid coordinates")


def _validate_url(url: str, label: str) -> None:
    if url:
        try:
            validate_public_url(url, resolve_dns=False)
        except ValueError as exc:
            raise ValueError(f"{label}: unsafe URL") from exc


def _validate_timestamp(value: datetime, label: str, now: datetime) -> None:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError(f"{label}: timestamp must include a timezone")
    if value > now + timedelta(minutes=5):
        raise ValueError(f"{label}: timestamp cannot be in the future")


def _validate_metadata(metadata: dict[str, Any], label: str) -> None:
    try:
        size = len(json.dumps(metadata, ensure_ascii=False, separators=(",", ":")).encode())
    except (TypeError, ValueError) as exc:
        raise ValueError(f"{label}: metadata must be JSON serializable") from exc
    if size > MAX_METADATA_BYTES:
        raise ValueError(f"{label}: metadata exceeds size limit")


class PostgresPersistence:
    def __init__(self, database_url: str) -> None:
        self.database_url = database_url
        self.conn: psycopg.Connection[Any] | None = None

    def __enter__(self) -> PostgresPersistence:
        self.conn = psycopg.connect(self.database_url)
        return self

    def __exit__(self, *_: object) -> None:
        if self.conn is not None:
            self.conn.close()
            self.conn = None

    @staticmethod
    def validate(batch: EvidenceBatch) -> None:
        if batch.contract_version != 1:
            raise ValueError("unsupported evidence contract version")
        if not batch.source.strip() or len(batch.source) > 100:
            raise ValueError("source is required and must be at most 100 characters")
        if not batch.source_family.strip() or len(batch.source_family) > 100:
            raise ValueError("source_family is required and must be at most 100 characters")
        if len(batch.companies) + len(batch.jobs) > MAX_BATCH_RECORDS:
            raise ValueError("evidence batch exceeds record limit")

        now = datetime.now(timezone.utc)
        _validate_timestamp(batch.observed_at, "observed_at", now)
        for company in batch.companies:
            if not company.name.strip() or len(company.name) > 500:
                raise ValueError("company name is required and must be at most 500 characters")
            if not _normalized_name(company.name):
                raise ValueError("company name must contain letters or numbers")
            _validate_coordinates(company.lat, company.lng, "company")
            _validate_url(company.evidence_url or "", "company evidence")
            _validate_metadata(company.metadata, "company")
        for job in batch.jobs:
            if not job.company_name.strip() or len(job.company_name) > 500:
                raise ValueError("job company name is required and must be at most 500 characters")
            if not job.title.strip() or len(job.title) > 500:
                raise ValueError("job title is required and must be at most 500 characters")
            if job.work_arrangement not in VALID_WORK_ARRANGEMENTS:
                raise ValueError("job has invalid work arrangement")
            if job.publication_state not in VALID_PUBLICATION_STATES:
                raise ValueError("job has invalid publication state")
            if job.posted_at is not None:
                _validate_timestamp(job.posted_at, "posted_at", now)
            _validate_timestamp(job.first_seen_at, "first_seen_at", now)
            _validate_timestamp(job.last_seen_at, "last_seen_at", now)
            _validate_coordinates(job.lat, job.lng, "job")
            if job.work_arrangement == "remote" and job.lat is not None:
                raise ValueError("remote job cannot have spatial coordinates")
            _validate_url(job.canonical_url, "job")
            if job.description_excerpt and len(job.description_excerpt) > MAX_DESCRIPTION_EXCERPT:
                raise ValueError("job description excerpt exceeds size limit")
            _validate_metadata(job.metadata, "job")

    def persist(self, batch: EvidenceBatch) -> BatchResult:
        if self.conn is None:
            raise RuntimeError("PostgresPersistence must be used as a context manager")
        self.validate(batch)
        with self.conn.transaction():
            data_source_id = self._register_source(batch.source, batch.source_family)
            ingestion_run_id = self._upsert_ingestion_run(
                data_source_id, self._discovery_job_id(batch.discovery_job_id)
            )
            company_results: list[BatchRecordResult] = []
            job_results: list[BatchRecordResult] = []
            for index, evidence in enumerate(batch.companies):
                company_id = self._upsert_company(evidence.name, evidence.domain)
                location_id = self._upsert_company_location(
                    company_id, evidence.address, evidence.lat, evidence.lng
                )
                self._upsert_sighting(batch, evidence, company_id, location_id)
                payload: dict[str, Any] = {"name": evidence.name, "address": evidence.address}
                if evidence.domain is not None:
                    payload["domain"] = evidence.domain
                if evidence.lat is not None:
                    payload["lat"] = evidence.lat
                if evidence.lng is not None:
                    payload["lng"] = evidence.lng
                if evidence.phone is not None:
                    payload["phone"] = evidence.phone
                record_id = self._upsert_source_record(
                    data_source_id,
                    ingestion_run_id,
                    "company",
                    evidence.source_record_id or None,
                    evidence.evidence_url,
                    payload,
                )
                self._link_company_record(company_id, record_id)
                company_results.append(BatchRecordResult(index=index, status="accepted"))
            for index, evidence in enumerate(batch.jobs):
                company_id = self._upsert_company(evidence.company_name, evidence.company_domain)
                job_id = self._upsert_job(batch, evidence, company_id)
                record_id = self._upsert_source_record(
                    data_source_id,
                    ingestion_run_id,
                    "job",
                    evidence.source_job_id or None,
                    evidence.canonical_url or None,
                    evidence.to_dict(),
                )
                self._link_job_record(job_id, record_id)
                job_results.append(BatchRecordResult(index=index, status="accepted"))
            self._complete_ingestion_run(
                ingestion_run_id, len(company_results) + len(job_results)
            )
        return BatchResult(companies=company_results, jobs=job_results)

    def _upsert_company(self, name: str, domain: str | None) -> str:
        assert self.conn is not None
        normalized = _normalized_name(name)
        clean_domain = (domain or "").strip().lower().removeprefix("www.") or None
        keys = sorted({normalized, clean_domain or ""} - {""})
        with self.conn.cursor() as cur:
            for key in keys:
                cur.execute("SELECT pg_advisory_xact_lock(hashtextextended(%s, 0))", (key,))
            if clean_domain:
                cur.execute(
                    """SELECT id FROM companies
                       WHERE lower(domain) = %s OR (normalized_name = %s AND domain IS NULL)
                       ORDER BY (lower(domain) = %s) DESC NULLS LAST LIMIT 1""",
                    (clean_domain, normalized, clean_domain),
                )
            else:
                cur.execute(
                    "SELECT id FROM companies WHERE normalized_name = %s LIMIT 1",
                    (normalized,),
                )
            row = cur.fetchone()
            if row:
                company_id = str(row[0])
                cur.execute(
                    "UPDATE companies SET domain = COALESCE(domain, %s), updated_at = NOW() WHERE id = %s",
                    (clean_domain, company_id),
                )
            else:
                cur.execute(
                    """INSERT INTO companies (name, normalized_name, domain)
                       VALUES (%s, %s, %s) RETURNING id""",
                    (name.strip(), normalized, clean_domain),
                )
                company_id = str(cur.fetchone()[0])
                cur.execute(
                    """INSERT INTO company_aliases (company_id, alias, normalized_alias, alias_type)
                       VALUES (%s, %s, %s, 'source_name')
                       ON CONFLICT (company_id, normalized_alias, alias_type) DO NOTHING""",
                    (company_id, name.strip(), normalized),
                )
            if clean_domain:
                cur.execute(
                    """INSERT INTO company_domains (company_id, normalized_domain, domain_type, is_primary)
                       VALUES (%s, %s, 'primary', true)
                       ON CONFLICT (company_id, normalized_domain) DO UPDATE SET
                           is_primary = EXCLUDED.is_primary, last_seen_at = EXCLUDED.last_seen_at""",
                    (company_id, clean_domain),
                )
            return company_id

    def _upsert_company_location(
        self, company_id: str, address: str, lat: float | None, lng: float | None
    ) -> str | None:
        if lat is None or lng is None:
            return None
        assert self.conn is not None
        address_hash = _content_hash(re.sub(r"\s+", " ", address.strip()))
        point = "ST_SetSRID(ST_MakePoint(%s, %s), 4326)::geography"
        with self.conn.cursor() as cur:
            cur.execute(
                "SELECT id FROM locations WHERE company_id = %s AND address_hash = %s LIMIT 1",
                (company_id, address_hash),
            )
            row = cur.fetchone()
            if row:
                location_id = str(row[0])
                cur.execute(
                    "UPDATE locations SET last_seen_at = NOW() WHERE id = %s",
                    (location_id,),
                )
                return location_id
            cur.execute(
                f"""SELECT id FROM locations WHERE company_id = %s
                    AND presence_type <> 'job_location_only'
                    AND ST_DWithin(coords, {point}, 500)
                    ORDER BY verified DESC, confidence DESC LIMIT 1""",
                (company_id, lng, lat),
            )
            row = cur.fetchone()
            if row:
                location_id = str(row[0])
                cur.execute(
                    "UPDATE locations SET last_seen_at = NOW() WHERE id = %s",
                    (location_id,),
                )
                return location_id
            cur.execute(
                f"""INSERT INTO locations (company_id, address, coords, presence_type,
                        location_type, status, country_code, address_hash)
                    VALUES (%s, %s, {point}, 'probable_office', 'office', 'unverified', 'IN', %s)
                    RETURNING id""",
                (company_id, address.strip(), lng, lat, address_hash),
            )
            return str(cur.fetchone()[0])

    def _upsert_sighting(
        self, batch: EvidenceBatch, evidence: Any, company_id: str, location_id: str | None
    ) -> None:
        assert self.conn is not None
        record_id = evidence.source_record_id
        digest = evidence.content_hash or _content_hash(evidence.name, evidence.address, evidence.domain or "")
        source_url = evidence.evidence_url
        lat, lng = evidence.lat, evidence.lng
        metadata = dict(evidence.metadata)
        if evidence.domain:
            metadata["domain"] = evidence.domain
        if evidence.phone:
            metadata["phone"] = evidence.phone
        conflict = (
            "(source, source_record_id) WHERE source_record_id IS NOT NULL"
            if record_id
            else "(source, content_hash) WHERE content_hash IS NOT NULL AND source_record_id IS NULL"
        )
        discovery_job_id = self._discovery_job_id(batch.discovery_job_id)
        with self.conn.cursor() as cur:
            cur.execute(
                f"""INSERT INTO sightings (
                    source, source_family, source_record_id, content_hash, discovery_job_id,
                    source_url, company_name, raw_address, lat, lng, metadata, company_id,
                    location_id, first_seen_at, last_seen_at, scraped_at
                ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                ON CONFLICT {conflict} DO UPDATE SET
                    content_hash = EXCLUDED.content_hash, discovery_job_id = EXCLUDED.discovery_job_id,
                    source_url = EXCLUDED.source_url, company_name = EXCLUDED.company_name,
                    raw_address = EXCLUDED.raw_address, lat = EXCLUDED.lat, lng = EXCLUDED.lng,
                    metadata = EXCLUDED.metadata, company_id = EXCLUDED.company_id,
                    location_id = EXCLUDED.location_id, last_seen_at = EXCLUDED.last_seen_at,
                    scraped_at = EXCLUDED.scraped_at""",
                (
                    batch.source, batch.source_family, record_id, digest, discovery_job_id,
                    source_url, evidence.name, evidence.address, lat, lng, Jsonb(metadata),
                    company_id, location_id, batch.observed_at, batch.observed_at, batch.observed_at,
                ),
            )

    def _upsert_job(self, batch: EvidenceBatch, evidence: Any, company_id: str) -> str:
        assert self.conn is not None
        source_job_id = evidence.source_job_id
        digest = evidence.content_hash or _content_hash(
            evidence.company_name, evidence.title, evidence.canonical_url
        )
        conflict = (
            "(source, source_job_id) WHERE source_job_id IS NOT NULL"
            if source_job_id
            else "(source, content_hash) WHERE content_hash IS NOT NULL AND source_job_id IS NULL"
        )
        with self.conn.cursor() as cur:
            publication_state = getattr(
                evidence.publication_state, "value", str(evidence.publication_state)
            )
            cur.execute(
                f"""INSERT INTO technical_job_postings (
                    company_id, discovery_job_id, source, source_family, source_job_id,
                    canonical_url, title, normalized_title, description_excerpt, content_hash,
                    work_arrangement, publication_state, posted_at, posted_at_confidence,
                    first_seen_at, last_seen_at, technical_classification, rule_version,
                    classification_reasons, metadata,
                    role_family_id, seniority, employment_type, state, activity_confidence
                ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s,
                    %s, %s, %s, %s,
                    (SELECT id FROM role_families WHERE slug = 'engineering'),
                    %s, %s, %s, %s)
                ON CONFLICT {conflict} DO UPDATE SET
                    company_id = EXCLUDED.company_id, discovery_job_id = EXCLUDED.discovery_job_id,
                    source_family = EXCLUDED.source_family, canonical_url = EXCLUDED.canonical_url,
                    title = EXCLUDED.title, normalized_title = EXCLUDED.normalized_title,
                    description_excerpt = EXCLUDED.description_excerpt, content_hash = EXCLUDED.content_hash,
                    work_arrangement = EXCLUDED.work_arrangement,
                    publication_state = EXCLUDED.publication_state, posted_at = EXCLUDED.posted_at,
                    posted_at_confidence = EXCLUDED.posted_at_confidence,
                    last_seen_at = EXCLUDED.last_seen_at, technical_classification = EXCLUDED.technical_classification,
                    rule_version = EXCLUDED.rule_version, classification_reasons = EXCLUDED.classification_reasons,
                    metadata = EXCLUDED.metadata,
                    role_family_id = EXCLUDED.role_family_id, seniority = EXCLUDED.seniority,
                    employment_type = EXCLUDED.employment_type, state = EXCLUDED.state,
                    activity_confidence = EXCLUDED.activity_confidence
                RETURNING id""",
                (
                    company_id, self._discovery_job_id(batch.discovery_job_id), batch.source, batch.source_family,
                    source_job_id, evidence.canonical_url or None, evidence.title.strip(),
                    evidence.title.strip().casefold(), evidence.description_excerpt, digest,
                    getattr(evidence.work_arrangement, "value", str(evidence.work_arrangement)),
                    publication_state, evidence.posted_at,
                    evidence.posted_at_confidence, evidence.first_seen_at, evidence.last_seen_at,
                    evidence.technical_classification or "software_engineering", evidence.rule_version,
                    evidence.classification_reasons, Jsonb(evidence.metadata),
                    "unknown", "unknown",
                    JOB_STATE_BY_PUBLICATION.get(publication_state, "unknown"),
                    evidence.posted_at_confidence,
                ),
            )
            job_id = cur.fetchone()[0]
            if evidence.location_raw or evidence.lat is not None:
                cur.execute(
                    """INSERT INTO job_locations (
                        job_id, ordinal, location_raw, latitude, longitude,
                        coordinate_source, confidence, location_kind, country_code,
                        first_seen_at, last_seen_at
                    ) VALUES (%s, 0, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (job_id, ordinal) DO UPDATE SET
                        location_raw = EXCLUDED.location_raw,
                        latitude = CASE
                            WHEN job_locations.coordinate_source IN ('structured', 'provider')
                            THEN job_locations.latitude ELSE EXCLUDED.latitude END,
                        longitude = CASE
                            WHEN job_locations.coordinate_source IN ('structured', 'provider')
                            THEN job_locations.longitude ELSE EXCLUDED.longitude END,
                        coordinate_source = CASE
                            WHEN job_locations.coordinate_source IN ('structured', 'provider')
                            THEN job_locations.coordinate_source ELSE EXCLUDED.coordinate_source END,
                        confidence = CASE
                            WHEN job_locations.coordinate_source IN ('structured', 'provider')
                            THEN job_locations.confidence ELSE EXCLUDED.confidence END,
                        location_kind = EXCLUDED.location_kind,
                        country_code = EXCLUDED.country_code,
                        last_seen_at = EXCLUDED.last_seen_at""",
                    (
                        job_id, evidence.location_raw or "", evidence.lat, evidence.lng,
                        "provider" if evidence.lat is not None else "unknown",
                        1.0 if evidence.lat is not None else 0.0,
                        "stated_job_location" if evidence.location_raw else "inferred",
                        "IN",
                        evidence.first_seen_at, evidence.last_seen_at,
                    ),
                )
            else:
                cur.execute("DELETE FROM job_locations WHERE job_id = %s", (job_id,))
            return str(job_id)

    def _register_source(self, source: str, source_family: str) -> str:
        assert self.conn is not None
        source_type = {"official_site": "official_site", "job_ats": "official_ats"}.get(
            source_family, "open_dataset"
        )
        with self.conn.cursor() as cur:
            cur.execute(
                """INSERT INTO data_sources (slug, name, source_type, source_family)
                   VALUES (%s, %s, %s, %s) ON CONFLICT (slug) DO NOTHING""",
                (source, source, source_type, source_family),
            )
            cur.execute("SELECT id FROM data_sources WHERE slug = %s", (source,))
            return str(cur.fetchone()[0])

    def _upsert_ingestion_run(
        self, data_source_id: str, discovery_job_id: str | None, adapter_version: str = "v1"
    ) -> str:
        assert self.conn is not None
        with self.conn.cursor() as cur:
            cur.execute(
                """INSERT INTO ingestion_runs (data_source_id, discovery_job_id, adapter_version, status)
                   VALUES (%s, %s, %s, 'running') RETURNING id""",
                (data_source_id, discovery_job_id, adapter_version),
            )
            return str(cur.fetchone()[0])

    def _complete_ingestion_run(self, ingestion_run_id: str, accepted: int, rejected: int = 0) -> None:
        assert self.conn is not None
        with self.conn.cursor() as cur:
            cur.execute(
                """UPDATE ingestion_runs
                   SET accepted_count = %s, rejected_count = %s, status = 'completed',
                       finished_at = NOW(), updated_at = NOW()
                   WHERE id = %s""",
                (accepted, rejected, ingestion_run_id),
            )

    def _upsert_source_record(
        self,
        data_source_id: str,
        ingestion_run_id: str,
        record_type: str,
        external_id: str | None,
        source_url: str | None,
        payload: dict,
    ) -> str:
        assert self.conn is not None
        payload_hash = _content_hash(json.dumps(payload, sort_keys=True))
        if external_id is not None:
            conflict = "(data_source_id, record_type, external_id) WHERE external_id IS NOT NULL"
            external_slot = "%s"
            params: tuple[Any, ...] = (
                data_source_id, ingestion_run_id, record_type, external_id, source_url,
            )
        else:
            conflict = "(data_source_id, record_type, normalized_payload_hash) WHERE external_id IS NULL"
            external_slot = "NULL"
            params = (data_source_id, ingestion_run_id, record_type, source_url)
        with self.conn.cursor() as cur:
            cur.execute(
                f"""INSERT INTO source_records (
                        data_source_id, ingestion_run_id, record_type, external_id,
                        source_url, normalized_payload, normalized_payload_hash, last_seen_at
                    ) VALUES (%s, %s, %s, {external_slot}, %s, %s, %s, clock_timestamp())
                    ON CONFLICT {conflict} DO UPDATE SET
                        last_seen_at = EXCLUDED.last_seen_at,
                        normalized_payload = EXCLUDED.normalized_payload
                    RETURNING id""",
                (*params, Jsonb(payload), payload_hash),
            )
            return str(cur.fetchone()[0])

    def _link_company_record(self, company_id: str, source_record_id: str) -> None:
        assert self.conn is not None
        with self.conn.cursor() as cur:
            cur.execute(
                """INSERT INTO company_source_links (company_id, source_record_id, relation, confidence)
                   VALUES (%s, %s, 'evidence', 1.0)
                   ON CONFLICT (company_id, source_record_id) DO UPDATE SET
                       last_seen_at = EXCLUDED.last_seen_at""",
                (company_id, source_record_id),
            )

    def _link_job_record(self, job_id: str, source_record_id: str) -> None:
        assert self.conn is not None
        with self.conn.cursor() as cur:
            cur.execute(
                """INSERT INTO job_source_links (job_id, source_record_id, relation, confidence)
                   VALUES (%s, %s, 'evidence', 1.0)
                   ON CONFLICT (job_id, source_record_id) DO UPDATE SET
                       last_seen_at = EXCLUDED.last_seen_at""",
                (job_id, source_record_id),
            )

    def _discovery_job_id(self, job_id: str) -> str | None:
        assert self.conn is not None
        with self.conn.cursor() as cur:
            cur.execute("SELECT id FROM discovery_jobs WHERE id = %s", (str(job_id),))
            row = cur.fetchone()
            return str(row[0]) if row else None
