import json
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import Enum
from typing import Any
from uuid import UUID

CONTRACT_VERSION = 1


class PresenceType(str, Enum):
    CONFIRMED_OFFICE = "confirmed_office"
    PROBABLE_OFFICE = "probable_office"
    JOB_LOCATION_ONLY = "job_location_only"


class WorkArrangement(str, Enum):
    IN_OFFICE = "in_office"
    HYBRID = "hybrid"
    REMOTE = "remote"
    UNKNOWN = "unknown"


class PublicationState(str, Enum):
    POSTED_RECENTLY = "posted_recently"
    OBSERVED_RECENTLY = "observed_recently"
    STALE = "stale"


class DiscoveryStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    PARTIAL = "partial"
    FAILED = "failed"
    CANCELLED = "cancelled"


def format_iso8601(dt: datetime | None) -> str | None:
    if dt is None:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    else:
        dt = dt.astimezone(timezone.utc)
    return dt.strftime("%Y-%m-%dT%H:%M:%SZ")


@dataclass(frozen=True)
class CompanyEvidence:
    name: str
    source_record_id: str | None = None
    evidence_url: str | None = None
    domain: str | None = None
    address: str = ""
    lat: float | None = None
    lng: float | None = None
    phone: str | None = None
    content_hash: str = ""
    metadata: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {
            "name": self.name,
            "address": self.address,
            "content_hash": self.content_hash,
            "metadata": dict(self.metadata),
        }
        if self.source_record_id is not None:
            d["source_record_id"] = self.source_record_id
        if self.evidence_url is not None:
            d["evidence_url"] = self.evidence_url
        if self.domain is not None:
            d["domain"] = self.domain
        if self.lat is not None:
            d["lat"] = self.lat
        if self.lng is not None:
            d["lng"] = self.lng
        if self.phone is not None:
            d["phone"] = self.phone
        return d


@dataclass(frozen=True)
class TechnicalJobEvidence:
    company_name: str
    title: str
    source_job_id: str | None = None
    canonical_url: str = ""
    company_domain: str | None = None
    description_excerpt: str | None = None
    content_hash: str = ""
    location_raw: str = ""
    lat: float | None = None
    lng: float | None = None
    work_arrangement: WorkArrangement = WorkArrangement.UNKNOWN
    publication_state: PublicationState = PublicationState.OBSERVED_RECENTLY
    posted_at: datetime | None = None
    posted_at_confidence: float = 0.0
    first_seen_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))
    last_seen_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))
    technical_classification: str = ""
    rule_version: str = "v1"
    classification_reasons: list[str] = field(default_factory=list)
    metadata: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {
            "company_name": self.company_name,
            "title": self.title,
            "canonical_url": self.canonical_url,
            "content_hash": self.content_hash,
            "location_raw": self.location_raw,
            "work_arrangement": self.work_arrangement.value
            if isinstance(self.work_arrangement, WorkArrangement)
            else str(self.work_arrangement),
            "publication_state": self.publication_state.value
            if isinstance(self.publication_state, PublicationState)
            else str(self.publication_state),
            "posted_at_confidence": self.posted_at_confidence,
            "first_seen_at": format_iso8601(self.first_seen_at),
            "last_seen_at": format_iso8601(self.last_seen_at),
            "technical_classification": self.technical_classification,
            "rule_version": self.rule_version,
            "classification_reasons": list(self.classification_reasons),
            "metadata": dict(self.metadata),
        }
        if self.source_job_id is not None:
            d["source_job_id"] = self.source_job_id
        if self.company_domain is not None:
            d["company_domain"] = self.company_domain
        if self.description_excerpt is not None:
            d["description_excerpt"] = self.description_excerpt
        if self.lat is not None:
            d["lat"] = self.lat
        if self.lng is not None:
            d["lng"] = self.lng
        if self.posted_at is not None:
            d["posted_at"] = format_iso8601(self.posted_at)
        return d


@dataclass(frozen=True)
class EvidenceBatch:
    contract_version: int
    discovery_job_id: str | UUID
    source: str
    source_family: str
    observed_at: datetime
    companies: list[CompanyEvidence] = field(default_factory=list)
    jobs: list[TechnicalJobEvidence] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        return {
            "contract_version": self.contract_version,
            "discovery_job_id": str(self.discovery_job_id),
            "source": self.source,
            "source_family": self.source_family,
            "observed_at": format_iso8601(self.observed_at),
            "companies": [c.to_dict() for c in self.companies],
            "jobs": [j.to_dict() for j in self.jobs],
        }


# Alias EvidenceBatch to DiscoveryBatch so both names can be used interchangeably
DiscoveryBatch = EvidenceBatch


def batch_to_dict(batch: EvidenceBatch) -> dict[str, Any]:
    return batch.to_dict()


def batch_to_json(batch: EvidenceBatch) -> str:
    return json.dumps(batch_to_dict(batch))


@dataclass(frozen=True)
class BatchRecordResult:
    index: int
    status: str
    id: str | None = None
    error: str | None = None

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "BatchRecordResult":
        return cls(
            index=data.get("index", 0),
            status=data.get("status", "unknown"),
            id=data.get("id"),
            error=data.get("error"),
        )


@dataclass(frozen=True)
class BatchResult:
    companies: list[BatchRecordResult] = field(default_factory=list)
    jobs: list[BatchRecordResult] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "BatchResult":
        companies = [BatchRecordResult.from_dict(item) for item in data.get("companies", [])]
        jobs = [BatchRecordResult.from_dict(item) for item in data.get("jobs", [])]
        return cls(companies=companies, jobs=jobs)

    @property
    def accepted_companies(self) -> int:
        return sum(1 for c in self.companies if c.status == "accepted")

    @property
    def rejected_companies(self) -> int:
        return sum(1 for c in self.companies if c.status == "rejected")

    @property
    def accepted_jobs(self) -> int:
        return sum(1 for j in self.jobs if j.status == "accepted")

    @property
    def rejected_jobs(self) -> int:
        return sum(1 for j in self.jobs if j.status == "rejected")

    @property
    def total_accepted(self) -> int:
        return self.accepted_companies + self.accepted_jobs

    @property
    def total_rejected(self) -> int:
        return self.rejected_companies + self.rejected_jobs


@dataclass(frozen=True)
class DiscoveryJob:
    id: UUID | str
    user_id: UUID | str
    status: DiscoveryStatus
    lat: float
    lng: float
    radius_km: float
    worker_id: str | None = None
    lease_expires_at: datetime | None = None
    last_heartbeat_at: datetime | None = None
    attempts: int = 0
    max_attempts: int = 3
    error: str | None = None
    company_count: int = 0
    job_count: int = 0
    evidence_count: int = 0
    started_at: datetime | None = None
    finished_at: datetime | None = None
    created_at: datetime | None = None
    updated_at: datetime | None = None


@dataclass(frozen=True)
class DiscoverySourceRun:
    discovery_job_id: UUID | str
    source: str
    source_family: str
    id: UUID | str | None = None
    status: DiscoveryStatus = DiscoveryStatus.PENDING
    attempts: int = 0
    company_count: int = 0
    job_count: int = 0
    evidence_count: int = 0
    error: str | None = None
    duration_ms: int = 0
    started_at: datetime | None = None
    finished_at: datetime | None = None
    created_at: datetime | None = None
    updated_at: datetime | None = None
