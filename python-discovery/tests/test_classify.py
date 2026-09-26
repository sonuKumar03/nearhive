from datetime import datetime, timedelta, timezone
import pytest

from nearhive_discovery.classify import (
    RULE_VERSION,
    Classification,
    classify_arrangement,
    classify_technical_role,
    publication_state,
)
from nearhive_discovery.contracts import PublicationState, WorkArrangement


class TestClassifyTechnicalRole:
    """Step 1: Classification-table tests for technical and non-technical roles."""

    @pytest.mark.parametrize(
        "title,categories,expected_category",
        [
            ("Software Engineer", [], "software"),
            ("Senior Backend Developer", ["Engineering"], "software"),
            ("Frontend Engineer", [], "software"),
            ("Full Stack Developer", [], "software"),
            ("Mobile Engineer (iOS/Android)", [], "software"),
            ("Embedded Systems Engineer", [], "software"),
            ("Firmware Developer", [], "software"),
            ("Data Engineer", [], "data"),
            ("Senior Data Scientist", ["Data"], "data"),
            ("Machine Learning Engineer", [], "data"),
            ("AI Research Scientist", [], "data"),
            ("Database Administrator (DBA)", [], "data"),
            ("Analytics Engineer", [], "data"),
            ("DevOps Engineer", ["Infrastructure"], "infrastructure"),
            ("Site Reliability Engineer (SRE)", [], "infrastructure"),
            ("Cloud Infrastructure Architect", [], "infrastructure"),
            ("Platform Engineer", [], "infrastructure"),
            ("Systems Administrator", [], "infrastructure"),
            ("Security Engineer", ["Security"], "security"),
            ("Information Security Analyst", [], "security"),
            ("Application Security Engineer", [], "security"),
            ("Cybersecurity Specialist", [], "security"),
            ("QA Automation Engineer", ["Quality"], "qa_automation"),
            ("SDET (Software Development Engineer in Test)", [], "qa_automation"),
            ("Quality Assurance Engineer", [], "qa_automation"),
            ("Test Automation Engineer", [], "qa_automation"),
            ("Product Engineer", [], "product_engineering"),
            ("Solutions Architect", ["Pre-Sales Engineering"], "product_engineering"),
            ("Technical Product Manager", [], "product_engineering"),
            ("Technical Support Engineer", ["Support"], "technical_support"),
            ("Customer Support Engineer", [], "technical_support"),
            ("Tier 3 Support Specialist", [], "technical_support"),
            ("Engineering Manager", ["Engineering"], "engineering_leadership"),
            ("VP of Engineering", [], "engineering_leadership"),
            ("Director of Software Engineering", [], "engineering_leadership"),
            ("Chief Technology Officer", [], "engineering_leadership"),
            ("CTO", [], "engineering_leadership"),
            ("Head of Engineering", [], "engineering_leadership"),
            ("Tech Lead", [], "engineering_leadership"),
        ],
    )
    def test_positive_technical_roles(
        self, title: str, categories: list[str], expected_category: str
    ) -> None:
        result = classify_technical_role(title, categories)
        assert isinstance(result, Classification)
        assert result.is_technical is True
        assert bool(result) is True
        assert result.classification == expected_category
        assert result.rule_version == RULE_VERSION
        assert RULE_VERSION == "technical-title-v1"
        assert len(result.reasons) > 0

    @pytest.mark.parametrize(
        "title,categories,expected_negative_reason",
        [
            # Sales & Marketing
            ("Account Executive", ["Sales"], "sales"),
            ("Sales Representative", [], "sales"),
            ("Enterprise Sales Director", [], "sales"),
            ("Business Development Manager", [], "sales"),
            ("Marketing Coordinator", ["Marketing"], "marketing"),
            ("Content Writer", [], "marketing"),
            # Recruiting & HR
            ("Technical Recruiter", ["People"], "recruiting"),
            ("Senior IT Recruiter", [], "recruiting"),
            ("Engineering Recruiter", [], "recruiting"),
            ("Talent Acquisition Specialist", [], "recruiting"),
            ("Head of People Operations", [], "hr"),
            ("HR Business Partner", [], "hr"),
            # Accounting & Finance
            ("Staff Accountant", ["Finance"], "accounting"),
            ("Senior Financial Analyst", [], "finance"),
            ("Corporate Controller", [], "accounting"),
            ("Payroll Specialist", [], "accounting"),
            # Ambiguous non-technical "engineer" usage
            ("Sanitation Engineer", [], "non_technical_engineer"),
            ("Locomotive Engineer", [], "non_technical_engineer"),
            ("Train Engineer", [], "non_technical_engineer"),
            ("Stationary Engineer", [], "non_technical_engineer"),
            ("Building Engineer", [], "non_technical_engineer"),
            ("Custodial Engineer", [], "non_technical_engineer"),
            ("Civil Engineer", [], "non_technical_engineer"),
            ("Flight Engineer", [], "non_technical_engineer"),
            # Administrative / Operations
            ("Office Manager", [], "administrative"),
            ("Executive Assistant", [], "administrative"),
            ("Legal Counsel", [], "legal"),
        ],
    )
    def test_negative_non_technical_roles(
        self, title: str, categories: list[str], expected_negative_reason: str
    ) -> None:
        result = classify_technical_role(title, categories)
        assert isinstance(result, Classification)
        assert result.is_technical is False
        assert bool(result) is False
        assert result.rule_version == RULE_VERSION
        assert len(result.reasons) > 0


class TestClassifyArrangement:
    """Step 2: Tests for work arrangement classification."""

    def test_structured_takes_precedence_over_free_text(self) -> None:
        # Structured says remote, text says on-site
        res = classify_arrangement(
            {"workplace_type": "remote"},
            text="Position requires working on-site in Austin 5 days a week.",
        )
        assert res == WorkArrangement.REMOTE

        # Structured says hybrid, text says remote
        res = classify_arrangement(
            {"workplaceType": "hybrid"},
            text="100% remote anywhere in the US.",
        )
        assert res == WorkArrangement.HYBRID

        # Structured says on-site, text says remote
        res = classify_arrangement(
            {"workplace_type": "in_office"},
            text="Remote work options available.",
        )
        assert res == WorkArrangement.IN_OFFICE

        # Schema.org telecommute
        res = classify_arrangement(
            {"jobLocationType": "TELECOMMUTE"},
            text="Office in Seattle.",
        )
        assert res == WorkArrangement.REMOTE

    def test_free_text_fallback_when_structured_is_absent_or_unspecified(self) -> None:
        assert (
            classify_arrangement({}, "This is a fully remote role open to US residents.")
            == WorkArrangement.REMOTE
        )
        assert (
            classify_arrangement(
                {"workplace_type": "unspecified"},
                "Hybrid role requiring 2 days in office and 3 days work from home.",
            )
            == WorkArrangement.HYBRID
        )
        assert (
            classify_arrangement(
                None,  # type: ignore
                "On-site role based at our headquarters in New York.",
            )
            == WorkArrangement.IN_OFFICE
        )

    def test_conflicting_evidence_returns_unknown(self) -> None:
        # Conflicting structured flags
        res = classify_arrangement({"workplace_type": "in_office", "remote": True}, "")
        assert res == WorkArrangement.UNKNOWN

        # Conflicting text with no resolution
        res = classify_arrangement(
            {}, "We offer fully remote work but this job is strictly on-site in Austin."
        )
        assert res == WorkArrangement.UNKNOWN

    def test_absent_evidence_returns_unknown(self) -> None:
        assert classify_arrangement({}, "") == WorkArrangement.UNKNOWN
        assert (
            classify_arrangement({}, "San Francisco, CA. Competitive compensation.")
            == WorkArrangement.UNKNOWN
        )
        assert (
            classify_arrangement({"workplace_type": "unknown"}, "")
            == WorkArrangement.UNKNOWN
        )


class TestPublicationState:
    """Step 2: Tests for publication recency and staleness."""

    def test_posted_recently_within_14_days(self) -> None:
        now = datetime(2026, 9, 27, 12, 0, 0, tzinfo=timezone.utc)
        posted_at = now - timedelta(days=5)
        first_seen = now - timedelta(days=2)

        state = publication_state(posted_at, first_seen, now=now)
        assert state == PublicationState.POSTED_RECENTLY

    def test_posted_recently_at_exactly_14_days_boundary(self) -> None:
        now = datetime(2026, 9, 27, 12, 0, 0, tzinfo=timezone.utc)
        posted_at = now - timedelta(days=14)

        state = publication_state(posted_at, None, now=now)
        assert state == PublicationState.POSTED_RECENTLY

    def test_stale_when_posted_older_than_14_days(self) -> None:
        now = datetime(2026, 9, 27, 12, 0, 0, tzinfo=timezone.utc)
        posted_at = now - timedelta(days=14, seconds=1)

        state = publication_state(posted_at, now, now=now)
        assert state == PublicationState.STALE

    def test_observed_recently_when_publication_time_missing(self) -> None:
        now = datetime(2026, 9, 27, 12, 0, 0, tzinfo=timezone.utc)
        first_seen = now - timedelta(days=3)

        state = publication_state(None, first_seen, now=now)
        assert state == PublicationState.OBSERVED_RECENTLY

    def test_stale_when_observation_older_than_14_days_and_publication_missing(self) -> None:
        now = datetime(2026, 9, 27, 12, 0, 0, tzinfo=timezone.utc)
        first_seen = now - timedelta(days=20)

        state = publication_state(None, first_seen, now=now)
        assert state == PublicationState.STALE

    def test_untrusted_publication_time_treated_as_observed_recently(self) -> None:
        now = datetime(2026, 9, 27, 12, 0, 0, tzinfo=timezone.utc)
        untrusted_posted = now - timedelta(days=1)
        first_seen = now - timedelta(days=1)

        state = publication_state(
            untrusted_posted, first_seen, now=now, posted_at_trusted=False
        )
        assert state == PublicationState.OBSERVED_RECENTLY
