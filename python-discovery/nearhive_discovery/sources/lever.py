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


class LeverSource(BaseSourceAdapter):
    """Source adapter for public Lever job boards."""

    name: str = "lever"
    source_family: str = "job_ats"

    def __init__(
        self,
        site: str | None = None,
        company_name: str | None = None,
        company_domain: str | None = None,
        sites: list[dict[str, Any]] | None = None,
        fetcher: Callable[[str], Awaitable[str]] | None = None,
        http_client: httpx.AsyncClient | None = None,
        batch_size: int = 50,
        page_ceiling: int = 10,
        limit: int = 20,
        reference_time: datetime | None = None,
    ) -> None:
        self.batch_size = max(1, batch_size)
        self.page_ceiling = max(1, page_ceiling)
        self.limit = max(1, limit)
        self.fetcher = fetcher
        self.http_client = http_client
        self.reference_time = reference_time

        if sites is not None:
            self.sites = list(sites)
        elif site is not None:
            self.sites = [
                {
                    "site": site,
                    "company_name": company_name or site,
                    "company_domain": company_domain,
                }
            ]
        else:
            self.sites = []

    async def _fetch(self, url: str) -> str:
        if self.fetcher is not None:
            return await self.fetcher(url)
        return await safe_fetch_text(url, client=self.http_client, timeout=10.0)

    async def run(
        self, job: DiscoveryJob, now: datetime | None = None
    ) -> AsyncIterator[EvidenceBatch]:
        """Executes discovery against configured Lever job boards."""
        current_time = now or self.reference_time or datetime.now(timezone.utc)

        for site_cfg in self.sites:
            site = site_cfg.get("site") or site_cfg.get("board_token") or site_cfg.get("token")
            if not site:
                continue

            company_name = site_cfg.get("company_name") or site
            company_domain = site_cfg.get("company_domain")

            comp_hash = compute_company_content_hash(
                name=company_name,
                domain=company_domain,
                source_record_id=site,
            )
            company_evidence = CompanyEvidence(
                name=company_name,
                domain=company_domain,
                source_record_id=site,
                evidence_url=f"https://jobs.lever.co/{site}",
                content_hash=comp_hash,
                metadata={"ats": "lever", "site": site},
            )

            seen_job_ids: set[str] = set()
            pending_jobs: list[TechnicalJobEvidence] = []
            company_emitted = False

            for page in range(1, self.page_ceiling + 1):
                skip = (page - 1) * self.limit
                url = f"https://api.lever.co/v0/postings/{site}?mode=json&skip={skip}&limit={self.limit}"
                try:
                    raw_text = await self._fetch(url)
                    data = json.loads(raw_text)
                except Exception as exc:
                    logger.warning("Lever fetch error for site %s page %d: %s", site, page, exc)
                    break

                raw_postings: list[dict[str, Any]]
                if isinstance(data, list):
                    raw_postings = data
                elif isinstance(data, dict):
                    raw_postings = data.get("postings", data.get("jobs", []))
                else:
                    raw_postings = []

                if not raw_postings:
                    break

                new_jobs_found = False
                for posting in raw_postings:
                    if not isinstance(posting, dict):
                        continue
                    job_id = str(posting.get("id"))
                    if job_id in seen_job_ids:
                        continue
                    seen_job_ids.add(job_id)
                    new_jobs_found = True

                    title = (posting.get("text") or "").strip()
                    categories_dict = posting.get("categories", {})
                    categories_list: list[str] = []
                    if isinstance(categories_dict, dict):
                        for k, v in categories_dict.items():
                            if isinstance(v, str) and v and k != "workplaceType":
                                categories_list.append(v)

                    # Deterministic classification: retain only technical jobs
                    classification = classify_technical_role(title, categories_list)
                    if not classification.is_technical:
                        continue

                    # Workplace arrangement
                    loc_name = ""
                    structured_meta: dict[str, Any] = {}
                    if isinstance(categories_dict, dict):
                        loc_name = categories_dict.get("location", "")
                        if "workplaceType" in categories_dict:
                            structured_meta["workplaceType"] = categories_dict["workplaceType"]

                    arrangement = classify_arrangement(
                        structured=structured_meta,
                        text=f"{loc_name} {title}",
                    )

                    # Publication timestamps and recency
                    created_at_val = posting.get("createdAt")
                    posted_at: datetime | None = None
                    if isinstance(created_at_val, (int, float)):
                        try:
                            if created_at_val > 1e11:
                                posted_at = datetime.fromtimestamp(
                                    created_at_val / 1000.0, tz=timezone.utc
                                )
                            else:
                                posted_at = datetime.fromtimestamp(
                                    created_at_val, tz=timezone.utc
                                )
                        except Exception:
                            posted_at = None
                    elif isinstance(created_at_val, str):
                        try:
                            posted_at = datetime.fromisoformat(
                                created_at_val.replace("Z", "+00:00")
                            )
                        except Exception:
                            posted_at = None

                    pub_state = publication_state(
                        posted_at=posted_at,
                        first_seen_at=current_time,
                        now=current_time,
                        posted_at_trusted=True,
                    )

                    canonical_url = posting.get("hostedUrl") or f"https://jobs.lever.co/{site}/{job_id}"
                    raw_desc = posting.get("descriptionPlain") or posting.get("description")
                    excerpt = clean_excerpt(raw_desc)
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
                            "ats": "lever",
                            "categories": categories_dict if isinstance(categories_dict, dict) else {},
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
