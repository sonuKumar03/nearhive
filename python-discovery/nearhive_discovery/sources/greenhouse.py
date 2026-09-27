from collections.abc import AsyncIterator, Awaitable, Callable
from datetime import datetime, timezone
import json
import logging
from typing import Any

import httpx

from nearhive_discovery.classify import (
    clean_excerpt,
    classify_arrangement,
    classify_technical_role,
    compute_company_content_hash,
    compute_job_content_hash,
    publication_state,
)
from nearhive_discovery.contracts import (
    CONTRACT_VERSION,
    CompanyEvidence,
    DiscoveryJob,
    EvidenceBatch,
    TechnicalJobEvidence,
)
from nearhive_discovery.http import safe_fetch_text
from nearhive_discovery.sources.base import BaseSourceAdapter

logger = logging.getLogger(__name__)


class GreenhouseSource(BaseSourceAdapter):
    """Source adapter for public Greenhouse job boards."""

    name: str = "greenhouse"
    source_family: str = "job_ats"

    def __init__(
        self,
        board_token: str | None = None,
        company_name: str | None = None,
        company_domain: str | None = None,
        boards: list[dict[str, Any]] | None = None,
        fetcher: Callable[[str], Awaitable[str]] | None = None,
        http_client: httpx.AsyncClient | None = None,
        batch_size: int = 50,
        page_ceiling: int = 10,
        reference_time: datetime | None = None,
        api_base_url: str = "https://boards-api.greenhouse.io",
    ) -> None:
        self.batch_size = max(1, batch_size)
        self.page_ceiling = max(1, page_ceiling)
        self.fetcher = fetcher
        self.http_client = http_client
        self.reference_time = reference_time
        self.api_base_url = api_base_url.rstrip("/")

        if boards is not None:
            self.boards = list(boards)
        elif board_token is not None:
            self.boards = [
                {
                    "board_token": board_token,
                    "company_name": company_name or board_token,
                    "company_domain": company_domain,
                }
            ]
        else:
            self.boards = []

    async def _fetch(self, url: str) -> str:
        if self.fetcher is not None:
            return await self.fetcher(url)
        return await safe_fetch_text(url, client=self.http_client, timeout=10.0)

    async def run(
        self, job: DiscoveryJob, now: datetime | None = None
    ) -> AsyncIterator[EvidenceBatch]:
        """Executes discovery against configured Greenhouse job boards."""
        current_time = now or self.reference_time or datetime.now(timezone.utc)

        for board_cfg in self.boards:
            token = board_cfg.get("board_token") or board_cfg.get("token") or board_cfg.get("site")
            if not token:
                continue

            company_name = board_cfg.get("company_name") or token
            company_domain = board_cfg.get("company_domain")

            comp_hash = compute_company_content_hash(
                name=company_name,
                domain=company_domain,
                source_record_id=token,
            )
            company_evidence = CompanyEvidence(
                name=company_name,
                domain=company_domain,
                source_record_id=token,
                evidence_url=f"https://boards.greenhouse.io/{token}",
                content_hash=comp_hash,
                metadata={"ats": "greenhouse", "board_token": token},
            )

            seen_job_ids: set[str] = set()
            pending_jobs: list[TechnicalJobEvidence] = []
            company_emitted = False

            for page in range(1, self.page_ceiling + 1):
                url = f"{self.api_base_url}/v1/boards/{token}/jobs?content=true&page={page}"
                try:
                    raw_text = await self._fetch(url)
                    data = json.loads(raw_text)
                except Exception as exc:
                    logger.warning("Greenhouse fetch error for board %s page %d: %s", token, page, exc)
                    break

                raw_jobs: list[dict[str, Any]]
                if isinstance(data, dict):
                    raw_jobs = data.get("jobs", [])
                elif isinstance(data, list):
                    raw_jobs = data
                else:
                    raw_jobs = []

                if not raw_jobs:
                    break

                # Check if all jobs on this page have already been seen
                new_jobs_found = False
                for raw_job in raw_jobs:
                    if not isinstance(raw_job, dict):
                        continue
                    job_id = str(raw_job.get("id"))
                    if job_id in seen_job_ids:
                        continue
                    seen_job_ids.add(job_id)
                    new_jobs_found = True

                    title = (raw_job.get("title") or "").strip()
                    departments = raw_job.get("departments", [])
                    categories = [
                        d.get("name")
                        for d in departments
                        if isinstance(d, dict) and d.get("name")
                    ]

                    # Deterministic classification: retain only technical jobs
                    classification = classify_technical_role(title, categories)
                    if not classification.is_technical:
                        continue

                    # Workplace arrangement
                    loc = raw_job.get("location")
                    loc_name = loc.get("name", "") if isinstance(loc, dict) else (str(loc) if loc else "")
                    structured_meta: dict[str, Any] = {}
                    if "workplace_type" in raw_job:
                        structured_meta["workplace_type"] = raw_job["workplace_type"]

                    arrangement = classify_arrangement(
                        structured=structured_meta,
                        text=f"{loc_name} {title}",
                    )

                    # Publication timestamps and recency
                    updated_at_str = raw_job.get("updated_at")
                    posted_at: datetime | None = None
                    if updated_at_str:
                        try:
                            posted_at = datetime.fromisoformat(
                                updated_at_str.replace("Z", "+00:00")
                            )
                        except Exception:
                            posted_at = None

                    pub_state = publication_state(
                        posted_at=posted_at,
                        first_seen_at=current_time,
                        now=current_time,
                        posted_at_trusted=True,
                    )

                    canonical_url = raw_job.get("absolute_url") or f"https://boards.greenhouse.io/{token}/jobs/{job_id}"
                    excerpt = clean_excerpt(raw_job.get("content"))
                    job_hash = compute_job_content_hash(
                        source=self.name,
                        source_job_id=job_id,
                        title=title,
                        canonical_url=canonical_url,
                    )

                    evidence = TechnicalJobEvidence(
                        company_name=company_name,
                        title=title,
                        source_job_id=job_id,
                        canonical_url=canonical_url,
                        company_domain=company_domain,
                        description_excerpt=excerpt,
                        content_hash=job_hash,
                        location_raw=loc_name,
                        work_arrangement=arrangement,
                        publication_state=pub_state,
                        posted_at=posted_at,
                        posted_at_confidence=1.0 if posted_at is not None else 0.0,
                        first_seen_at=current_time,
                        last_seen_at=current_time,
                        technical_classification=classification.classification,
                        rule_version=classification.rule_version,
                        classification_reasons=classification.reasons,
                        metadata={
                            "ats": "greenhouse",
                            "departments": categories,
                        },
                    )
                    pending_jobs.append(evidence)

                    if len(pending_jobs) >= self.batch_size:
                        companies_to_send = [company_evidence] if not company_emitted else []
                        company_emitted = True
                        yield EvidenceBatch(
                            contract_version=CONTRACT_VERSION,
                            discovery_job_id=job.id,
                            source=self.name,
                            source_family=self.source_family,
                            observed_at=current_time,
                            companies=companies_to_send,
                            jobs=pending_jobs,
                        )
                        pending_jobs = []

                if not new_jobs_found:
                    break

            # Flush remaining jobs or emit company evidence if none emitted yet
            if pending_jobs or not company_emitted:
                companies_to_send = [company_evidence] if not company_emitted else []
                yield EvidenceBatch(
                    contract_version=CONTRACT_VERSION,
                    discovery_job_id=job.id,
                    source=self.name,
                    source_family=self.source_family,
                    observed_at=current_time,
                    companies=companies_to_send,
                    jobs=pending_jobs,
                )
