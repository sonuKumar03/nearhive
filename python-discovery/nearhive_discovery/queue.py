from datetime import datetime
from typing import Any
from uuid import UUID

from nearhive_discovery.contracts import (
    DiscoveryJob,
    DiscoverySourceRun,
    DiscoveryStatus,
)


def claim_job(
    conn: Any,
    worker_id: str,
    lease_seconds: int = 60,
    job_id: str | UUID | None = None,
) -> DiscoveryJob | None:
    query = """
    WITH candidate AS (
        SELECT id
        FROM discovery_jobs
        WHERE (status = 'pending' OR (status = 'running' AND lease_expires_at < NOW()))
          AND attempts < max_attempts
          AND (%(job_id)s::uuid IS NULL OR id = %(job_id)s::uuid)
        ORDER BY created_at ASC
        FOR UPDATE SKIP LOCKED
        LIMIT 1
    )
    UPDATE discovery_jobs j
    SET status = 'running',
        worker_id = %(worker_id)s,
        lease_expires_at = NOW() + (%(lease_seconds)s || ' seconds')::interval,
        last_heartbeat_at = NOW(),
        attempts = j.attempts + 1,
        started_at = COALESCE(j.started_at, NOW()),
        updated_at = NOW()
    FROM candidate
    WHERE j.id = candidate.id
    RETURNING j.id, j.user_id, j.status, j.lat, j.lng, j.radius_km, j.worker_id,
              j.lease_expires_at, j.last_heartbeat_at, j.attempts, j.max_attempts,
              j.error, j.company_count, j.job_count, j.evidence_count,
              j.started_at, j.finished_at, j.created_at, j.updated_at;
    """
    with conn.cursor() as cur:
        cur.execute(
            query,
            {
                "worker_id": worker_id,
                "lease_seconds": lease_seconds,
                "job_id": str(job_id) if job_id else None,
            },
        )
        row = cur.fetchone()
        if not row:
            return None

        return DiscoveryJob(
            id=row[0],
            user_id=row[1],
            status=DiscoveryStatus(row[2]),
            lat=float(row[3]),
            lng=float(row[4]),
            radius_km=float(row[5]),
            worker_id=row[6],
            lease_expires_at=row[7],
            last_heartbeat_at=row[8],
            attempts=int(row[9]),
            max_attempts=int(row[10]),
            error=row[11],
            company_count=int(row[12]),
            job_count=int(row[13]),
            evidence_count=int(row[14]),
            started_at=row[15],
            finished_at=row[16],
            created_at=row[17],
            updated_at=row[18],
        )


def heartbeat(
    conn: Any,
    job_id: str | UUID,
    worker_id: str,
    extend_seconds: int = 60,
) -> bool:
    query = """
    UPDATE discovery_jobs
    SET last_heartbeat_at = NOW(),
        lease_expires_at = NOW() + (%(extend_seconds)s || ' seconds')::interval,
        updated_at = NOW()
    WHERE id = %(job_id)s
      AND worker_id = %(worker_id)s
      AND status = 'running';
    """
    with conn.cursor() as cur:
        cur.execute(
            query,
            {
                "job_id": str(job_id),
                "worker_id": worker_id,
                "extend_seconds": extend_seconds,
            },
        )
        return cur.rowcount > 0


def is_cancelled(conn: Any, job_id: str | UUID) -> bool:
    query = "SELECT status FROM discovery_jobs WHERE id = %(job_id)s;"
    with conn.cursor() as cur:
        cur.execute(query, {"job_id": str(job_id)})
        row = cur.fetchone()
        if not row:
            return False
        return str(row[0]) == DiscoveryStatus.CANCELLED.value


def finish_job(
    conn: Any,
    job_id: str | UUID,
    status: DiscoveryStatus,
    worker_id: str | None = None,
    error: str | None = None,
    company_count: int | None = None,
    job_count: int | None = None,
    evidence_count: int | None = None,
) -> bool:
    status_str = status.value if isinstance(status, DiscoveryStatus) else str(status)
    query = """
    UPDATE discovery_jobs
    SET status = CASE
            WHEN status = 'cancelled' THEN 'cancelled'
            ELSE %(status)s
        END,
        finished_at = NOW(),
        updated_at = NOW(),
        error = CASE
            WHEN %(status)s = 'completed' THEN NULL
            ELSE COALESCE(%(error)s, error)
        END,
        company_count = COALESCE(%(company_count)s, company_count),
        job_count = COALESCE(%(job_count)s, job_count),
        evidence_count = COALESCE(%(evidence_count)s, evidence_count)
    WHERE id = %(job_id)s
      AND (status = 'running' OR status = 'cancelled')
      AND (%(worker_id)s::varchar IS NULL OR worker_id = %(worker_id)s);
    """
    with conn.cursor() as cur:
        cur.execute(
            query,
            {
                "job_id": str(job_id),
                "status": status_str,
                "worker_id": worker_id,
                "error": error,
                "company_count": company_count,
                "job_count": job_count,
                "evidence_count": evidence_count,
            },
        )
        return cur.rowcount > 0


def record_source_run(conn: Any, run: DiscoverySourceRun) -> None:
    status_str = run.status.value if isinstance(run.status, DiscoveryStatus) else str(run.status)
    query = """
    INSERT INTO discovery_source_runs (
        id, discovery_job_id, source, source_family, status,
        attempts, company_count, job_count, evidence_count,
        error, duration_ms, started_at, finished_at, created_at, updated_at
    ) VALUES (
        COALESCE(%(id)s, gen_random_uuid()), %(discovery_job_id)s, %(source)s, %(source_family)s, %(status)s,
        %(attempts)s, %(company_count)s, %(job_count)s, %(evidence_count)s,
        %(error)s, %(duration_ms)s, %(started_at)s, %(finished_at)s, NOW(), NOW()
    )
    ON CONFLICT (discovery_job_id, source) DO UPDATE SET
        status = EXCLUDED.status,
        attempts = EXCLUDED.attempts,
        company_count = EXCLUDED.company_count,
        job_count = EXCLUDED.job_count,
        evidence_count = EXCLUDED.evidence_count,
        error = EXCLUDED.error,
        duration_ms = EXCLUDED.duration_ms,
        started_at = EXCLUDED.started_at,
        finished_at = EXCLUDED.finished_at,
        updated_at = NOW();
    """
    with conn.cursor() as cur:
        cur.execute(
            query,
            {
                "id": str(run.id) if run.id else None,
                "discovery_job_id": str(run.discovery_job_id),
                "source": run.source,
                "source_family": run.source_family,
                "status": status_str,
                "attempts": run.attempts,
                "company_count": run.company_count,
                "job_count": run.job_count,
                "evidence_count": run.evidence_count,
                "error": run.error,
                "duration_ms": run.duration_ms,
                "started_at": run.started_at,
                "finished_at": run.finished_at,
            },
        )
