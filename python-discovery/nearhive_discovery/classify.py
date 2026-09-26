from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
import hashlib
import html
import re

from nearhive_discovery.contracts import PublicationState, WorkArrangement

RULE_VERSION = "technical-title-v1"


@dataclass(frozen=True)
class Classification:
    is_technical: bool
    classification: str
    rule_version: str
    reasons: list[str] = field(default_factory=list)

    def __bool__(self) -> bool:
        return self.is_technical


# Regex patterns for negative exclusions (checked first)
_NEGATIVE_RULES: list[tuple[str, re.Pattern[str]]] = [
    # Non-technical "engineer" usages
    (
        "non_technical_engineer",
        re.compile(
            r"\b(sanitation\s+engineer|locomotive\s+engineer|train\s+engineer|"
            r"stationary\s+engineer|building\s+engineer|custodial\s+engineer|"
            r"civil\s+engineer|flight\s+engineer|sound\s+engineer|audio\s+engineer|"
            r"marine\s+engineer|structural\s+engineer|mining\s+engineer)\b",
            re.IGNORECASE,
        ),
    ),
    # Recruiting and Talent Acquisition
    (
        "recruiting",
        re.compile(
            r"\b(technical\s+recruiter|it\s+recruiter|engineering\s+recruiter|"
            r"recruiter|recruiting|talent\s+acquisition|sourcer|headhunter)\b",
            re.IGNORECASE,
        ),
    ),
    # Human Resources and People Operations
    (
        "hr",
        re.compile(
            r"\b(people\s+operations|people\s+partner|human\s+resources|\bhr\b|"
            r"hr\s+business\s+partner|hr\s+manager|people\s+lead)\b",
            re.IGNORECASE,
        ),
    ),
    # Sales and Business Development
    (
        "sales",
        re.compile(
            r"\b(account\s+executive|sales\s+representative|sales\s+rep|sales\s+associate|"
            r"sales\s+director|sales\s+manager|\bsales\b|business\s+development|"
            r"\bbdr\b|\bsdr\b|account\s+manager)\b",
            re.IGNORECASE,
        ),
    ),
    # Marketing and Content
    (
        "marketing",
        re.compile(
            r"\b(marketing|seo|sem|growth\s+marketer|content\s+writer|copywriter|"
            r"social\s+media|communications|public\s+relations|\bpr\b)\b",
            re.IGNORECASE,
        ),
    ),
    # Accounting and Finance
    (
        "accounting",
        re.compile(
            r"\b(accountant|accounting|controller|comptroller|payroll|"
            r"accounts\s+payable|accounts\s+receivable|ap/ar|auditor|audit)\b",
            re.IGNORECASE,
        ),
    ),
    (
        "finance",
        re.compile(
            r"\b(financial\s+analyst|finance\s+manager|finance\s+director|"
            r"treasury|investment\s+analyst)\b",
            re.IGNORECASE,
        ),
    ),
    # Administrative, Operations, Legal
    (
        "administrative",
        re.compile(
            r"\b(office\s+manager|executive\s+assistant|admin\s+assistant|"
            r"administrative\s+assistant|receptionist|facilities\s+manager)\b",
            re.IGNORECASE,
        ),
    ),
    (
        "legal",
        re.compile(
            r"\b(legal\s+counsel|general\s+counsel|paralegal|attorney|lawyer|compliance\s+officer)\b",
            re.IGNORECASE,
        ),
    ),
]


# Regex patterns for positive technical classifications
_POSITIVE_RULES: list[tuple[str, re.Pattern[str]]] = [
    # Engineering Leadership
    (
        "engineering_leadership",
        re.compile(
            r"\b(cto|chief\s+technology\s+officer|chief\s+architect|"
            r"vp\s+of\s+engineering|vice\s+president\s+of\s+engineering|"
            r"director\s+of\s+(?:software\s+)?engineering|director\s+of\s+technology|"
            r"engineering\s+manager|software\s+engineering\s+manager|"
            r"tech\s+lead|technical\s+lead|head\s+of\s+engineering|"
            r"head\s+of\s+infrastructure|head\s+of\s+data)\b",
            re.IGNORECASE,
        ),
    ),
    # QA Automation & SDET
    (
        "qa_automation",
        re.compile(
            r"\b(sdet|software\s+development\s+engineer\s+in\s+test|"
            r"qa\s+automation|automation\s+engineer|test\s+automation|"
            r"quality\s+assurance\s+engineer|qa\s+engineer|quality\s+engineer|"
            r"test\s+engineer|qa\s+analyst|quality\s+assurance\s+tester)\b",
            re.IGNORECASE,
        ),
    ),
    # Security
    (
        "security",
        re.compile(
            r"\b(security\s+engineer|information\s+security|infosec|"
            r"application\s+security|appsec|cybersecurity|cloud\s+security|"
            r"soc\s+analyst|penetration\s+tester|security\s+analyst|cryptographer)\b",
            re.IGNORECASE,
        ),
    ),
    # Infrastructure, DevOps, SRE, Cloud, Platform
    (
        "infrastructure",
        re.compile(
            r"\b(devops|site\s+reliability|sre|cloud\s+architect|"
            r"cloud\s+infrastructure|infrastructure\s+engineer|platform\s+engineer|"
            r"(?<!embedded\s)systems?\s+engineer|systems?\s+administrator|sysadmin|"
            r"network\s+engineer|kubernetes\s+engineer|build\s+engineer|"
            r"release\s+engineer)\b",
            re.IGNORECASE,
        ),
    ),
    # Data, ML, AI, DBA
    (
        "data",
        re.compile(
            r"\b(data\s+engineer|data\s+scientist|machine\s+learning|"
            r"ml\s+engineer|ai\s+engineer|ai\s+research|research\s+scientist|"
            r"database\s+administrator|\bdba\b|analytics\s+engineer|"
            r"business\s+intelligence|bi\s+developer|deep\s+learning|"
            r"computer\s+vision|nlp\s+engineer)\b",
            re.IGNORECASE,
        ),
    ),
    # Product Engineering & Solutions Architecture
    (
        "product_engineering",
        re.compile(
            r"\b(product\s+engineer|solutions?\s+architect|"
            r"technical\s+product\s+manager|\btpm\b|engineering\s+solutions?|"
            r"solutions?\s+engineer)\b",
            re.IGNORECASE,
        ),
    ),
    # Technical Support
    (
        "technical_support",
        re.compile(
            r"\b(technical\s+support|customer\s+support\s+engineer|"
            r"support\s+engineer|tier\s+[123]\s+support|application\s+support|"
            r"it\s+support|escalation\s+engineer)\b",
            re.IGNORECASE,
        ),
    ),
    # Core Software Engineering
    (
        "software",
        re.compile(
            r"\b(software\s+engineer|software\s+developer|backend|frontend|"
            r"front\s+end|back\s+end|full\s*stack|mobile\s+engineer|"
            r"mobile\s+developer|ios\s+developer|android\s+developer|"
            r"embedded\s+(?:systems\s+)?engineer|firmware|"
            r"web\s+developer|systems?\s+programmer|algorithm\s+engineer|"
            r"application\s+developer|kernel\s+developer|programmer|coder)\b",
            re.IGNORECASE,
        ),
    ),
]


def classify_technical_role(
    title: str, categories: Sequence[str] = ()
) -> Classification:
    """Classifies a job title and categories deterministically into technical or non-technical role."""
    title_clean = (title or "").strip()
    categories_clean = [c.strip() for c in categories if c]

    # Combine title with categories for keyword matching
    combined_text = f"{title_clean} {' '.join(categories_clean)}"

    # 1. Evaluate negative exclusion rules first
    for rule_name, pattern in _NEGATIVE_RULES:
        match = pattern.search(title_clean)
        if match:
            return Classification(
                is_technical=False,
                classification="",
                rule_version=RULE_VERSION,
                reasons=[f"negative_rule:{rule_name}:{match.group(0).lower()}"],
            )
        # Also check category exclusion if title does not explicitly contain software/dev
        cat_match = pattern.search(" ".join(categories_clean))
        if cat_match and not re.search(
            r"\b(software|developer|architect|engineer|sdet)\b", title_clean, re.I
        ):
            return Classification(
                is_technical=False,
                classification="",
                rule_version=RULE_VERSION,
                reasons=[f"negative_rule_category:{rule_name}:{cat_match.group(0).lower()}"],
            )

    # 2. Evaluate positive technical rules
    reasons: list[str] = []
    for cat_name, pattern in _POSITIVE_RULES:
        # Match title first
        match = pattern.search(title_clean)
        if match:
            reasons.append(f"title_match:{cat_name}:{match.group(0).lower()}")
            return Classification(
                is_technical=True,
                classification=cat_name,
                rule_version=RULE_VERSION,
                reasons=reasons,
            )

    # Check combined text or general engineer / developer with technical category
    for cat_name, pattern in _POSITIVE_RULES:
        match = pattern.search(combined_text)
        if match:
            reasons.append(f"combined_match:{cat_name}:{match.group(0).lower()}")
            return Classification(
                is_technical=True,
                classification=cat_name,
                rule_version=RULE_VERSION,
                reasons=reasons,
            )

    # Check general engineer in technical category
    tech_dept_pattern = re.compile(
        r"\b(engineering|software|technology|tech|infrastructure|data|platform)\b", re.I
    )
    if re.search(r"\b(engineer|developer|architect)\b", title_clean, re.I) and any(
        tech_dept_pattern.search(c) for c in categories_clean
    ):
        return Classification(
            is_technical=True,
            classification="software",
            rule_version=RULE_VERSION,
            reasons=["category_and_title_engineer"],
        )

    return Classification(
        is_technical=False,
        classification="",
        rule_version=RULE_VERSION,
        reasons=["no_technical_rules_matched"],
    )


def classify_arrangement(
    structured: Mapping[str, object] | None = None,
    text: str = "",
) -> WorkArrangement:
    """Classifies workplace arrangement into in_office, hybrid, remote, or unknown.

    Explicit structured fields take strict precedence over free text.
    Conflicting or absent evidence returns WorkArrangement.UNKNOWN.
    """
    if structured is not None and isinstance(structured, Mapping):
        # Look for explicit structured fields
        workplace_type = (
            structured.get("workplace_type")
            or structured.get("workplaceType")
            or structured.get("work_arrangement")
            or structured.get("workArrangement")
        )
        job_loc_type = str(structured.get("jobLocationType") or "").strip().upper()
        remote_flag = structured.get("remote")
        is_remote_flag = structured.get("isRemote") or structured.get("is_remote")

        # Conflict check in structured fields
        wt_str = str(workplace_type or "").lower().strip().replace("-", "_").replace(" ", "_")
        is_office_type = wt_str in ("in_office", "on_site", "onsite", "office")
        is_remote_type = wt_str in ("remote", "telecommute") or job_loc_type == "TELECOMMUTE" or remote_flag is True or is_remote_flag is True

        if is_office_type and is_remote_type:
            return WorkArrangement.UNKNOWN

        if wt_str in ("remote", "telecommute"):
            return WorkArrangement.REMOTE
        if wt_str in ("hybrid", "flexible"):
            return WorkArrangement.HYBRID
        if is_office_type:
            return WorkArrangement.IN_OFFICE
        if job_loc_type == "TELECOMMUTE" or remote_flag is True or is_remote_flag is True:
            return WorkArrangement.REMOTE

    # Free text fallback
    if text:
        text_lower = text.lower()

        # Hybrid detection
        has_hybrid = bool(
            re.search(
                r"\b(hybrid|partially\s+remote|flexible\s+remote|\d+\s+days\s+(?:in\s+office|wfh))\b",
                text_lower,
            )
        )
        if has_hybrid:
            return WorkArrangement.HYBRID

        # Remote detection
        has_remote = bool(
            re.search(
                r"\b(remote|work\s+from\s+home|\bwfh\b|telecommute|anywhere\s+in\s+(?:the\s+)?us|100%\s+remote|fully\s+remote)\b",
                text_lower,
            )
        )

        # On-site / in-office detection
        has_onsite = bool(
            re.search(
                r"\b(on-site|onsite|on\s+site|in-office|in\s+office|office-based|strictly\s+on-site|at\s+our\s+headquarters|relocation\s+required)\b",
                text_lower,
            )
        )

        # Conflicting text
        if has_remote and has_onsite:
            return WorkArrangement.UNKNOWN

        if has_remote:
            return WorkArrangement.REMOTE
        if has_onsite:
            return WorkArrangement.IN_OFFICE

    return WorkArrangement.UNKNOWN


def _ensure_utc(dt: datetime | None) -> datetime | None:
    if dt is None:
        return None
    if dt.tzinfo is None:
        return dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc)


def publication_state(
    posted_at: datetime | None,
    first_seen_at: datetime | None = None,
    now: datetime | None = None,
    posted_at_trusted: bool = True,
) -> PublicationState:
    """Determines publication recency: posted_recently, observed_recently, or stale.

    - Trusted posted_at >= now - 14 days is posted_recently.
    - Exactly 14 days is recent; older is stale.
    - Missing or untrusted publication time is observed_recently (if within 14 days) or stale.
    """
    now_utc = _ensure_utc(now) or datetime.now(timezone.utc)
    cutoff = now_utc - timedelta(days=14)

    if posted_at is not None and posted_at_trusted:
        p_utc = _ensure_utc(posted_at)
        if p_utc >= cutoff:
            return PublicationState.POSTED_RECENTLY
        return PublicationState.STALE

    if first_seen_at is not None:
        fs_utc = _ensure_utc(first_seen_at)
        if fs_utc >= cutoff:
            return PublicationState.OBSERVED_RECENTLY
        return PublicationState.STALE

    return PublicationState.OBSERVED_RECENTLY


def clean_excerpt(raw_html_or_text: str | None, max_length: int = 300) -> str | None:
    """Sanitizes HTML tags and condenses whitespace into a clean short excerpt."""
    if not raw_html_or_text:
        return None
    # Strip HTML tags
    stripped = re.sub(r"<[^>]+>", " ", raw_html_or_text)
    # Unescape HTML entities
    unescaped = html.unescape(stripped)
    # Condense whitespace
    condensed = " ".join(unescaped.split())
    if not condensed:
        return None
    if len(condensed) > max_length:
        return condensed[:max_length].rstrip() + "..."
    return condensed


def compute_job_content_hash(
    source: str,
    source_job_id: str | None,
    title: str,
    canonical_url: str = "",
) -> str:
    """Computes a stable deduplication hash for a job posting."""
    norm = f"{source.strip().lower()}:{str(source_job_id or '').strip()}:{title.strip().lower()}:{canonical_url.strip().lower()}"
    return hashlib.sha256(norm.encode("utf-8")).hexdigest()


def compute_company_content_hash(
    name: str,
    domain: str | None = None,
    source_record_id: str | None = None,
) -> str:
    """Computes a stable deduplication hash for company evidence."""
    norm = f"{name.strip().lower()}:{str(domain or '').strip().lower()}:{str(source_record_id or '').strip().lower()}"
    return hashlib.sha256(norm.encode("utf-8")).hexdigest()
