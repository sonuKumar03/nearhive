import asyncio
import concurrent.futures
import inspect
import logging
import threading
import time
import uuid
from collections.abc import AsyncIterable, Iterable
from datetime import datetime, timezone
from typing import Any, Callable, Protocol, runtime_checkable

import psycopg

from nearhive_discovery.client import IngestionClient
from nearhive_discovery.contracts import (
    DiscoveryJob,
    DiscoverySourceRun,
    DiscoveryStatus,
    EvidenceBatch,
)
from nearhive_discovery.http import PlaywrightPool
from nearhive_discovery.queue import (
    claim_job,
    finish_job,
    heartbeat,
    is_cancelled,
    record_source_run,
)
from nearhive_discovery.settings import settings

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
        client: IngestionClient | None = None,
        sources: list[Source] | None = None,
        worker_id: str | None = None,
        lease_seconds: int = settings.lease_seconds,
        heartbeat_interval_seconds: float = settings.heartbeat_interval_seconds,
        poll_interval_seconds: float = settings.poll_interval_seconds,
        playwright_contexts: int = settings.playwright_contexts,
        playwright_pool: Any | None = None,
    ) -> None:
        self.db_url = db_url
        self.client = client if client is not None else IngestionClient(
            base_url=settings.api_base_url,
            worker_token=settings.worker_token,
        )
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
            job = claim_job(conn, self.worker_id, self.lease_seconds)
            if not job:
                return False

            logger.info("Claimed job %s on worker %s", job.id, self.worker_id)
            self._process_job(conn, job)
            return True

    def _process_job(self, conn: Any, job: DiscoveryJob) -> None:
        stop_heartbeat = threading.Event()
        lease_lost = threading.Event()

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
                            logger.warning(
                                "Failed to extend lease for job %s (worker %s); lease lost",
                                job.id,
                                self.worker_id,
                            )
                            lease_lost.set()
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
            for source in self.sources:
                if lease_lost.is_set():
                    logger.warning(
                        "Lease lost for job %s; aborting before source %s",
                        job.id,
                        source.name,
                    )
                    execution_error = "Lease lost during execution"
                    break

                if is_cancelled(conn, job.id):
                    logger.info("Job %s cancelled; halting before source %s", job.id, source.name)
                    was_cancelled = True
                    self._close_contexts()
                    break

                started_at = datetime.now(timezone.utc)
                start_mono = time.monotonic()
                source_comp_count = 0
                source_job_count = 0
                source_ev_count = 0
                s_status = DiscoveryStatus.FAILED
                s_error: str | None = None

                try:
                    raw_batches = source.run(job)
                    batch_iter: Iterable[EvidenceBatch]
                    if isinstance(raw_batches, EvidenceBatch):
                        batch_iter = [raw_batches]
                    elif inspect.isasyncgen(raw_batches) or isinstance(
                        raw_batches, AsyncIterable
                    ):
                        async def _collect() -> list[EvidenceBatch]:
                            collected = []
                            async for b in raw_batches:
                                collected.append(b)
                            return collected

                        try:
                            loop = asyncio.get_running_loop()
                        except RuntimeError:
                            loop = None

                        if loop is not None and loop.is_running():
                            with concurrent.futures.ThreadPoolExecutor(max_workers=1) as pool:
                                batch_iter = pool.submit(asyncio.run, _collect()).result()
                        else:
                            batch_iter = asyncio.run(_collect())
                    elif isinstance(raw_batches, Iterable) and not isinstance(
                        raw_batches, (str, bytes, dict)
                    ):
                        batch_iter = raw_batches
                    else:
                        batch_iter = []

                    for batch in batch_iter:
                        if lease_lost.is_set():
                            raise RuntimeError("Lease lost during batch submission")
                        if not isinstance(batch, EvidenceBatch):
                            continue
                        result = self.client.submit(batch)
                        source_comp_count += result.accepted_companies
                        source_job_count += result.accepted_jobs
                        source_ev_count += result.total_accepted

                    s_status = DiscoveryStatus.COMPLETED
                except Exception as exc:
                    logger.exception("Error running source %s for job %s: %s", source.name, job.id, exc)
                    s_status = DiscoveryStatus.FAILED
                    s_error = str(exc)
                finally:
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

                    total_companies += source_comp_count
                    total_jobs += source_job_count
                    total_evidence += source_ev_count
                    source_statuses.append(s_status)

                if lease_lost.is_set():
                    logger.warning(
                        "Lease lost for job %s; aborting after source %s",
                        job.id,
                        source.name,
                    )
                    execution_error = "Lease lost during execution"
                    break

                if is_cancelled(conn, job.id):
                    logger.info("Job %s cancelled after source %s", job.id, source.name)
                    was_cancelled = True
                    self._close_contexts()
                    break

        except Exception as exc:
            logger.exception("Fatal error processing job %s: %s", job.id, exc)
            execution_error = str(exc)
        finally:
            stop_heartbeat.set()
            hb_thread.join(timeout=2.0)

            # Determine final status
            if was_cancelled or is_cancelled(conn, job.id):
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
    try:
        worker.run(stop_event=stop_event)
    except KeyboardInterrupt:
        logger.info("Worker interrupted by user.")
    finally:
        worker.close()


if __name__ == "__main__":
    main()


