import logging
import threading
import time
import uuid
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

    def run(self, job: DiscoveryJob) -> list[EvidenceBatch] | EvidenceBatch:
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
    ) -> None:
        self.db_url = db_url
        self.client = client or IngestionClient(
            base_url=settings.api_base_url,
            worker_token=settings.worker_token,
        )
        self.sources: list[Source] = sources or []
        self.worker_id = worker_id or settings.worker_id
        self.lease_seconds = lease_seconds
        self.heartbeat_interval_seconds = heartbeat_interval_seconds
        self.poll_interval_seconds = poll_interval_seconds

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
                                "Failed to extend lease for job %s (worker %s)",
                                job.id,
                                self.worker_id,
                            )
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
                if is_cancelled(conn, job.id):
                    logger.info("Job %s cancelled; halting before source %s", job.id, source.name)
                    was_cancelled = True
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
                    batches: list[EvidenceBatch]
                    if isinstance(raw_batches, EvidenceBatch):
                        batches = [raw_batches]
                    elif isinstance(raw_batches, list):
                        batches = raw_batches
                    else:
                        batches = []

                    for batch in batches:
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

                if is_cancelled(conn, job.id):
                    logger.info("Job %s cancelled after source %s", job.id, source.name)
                    was_cancelled = True
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
            elif not source_statuses:
                final_status = DiscoveryStatus.COMPLETED
            elif all(s == DiscoveryStatus.COMPLETED for s in source_statuses):
                final_status = DiscoveryStatus.COMPLETED
            elif any(s == DiscoveryStatus.COMPLETED for s in source_statuses):
                final_status = DiscoveryStatus.PARTIAL
            else:
                final_status = DiscoveryStatus.FAILED

            finish_job(
                conn,
                job_id=job.id,
                status=final_status,
                error=execution_error,
                company_count=total_companies,
                job_count=total_jobs,
                evidence_count=total_evidence,
            )
            logger.info("Job %s finalized with status %s", job.id, final_status)

    def run(
        self,
        poll_interval: float | None = None,
        max_jobs: int | None = None,
        stop_event: threading.Event | None = None,
    ) -> None:
        interval = poll_interval if poll_interval is not None else self.poll_interval_seconds
        jobs_processed = 0

        logger.info("Starting worker loop on %s...", self.worker_id)
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
