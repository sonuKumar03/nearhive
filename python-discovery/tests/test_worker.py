import asyncio
import os
import time
import uuid
from datetime import datetime, timezone
import pytest
import httpx
import psycopg

from nearhive_discovery.client import (
    AuthenticationError,
    IngestionClient,
    IngestionError,
    PayloadTooLargeError,
    ValidationError,
)
from nearhive_discovery.contracts import (
    CONTRACT_VERSION,
    BatchRecordResult,
    BatchResult,
    CompanyEvidence,
    DiscoveryJob,
    DiscoveryStatus,
    EvidenceBatch,
    TechnicalJobEvidence,
    WorkArrangement,
)
from nearhive_discovery.worker import Source, Worker

DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgres://nearhive:password@localhost:5432/nearhive?sslmode=disable",
)


@pytest.fixture
def db_conn():
    conn = psycopg.connect(DATABASE_URL, autocommit=True)
    yield conn
    conn.close()


@pytest.fixture(autouse=True)
def clean_jobs(db_conn):
    with db_conn.cursor() as cur:
        cur.execute("DELETE FROM discovery_jobs WHERE TRUE;")
    yield
    with db_conn.cursor() as cur:
        cur.execute("DELETE FROM discovery_jobs WHERE TRUE;")


@pytest.fixture
def test_user_id(db_conn):
    uid = uuid.uuid4()
    email = f"worker-test-{uid.hex[:8]}@example.com"
    with db_conn.cursor() as cur:
        cur.execute(
            "INSERT INTO users (id, email, password) VALUES (%s, %s, %s)",
            (uid, email, "hash"),
        )
    yield uid
    with db_conn.cursor() as cur:
        cur.execute("DELETE FROM users WHERE id = %s", (uid,))


def test_client_submit_success():
    def mock_handler(request: httpx.Request) -> httpx.Response:
        assert request.headers["X-NearHive-Worker-Token"] == "secret-token"
        assert request.url.path == "/api/v1/internal/discovery/batches"
        return httpx.Response(
            200,
            json={
                "companies": [{"index": 0, "status": "accepted", "id": "uuid-1"}],
                "jobs": [],
            },
        )

    transport = httpx.MockTransport(mock_handler)
    http_client = httpx.Client(transport=transport)
    client = IngestionClient(
        base_url="http://localhost:8080",
        worker_token="secret-token",
        client=http_client,
    )

    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(uuid.uuid4()),
        source="test_source",
        source_family="test_family",
        observed_at=datetime.now(timezone.utc),
        companies=[CompanyEvidence(name="Test Co")],
        jobs=[],
    )

    result = client.submit(batch)
    assert isinstance(result, BatchResult)
    assert result.total_accepted == 1
    assert result.total_rejected == 0
    assert result.companies[0].id == "uuid-1"


def test_client_auth_error():
    def mock_handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(401, json={"error": "unauthorized"})

    transport = httpx.MockTransport(mock_handler)
    http_client = httpx.Client(transport=transport)
    client = IngestionClient("http://localhost:8080", "bad-token", client=http_client)

    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(uuid.uuid4()),
        source="test",
        source_family="test",
        observed_at=datetime.now(timezone.utc),
        companies=[],
        jobs=[],
    )

    with pytest.raises(AuthenticationError):
        client.submit(batch)


def test_client_validation_error():
    def mock_handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(400, json={"error": "invalid batch"})

    transport = httpx.MockTransport(mock_handler)
    http_client = httpx.Client(transport=transport)
    client = IngestionClient("http://localhost:8080", "token", client=http_client)

    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(uuid.uuid4()),
        source="test",
        source_family="test",
        observed_at=datetime.now(timezone.utc),
        companies=[],
        jobs=[],
    )

    with pytest.raises(ValidationError):
        client.submit(batch)


def test_client_payload_too_large():
    def mock_handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(413, json={"error": "payload too large"})

    transport = httpx.MockTransport(mock_handler)
    http_client = httpx.Client(transport=transport)
    client = IngestionClient("http://localhost:8080", "token", client=http_client)

    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(uuid.uuid4()),
        source="test",
        source_family="test",
        observed_at=datetime.now(timezone.utc),
        companies=[],
        jobs=[],
    )

    with pytest.raises(PayloadTooLargeError):
        client.submit(batch)


class MockSource(Source):
    def __init__(self, name: str, source_family: str, return_batches=None, should_fail=False):
        self.name = name
        self.source_family = source_family
        self.return_batches = return_batches or []
        self.should_fail = should_fail
        self.called_with_job = None

    def run(self, job: DiscoveryJob) -> list[EvidenceBatch]:
        self.called_with_job = job
        if self.should_fail:
            raise RuntimeError(f"Source {self.name} failed deliberately")
        return self.return_batches


def test_worker_full_lifecycle_success(db_conn, test_user_id):
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(job_id),
        source="maps",
        source_family="search_engine",
        observed_at=datetime.now(timezone.utc),
        companies=[CompanyEvidence(name="Acme Corp")],
        jobs=[],
    )

    submitted_batches = []

    def mock_handler(request: httpx.Request) -> httpx.Response:
        submitted_batches.append(request)
        return httpx.Response(
            200,
            json={
                "companies": [{"index": 0, "status": "accepted", "id": "uuid-1"}],
                "jobs": [],
            },
        )

    transport = httpx.MockTransport(mock_handler)
    http_client = httpx.Client(transport=transport)
    client = IngestionClient("http://localhost:8080", "secret", client=http_client)

    source1 = MockSource(name="maps", source_family="search_engine", return_batches=[batch])

    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=[source1],
        worker_id="test-worker-1",
        lease_seconds=60,
    )

    handled = worker.run_once()
    assert handled is True
    assert len(submitted_batches) == 1

    # Verify job marked completed in database
    with db_conn.cursor() as cur:
        cur.execute("SELECT status, company_count, evidence_count FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row is not None
        assert row[0] == "completed"
        assert row[1] == 1
        assert row[2] == 1

        cur.execute("SELECT status, company_count FROM discovery_source_runs WHERE discovery_job_id = %s", (job_id,))
        s_row = cur.fetchone()
        assert s_row is not None
        assert s_row[0] == "completed"
        assert s_row[1] == 1


def test_worker_partial_failure(db_conn, test_user_id):
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(job_id),
        source="source_ok",
        source_family="search_engine",
        observed_at=datetime.now(timezone.utc),
        companies=[CompanyEvidence(name="Acme Corp")],
        jobs=[],
    )

    transport = httpx.MockTransport(
        lambda req: httpx.Response(
            200,
            json={
                "companies": [{"index": 0, "status": "accepted", "id": "uuid-1"}],
                "jobs": [],
            },
        )
    )
    client = IngestionClient("http://localhost:8080", "secret", client=httpx.Client(transport=transport))

    source_ok = MockSource(name="source_ok", source_family="search_engine", return_batches=[batch])
    source_fail = MockSource(name="source_fail", source_family="ats", should_fail=True)

    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=[source_ok, source_fail],
        worker_id="test-worker-partial",
    )

    handled = worker.run_once()
    assert handled is True

    # 1 succeeded, 1 failed -> status should be PARTIAL
    with db_conn.cursor() as cur:
        cur.execute("SELECT status, company_count FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row[0] == "partial"
        assert row[1] == 1

        cur.execute(
            "SELECT source, status, error FROM discovery_source_runs WHERE discovery_job_id = %s ORDER BY source",
            (job_id,),
        )
        rows = cur.fetchall()
        assert len(rows) == 2
        assert rows[0][0] == "source_fail"
        assert rows[0][1] == "failed"
        assert "deliberately" in rows[0][2]
        assert rows[1][0] == "source_ok"
        assert rows[1][1] == "completed"


def test_worker_observes_cancellation_between_sources(db_conn, test_user_id):
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(job_id),
        source="source1",
        source_family="search_engine",
        observed_at=datetime.now(timezone.utc),
        companies=[CompanyEvidence(name="Acme Corp")],
        jobs=[],
    )

    transport = httpx.MockTransport(
        lambda req: httpx.Response(
            200,
            json={
                "companies": [{"index": 0, "status": "accepted", "id": "uuid-1"}],
                "jobs": [],
            },
        )
    )
    client = IngestionClient("http://localhost:8080", "secret", client=httpx.Client(transport=transport))

    class CancellingSource(MockSource):
        def run(self, job: DiscoveryJob) -> list[EvidenceBatch]:
            # Simulate cancellation occurring while source 1 ran
            with psycopg.connect(DATABASE_URL, autocommit=True) as cancel_conn:
                with cancel_conn.cursor() as c:
                    c.execute("UPDATE discovery_jobs SET status = 'cancelled' WHERE id = %s", (job.id,))
            return super().run(job)

    source1 = CancellingSource(name="source1", source_family="search_engine", return_batches=[batch])
    source2 = MockSource(name="source2", source_family="ats", return_batches=[])

    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=[source1, source2],
        worker_id="test-worker-cancel",
    )

    handled = worker.run_once()
    assert handled is True
    # Source 2 should NEVER have been called because worker saw cancellation!
    assert source2.called_with_job is None

    # Job remains cancelled
    with db_conn.cursor() as cur:
        cur.execute("SELECT status FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row[0] == "cancelled"


def test_worker_all_sources_fail_marks_job_failed(db_conn, test_user_id):
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    client = IngestionClient("http://localhost:8080", "secret", client=httpx.Client())
    source_fail1 = MockSource(name="fail1", source_family="ats", should_fail=True)
    source_fail2 = MockSource(name="fail2", source_family="search_engine", should_fail=True)

    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=[source_fail1, source_fail2],
        worker_id="test-worker-all-fail",
    )

    handled = worker.run_once()
    assert handled is True

    with db_conn.cursor() as cur:
        cur.execute("SELECT status FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row[0] == "failed"


def test_worker_heartbeat_loop_updates_db(db_conn, test_user_id):
    import time
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    class SlowSource(MockSource):
        def run(self, job: DiscoveryJob) -> list[EvidenceBatch]:
            # Sleep slightly longer than heartbeat interval to ensure heartbeat fires
            time.sleep(0.3)
            return []

    client = IngestionClient("http://localhost:8080", "secret", client=httpx.Client())
    slow_source = SlowSource(name="slow", source_family="test")

    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=[slow_source],
        worker_id="test-worker-hb",
        lease_seconds=60,
        heartbeat_interval_seconds=0.1,  # Fast heartbeat for testing
    )

    handled = worker.run_once()
    assert handled is True

    with db_conn.cursor() as cur:
        cur.execute("SELECT last_heartbeat_at FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row is not None
        assert row[0] is not None


def test_worker_run_loop_with_max_jobs(db_conn, test_user_id):
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    client = IngestionClient("http://localhost:8080", "secret", client=httpx.Client())
    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=[],
        worker_id="test-worker-loop",
        poll_interval_seconds=0.05,
    )

    worker.run(max_jobs=1)

    with db_conn.cursor() as cur:
        cur.execute("SELECT status FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row[0] == "completed"


def test_worker_fatal_exception_marks_job_failed(db_conn, test_user_id, monkeypatch):
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    client = IngestionClient("http://localhost:8080", "secret", client=httpx.Client())

    # Simulate fatal crash when accessing sources or inside process loop before source_statuses populated
    class CrashingSourcesList(list):
        def __iter__(self):
            raise RuntimeError("Fatal database corruption or memory crash before sources run")

    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=CrashingSourcesList(),
        worker_id="test-worker-crash",
    )

    handled = worker.run_once()
    assert handled is True

    # Must be marked failed, NOT completed!
    with db_conn.cursor() as cur:
        cur.execute("SELECT status, error FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row[0] == "failed"
        assert "Fatal database corruption" in row[1]


def test_worker_generator_source_yields_batches(db_conn, test_user_id):
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    batch1 = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(job_id),
        source="gen_source",
        source_family="search_engine",
        observed_at=datetime.now(timezone.utc),
        companies=[CompanyEvidence(name="Company 1")],
        jobs=[],
    )
    batch2 = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(job_id),
        source="gen_source",
        source_family="search_engine",
        observed_at=datetime.now(timezone.utc),
        companies=[CompanyEvidence(name="Company 2")],
        jobs=[],
    )

    class GeneratorSource(Source):
        name = "gen_source"
        source_family = "search_engine"

        def run(self, job: DiscoveryJob):
            yield batch1
            yield batch2

    transport = httpx.MockTransport(
        lambda req: httpx.Response(
            200,
            json={
                "companies": [{"index": 0, "status": "accepted", "id": "uuid-1"}],
                "jobs": [],
            },
        )
    )
    client = IngestionClient("http://localhost:8080", "secret", client=httpx.Client(transport=transport))

    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=[GeneratorSource()],
        worker_id="test-worker-gen",
    )

    handled = worker.run_once()
    assert handled is True

    # Both yielded batches were processed: total 2 companies accepted
    with db_conn.cursor() as cur:
        cur.execute("SELECT status, company_count FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row[0] == "completed"
        assert row[1] == 2


def test_worker_lease_lost_aborts_execution(db_conn, test_user_id):
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    class StealingSource(MockSource):
        def run(self, job: DiscoveryJob) -> list[EvidenceBatch]:
            # Simulate another worker taking the lease or job being reassigned
            with psycopg.connect(DATABASE_URL, autocommit=True) as steal_conn:
                with steal_conn.cursor() as c:
                    c.execute("UPDATE discovery_jobs SET worker_id = 'new-worker' WHERE id = %s", (job.id,))
            # Give background heartbeat thread a moment to notice lease is lost
            time.sleep(0.25)
            return []

    client = IngestionClient("http://localhost:8080", "secret", client=httpx.Client())
    source1 = StealingSource(name="source1", source_family="test")
    source2 = MockSource(name="source2", source_family="test")

    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=[source1, source2],
        worker_id="test-worker-lost",
        heartbeat_interval_seconds=0.05,
    )

    handled = worker.run_once()
    assert handled is True

    # Source 2 should NOT have been executed because lease was lost!
    assert source2.called_with_job is None


def test_worker_async_generator_source_in_running_event_loop(db_conn, test_user_id):
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    batch1 = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(job_id),
        source="async_gen_source",
        source_family="search_engine",
        observed_at=datetime.now(timezone.utc),
        companies=[CompanyEvidence(name="Async Company 1")],
        jobs=[],
    )
    batch2 = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(job_id),
        source="async_gen_source",
        source_family="search_engine",
        observed_at=datetime.now(timezone.utc),
        companies=[CompanyEvidence(name="Async Company 2")],
        jobs=[],
    )

    class AsyncGeneratorSource(Source):
        name = "async_gen_source"
        source_family = "search_engine"

        async def run(self, job: DiscoveryJob):
            await asyncio.sleep(0.01)
            yield batch1
            await asyncio.sleep(0.01)
            yield batch2

    transport = httpx.MockTransport(
        lambda req: httpx.Response(
            200,
            json={
                "companies": [{"index": 0, "status": "accepted", "id": "uuid-async"}],
                "jobs": [],
            },
        )
    )
    client = IngestionClient("http://localhost:8080", "secret", client=httpx.Client(transport=transport))

    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=[AsyncGeneratorSource()],
        worker_id="test-worker-async-gen",
    )

    # Invoke run_once from inside an actively running asyncio event loop
    async def _run_in_active_loop():
        return worker.run_once()

    handled = asyncio.run(_run_in_active_loop())
    assert handled is True

    with db_conn.cursor() as cur:
        cur.execute("SELECT status, company_count FROM discovery_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        assert row[0] == "completed"
        assert row[1] == 2


def test_worker_staged_execution_candidate_to_site_to_ats(db_conn, test_user_id):
    """Test staged execution: candidates -> company sites (ceiling honored) -> ATS adapters (deduplicated)."""
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    # 1. Candidate source emits 3 companies, with duplicate domains
    candidate_batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(job_id),
        source="osm",
        source_family="open_dataset",
        observed_at=datetime.now(timezone.utc),
        companies=[
            CompanyEvidence(name="Alpha Corp", domain="alpha.example.com"),
            CompanyEvidence(name="Alpha Corp Branch", domain="alpha.example.com"),  # duplicate domain
            CompanyEvidence(name="Beta Corp", domain="beta.example.com"),
            CompanyEvidence(name="Gamma Corp", domain="gamma.example.com"),
        ],
        jobs=[],
    )

    candidate_source = MockSource(name="osm", source_family="open_dataset", return_batches=[candidate_batch])

    # Track ATS provider runs
    executed_greenhouse_boards: list[str] = []
    executed_lever_sites: list[str] = []

    class MockGHSource(Source):
        name = "greenhouse"
        source_family = "job_ats"
        def __init__(self, boards=None):
            self.boards = list(boards or [])
        def add_board(self, board_token, company_name=None, company_domain=None):
            if not any(b["board_token"] == board_token for b in self.boards):
                self.boards.append({"board_token": board_token, "company_name": company_name, "company_domain": company_domain})
        def run(self, job: DiscoveryJob):
            for b in self.boards:
                executed_greenhouse_boards.append(b["board_token"])
            return []

    class MockLevSource(Source):
        name = "lever"
        source_family = "job_ats"
        def __init__(self, sites=None):
            self.sites = list(sites or [])
        def add_site(self, site, company_name=None, company_domain=None):
            if not any(s["site"] == site for s in self.sites):
                self.sites.append({"site": site, "company_name": company_name, "company_domain": company_domain})
        def run(self, job: DiscoveryJob):
            for s in self.sites:
                executed_lever_sites.append(s["site"])
            return []

    # Configured ATS targets (one overlaps with what company site will discover)
    gh_source = MockGHSource(boards=[{"board_token": "overlap-gh", "company_name": "Alpha Corp", "company_domain": "alpha.example.com"}])
    lever_source = MockLevSource(sites=[{"site": "configured-lever", "company_name": "Other", "company_domain": "other.com"}])

    crawled_domains: list[str] = []

    async def mock_site_fetch(url: str) -> str:
        crawled_domains.append(url)
        if "alpha.example.com" in url:
            return """
            <html>
                <head><title>Alpha Corp</title></head>
                <body>
                    <a href="https://boards.greenhouse.io/overlap-gh">Greenhouse</a>
                    <a href="https://jobs.lever.co/discovered-lever">Lever</a>
                </body>
            </html>
            """
        return "<html><head><title>Other</title></head><body>No jobs</body></html>"

    transport = httpx.MockTransport(
        lambda req: httpx.Response(
            200,
            json={
                "companies": [{"index": i, "status": "accepted", "id": f"c-{i}"} for i in range(10)],
                "jobs": [],
            },
        )
    )
    client = IngestionClient("http://localhost:8080", "secret", client=httpx.Client(transport=transport))

    # Worker configured with max_company_sites = 2 (so gamma is capped out)
    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=[candidate_source, gh_source, lever_source],
        worker_id="test-worker-staged",
        max_company_sites=2,
        site_fetcher=mock_site_fetch,
    )

    handled = worker.run_once()
    assert handled is True

    # 1. Candidate source ran
    assert candidate_source.called_with_job is not None

    # 2. Company site crawl ran only for unique domains and respected ceiling of 2
    assert len(crawled_domains) == 2
    assert any("alpha.example.com" in u for u in crawled_domains)
    assert any("beta.example.com" in u for u in crawled_domains)
    assert not any("gamma.example.com" in u for u in crawled_domains)

    # 3. Greenhouse ran with overlap deduplicated (ran exactly once for overlap-gh)
    assert executed_greenhouse_boards == ["overlap-gh"]

    # 4. Lever ran both configured and discovered
    assert "configured-lever" in executed_lever_sites
    assert "discovered-lever" in executed_lever_sites
    assert len(executed_lever_sites) == 2


def test_worker_staged_cancellation_stops_before_next_stage(db_conn, test_user_id):
    """Test that cancelling a job during candidate stage stops execution before company site and ATS stages."""
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    class CancellingCandidateSource(Source):
        name = "osm"
        source_family = "open_dataset"
        def run(self, job: DiscoveryJob):
            # Cancel job in DB mid-run
            with psycopg.connect(DATABASE_URL, autocommit=True) as conn:
                with conn.cursor() as c:
                    c.execute("UPDATE discovery_jobs SET status = 'cancelled' WHERE id = %s", (job.id,))
            return [
                EvidenceBatch(
                    contract_version=1,
                    discovery_job_id=str(job.id),
                    source="osm",
                    source_family="open_dataset",
                    observed_at=datetime.now(timezone.utc),
                    companies=[CompanyEvidence(name="Cancelled Corp", domain="cancelled.com")],
                    jobs=[],
                )
            ]

    site_crawled = False
    async def mock_fetch(url: str) -> str:
        nonlocal site_crawled
        site_crawled = True
        return "<html></html>"

    gh_ran = False
    class MockGH(Source):
        name = "greenhouse"
        source_family = "job_ats"
        def run(self, job: DiscoveryJob):
            nonlocal gh_ran
            gh_ran = True
            return []

    transport = httpx.MockTransport(
        lambda req: httpx.Response(
            200,
            json={
                "companies": [{"index": 0, "status": "accepted", "id": "c-0"}],
                "jobs": [],
            },
        )
    )
    client = IngestionClient("http://localhost:8080", "secret", client=httpx.Client(transport=transport))
    worker = Worker(
        db_url=DATABASE_URL,
        client=client,
        sources=[CancellingCandidateSource(), MockGH()],
        worker_id="test-worker-cancel-staged",
        site_fetcher=mock_fetch,
    )

    handled = worker.run_once()
    assert handled is True
    assert site_crawled is False
    assert gh_ran is False


def test_worker_resolves_job_locations_before_submission(db_conn, test_user_id):
    job_id = uuid.uuid4()
    with db_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO discovery_jobs (id, user_id, status, lat, lng, radius_km, attempts, max_attempts)
            VALUES (%s, %s, 'pending', 37.7749, -122.4194, 10.0, 0, 3)
            """,
            (job_id, test_user_id),
        )

    job_evidence = TechnicalJobEvidence(
        company_name="Austin Tech",
        title="Python Engineer",
        location_raw="Austin, TX",
        work_arrangement=WorkArrangement.ON_SITE,
    )
    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=str(job_id),
        source="custom_jobs",
        source_family="job_ats",
        observed_at=datetime.now(timezone.utc),
        companies=[],
        jobs=[job_evidence],
    )

    submitted_batches: list[EvidenceBatch] = []

    class DummyClient:
        def submit(self, b):
            submitted_batches.append(b)
            return BatchResult(
                accepted_companies=0,
                accepted_jobs=len(b.jobs),
                total_accepted=len(b.jobs),
                record_results=[],
            )

    def geo_handler(req: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json=[{"lat": "30.2672", "lon": "-97.7431"}])

    geo_client = httpx.AsyncClient(transport=httpx.MockTransport(geo_handler))
    from nearhive_discovery.location import LocationResolver
    resolver = LocationResolver(http_client=geo_client)

    worker = Worker(
        db_url=DATABASE_URL,
        client=DummyClient(),
        sources=[MockSource(name="custom_jobs", source_family="job_ats", return_batches=[batch])],
        worker_id="test-worker-geo",
        location_resolver=resolver,
    )

    handled = worker.run_once()
    assert handled is True
    assert len(submitted_batches) == 1
    submitted_job = submitted_batches[0].jobs[0]
    assert submitted_job.lat == 30.2672
    assert submitted_job.lng == -97.7431
    assert submitted_job.metadata.get("location_resolution") == "geocoded"



