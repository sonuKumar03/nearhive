import json
from datetime import datetime, timezone
import pytest

from nearhive_discovery.contracts import (
    CONTRACT_VERSION,
    BatchRecordResult,
    BatchResult,
    CompanyEvidence,
    DiscoveryBatch,
    DiscoveryJob,
    DiscoverySourceRun,
    DiscoveryStatus,
    EvidenceBatch,
    PresenceType,
    PublicationState,
    TechnicalJobEvidence,
    WorkArrangement,
    batch_to_dict,
    batch_to_json,
)


def test_contract_constants_and_enums():
    assert CONTRACT_VERSION == 1

    # Presence values (3)
    assert PresenceType.CONFIRMED_OFFICE.value == "confirmed_office"
    assert PresenceType.PROBABLE_OFFICE.value == "probable_office"
    assert PresenceType.JOB_LOCATION_ONLY.value == "job_location_only"
    assert len(PresenceType) == 3

    # Work arrangements (4)
    assert WorkArrangement.IN_OFFICE.value == "in_office"
    assert WorkArrangement.HYBRID.value == "hybrid"
    assert WorkArrangement.REMOTE.value == "remote"
    assert WorkArrangement.UNKNOWN.value == "unknown"
    assert len(WorkArrangement) == 4

    # Publication states (3)
    assert PublicationState.POSTED_RECENTLY.value == "posted_recently"
    assert PublicationState.OBSERVED_RECENTLY.value == "observed_recently"
    assert PublicationState.STALE.value == "stale"
    assert len(PublicationState) == 3

    # Discovery statuses (6)
    assert DiscoveryStatus.PENDING.value == "pending"
    assert DiscoveryStatus.RUNNING.value == "running"
    assert DiscoveryStatus.COMPLETED.value == "completed"
    assert DiscoveryStatus.PARTIAL.value == "partial"
    assert DiscoveryStatus.FAILED.value == "failed"
    assert DiscoveryStatus.CANCELLED.value == "cancelled"
    assert len(DiscoveryStatus) == 6


def test_frozen_dataclasses():
    comp = CompanyEvidence(name="Acme Corp", domain="acme.com")
    with pytest.raises(Exception):
        comp.name = "Other Corp"  # type: ignore

    job = TechnicalJobEvidence(
        company_name="Acme Corp",
        title="Software Engineer",
        work_arrangement=WorkArrangement.HYBRID,
        first_seen_at=datetime.now(timezone.utc),
        last_seen_at=datetime.now(timezone.utc),
    )
    with pytest.raises(Exception):
        job.title = "Lead Engineer"  # type: ignore


def test_batch_serialization_matches_go_schema():
    now_utc = datetime(2026, 9, 27, 4, 0, 0, tzinfo=timezone.utc)
    posted_utc = datetime(2026, 9, 26, 12, 0, 0, tzinfo=timezone.utc)

    company = CompanyEvidence(
        source_record_id="rec-123",
        evidence_url="https://example.com/biz/123",
        name="Acme Robotics",
        domain="acmerobotics.io",
        address="100 Tech Blvd, Austin, TX",
        lat=30.2672,
        lng=-97.7431,
        phone="+15125550100",
        content_hash="abc123hash",
        metadata={"category": "robotics", "verified": True},
    )

    job_evidence = TechnicalJobEvidence(
        source_job_id="job-999",
        canonical_url="https://acmerobotics.io/careers/999",
        company_name="Acme Robotics",
        company_domain="acmerobotics.io",
        title="Senior Robotics Engineer",
        description_excerpt="Join our perception team working on ROS2.",
        content_hash="jobhash999",
        location_raw="Austin, TX (Hybrid)",
        lat=30.2672,
        lng=-97.7431,
        work_arrangement=WorkArrangement.HYBRID,
        publication_state=PublicationState.POSTED_RECENTLY,
        posted_at=posted_utc,
        posted_at_confidence=0.95,
        first_seen_at=now_utc,
        last_seen_at=now_utc,
        technical_classification="robotics_software",
        rule_version="v1.0",
        classification_reasons=["ros2", "perception"],
        metadata={"ats": "greenhouse"},
    )

    batch = EvidenceBatch(
        contract_version=CONTRACT_VERSION,
        discovery_job_id="11111111-1111-1111-1111-111111111111",
        source="google_maps",
        source_family="search_engine",
        observed_at=now_utc,
        companies=[company],
        jobs=[job_evidence],
    )

    # Test alias DiscoveryBatch == EvidenceBatch
    assert EvidenceBatch is DiscoveryBatch

    data = batch_to_dict(batch)
    assert data["contract_version"] == 1
    assert data["discovery_job_id"] == "11111111-1111-1111-1111-111111111111"
    assert data["source"] == "google_maps"
    assert data["source_family"] == "search_engine"
    assert data["observed_at"] == "2026-09-27T04:00:00Z"

    # Verify company field names match Go CompanyEvidence tags exactly
    c_data = data["companies"][0]
    assert c_data["source_record_id"] == "rec-123"
    assert c_data["evidence_url"] == "https://example.com/biz/123"
    assert c_data["name"] == "Acme Robotics"
    assert c_data["domain"] == "acmerobotics.io"
    assert c_data["address"] == "100 Tech Blvd, Austin, TX"
    assert c_data["lat"] == 30.2672
    assert c_data["lng"] == -97.7431
    assert c_data["phone"] == "+15125550100"
    assert c_data["content_hash"] == "abc123hash"
    assert c_data["metadata"] == {"category": "robotics", "verified": True}

    # Verify job field names match Go TechnicalJobEvidence tags exactly
    j_data = data["jobs"][0]
    assert j_data["source_job_id"] == "job-999"
    assert j_data["canonical_url"] == "https://acmerobotics.io/careers/999"
    assert j_data["company_name"] == "Acme Robotics"
    assert j_data["company_domain"] == "acmerobotics.io"
    assert j_data["title"] == "Senior Robotics Engineer"
    assert j_data["description_excerpt"] == "Join our perception team working on ROS2."
    assert j_data["content_hash"] == "jobhash999"
    assert j_data["location_raw"] == "Austin, TX (Hybrid)"
    assert j_data["lat"] == 30.2672
    assert j_data["lng"] == -97.7431
    assert j_data["work_arrangement"] == "hybrid"
    assert j_data["publication_state"] == "posted_recently"
    assert j_data["posted_at"] == "2026-09-26T12:00:00Z"
    assert j_data["posted_at_confidence"] == 0.95
    assert j_data["first_seen_at"] == "2026-09-27T04:00:00Z"
    assert j_data["last_seen_at"] == "2026-09-27T04:00:00Z"
    assert j_data["technical_classification"] == "robotics_software"
    assert j_data["rule_version"] == "v1.0"
    assert j_data["classification_reasons"] == ["ros2", "perception"]
    assert j_data["metadata"] == {"ats": "greenhouse"}

    # JSON round-trip
    json_str = batch_to_json(batch)
    parsed = json.loads(json_str)
    assert parsed == data


def test_batch_result_deserialization():
    raw_response = {
        "companies": [
            {"index": 0, "status": "accepted", "id": "uuid-c1"},
            {"index": 1, "status": "rejected", "error": "company name is required"},
        ],
        "jobs": [
            {"index": 0, "status": "accepted", "id": "uuid-j1"},
        ],
    }
    result = BatchResult.from_dict(raw_response)
    assert len(result.companies) == 2
    assert len(result.jobs) == 1
    assert result.accepted_companies == 1
    assert result.rejected_companies == 1
    assert result.accepted_jobs == 1
    assert result.rejected_jobs == 0
    assert result.total_accepted == 2
    assert result.total_rejected == 1
    assert result.companies[0].id == "uuid-c1"
    assert result.companies[1].error == "company name is required"
