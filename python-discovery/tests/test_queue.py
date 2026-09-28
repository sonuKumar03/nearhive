import os
import uuid
from datetime import datetime, timezone
import pytest
import psycopg

from nearhive_discovery.contracts import DiscoveryStatus, DiscoverySourceRun
from nearhive_discovery.queue import (
    claim_job,
    heartbeat,
    is_cancelled,
    finish_job,
    record_source_run,
)

DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgres://nearhive:password@localhost:5432/nearhive?sslmode=disable",
)


@pytest.fixture
def db_connections():
    """Provides two independent connections for concurrency testing."""
    conn1 = psycopg.connect(DATABASE_URL, autocommit=True)
    conn2 = psycopg.connect(DATABASE_URL, autocommit=True)
    yield conn1, conn2
    conn1.close()
    conn2.close()


@pytest.fixture(autouse=True)
def clean_jobs(db_connections):
    conn1, _ = db_connections
    with conn1.cursor() as cur:
        cur.execute("DELETE FROM discovery_jobs WHERE TRUE;")
    yield
    with conn1.cursor() as cur:
        cur.execute("DELETE FROM discovery_jobs WHERE TRUE;")


@pytest.fixture
def test_user_id(db_connections):
    conn1, _ = db_connections
    uid = uuid.uuid4()
    email = f"worker-test-{uid.hex[:8]}@example.com"
    with conn1.cursor() as cur:
        cur.execute(
            "INSERT INTO users (id, email, password) VALUES (%s, %s, %s)",
            (uid, email, "hash"),
        )
    yield uid
    with conn1.cursor() as cur:
        cur.execute("DELETE FROM users WHERE id = %s", (uid,))


def test_atomic_claim_and_no_steal(db_connections, test_user_id):
    conn1, conn2 = db_connections
    job_id = uuid.uuid4()

    # Insert a single pending job
    with conn1.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    # Worker 1 claims job
    job1 = claim_job(conn1, worker_id="worker-1", lease_seconds=60)
    assert job1 is not None
    assert str(job1.id) == str(job_id)
    assert job1.status == DiscoveryStatus.RUNNING
    assert job1.worker_id == "worker-1"
    assert job1.attempts == 1
    assert job1.lease_expires_at is not None

    # Worker 2 attempts to claim while lease is active: should return None
    job2 = claim_job(conn2, worker_id="worker-2", lease_seconds=60)
    assert job2 is None


def test_reclaim_expired_lease_and_attempts_increment(db_connections, test_user_id):
    conn1, conn2 = db_connections
    job_id = uuid.uuid4()

    # Insert an expired running job (lease expired 10 seconds ago)
    with conn1.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (
                id, user_id, status, lat, lng, radius_km, worker_id,
                lease_expires_at, last_heartbeat_at, attempts, max_attempts
            ) VALUES (
                %s, %s, 'running', 37.7749, -122.4194, 10.0, 'dead-worker',
                NOW() - INTERVAL '10 seconds', NOW() - INTERVAL '30 seconds', 1, 3
            )
            """,
            (job_id, test_user_id),
        )

    # Worker 2 claims expired job: attempts should increment from 1 to 2
    reclaimed = claim_job(conn2, worker_id="worker-2", lease_seconds=120)
    assert reclaimed is not None
    assert str(reclaimed.id) == str(job_id)
    assert reclaimed.worker_id == "worker-2"
    assert reclaimed.attempts == 2
    assert reclaimed.status == DiscoveryStatus.RUNNING

    # Worker 1 tries again while worker 2's lease is fresh: must get None
    job_retry = claim_job(conn1, worker_id="worker-1", lease_seconds=60)
    assert job_retry is None


def test_heartbeat_extends_lease(db_connections, test_user_id):
    conn1, conn2 = db_connections
    job_id = uuid.uuid4()

    with conn1.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (
                id, user_id, status, lat, lng, radius_km, worker_id,
                lease_expires_at, last_heartbeat_at, attempts, max_attempts
            ) VALUES (
                %s, %s, 'running', 37.7749, -122.4194, 10.0, 'worker-1',
                NOW() + INTERVAL '10 seconds', NOW(), 1, 3
            )
            """,
            (job_id, test_user_id),
        )

    # Extend lease by 300 seconds
    success = heartbeat(conn1, job_id=job_id, worker_id="worker-1", extend_seconds=300)
    assert success is True

    # Other worker cannot heartbeat on someone else's job
    fail = heartbeat(conn2, job_id=job_id, worker_id="wrong-worker", extend_seconds=300)
    assert fail is False


def test_is_cancelled_observation(db_connections, test_user_id):
    conn1, conn2 = db_connections
    job_id = uuid.uuid4()

    with conn1.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, worker_id, attempts, max_attempts)
            VALUES (%s, %s, 'running', 37.7749, -122.4194, 10.0, 'worker-1', 1, 3)
            """,
            (job_id, test_user_id),
        )

    # Not cancelled yet
    assert is_cancelled(conn1, job_id) is False

    # External user/API cancels the job via conn2
    with conn2.cursor() as cur:
        cur.execute("UPDATE discovery_jobs SET status = 'cancelled' WHERE id = %s", (job_id,))

    # Worker observes cancellation before starting next source
    assert is_cancelled(conn1, job_id) is True


def test_record_source_run_and_finish_job(db_connections, test_user_id):
    conn1, _ = db_connections
    job_id = uuid.uuid4()

    with conn1.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, worker_id, attempts, max_attempts)
            VALUES (%s, %s, 'running', 37.7749, -122.4194, 10.0, 'worker-1', 1, 3)
            """,
            (job_id, test_user_id),
        )

    # Upsert a source run
    run = DiscoverySourceRun(
        id=uuid.uuid4(),
        discovery_job_id=job_id,
        source="google_maps",
        source_family="search_engine",
        status=DiscoveryStatus.COMPLETED,
        company_count=5,
        job_count=0,
        evidence_count=5,
        duration_ms=1250,
    )
    record_source_run(conn1, run)

    # Verify source run in db
    with conn1.cursor() as cur:
        cur.execute(
            "SELECT company_count, status FROM discovery_source_runs WHERE discovery_job_id = %s AND source = %s",
            (job_id, "google_maps"),
        )
        row = cur.fetchone()
        assert row is not None
        assert row[0] == 5
        assert row[1] == "completed"

    # Finish job
    ok = finish_job(
        conn1,
        job_id=job_id,
        status=DiscoveryStatus.COMPLETED,
        company_count=5,
        job_count=0,
        evidence_count=5,
    )
    assert ok is True

    with conn1.cursor() as cur:
        cur.execute("SELECT status, finished_at, company_count FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row[0] == "completed"
        assert row[1] is not None
        assert row[2] == 5


def test_max_attempts_exceeded_not_claimed(db_connections, test_user_id):
    conn1, _ = db_connections
    job_id = uuid.uuid4()

    with conn1.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (
                id, user_id, status, lat, lng, radius_km, worker_id,
                lease_expires_at, last_heartbeat_at, attempts, max_attempts
            ) VALUES (
                %s, %s, 'running', 37.7749, -122.4194, 10.0, 'dead-worker',
                NOW() - INTERVAL '10 seconds', NOW() - INTERVAL '30 seconds', 3, 3
            )
            """,
            (job_id, test_user_id),
        )

    # attempts >= max_attempts: must not be claimed
    claimed = claim_job(conn1, worker_id="worker-1", lease_seconds=60)
    assert claimed is None


def test_finish_job_stale_worker_rejected(db_connections, test_user_id):
    conn1, conn2 = db_connections
    job_id = uuid.uuid4()

    # Job is currently running by worker-2
    with conn1.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, worker_id, attempts, max_attempts)
            VALUES (%s, %s, 'running', 37.7749, -122.4194, 10.0, 'worker-2', 2, 3)
            """,
            (job_id, test_user_id),
        )

    # Stale worker-1 attempts to finish the job: must return False and NOT overwrite
    ok = finish_job(
        conn1,
        job_id=job_id,
        status=DiscoveryStatus.FAILED,
        worker_id="worker-1",
        error="stale worker failure",
    )
    assert ok is False

    # Check job is still running by worker-2
    with conn2.cursor() as cur:
        cur.execute("SELECT status, worker_id, error FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row[0] == "running"
        assert row[1] == "worker-2"
        assert row[2] is None

    # Correct worker-2 finishing the job succeeds
    ok2 = finish_job(
        conn2,
        job_id=job_id,
        status=DiscoveryStatus.COMPLETED,
        worker_id="worker-2",
        company_count=3,
    )
    assert ok2 is True


def test_finish_job_clears_error_on_completed(db_connections, test_user_id):
    conn1, _ = db_connections
    job_id = uuid.uuid4()

    # Job has previous error from attempt 1
    with conn1.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (
                id, user_id, status, lat, lng, radius_km, worker_id, error, attempts, max_attempts
            ) VALUES (
                %s, %s, 'running', 37.7749, -122.4194, 10.0, 'worker-1', 'previous network error', 2, 3
            )
            """,
            (job_id, test_user_id),
        )

    # Attempt 2 completes successfully with no error
    ok = finish_job(
        conn1,
        job_id=job_id,
        status=DiscoveryStatus.COMPLETED,
        worker_id="worker-1",
        error=None,
    )
    assert ok is True

    # Error must be cleared to NULL
    with conn1.cursor() as cur:
        cur.execute("SELECT status, error FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row[0] == "completed"
        assert row[1] is None


def test_reap_exhausted_expired_jobs_marks_failed(db_connections, test_user_id):
    conn1, conn2 = db_connections
    exhausted_id = uuid.uuid4()
    reclaimable_id = uuid.uuid4()

    with conn1.cursor() as cur:
        # Exhausted: expired lease and attempts == max_attempts
        cur.execute(
            """
            INSERT INTO discovery_jobs (
                id, user_id, status, lat, lng, radius_km, worker_id,
                lease_expires_at, last_heartbeat_at, attempts, max_attempts
            ) VALUES (
                %s, %s, 'running', 37.7749, -122.4194, 10.0, 'dead-worker',
                NOW() - INTERVAL '10 seconds', NOW() - INTERVAL '30 seconds', 3, 3
            )
            """,
            (exhausted_id, test_user_id),
        )
        # Reclaimable: expired lease but attempts remaining
        cur.execute(
            """
            INSERT INTO discovery_jobs (
                id, user_id, status, lat, lng, radius_km, worker_id,
                lease_expires_at, last_heartbeat_at, attempts, max_attempts
            ) VALUES (
                %s, %s, 'running', 37.7749, -122.4194, 10.0, 'dead-worker',
                NOW() - INTERVAL '10 seconds', NOW() - INTERVAL '30 seconds', 1, 3
            )
            """,
            (reclaimable_id, test_user_id),
        )

    from nearhive_discovery.queue import reap_exhausted_jobs

    assert reap_exhausted_jobs(conn2) == 1

    with conn1.cursor() as cur:
        cur.execute(
            "SELECT status, error, finished_at FROM discovery_jobs WHERE id = %s",
            (exhausted_id,),
        )
        row = cur.fetchone()
        assert row[0] == "failed"
        assert row[1] is not None
        assert row[2] is not None

        cur.execute("SELECT status FROM discovery_jobs WHERE id = %s", (reclaimable_id,))
        assert cur.fetchone()[0] == "running"

    # The reclaimable job can still be picked up and is not reaped.
    claimed = claim_job(conn2, worker_id="worker-2", lease_seconds=60)
    assert claimed is not None
    assert str(claimed.id) == str(reclaimable_id)
