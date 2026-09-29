# Company Intelligence Phase 2 — Hiring Projections and Discovery Candidates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Return blended nearby and India-remote hiring candidates without joining canonical, observation, and community tables at request time.

**Architecture:** Python enqueues dirty companies in the canonical-write transaction. A separate worker claims a queue row, locks it while recomputing all four company projections in one transaction, and deletes it on success. A periodic sweep enqueues companies when time-dependent facts expire. A CLI rebuilds projections transactionally without clearing concurrent refresh requests. Go reads a blended candidate query over projections and active company locations. Its keyset cursor includes a data revision; a changed revision requires a new search. Projection failure leaves canonical writes intact and a visible retry/error state.

**Tech Stack:** PostgreSQL 17 / PostGIS 3.5, Python 3.12 + psycopg 3, Go 1.26 (read API), pytest against `NEARHIVE_TEST_DATABASE_URL`.

**Spec:** `docs/superpowers/specs/2026-09-28-company-intelligence-database-design.md` (§7; Phase 1 provenance tables from `docs/superpowers/plans/2026-09-28-company-intelligence-phase-1.md`, implemented in PR #30)

## Global Constraints

- Spec §4.2: UUID PKs; `TIMESTAMPTZ` UTC; lowercase stable slugs; `VARCHAR + CHECK` state sets (no PG enums); JSONB only for compact breakdowns (role-family counts), never core relationships.
- Spec §7.2: one row per grain — `company_hiring_summary` per company, `company_location_hiring_summary` per (company, location), `company_remote_hiring_summary` per (company, scope), `company_discovery_projection` per company. Every projection row carries `projection_version` (INTEGER) and `refreshed_at` (TIMESTAMPTZ).
- Spec §7.3: labels are query-time derivations, never stored company attributes. Use the exact spec names: `hiring_nearby_in_office`, `hiring_nearby_hybrid`, `hiring_remote_india`, `mixed_hiring`, `nearby_not_hiring`, and `insufficient_data`.
- Spec §7.6: refresh queue `company_projection_refresh_queue` keyed by company ID; `FOR UPDATE SKIP LOCKED` claim; recompute all four projections in one transaction; delete the queue row on success. Repeated writes collapse into one row. A newer write must never be erased by an older refresh.
- Spec §7.6: an idempotent full-rebuild command produces the same facts as incremental updates and preserves refresh requests made during the rebuild.
- Spec §7.7: normal projection freshness under 5 minutes; candidate query p95 ≤ 200 ms at target volume. Cursor pages are deterministic while the data revision is unchanged; return a restartable stale-cursor response if it changes.
- A qualifying job has `is_active=true` and `state='open'`, has at least one valid linked source record, and either (a) comes from an official ATS source checked within that source's configured refresh interval (24-hour fallback), or (b) is `posted_recently` with `posted_at` within 14 days and `posted_at_confidence >= 0.7`. An undated `observed_recently` job never qualifies by `state='open'` alone. Closed, stale, and inactive jobs never qualify. The same predicate drives all four projections.
- Fresh-volume schema: Compose initializes `python-discovery/schema.sql`; the approved database reset permits plain CREATE statements and no historical backfill. Use `make clean`/`make up` only during execution, not during plan review. Preserve the dirty checkout and do not commit without separate authorization.

## Review Focus

- A remote job with unknown eligibility (no `job_remote_eligibility` row) must never appear in India-remote summaries — pinned in Task 3 test `test_remote_unknown_eligibility_excluded`.
- A nearby company with an office but no local job must be labeled `nearby_not_hiring`, never `hiring_nearby_*` — pinned in Task 6 test `test_office_without_job_labeled_not_hiring`.
- A company's office must not localize an unrelated job: only jobs linked to the location or spatially resolved within the configured matching distance count toward `company_location_hiring_summary` — pinned in Task 2 test `test_office_does_not_localize_unrelated_job`.
- Rebuilding all projections must equal incremental refresh results (acceptance §7.8) — pinned in Task 5 test `test_full_rebuild_matches_incremental`.
- A projection refresh failure must leave the queue row for retry and never roll back canonical writes — pinned in Task 5 test `test_refresh_failure_leaves_queue_row`.
- A second canonical write during refresh must remain queued; expiry of a 14-day posting must remove its hiring counts without another ingestion batch — pinned in Task 5 tests.
- Cursor pages with ties/null dates must not duplicate or omit companies; a changed data revision must reject the old cursor — pinned in Task 6 tests.

---

### Task 1: Projection schema

**Files:**
- Modify: `python-discovery/schema.sql`

**Interfaces:**
- Produces: four projection tables, `company_projection_refresh_queue`, and one-row `company_projection_revision` — exact columns below; Tasks 2–6 rely on these names.

- [ ] **Step 1: Add the projection tables, refresh queue, and revision row to `schema.sql`** (after the canonical table extensions, before the DO/GRANT block)

```sql
CREATE TABLE company_hiring_summary (
    company_id UUID PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    open_job_count INTEGER NOT NULL DEFAULT 0,
    recently_posted_job_count INTEGER NOT NULL DEFAULT 0,
    technical_open_count INTEGER NOT NULL DEFAULT 0,
    in_office_count INTEGER NOT NULL DEFAULT 0,
    hybrid_count INTEGER NOT NULL DEFAULT 0,
    remote_count INTEGER NOT NULL DEFAULT 0,
    unknown_arrangement_count INTEGER NOT NULL DEFAULT 0,
    role_family_counts JSONB NOT NULL DEFAULT '{}',
    latest_job_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    activity_confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (activity_confidence BETWEEN 0 AND 1),
    next_refresh_at TIMESTAMPTZ,
    projection_version INTEGER NOT NULL DEFAULT 0,
    refreshed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX company_hiring_summary_due ON company_hiring_summary (next_refresh_at)
    WHERE next_refresh_at IS NOT NULL;

CREATE TABLE company_location_hiring_summary (
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
    active_in_office_count INTEGER NOT NULL DEFAULT 0,
    active_hybrid_count INTEGER NOT NULL DEFAULT 0,
    recently_posted_count INTEGER NOT NULL DEFAULT 0,
    technical_count INTEGER NOT NULL DEFAULT 0,
    role_family_counts JSONB NOT NULL DEFAULT '{}',
    latest_job_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1),
    projection_version INTEGER NOT NULL DEFAULT 0,
    refreshed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, location_id)
);

CREATE TABLE company_remote_hiring_summary (
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    scope_type VARCHAR(20) NOT NULL CHECK (scope_type IN ('global', 'country', 'region', 'timezone')),
    scope_code VARCHAR(100) NOT NULL,
    active_remote_count INTEGER NOT NULL DEFAULT 0,
    technical_count INTEGER NOT NULL DEFAULT 0,
    role_family_counts JSONB NOT NULL DEFAULT '{}',
    latest_job_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1),
    projection_version INTEGER NOT NULL DEFAULT 0,
    refreshed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, scope_type, scope_code)
);
CREATE INDEX company_remote_summary_country
    ON company_remote_hiring_summary (scope_type, scope_code, active_remote_count DESC, refreshed_at);

CREATE TABLE company_discovery_projection (
    company_id UUID PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL,
    normalized_name VARCHAR(500) NOT NULL,
    domain VARCHAR(255),
    industry TEXT,
    employee_count TEXT,
    company_confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (company_confidence BETWEEN 0 AND 1),
    active_hiring_total INTEGER NOT NULL DEFAULT 0,
    latest_hiring_at TIMESTAMPTZ,
    location_checked_until_at TIMESTAMPTZ,
    job_checked_until_at TIMESTAMPTZ,
    projection_version INTEGER NOT NULL DEFAULT 0,
    refreshed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE company_projection_refresh_queue (
    company_id UUID PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    reasons JSONB NOT NULL DEFAULT '[]',
    first_requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    lease_owner VARCHAR(100),
    lease_expires_at TIMESTAMPTZ,
    last_error TEXT
);
CREATE INDEX refresh_queue_claim
    ON company_projection_refresh_queue (available_at);

CREATE TABLE company_projection_revision (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    revision BIGINT NOT NULL DEFAULT 0
);
INSERT INTO company_projection_revision (singleton) VALUES (true);
```

- [ ] **Step 2: Verify schema applies cleanly**

Run: `make clean`, `make up`, then `make ps` and `docker compose exec -T db psql -U postgres -d nearhive -c '\dt'`.
Expected: all six new tables listed; services healthy. This reset is explicitly approved for disposable data. The test also catches invalid index predicates.

---

### Task 2: Projection computation

**Files:**
- Create: `python-discovery/nearhive_discovery/projections.py`
- Test: `python-discovery/tests/test_projections.py`

**Interfaces:**
- Consumes: schema tables from Task 1; canonical tables `technical_job_postings`, `job_locations`, `job_remote_eligibility`, `companies`, `locations`.
- Produces:
  - `compute_company_projections(conn, company_id: str, projection_version: int, as_of: datetime | None = None) -> dict[str, int]` — deletes obsolete location/scope rows, upserts current rows in all four tables, and returns row counts. Evaluate all time cutoffs against one `as_of` value (UTC now by default); the caller owns the transaction.

- [ ] **Step 1: Write the failing integration test** in `tests/test_projections.py` (existing uuid-suffix + `NEARHIVE_TEST_DATABASE_URL` skipif pattern):

```python
def test_single_company_projections():
    # seed an active company office, one in-office job linked/spatially matched
    # to it, and one remote job with IN eligibility; both have qualifying evidence
    # compute_company_projections(conn, company_id, 1)
    # assert company_hiring_summary has open_job_count=2, in_office_count=1, remote_count=1
    # assert company_remote_hiring_summary has (IN, active_remote_count=1)
    # assert company_discovery_projection has active_hiring_total=2
```

- [ ] **Step 2: Run to verify failure.** Expected: FAIL — `projections` module not found.
- [ ] **Step 3: Implement** one shared qualifying-job selection using the Global Constraints predicate. Require a valid linked `source_record` and derive official ATS freshness from `data_sources.refresh_interval_seconds` (24-hour fallback). For a recently posted fallback, require `posted_at > as_of - interval '14 days'` and confidence at least 0.7. `state='open'` or `observed_recently` alone is insufficient. Use `COUNT(DISTINCT job_id)` so multiple job locations or both `GLOBAL` and `IN` scopes do not inflate company totals. Compute all tables from this same selection; delete location/scope rows that no longer qualify. `activity_confidence` is the mean of qualifying jobs; `role_family_counts` is a bounded JSONB map of family ID to distinct-job count.
- [ ] **Step 4: Locality and freshness rules.** A local summary includes active in-office/hybrid jobs only when an active `job_locations` row has `company_location_id` equal to the office, or its point is within 500 m of the office point. Require the office to be active, have coordinates, and have `presence_type <> 'job_location_only'`. `company_hiring_summary.next_refresh_at` is the earliest future expiry of a counted 14-day posting or source-check window; its due index lets the worker enqueue time-based changes. Set discovery `location_checked_until_at` from the latest `sightings` check linked to a company location and its `data_sources` interval; set `job_checked_until_at` from the latest valid `job_source_links`/`source_records` check and its source interval. Use 24 hours when an interval is null; leave the deadline null when no check exists. An expired or missing check can produce `insufficient_data` at query time.
- [ ] **Step 5: Write negative tests** `test_office_does_not_localize_unrelated_job`, `test_observed_undated_and_closed_jobs_excluded`, and `test_duplicate_job_locations_count_once`. The first uses a job point beyond 500 m with no company-location link; the second uses weak/closed evidence; the third gives one job two matching `job_locations` rows.
- [ ] **Step 6: Run** the focused projection tests against the fresh test database.

---

### Task 3: Remote eligibility in projections

**Files:**
- Modify: `python-discovery/nearhive_discovery/projections.py`
- Test: `python-discovery/tests/test_projections.py`

**Interfaces:**
- Consumes: `compute_company_projections` (Task 2), `job_remote_eligibility` (Phase 1).
- Produces: remote summary rows only for remote jobs with an explicit active eligibility scope. A job with both `GLOBAL` and `IN` contributes once to company totals and once to each distinct scope row.

- [ ] **Step 1: Write the failing test** `test_remote_unknown_eligibility_excluded`: company with a remote job that has NO `job_remote_eligibility` row and one remote job with `IN` scope → `company_remote_hiring_summary` has exactly one row (IN, count 1), and `company_hiring_summary.remote_count` counts only the eligible job (Review Focus item).
- [ ] **Step 2: Run to verify failure.** Expected: FAIL — unknown-eligibility job counted.
- [ ] **Step 3: Complete** the shared qualifying-job selection so remote jobs with no active eligibility are excluded from all hiring counts. Group scope rows by `(scope_type, scope_code)`; India eligibility is exactly `(global, GLOBAL)` or `(country, IN)`. Delete remote scope rows when eligibility is removed or the job ceases to qualify.
- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Add** `test_scope_removed_clears_remote_projection` and `test_global_and_in_count_company_job_once`.

---

### Task 4: Dirty-company queue writes

**Files:**
- Modify: `python-discovery/nearhive_discovery/persistence.py`
- Test: `python-discovery/tests/test_persistence.py`

**Interfaces:**
- Consumes: `company_projection_refresh_queue` (Task 1), `persist()` transaction (Phase 1).
- Produces:
  - `PostgresPersistence._mark_company_dirty(company_id: str, reason: str) -> None` — upserts one queue row per company, appends a reason (last 20), updates `last_requested_at` and `available_at` to now, and retains `first_requested_at`.
  - `persist()` marks every touched company inside its existing transaction. If an upsert reassigns a job to another company, mark both old and new company IDs.

- [ ] **Step 1: Write the failing tests** `test_persist_marks_company_dirty` and `test_reassigned_job_marks_both_companies`: repeated evidence makes one queue row per company; a job changing company dirties both IDs.
- [ ] **Step 2: Run to verify failure.** Expected: FAIL — queue empty.
- [ ] **Step 3: Implement** `_mark_company_dirty` and wire it into both loops of `persist()` inside `self.conn.transaction()`. Use reason `evidence_batch:<source>`; read a job's previous company ID before an upsert can reassign it. A conflicting queue upsert must not clear an active worker lease; a new canonical write may reset exhausted attempts so fresh evidence is retried. When a fresh resolved company-location sighting is accepted, set that location's `status='active'` (leave `presence_type='probable_office'` and confidence unchanged); mark the company dirty on location changes. This makes active mean currently observed, not independently verified.
- [ ] **Step 4: Run to verify pass** plus full persistence suite.
- [ ] **Step 5: Add** `test_fresh_office_becomes_active_without_becoming_confirmed`.

---

### Task 5: Projection worker and full rebuild

**Files:**
- Create: `python-discovery/nearhive_discovery/projection_worker.py`
- Modify: `python-discovery/nearhive_discovery/worker.py` (start the projection worker thread in `main()`)
- Test: `python-discovery/tests/test_projection_worker.py`

**Interfaces:**
- Consumes: `compute_company_projections` (Task 2/3), `company_projection_refresh_queue` (Task 1).
- Produces:
  - `claim_next_dirty_company(conn, worker_id: str, lease_seconds: int = 60) -> str | None` — atomically claims one due row with `FOR UPDATE SKIP LOCKED`, `attempts < max_attempts`, and an absent/expired lease; commits the lease and increments attempts.
  - `run_projection_refresh_once(conn, worker_id: str) -> bool` — in a second transaction, takes the shared projection advisory lock, locks the claimed queue row, verifies lease ownership, recomputes, deletes that row, and increments `company_projection_revision.revision` atomically. On failure, rolls back projection writes with a savepoint, keeps the row, clears its lease, records the error, and schedules bounded exponential backoff (60 seconds to 15 minutes). Rows at `max_attempts` stay visible with `last_error` and are not reclaimed automatically.
  - `ProjectionWorker.run(stop_event, poll_interval=5)` — mirrors the existing `Worker` stop-event lifecycle. Each pass also enqueues companies with `company_hiring_summary.next_refresh_at <= now()` using `ON CONFLICT DO NOTHING`; the due index prevents scanning every company, and an existing failed row keeps its backoff.
  - `rebuild_all_projections(conn, projection_version: int, as_of: datetime | None = None) -> int` — takes the same advisory lock, recomputes every company in one transaction at one UTC `as_of`, validates company row counts, and increments the revision once. It never clears the refresh queue; canonical writes during rebuild remain queued for a later incremental refresh.
  - CLI: `python -m nearhive_discovery.projection_worker rebuild` uses `settings.database_url` and exits nonzero on failed validation.

- [ ] **Step 1: Write the failing test** `test_refresh_worker_processes_queue`: seed a company with jobs (via persist), run `run_projection_refresh_once` → projections populated, queue row gone.
- [ ] **Step 2: Run to verify failure.** Expected: FAIL — projection worker module missing.
- [ ] **Step 3: Implement** the claim and processing transactions per the Interfaces block. Keep the queue-row lock through recomputation so an upsert made during refresh waits, then creates a new queue row after success. Verify this interleaving explicitly; a lease alone does not protect against losing an update. Use one fixed PostgreSQL advisory-lock key shared with rebuild so the two projection writers cannot overlap.
- [ ] **Step 4: Write the failure test** `test_refresh_failure_leaves_queue_row`: force `compute_company_projections` to raise (e.g. monkeypatch it to throw), run `run_projection_refresh_once` → queue row still present with `attempts=1` and `last_error` set (Review Focus item).
- [ ] **Step 5: Write** `test_write_during_refresh_remains_queued`, `test_expired_posting_is_requeued_without_ingestion`, and `test_exhausted_refresh_remains_visible`. Use two connections for the write/refresh interleaving.
- [ ] **Step 6: Write the rebuild test** `test_full_rebuild_matches_incremental`: seed two companies; compare factual projection columns after incremental and full rebuild at a fixed `as_of` time (exclude `refreshed_at` and revision metadata). Assert a queue write made during rebuild remains queued.
- [ ] **Step 7: Wire** `ProjectionWorker` into `worker.py` `main()` with the same `stop_event` and join/close behavior as the discovery worker; do not leave an unobserved daemon thread.
- [ ] **Step 8: Add and invoke the CLI** with `docker compose exec -T python-discovery python -m nearhive_discovery.projection_worker rebuild`; verify exit status and row counts.
- [ ] **Step 9: Run** focused projection and worker tests, then `make test-discovery`.

---

### Task 6: Candidate query in Go

**Files:**
- Modify: `internal/store/discovery.go` (candidate SQL beside existing discovery search)
- Modify: `internal/store/store.go` and `internal/store/store_mock.go` (interface and mock)
- Modify: `internal/model/models.go` (candidate result)
- Modify: `internal/api/router.go` and `internal/api/handlers_search.go` (protected `GET /api/v1/search/candidates`)
- Test: `internal/store/postgres_integration_test.go`; create `internal/api/handlers_search_test.go`

**Interfaces:**
- Consumes: Task 1 projection tables, `locations`, `role_families`, and `company_projection_revision`.
- Produces:
  - `Store.SearchBlended(ctx, lat, lng, radiusMeters float64, opts BlendedSearchOpts) ([]model.BlendedCandidateResult, int64, error)`; opts contains `Limit`, typed `Cursor`, `WorkArrangement`, `RoleFamilySlug`, `MinConfidence`, and `HiringState` (`any`, `hiring`, `not_hiring`). Return the current data revision with results; fetch `Limit+1` to form `next_cursor`, with no total-count query.
  - `model.BlendedCandidateResult` contains company ID/name/domain/industry, exact spec labels, nullable `DistanceM` and `LatestHiringAt`, active hiring count, and company confidence. Remote-only rows have null distance.
  - Cursor is base64url JSON with ranking algorithm version (`1` for this query), data revision, request-filter hash, active hiring count, nullable latest-hiring timestamp, and company ID. Decode and validate all fields; a changed data revision returns HTTP 409 `STALE_CURSOR` with instructions to restart from page one. A filter hash mismatch or malformed cursor returns HTTP 400.

- [ ] **Step 1: Add failing integration tests** for local-only, remote-only (`GLOBAL` and `IN`), mixed, office-without-job, and minimum distance across multiple offices. Include a remote-only company with null distance and an unverified-but-active probable office.
- [ ] **Step 2: Implement the SQL** with these CTE grains: nearby active company offices (`locations.status='active'`, `presence_type <> 'job_location_only'`, coordinates, `ST_DWithin` and `ST_Distance`); left-joined local hiring summaries; India remote summaries; `UNION ALL` of local office candidates and remote candidates; then group once by company. Keep an office candidate even when both local counts are zero so `nearby_not_hiring` can be derived. Deduplicate the three base labels. Add `mixed_hiring` when at least two base labels apply; add `nearby_not_hiring` only when an office matches and no qualifying nearby or India-remote label applies. Add `insufficient_data` when a returned nearby office has no current location or job check, or a returned remote candidate has no current job check; a null/expired `checked_until_at` is not current. `MIN(distance_m)` uses local rows; remote-only stays null.
- [ ] **Step 3: Apply filters before final grouping.** Arrangement selects matching local/remote branches. Role-family slug resolves to `role_families.id` and checks the relevant summary's `role_family_counts`; an unknown slug returns HTTP 400. `MinConfidence` filters the company projection. Hiring state filters after labels are derived. Do not advertise a filter that is ignored. Empty-office candidates are retained only when no arrangement or role-family filter excludes them.
- [ ] **Step 4: Implement one keyset order and predicate.** Order by `active_hiring_total DESC`, `COALESCE(latest_hiring_at, '0001-01-01T00:00:00Z'::timestamptz) DESC`, `company_id ASC`. For cursor values `(count, time, id)`, the next row satisfies `total < count OR (total = count AND sort_time < time) OR (total = count AND sort_time = time AND company_id > id)`. No `OFFSET`. Read revision and candidate rows in one read-only repeatable-read transaction; a projection refresh increments revision in its write transaction, so a cursor never silently mixes revisions. Hash the canonicalized lat/lng/radius and all filters into the cursor.
- [ ] **Step 5: Add the protected route and handler** with `lat`, `lng`, `radius_km`, `limit`, `cursor`, `arrangement`, `role_family`, `min_confidence`, and `hiring_state`. Validate ranges and respond using the API's existing JSON error shape. Expose `next_cursor` only when an extra row exists.
- [ ] **Step 6: Test** `test_office_without_job_labeled_not_hiring`, `test_cursor_ties_and_null_dates`, `test_cursor_rejected_after_revision_change`, `test_cursor_rejected_for_other_filters`, `test_filters_match_returned_labels`, and the auth/error cases. Run focused Go store and API tests.

---

### Task 7: Integration and performance gate

**Files:**
- Modify: `Makefile` (add `test-projections` target)
- Modify: `docs/superpowers/README.md` (mark Phase 2 plan implemented after green)

**Interfaces:**
- Consumes: everything above.
- Produces: confidence that the candidate query meets §7.7 latency and correctness gates.

- [ ] **Step 1: Prepare a schema-initialized test DB after the approved fresh-volume reset.** `make clean`, `make up`, `docker compose exec -T db createdb -U postgres nearhive_test`, then `docker compose exec -T db psql -U postgres -d nearhive_test -f /docker-entrypoint-initdb.d/01-nearhive.sql`. The schema file runs in the test database too; both languages' integration tests otherwise skip or fail.
- [ ] **Step 2: Add `make test-projections`** to run the focused Python tests against `nearhive_test`; keep `make test-discovery` as the broader Python gate. Run the projection suite:

Run: `cd python-discovery && NEARHIVE_TEST_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/nearhive_test uv run pytest -q tests/test_projections.py tests/test_projection_worker.py tests/test_persistence.py tests/test_operations_api.py`
Expected: PASS. Then run `make test-discovery`; investigate any failures before marking the phase implemented.

- [ ] **Step 3: Run Go tests with integration enabled**

Run: `NEARHIVE_TEST_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/nearhive_test make test`
Expected: PASS, including the blended-search integration tests. Confirm those tests ran rather than skipped.

- [ ] **Step 4: Live smoke check**

Use the fresh stack from Step 1. Register a temporary test account through `/api/v1/auth/register` and retain its returned JWT; launch a discovery run, wait for completion, then request candidates with `Authorization: Bearer <token>`:

```bash
curl -fsS -H "Authorization: Bearer $NEARHIVE_TEST_TOKEN" \
  "http://localhost:8080/api/v1/search/candidates?lat=12.97&lng=77.59&radius_km=10"
```

Expected: HTTP 200 JSON; seeded local and remote companies have the exact spec labels, remote-only distance is null, and an office-only company has `nearby_not_hiring`.

- [ ] **Step 5: Verify the queue and rebuild CLI**

Run: `docker compose exec -T python-discovery python -m nearhive_discovery.projection_worker rebuild`, then query queue rows grouped by `attempts >= max_attempts` and inspect `last_error`. Expected: healthy rows drain after discovery; failed rows stay visible, and rebuild exits 0 without dropping pending requests.

- [ ] **Step 6: Measure the query target.** Seed representative projections for at least 50,000 companies, 150,000 locations, and 100,000 India-eligible remote scope rows, with aggregate counts representing 500,000 jobs; the read benchmark need not insert canonical jobs. Warm the database, run at least 100 local/remote/blended queries across dense and sparse centers, record `EXPLAIN (ANALYZE, BUFFERS)` and database-only duration, and calculate p95. Inspect use of spatial and scope indexes. Acceptance: p95 ≤ 200 ms; record fixture size, machine/container resources, and result in the plan or README.

- [ ] **Step 7: Update `docs/superpowers/README.md`** only after all correctness, rebuild, freshness, cursor, and measured performance gates pass. Do not commit plan or implementation files without separate authorization.

---

## Deferred (documented deviations)

- Source SLA fallback: sources without `refresh_interval_seconds` use 24 hours until source-specific intervals are configured. This may mark a source stale earlier than its eventual policy.
- Confirmed-open site jobs older than 14 days are excluded until ingestion records an explicit open confirmation; official ATS jobs can qualify while their source check is fresh. This conservative rule prevents weak undated observations from inflating hiring.
- Ranking uses qualifying hiring count, then latest hiring timestamp, then company ID. Distance decay, confidence weighting, and reputation completeness remain outside this first ranking version; distance and company confidence remain available in results and filters.
- `company_discovery_projection.employee_count`/`industry` are copied from `companies` at refresh time (no enrichment pipeline yet).

## Out of scope (later plans)

- Phase 3 — user preferences, saved/dismissed filtering, moderation, unique identity.
- Phase 4 — reviews, compensation, interview intelligence, reputation in ranking.
- Elasticsearch/Redis/Kafka (spec §16 explicit non-goals).
