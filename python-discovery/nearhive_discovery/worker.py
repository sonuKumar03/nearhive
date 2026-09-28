import asyncio
import concurrent.futures
import inspect
import logging
import threading
import time
import uuid
from collections.abc import AsyncIterable, Iterable
from dataclasses import replace
from datetime import datetime, timezone
from typing import Any, Callable, Protocol, runtime_checkable

import psycopg

from nearhive_discovery.location import LocationResolver, resolve_job_location
from nearhive_discovery.contracts import (
    BatchResult,
    DiscoveryJob,
    DiscoverySourceRun,
    DiscoveryStatus,
    EvidenceBatch,
)
from nearhive_discovery.http import PlaywrightPool
from nearhive_discovery.persistence import PostgresPersistence
from nearhive_discovery.queue import (
    claim_job,
    finish_job,
    heartbeat,
    is_cancelled,
    reap_exhausted_jobs,
    record_source_run,
)
from nearhive_discovery.settings import settings
from nearhive_discovery.sources.company_site import CompanySiteSource, _clean_domain
from nearhive_discovery.sources.greenhouse import GreenhouseSource
from nearhive_discovery.sources.lever import LeverSource

logger = logging.getLogger(__name__)


@runtime_checkable
class Source(Protocol):
    name: str
    source_family: str

    def run(
        self, job: DiscoveryJob
    ) -> list[EvidenceBatch] | EvidenceBatch | Iterable[EvidenceBatch]:
        ...


class Worker:
    def __init__(
        self,
        db_url: str = settings.database_url,
        sources: list[Source] | None = None,
        worker_id: str | None = None,
        lease_seconds: int = settings.lease_seconds,
        heartbeat_interval_seconds: float = settings.heartbeat_interval_seconds,
        poll_interval_seconds: float = settings.poll_interval_seconds,
        playwright_contexts: int = settings.playwright_contexts,
        playwright_pool: Any | None = None,
        max_company_sites: int = settings.max_company_sites,
        site_fetcher: Any | None = None,
        location_resolver: Any | None = None,
    ) -> None:
        self.db_url = db_url
        self.worker_id = worker_id if worker_id is not None else settings.worker_id
        self.lease_seconds = lease_seconds
        self.heartbeat_interval_seconds = heartbeat_interval_seconds
        self.poll_interval_seconds = poll_interval_seconds
        self.playwright_contexts = playwright_contexts
        self.playwright_pool = (
            playwright_pool
            if playwright_pool is not None
            else PlaywrightPool(max_contexts=playwright_contexts)
        )
        self.max_company_sites = max_company_sites
        self.site_fetcher = site_fetcher
        self.location_resolver = (
            location_resolver if location_resolver is not None else LocationResolver()
        )
        self.sources: list[Source] = sources if sources is not None else []
        try:
            for s in self.sources:
                if hasattr(s, "playwright_pool") and getattr(s, "playwright_pool") is None:
                    try:
                        s.playwright_pool = self.playwright_pool
                    except Exception:
                        pass
        except Exception:
            pass

    def run_once(self) -> bool:
        """Attempts to claim and execute one discovery job.

        Returns:
            True if a job was claimed and processed, False otherwise.
        """
        with psycopg.connect(self.db_url, autocommit=True) as conn:
            reaped = reap_exhausted_jobs(conn)
            if reaped:
                logger.warning("Marked %d job(s) failed after exhausted leases", reaped)
            job = claim_job(conn, self.worker_id, self.lease_seconds)
            if not job:
                return False

            logger.info("Claimed job %s on worker %s", job.id, self.worker_id)
            self._process_job(conn, job)
            return True

    def _submit_batch(self, batch: EvidenceBatch) -> BatchResult:
        with PostgresPersistence(self.db_url) as persistence:
            return persistence.persist(batch)

    async def _resolve_batch_jobs_async(self, batch: EvidenceBatch) -> EvidenceBatch:
        if not self.location_resolver or not batch.jobs:
            return batch
        resolved_jobs = []
        for job in batch.jobs:
            try:
                resolved_jobs.append(await resolve_job_location(job, self.location_resolver))
            except Exception as exc:
                logger.debug("Failed resolving job location: %s", exc)
                resolved_jobs.append(job)
        return replace(batch, jobs=resolved_jobs)

    def _resolve_batch_jobs_sync(self, batch: EvidenceBatch) -> EvidenceBatch:
        if not self.location_resolver or not batch.jobs:
            return batch
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            loop = None

        if loop is not None and loop.is_running():
            with concurrent.futures.ThreadPoolExecutor(max_workers=1) as pool:
                return pool.submit(asyncio.run, self._resolve_batch_jobs_async(batch)).result()
        else:
            return asyncio.run(self._resolve_batch_jobs_async(batch))

    def _process_job(self, conn: Any, job: DiscoveryJob) -> None:
        stop_heartbeat = threading.Event()
        lease_lost = threading.Event()
        cancelled = threading.Event()

        def heartbeat_target() -> None:
            while not stop_heartbeat.wait(self.heartbeat_interval_seconds):
                try:
                    with psycopg.connect(self.db_url, autocommit=True) as hb_conn:
                        extended = heartbeat(
                            hb_conn,
                            job.id,
                            self.worker_id,
                            extend_seconds=self.lease_seconds,
                        )
                        if not extended:
                            with hb_conn.cursor() as cur:
                                cur.execute(
                                    "SELECT status, worker_id FROM discovery_jobs WHERE id = %s",
                                    (str(job.id),),
                                )
                                row = cur.fetchone()

                            if row is None:
                                logger.warning("Job %s was removed from database; stopping", job.id)
                                lease_lost.set()
                            elif row[0] == "cancelled":
                                logger.info(
                                    "Job %s was cancelled in database; stopping worker execution cleanly",
                                    job.id,
                                )
                                cancelled.set()
                            elif row[0] == "running" and row[1] != self.worker_id:
                                logger.warning(
                                    "Job %s lease was reclaimed by worker %s; lease lost",
                                    job.id,
                                    row[1],
                                )
                                lease_lost.set()
                            else:
                                logger.info(
                                    "Job %s status changed to %s; stopping worker execution",
                                    job.id,
                                    row[0],
                                )
                                cancelled.set()
                            break
                except Exception as exc:
                    logger.warning("Heartbeat error on job %s: %s", job.id, exc)

        hb_thread = threading.Thread(target=heartbeat_target, daemon=True)
        hb_thread.start()

        total_companies = 0
        total_jobs = 0
        total_evidence = 0
        source_statuses: list[DiscoveryStatus] = []
        was_cancelled = False
        execution_error: str | None = None
        try:
            candidate_sources: list[Source] = []
            configured_site_sources: list[Source] = []
            configured_ats_sources: list[Source] = []

            for s in self.sources:
                fam = getattr(s, "source_family", "")
                s_name = getattr(s, "name", "")
                if fam == "official_site" or s_name == "company_site":
                    configured_site_sources.append(s)
                elif fam == "job_ats" or s_name in ("greenhouse", "lever"):
                    configured_ats_sources.append(s)
                else:
                    candidate_sources.append(s)

            discovered_domains: dict[str, str] = {}

            def _collect_candidate_domains(batch: EvidenceBatch) -> None:
                for c in batch.companies:
                    if c.domain:
                        cleaned = _clean_domain(c.domain)
                        if cleaned and cleaned not in discovered_domains:
                            discovered_domains[cleaned] = c.name or cleaned

            def _run_single(source: Source) -> bool:
                nonlocal total_companies, total_jobs, total_evidence, execution_error, was_cancelled
                if lease_lost.is_set():
                    execution_error = "Lease lost during execution"
                    return False
                if was_cancelled or cancelled.is_set() or is_cancelled(conn, job.id):
                    was_cancelled = True
                    self._close_contexts()
                    return False

                st, cc, jc, ec, err = self._run_source_helper(
                    conn, job, source, lease_lost, cancelled, on_batch=_collect_candidate_domains
                )
                total_companies += cc
                total_jobs += jc
                total_evidence += ec
                source_statuses.append(st)

                if was_cancelled or cancelled.is_set() or is_cancelled(conn, job.id):
                    was_cancelled = True
                    self._close_contexts()
                    return False
                if lease_lost.is_set():
                    execution_error = "Lease lost during execution"
                    return False
                return True

            # Stage 1: Candidate Sources
            for src in candidate_sources:
                if not _run_single(src):
                    break

            # Stage 2: Company Site Crawl
            discovered_ats_targets: list[tuple[str, str, str | None, str | None]] = []
            if not was_cancelled and not lease_lost.is_set() and not is_cancelled(conn, job.id):
                selected_domains = list(discovered_domains.keys())[: self.max_company_sites]
                if selected_domains or configured_site_sources:
                    if configured_site_sources:
                        site_source = configured_site_sources[0]
                        for d in selected_domains:
                            u = f"https://{d}"
                            if hasattr(site_source, "target_urls") and u not in site_source.target_urls:
                                site_source.target_urls.append(u)
                        if hasattr(site_source, "target_company_names"):
                            for d, n in discovered_domains.items():
                                site_source.target_company_names.setdefault(d, n)
                    else:
                        site_source = CompanySiteSource(
                            target_urls=[f"https://{d}" for d in selected_domains],
                            target_company_names=discovered_domains,
                            fetcher=self.site_fetcher,
                            playwright_pool=self.playwright_pool,
                        )

                    if getattr(site_source, "target_urls", None):
                        _run_single(site_source)
                        if hasattr(site_source, "discovered_ats_targets"):
                            discovered_ats_targets.extend(site_source.discovered_ats_targets)

            # Stage 3: ATS Adapters
            if not was_cancelled and not lease_lost.is_set() and not is_cancelled(conn, job.id):
                gh_source = next((s for s in configured_ats_sources if getattr(s, "name", "") == "greenhouse"), None)
                lever_source = next((s for s in configured_ats_sources if getattr(s, "name", "") == "lever"), None)

                for target in discovered_ats_targets:
                    provider, token, comp_name, comp_domain = target
                    if provider == "greenhouse":
                        if gh_source is None:
                            gh_source = GreenhouseSource(fetcher=self.site_fetcher)
                            configured_ats_sources.append(gh_source)
                        if hasattr(gh_source, "add_board"):
                            gh_source.add_board(token, comp_name, comp_domain)
                    elif provider == "lever":
                        if lever_source is None:
                            lever_source = LeverSource(fetcher=self.site_fetcher)
                            configured_ats_sources.append(lever_source)
                        if hasattr(lever_source, "add_site"):
                            lever_source.add_site(token, comp_name, comp_domain)

                for ats_s in configured_ats_sources:
                    if was_cancelled or lease_lost.is_set() or is_cancelled(conn, job.id):
                        break
                    if getattr(ats_s, "name", "") == "greenhouse" and hasattr(ats_s, "boards") and not ats_s.boards:
                        continue
                    if getattr(ats_s, "name", "") == "lever" and hasattr(ats_s, "sites") and not ats_s.sites:
                        continue
                    _run_single(ats_s)

        except Exception as exc:
            logger.exception("Fatal error processing job %s: %s", job.id, exc)
            execution_error = str(exc)
        finally:
            stop_heartbeat.set()
            hb_thread.join(timeout=2.0)

            # Determine final status
            if was_cancelled or (cancelled is not None and cancelled.is_set()) or is_cancelled(conn, job.id):
                final_status = DiscoveryStatus.CANCELLED
                self._close_contexts()
            elif execution_error is not None:
                final_status = DiscoveryStatus.FAILED
            elif not source_statuses:
                final_status = DiscoveryStatus.COMPLETED
            elif all(s == DiscoveryStatus.COMPLETED for s in source_statuses):
                final_status = DiscoveryStatus.COMPLETED
            elif any(s == DiscoveryStatus.COMPLETED for s in source_statuses):
                final_status = DiscoveryStatus.PARTIAL
            else:
                final_status = DiscoveryStatus.FAILED

            updated = finish_job(
                conn,
                job_id=job.id,
                status=final_status,
                worker_id=self.worker_id,
                error=execution_error,
                company_count=total_companies,
                job_count=total_jobs,
                evidence_count=total_evidence,
            )
            logger.info("Job %s finalized with status %s (updated=%s)", job.id, final_status, updated)

    def _run_source_helper(
        self,
        conn: Any,
        job: DiscoveryJob,
        source: Source,
        lease_lost: threading.Event,
        cancelled: threading.Event | None = None,
        on_batch: Callable[[EvidenceBatch], None] | None = None,
    ) -> tuple[DiscoveryStatus, int, int, int, str | None]:
        started_at = datetime.now(timezone.utc)
        start_mono = time.monotonic()
        source_comp_count = 0
        source_job_count = 0
        source_ev_count = 0
        s_status = DiscoveryStatus.FAILED
        s_error: str | None = None

        def _is_job_cancelled() -> bool:
            return (cancelled is not None and cancelled.is_set()) or is_cancelled(conn, job.id)

        try:
            raw_batches = source.run(job)

            if inspect.isasyncgen(raw_batches) or isinstance(raw_batches, AsyncIterable):
                async def _consume_async() -> None:
                    nonlocal source_comp_count, source_job_count, source_ev_count
                    async for batch in raw_batches:
                        if _is_job_cancelled():
                            logger.info(
                                "Job %s cancelled; halting batch processing for source %s",
                                job.id,
                                source.name,
                            )
                            break
                        if lease_lost.is_set():
                            raise RuntimeError("Lease lost during batch submission")
                        if not isinstance(batch, EvidenceBatch):
                            continue
                        batch = await self._resolve_batch_jobs_async(batch)
                        if on_batch is not None:
                            on_batch(batch)
                        result = self._submit_batch(batch)
                        source_comp_count += result.accepted_companies
                        source_job_count += result.accepted_jobs
                        source_ev_count += result.total_accepted

                try:
                    loop = asyncio.get_running_loop()
                except RuntimeError:
                    loop = None

                if loop is not None and loop.is_running():
                    with concurrent.futures.ThreadPoolExecutor(max_workers=1) as pool:
                        pool.submit(asyncio.run, _consume_async()).result()
                else:
                    asyncio.run(_consume_async())

            elif isinstance(raw_batches, EvidenceBatch):
                if _is_job_cancelled():
                    s_status = DiscoveryStatus.CANCELLED
                    return s_status, source_comp_count, source_job_count, source_ev_count, None
                if lease_lost.is_set():
                    raise RuntimeError("Lease lost during batch submission")
                raw_batches = self._resolve_batch_jobs_sync(raw_batches)
                if on_batch is not None:
                    on_batch(raw_batches)
                result = self._submit_batch(raw_batches)
                source_comp_count += result.accepted_companies
                source_job_count += result.accepted_jobs
                source_ev_count += result.total_accepted

            elif isinstance(raw_batches, Iterable) and not isinstance(raw_batches, (str, bytes, dict)):
                for batch in raw_batches:
                    if _is_job_cancelled():
                        logger.info(
                            "Job %s cancelled; halting batch processing for source %s",
                            job.id,
                            source.name,
                        )
                        break
                    if lease_lost.is_set():
                        raise RuntimeError("Lease lost during batch submission")
                    if not isinstance(batch, EvidenceBatch):
                        continue
                    batch = self._resolve_batch_jobs_sync(batch)
                    if on_batch is not None:
                        on_batch(batch)
                    result = self._submit_batch(batch)
                    source_comp_count += result.accepted_companies
                    source_job_count += result.accepted_jobs
                    source_ev_count += result.total_accepted

            if _is_job_cancelled():
                s_status = DiscoveryStatus.CANCELLED
            else:
                s_status = DiscoveryStatus.COMPLETED
        except Exception as exc:
            if _is_job_cancelled():
                s_status = DiscoveryStatus.CANCELLED
                s_error = None
            else:
                logger.exception("Error running source %s for job %s: %s", source.name, job.id, exc)
                s_status = DiscoveryStatus.FAILED
                s_error = str(exc)
        finally:
            if _is_job_cancelled():
                s_status = DiscoveryStatus.CANCELLED
                s_error = None
            duration_ms = int((time.monotonic() - start_mono) * 1000)
            run_record = DiscoverySourceRun(
                id=uuid.uuid4(),
                discovery_job_id=job.id,
                source=source.name,
                source_family=source.source_family,
                status=s_status,
                attempts=1,
                company_count=source_comp_count,
                job_count=source_job_count,
                evidence_count=source_ev_count,
                error=s_error,
                duration_ms=duration_ms,
                started_at=started_at,
                finished_at=datetime.now(timezone.utc),
            )
            record_source_run(conn, run_record)

        return s_status, source_comp_count, source_job_count, source_ev_count, s_error

    def _close_contexts(self) -> None:
        """Closes active Playwright contexts on job cancellation."""
        if self.playwright_pool is not None:
            close_ctx = getattr(self.playwright_pool, "close_contexts", None)
            if callable(close_ctx):
                try:
                    res = close_ctx()
                    if inspect.isawaitable(res):
                        try:
                            loop = asyncio.get_running_loop()
                            loop.create_task(res)
                        except RuntimeError:
                            asyncio.run(res)
                except Exception as exc:
                    logger.debug("Error closing playwright contexts: %s", exc)

    def close(self) -> None:
        """Cleanly releases worker resources including Playwright browser contexts."""
        if self.playwright_pool is not None:
            close_fn = getattr(self.playwright_pool, "close", None)
            if callable(close_fn):
                try:
                    res = close_fn()
                    if inspect.isawaitable(res):
                        try:
                            loop = asyncio.get_running_loop()
                            loop.create_task(res)
                        except RuntimeError:
                            asyncio.run(res)
                except Exception as exc:
                    logger.debug("Error closing playwright pool: %s", exc)

        if self.location_resolver is not None:
            close_fn = getattr(self.location_resolver, "close", None)
            if callable(close_fn):
                try:
                    res = close_fn()
                    if inspect.isawaitable(res):
                        try:
                            loop = asyncio.get_running_loop()
                            loop.create_task(res)
                        except RuntimeError:
                            asyncio.run(res)
                except Exception as exc:
                    logger.debug("Error closing location resolver: %s", exc)

    def run(
        self,
        poll_interval: float | None = None,
        max_jobs: int | None = None,
        stop_event: threading.Event | None = None,
    ) -> None:
        interval = poll_interval if poll_interval is not None else self.poll_interval_seconds
        jobs_processed = 0

        logger.info("Starting worker loop on %s...", self.worker_id)
        try:
            while True:
                if stop_event is not None and stop_event.is_set():
                    break
                if max_jobs is not None and jobs_processed >= max_jobs:
                    break

                handled = self.run_once()
                if handled:
                    jobs_processed += 1
                else:
                    if stop_event is not None:
                        if stop_event.wait(interval):
                            break
                    else:
                        time.sleep(interval)
        finally:
            self.close()


def get_default_sources() -> list[Source]:
    """Loads configured discovery sources from config file or defaults."""
    from pathlib import Path
    import yaml
    from nearhive_discovery.sources.configured_directory import ConfiguredDirectorySource
    from nearhive_discovery.sources.greenhouse import GreenhouseSource
    from nearhive_discovery.sources.lever import LeverSource
    from nearhive_discovery.sources.osm import OpenStreetMapSource

    sources: list[Source] = []
    config_candidates = [
        Path("config/python_sources.yaml"),
        Path("/app/config/python_sources.yaml"),
        Path("../config/python_sources.yaml"),
    ]
    for p in config_candidates:
        if p.exists():
            try:
                loaded = ConfiguredDirectorySource.load_all(p)
                # Filter out unresolvable example placeholders
                valid_dirs = [s for s in loaded if "example.com" not in getattr(s, "url_template", "")]
                sources.extend(valid_dirs)
                if valid_dirs:
                    logger.info("Loaded %d directory sources from %s", len(valid_dirs), p)

                with p.open("r", encoding="utf-8") as f:
                    data = yaml.safe_load(f) or {}

                # Load OSM source (enabled by default unless explicitly disabled)
                osm_cfg = data.get("osm", {})
                if osm_cfg.get("enabled", True):
                    endpoints = osm_cfg.get("endpoints")
                    sources.append(OpenStreetMapSource(endpoints=endpoints))
                    logger.info("Loaded OpenStreetMap candidate discovery source")

                gh_boards = data.get("greenhouse", [])
                if gh_boards:
                    sources.append(GreenhouseSource(boards=gh_boards))
                    logger.info("Loaded Greenhouse source with %d boards from %s", len(gh_boards), p)

                lever_boards = data.get("lever", [])
                if lever_boards:
                    sources.append(LeverSource(boards=lever_boards))
                    logger.info("Loaded Lever source with %d boards from %s", len(lever_boards), p)

                break
            except Exception as exc:
                logger.warning("Failed to load sources from %s: %s", p, exc)

    return sources


def main() -> None:
    import os
    import signal
    from nearhive_discovery.operations_api import create_server

    log_level = os.getenv("LOG_LEVEL", "INFO").upper()
    logging.basicConfig(
        level=getattr(logging, log_level, logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    logger.info("Starting NearHive Python Discovery Worker daemon...")

    stop_event = threading.Event()

    def handle_signal(signum: int, frame: Any) -> None:
        logger.info("Received termination signal %s; shutting down worker...", signum)
        stop_event.set()

    signal.signal(signal.SIGINT, handle_signal)
    signal.signal(signal.SIGTERM, handle_signal)

    sources = get_default_sources()
    worker = Worker(sources=sources)
    api = create_server(
        settings.database_url,
        settings.jwt_secret,
        port=settings.api_port,
        allowed_origin=settings.allowed_origin,
    )
    api_thread = threading.Thread(target=api.serve_forever, daemon=True)
    api_thread.start()
    try:
        worker.run(stop_event=stop_event)
    except KeyboardInterrupt:
        logger.info("Worker interrupted by user.")
    finally:
        worker.close()
        api.shutdown()
        api.server_close()
        api_thread.join(timeout=2)


if __name__ == "__main__":
    main()
