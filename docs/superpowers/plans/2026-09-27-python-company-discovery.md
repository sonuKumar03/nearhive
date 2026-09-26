# Python Company Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a locally runnable Python worker that discovers nearby companies from public sources, enriches them with technical jobs from the previous 14 days, and sends evidence through Go without modifying the existing Go crawler packages.

**Architecture:** The Go API owns authenticated discovery jobs, canonical storage, matching, verification, and search. Python workers claim a separate PostgreSQL queue, crawl ordinary pages with Scrapy and JavaScript pages through a bounded Playwright fallback, then submit versioned evidence batches to a protected Go endpoint. Existing `internal/scraper` and `internal/crawler` code remains unchanged.

**Tech Stack:** Go 1.26, PostgreSQL/PostGIS 17, Python 3.12+, Scrapy, scrapy-playwright, Playwright, psycopg 3, httpx, pytest, Next.js 15, React 19, TanStack Query 5, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-09-27-python-company-discovery-design.md`

## Global Constraints

- Do not modify files under `internal/scraper/` or `internal/crawler/`, or change `cmd/crawler/main.go`.
- Retain every discovered company; technical-company classification is not an admission gate for Python evidence.
- “Recently hiring” requires a trustworthy publication time within 14 days; missing dates mean only “recently observed.”
- Job arrangement is exactly `in_office`, `hybrid`, `remote`, or `unknown`.
- Location presence is exactly `confirmed_office`, `probable_office`, or `job_location_only`.
- Python never writes canonical company, location, sighting, or technical-job rows directly.
- Respect robots directives, access controls, per-host rate limits, and public-only HTTP/HTTPS URLs.
- Do not add Redis, Kafka, proxy rotation, CAPTCHA bypass, an LLM, or automatic fuzzy merges.
- CI must use fixtures and local fake sources; it must not scrape third-party websites.

## Review Focus

- **Duplicate delivery:** submitting the same source ID or content hash twice must update observation time without creating duplicate evidence or jobs; covered in Task 4.
- **Correlated evidence:** ten records from one source family must not produce confirmed-office confidence; covered in Task 3.
- **Unsafe URLs:** loopback, private, link-local, non-HTTP, and public-to-private redirects must be rejected before fetching; covered in Task 6.
- **Ambiguous dates:** missing or untrustworthy job dates must never count toward the 14-day hiring flag; covered in Task 8.
- **Worker death:** an expired lease must be reclaimable without two workers processing the same live job; covered in Task 5.

---

## File Map

### Go and database

- `migrations/000011_python_discovery.up.sql` / `.down.sql` — discovery queue, source runs, evidence lineage, location presence, and technical jobs.
- `internal/model/discovery.go` — discovery jobs, source runs, ingestion contracts, technical jobs, and enum constants.
- `internal/store/discovery.go` — PostgreSQL implementation for discovery jobs, ingestion upserts, evidence queries, and technical-job reads.
- `internal/store/store.go` — add focused `DiscoveryStore` and `TechnicalJobStore` interfaces.
- `internal/store/store_mock.go` — in-memory behavior needed by API and verifier tests.
- `internal/api/handlers_discovery.go` — authenticated job endpoints and internal batch ingestion.
- `internal/api/router.go` — register discovery routes without changing existing job routes.
- `internal/config/config.go` — internal worker token and ingestion limits.
- `internal/verifier/discovery.go` — Python-evidence processing and independent-family location verification.
- `internal/verifier/matcher.go` — add strict discovery matching while preserving the crawler matching path.
- `internal/verifier/verifier.go` — share matching/merging without using the tech classifier for discovery evidence.
- `internal/store/postgres.go` — extend nearby search projection only; keep new discovery queries in `discovery.go`.
- `cmd/nearhive/main.go` — construct and inject the discovery handler; no crawler behavior changes.

### Python

- `python-discovery/pyproject.toml` — Python package and test dependencies.
- `python-discovery/Dockerfile` — local worker image and Playwright Chromium runtime.
- `python-discovery/nearhive_discovery/contracts.py` — immutable input/output dataclasses and JSON conversion.
- `python-discovery/nearhive_discovery/settings.py` — environment parsing with safe defaults.
- `python-discovery/nearhive_discovery/queue.py` — job leasing, heartbeat, cancellation, completion, and source-run updates.
- `python-discovery/nearhive_discovery/client.py` — bounded batch submission to Go.
- `python-discovery/nearhive_discovery/worker.py` — source orchestration and graceful shutdown.
- `python-discovery/nearhive_discovery/http.py` — Scrapy settings, URL safety, and Playwright fallback controls.
- `python-discovery/nearhive_discovery/classify.py` — deterministic technical-role, date-confidence, and arrangement rules.
- `python-discovery/nearhive_discovery/sources/base.py` — minimal adapter protocol.
- `python-discovery/nearhive_discovery/sources/jsonld.py` — reusable structured-data extraction.
- `python-discovery/nearhive_discovery/sources/configured_directory.py` — ordinary public directory adapter.
- `python-discovery/nearhive_discovery/sources/company_site.py` — bounded official-site and careers discovery.
- `python-discovery/nearhive_discovery/sources/greenhouse.py` / `lever.py` — public ATS adapters.
- `python-discovery/tests/fixtures/` — saved HTML/JSON/JavaScript responses.

### Frontend and local runtime

- `web/src/types/index.ts` — discovery, presence, and technical-job types.
- `web/src/hooks/useDiscoveryJobs.ts` — create/list/poll/cancel discovery jobs.
- `web/src/components/drawers/ScrapeModal.tsx` — start Python discovery from existing location controls.
- `web/src/components/scrapers/BackgroundScrapeWidget.tsx` — source-level discovery progress.
- `web/src/components/sidebar/CompanyCard.tsx` and `web/src/components/drawers/CompanyDetailDrawer.tsx` — presence and recent-job labels.
- `docker-compose.yml`, `.env.example`, `Makefile` — run and verify the Python worker locally.

---

### Task 1: Add the discovery schema, models, and store contracts

**Files:**
- Create: `migrations/000011_python_discovery.up.sql`
- Create: `migrations/000011_python_discovery.down.sql`
- Create: `internal/model/discovery.go`
- Create: `internal/store/discovery.go`
- Create: `internal/store/discovery_test.go`
- Modify: `internal/model/models.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_mock.go`

**Interfaces:**
- Produces: `model.DiscoveryJob`, `model.DiscoverySourceRun`, `model.DiscoveryBatch`, `model.CompanyEvidence`, `model.TechnicalJobEvidence`, `model.TechnicalJobPosting`.
- Produces: `DiscoveryStore.CreateDiscoveryJob`, `GetDiscoveryJob`, `ListDiscoveryJobs`, `CancelDiscoveryJob`, and source-run methods.
- Produces: `TechnicalJobStore.GetTechnicalJobsByCompany(ctx, companyID, since)`.

- [ ] **Step 1: Write failing migration/store tests**

Add `TestDiscoveryStore_CreateListCancel`, `TestDiscoveryStore_SourceRunUpsert`, and `TestTechnicalJobStore_RecentByCompany`. Assert user scoping, valid status transitions, the three presence values, and the four arrangement values.

- [ ] **Step 2: Run the focused tests and confirm failure**

Run: `go test ./internal/store -run 'TestDiscoveryStore|TestTechnicalJobStore' -v`  
Expected: FAIL because the migration, models, and methods do not exist.

- [ ] **Step 3: Add schema and exact model constants**

Define `DiscoveryStatus` values `pending`, `running`, `completed`, `partial`, `failed`, `cancelled`; `PresenceType` values from Global Constraints; and `WorkArrangement` values from Global Constraints. Add `PresenceType` to `model.Location` and presence/job summary fields to `model.CompanySearchResult`.

- [ ] **Step 4: Implement discovery store methods in `internal/store/discovery.go`**

Keep queue rows separate from `scrape_jobs`. Make user-facing getters require `userID`; cancellation may change only `pending` or `running` jobs. Keep canonical ingestion methods out of this step.

- [ ] **Step 5: Run store and migration checks**

Run: `go test ./internal/store ./migrations -v`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add migrations/000011_python_discovery.* internal/model internal/store
git commit -m "feat: add Python discovery data model"
```

### Task 2: Add authenticated discovery-job APIs

**Files:**
- Create: `internal/api/handlers_discovery.go`
- Create: `internal/api/handlers_discovery_test.go`
- Modify: `internal/api/router.go`
- Modify: `cmd/nearhive/main.go`

**Interfaces:**
- Consumes: `store.DiscoveryStore` from Task 1.
- Produces: `NewDiscoveryHandler(s store.Store, engine *verifier.Engine, workerToken string) *DiscoveryHandler`.
- Produces: `POST/GET /api/v1/discovery/jobs`, `GET /api/v1/discovery/jobs/{id}`, and `POST /api/v1/discovery/jobs/{id}/cancel`.

- [ ] **Step 1: Write failing API tests**

Add tests proving authentication is required; latitude accepts `[-90,90]`; longitude accepts `[-180,180]`; radius must be `(0,100]` km; a job is created as `pending`; another user cannot read/cancel it; cancellation is idempotent.

- [ ] **Step 2: Run and confirm route failures**

Run: `go test ./internal/api -run TestDiscoveryJobs -v`  
Expected: FAIL with 404 or missing handler symbols.

- [ ] **Step 3: Implement handler methods**

Add `CreateJob`, `ListJobs`, `GetJob`, and `CancelJob`. Extract the authenticated user ID through the existing middleware context rather than accepting it in JSON.

- [ ] **Step 4: Inject the handler into the router**

Change `NewRouterWithQueue` to accept `discoveryHandler *DiscoveryHandler`; keep `NewRouter` usable by tests by constructing a handler with the provided store and no internal token. Register only authenticated discovery-job routes in this task; Task 4 adds internal ingestion.

- [ ] **Step 5: Run API and full Go tests**

Run: `go test ./internal/api ./...`  
Expected: PASS; existing `/api/v1/jobs` tests remain unchanged.

- [ ] **Step 6: Commit**

```bash
git add internal/api cmd/nearhive/main.go
git commit -m "feat: expose company discovery jobs"
```

### Task 3: Verify all companies using independent evidence families

**Files:**
- Create: `internal/verifier/discovery.go`
- Create: `internal/verifier/discovery_test.go`
- Modify: `internal/verifier/verifier.go`
- Modify: `internal/verifier/matcher.go`
- Modify: `internal/store/discovery.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_mock.go`

**Interfaces:**
- Produces: `Engine.ProcessDiscoverySighting(ctx context.Context, s model.Sighting) error`.
- Produces: `Engine.RecalculateLocationEvidence(ctx context.Context, companyID uuid.UUID) error`.
- Produces: `Matcher.FindDiscoveryMatch(ctx context.Context, s model.Sighting) (*MatchResult, error)` using domain or corroborated exact identity, never fuzzy name alone.
- Produces: a store query returning distinct source families and evidence types per spatially matched location.

- [ ] **Step 1: Write failing verifier tests**

Add tests proving a hospital or retailer is retained, fuzzy name alone does not merge, ten `job_ats` sightings remain `probable_office`/`job_location_only`, and `official_site` plus `public_directory` yields `confirmed_office`, confidence `0.9`, and `verified=true`.

- [ ] **Step 2: Run and confirm current classifier behavior fails the new tests**

Run: `go test ./internal/verifier -run 'TestProcessDiscovery|TestIndependentEvidence' -v`  
Expected: FAIL because `ProcessDiscoverySighting` does not exist and current processing filters non-tech entities.

- [ ] **Step 3: Share matching and merging without the admission classifier**

Refactor only enough for `ProcessSighting` to preserve existing crawler behavior and `ProcessDiscoverySighting` to skip the tech admission filter. The discovery path uses `FindDiscoveryMatch`; both paths reuse the current merger.

- [ ] **Step 4: Implement deterministic presence recalculation**

Set `confirmed_office/0.9/verified=true` for two qualifying independent families, `probable_office/0.6/false` for one credible office family, and `job_location_only/0.4/false` when evidence is job-location-only. Count distinct family plus lineage, not row count.

- [ ] **Step 5: Run verifier and store tests**

Run: `go test ./internal/verifier ./internal/store -v`  
Expected: PASS, including existing Go crawler verifier tests.

- [ ] **Step 6: Commit**

```bash
git add internal/verifier internal/store
git commit -m "feat: verify companies from independent evidence"
```

### Task 4: Add idempotent internal batch ingestion

**Files:**
- Modify: `internal/api/handlers_discovery.go`
- Modify: `internal/api/handlers_discovery_test.go`
- Modify: `internal/api/router.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/store/discovery.go`
- Modify: `internal/store/discovery_test.go`
- Modify: `cmd/nearhive/main.go`
- Modify: `.env.example`

**Interfaces:**
- Consumes: `model.DiscoveryBatch` and `Engine.ProcessDiscoverySighting`.
- Produces: `POST /api/v1/internal/discovery/batches` with `X-NearHive-Worker-Token`.
- Produces: per-record `{index, status, id?, error?}` results and HTTP 200 for mixed accepted/rejected records.

- [ ] **Step 1: Write failing authentication and validation tests**

Cover missing/wrong token, constant-time comparison path, unknown contract version, payload over 2 MiB, more than 500 records, invalid coordinates/timestamps/URL schemes, and mixed valid/invalid records.

- [ ] **Step 2: Write failing idempotency tests**

Submit the same `(source, source_record_id)` twice, then repeat with a missing source ID and identical content hash. Assert one evidence/job row and an updated `last_seen_at` in both cases.

- [ ] **Step 3: Run and confirm failures**

Run: `go test ./internal/api ./internal/store -run 'TestDiscoveryIngest|TestDiscoveryIdempotency' -v`  
Expected: FAIL because ingestion and upserts do not exist.

- [ ] **Step 4: Implement bounded transactional ingestion**

Validate the envelope first, then process records independently. Upsert raw evidence, call `ProcessDiscoverySighting`, upsert technical jobs, and return per-record results. Never log tokens or job-description bodies.

- [ ] **Step 5: Register and wire the internal route**

Register `POST /api/v1/internal/discovery/batches` outside JWT authentication but behind exact worker-token validation. Pass the configured token and verifier engine from `cmd/nearhive/main.go` into `NewDiscoveryHandler`.

- [ ] **Step 6: Add configuration**

Load `DISCOVERY_WORKER_TOKEN`, `DISCOVERY_MAX_BATCH_RECORDS` default `500`, and `DISCOVERY_MAX_BODY_BYTES` default `2097152`. Production startup must reject an empty worker token; tests/development may omit it and leave the internal route disabled.

- [ ] **Step 7: Run Go verification**

Run: `go test ./...`  
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/api internal/config internal/store cmd/nearhive/main.go .env.example
git commit -m "feat: ingest Python discovery evidence"
```

### Task 5: Build the Python contracts, queue lease, and worker loop

**Files:**
- Create: `python-discovery/pyproject.toml`
- Create: `python-discovery/nearhive_discovery/__init__.py`
- Create: `python-discovery/nearhive_discovery/contracts.py`
- Create: `python-discovery/nearhive_discovery/settings.py`
- Create: `python-discovery/nearhive_discovery/queue.py`
- Create: `python-discovery/nearhive_discovery/client.py`
- Create: `python-discovery/nearhive_discovery/worker.py`
- Create: `python-discovery/tests/test_contracts.py`
- Create: `python-discovery/tests/test_queue.py`
- Create: `python-discovery/tests/test_worker.py`

**Interfaces:**
- Produces: `claim_job(conn, worker_id: str, lease_seconds: int) -> DiscoveryJob | None`.
- Produces: `heartbeat`, `is_cancelled`, `finish_job`, and `record_source_run` queue functions.
- Produces: `IngestionClient.submit(batch: EvidenceBatch) -> BatchResult`.
- Produces: `Worker.run_once() -> bool` and `Worker.run() -> None`.

- [ ] **Step 1: Add the package manifest and install test dependencies**

Create `pyproject.toml` for Python 3.12+ with Scrapy, scrapy-playwright, Playwright, psycopg 3, httpx, and pytest, then run `python -m pip install -e '.[dev]'` from `python-discovery/`.

- [ ] **Step 2: Write failing contract round-trip tests**

Load shared JSON fixtures for contract version `1`, all three presence values, all four arrangements, timezone-aware timestamps, and source lineage. Assert JSON emitted by Python matches the Go field names exactly.

- [ ] **Step 3: Write failing PostgreSQL lease tests**

With two connections, assert one pending row can be claimed once, a live lease cannot be stolen, an expired lease can be reclaimed, attempts increment, and cancellation is observed before another source starts.

- [ ] **Step 4: Run and confirm failures**

Run: `cd python-discovery && python -m pytest tests/test_contracts.py tests/test_queue.py tests/test_worker.py -q`  
Expected: FAIL because the package does not exist.

- [ ] **Step 5: Add the minimal package**

Use frozen dataclasses/enums; do not add Pydantic unless handwritten validation becomes measurably larger or inconsistent.

- [ ] **Step 6: Implement atomic lease and worker lifecycle**

Use one transaction containing `FOR UPDATE SKIP LOCKED`; set `running`, worker ID, lease expiry, heartbeat, and attempts atomically. Heartbeat periodically, check cancellation between sources, and mark `completed`, `partial`, or `failed` from source outcomes.

- [ ] **Step 7: Run Python tests**

Run: `cd python-discovery && python -m pytest -q`  
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add python-discovery
git commit -m "feat: add Python discovery worker core"
```

### Task 6: Enforce crawl policy and extract ordinary structured pages

**Files:**
- Create: `python-discovery/nearhive_discovery/http.py`
- Create: `python-discovery/nearhive_discovery/sources/__init__.py`
- Create: `python-discovery/nearhive_discovery/sources/base.py`
- Create: `python-discovery/nearhive_discovery/sources/jsonld.py`
- Create: `python-discovery/tests/test_http_policy.py`
- Create: `python-discovery/tests/test_jsonld.py`
- Create: `python-discovery/tests/fixtures/jsonld_company.html`
- Create: `python-discovery/tests/fixtures/jsonld_jobs.html`

**Interfaces:**
- Produces: `SourceAdapter.run(job: DiscoveryJob) -> AsyncIterator[EvidenceBatch]` protocol.
- Produces: `validate_public_url(url: str) -> NormalizedURL` and redirect revalidation.
- Produces: `extract_jsonld(response) -> tuple[list[CompanyEvidence], list[TechnicalJobEvidence]]`.

- [ ] **Step 1: Write failing URL-safety and robots tests**

Reject `file:`, credentials in URLs, localhost, RFC1918, link-local, IPv6 local/private, and a public URL redirecting to a private address. Assert `ROBOTSTXT_OBEY=True`, bounded response size, per-domain concurrency, delay, timeout, and identifiable user agent.

- [ ] **Step 2: Write failing JSON-LD fixture tests**

Assert `Organization`, `LocalBusiness`, `PostalAddress`, coordinates, and `JobPosting` are normalized; malformed JSON-LD is skipped without losing valid blocks.

- [ ] **Step 3: Run and confirm failures**

Run: `cd python-discovery && python -m pytest tests/test_http_policy.py tests/test_jsonld.py -q`  
Expected: FAIL because policy and extraction modules do not exist.

- [ ] **Step 4: Implement the shared Scrapy policy and JSON-LD extractor**

Use Scrapy settings/middleware for robots, limits, redirect checks, and request metadata. Keep extraction pure so fixtures test it without starting a crawler.

- [ ] **Step 5: Run focused and full Python tests**

Run: `cd python-discovery && python -m pytest -q`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add python-discovery
git commit -m "feat: add safe public-page extraction"
```

### Task 7: Add configured directories and bounded official-site enrichment

**Files:**
- Create: `python-discovery/nearhive_discovery/sources/configured_directory.py`
- Create: `python-discovery/nearhive_discovery/sources/company_site.py`
- Create: `python-discovery/tests/test_configured_directory.py`
- Create: `python-discovery/tests/test_company_site.py`
- Create: `python-discovery/tests/fixtures/directory.html`
- Create: `python-discovery/tests/fixtures/company_site/`
- Create: `config/python_sources.yaml`

**Interfaces:**
- Consumes: `SourceAdapter`, URL safety, JSON-LD extraction, and evidence contracts.
- Produces: `ConfiguredDirectorySource` and `CompanySiteSource`.

- [ ] **Step 1: Write failing directory fixture tests**

Define one YAML-configured list/detail source. Assert location/radius parameters, pagination/page ceiling, selectors, source family, provenance URL, and one malformed card being skipped rather than failing the source.

- [ ] **Step 2: Write failing official-site boundary tests**

Assert only same-site homepage, contact, locations, about, careers, and matching sitemap URLs are followed; enforce maximum pages/depth; reject logout, account, cart, form, and cross-domain links.

- [ ] **Step 3: Run and confirm failures**

Run: `cd python-discovery && python -m pytest tests/test_configured_directory.py tests/test_company_site.py -q`  
Expected: FAIL because adapters do not exist.

- [ ] **Step 4: Implement the two ordinary-page adapters**

Emit evidence as soon as a bounded batch fills rather than retaining an entire crawl in memory. Preserve source record IDs and canonical URLs when present.

- [ ] **Step 5: Run Python tests**

Run: `cd python-discovery && python -m pytest -q`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add python-discovery config/python_sources.yaml
git commit -m "feat: discover companies from public pages"
```

### Task 8: Add technical-job classification and public ATS adapters

**Files:**
- Create: `python-discovery/nearhive_discovery/classify.py`
- Create: `python-discovery/nearhive_discovery/sources/greenhouse.py`
- Create: `python-discovery/nearhive_discovery/sources/lever.py`
- Create: `python-discovery/tests/test_classify.py`
- Create: `python-discovery/tests/test_greenhouse.py`
- Create: `python-discovery/tests/test_lever.py`
- Create: `python-discovery/tests/fixtures/greenhouse.json`
- Create: `python-discovery/tests/fixtures/lever.json`

**Interfaces:**
- Produces: `classify_technical_role(title: str, categories: Sequence[str]) -> Classification`.
- Produces: `classify_arrangement(structured: Mapping[str, object], text: str) -> WorkArrangement`.
- Produces: `publication_state(posted_at, first_seen_at, now) -> posted_recently | observed_recently | stale`.
- Produces: `GreenhouseSource` and `LeverSource` adapters.

- [ ] **Step 1: Write failing classification-table tests**

Cover software, data, infrastructure, security, QA automation, product engineering, technical support, and engineering leadership; add negative cases for sales, recruiting, accounting, and ambiguous non-technical “engineer” usage.

- [ ] **Step 2: Write failing arrangement/date tests**

Assert explicit structured values beat free text; conflicting or absent evidence returns `unknown`; exactly 14 days is recent; older is stale; missing/untrusted publication time is only `observed_recently`.

- [ ] **Step 3: Write failing ATS fixture tests**

Assert stable source IDs, canonical URLs, company identity, location, publication provenance, pagination, and deduplication fingerprints for Greenhouse and Lever fixtures.

- [ ] **Step 4: Run and confirm failures**

Run: `cd python-discovery && python -m pytest tests/test_classify.py tests/test_greenhouse.py tests/test_lever.py -q`  
Expected: FAIL because classifiers and adapters do not exist.

- [ ] **Step 5: Implement deterministic rules and adapters**

Keep rule tables versioned with a string constant such as `technical-title-v1`. Store reasons with every decision. Do not store full job descriptions; emit a short excerpt only when required plus a stable normalized hash.

- [ ] **Step 6: Run Python tests**

Run: `cd python-discovery && python -m pytest -q`  
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add python-discovery
git commit -m "feat: enrich companies with recent technical jobs"
```

### Task 9: Add bounded Playwright fallback

**Files:**
- Modify: `python-discovery/nearhive_discovery/http.py`
- Modify: `python-discovery/nearhive_discovery/worker.py`
- Create: `python-discovery/tests/test_playwright_fallback.py`
- Create: `python-discovery/tests/fixtures/js_company.html`

**Interfaces:**
- Consumes: existing adapters and URL-safety policy.
- Produces: `should_render(response) -> bool` and a shared context pool capped by `PLAYWRIGHT_CONTEXTS`.

- [ ] **Step 1: Write failing fallback tests**

Assert ordinary HTML never opens a browser; a fixture-proven JavaScript shell does; redirect safety is rechecked; timeout/browser crash fails only that request/source; cancellation closes contexts.

- [ ] **Step 2: Run and confirm failures**

Run: `cd python-discovery && python -m pytest tests/test_playwright_fallback.py -q`  
Expected: FAIL because rendering fallback does not exist.

- [ ] **Step 3: Implement scrapy-playwright fallback**

Default `PLAYWRIGHT_CONTEXTS=2`. Trigger rendering only from adapter/request metadata or an empty-shell rule proven by the fixture; do not retry every failed HTTP page in a browser.

- [ ] **Step 4: Run Python tests with Chromium installed**

Run: `cd python-discovery && playwright install chromium && python -m pytest -q`  
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add python-discovery
git commit -m "feat: render JavaScript discovery sources"
```

### Task 10: Expose presence and jobs in Go search/company APIs

**Files:**
- Modify: `internal/store/postgres.go`
- Modify: `internal/store/store_mock.go`
- Modify: `internal/api/handlers_search.go`
- Modify: `internal/api/handlers_company.go`
- Modify: `internal/api/api_test.go`

**Interfaces:**
- Consumes: location presence and `TechnicalJobStore` from Task 1.
- Produces: search fields `presence_type`, `recent_technical_job_count`, and `arrangements`.
- Produces: `GET /api/v1/companies/{id}/technical-jobs?days=14`.

- [ ] **Step 1: Write failing API/search tests**

Assert search includes the three presence values and only trustworthy jobs within 14 days in the count; remote-only jobs do not create spatial results; company job listing includes recently observed entries but marks them separately.

- [ ] **Step 2: Run and confirm failures**

Run: `go test ./internal/api ./internal/store -run 'TestSearchDiscovery|TestCompanyTechnicalJobs' -v`  
Expected: FAIL because response fields and endpoint do not exist.

- [ ] **Step 3: Extend the existing PostGIS projection and company handler**

Use a lateral/aggregated job query to avoid one query per company. Keep the search center/radius behavior unchanged; only actual location rows participate in spatial search.

- [ ] **Step 4: Run full Go tests**

Run: `go test ./...`  
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store internal/api
git commit -m "feat: return company presence and technical jobs"
```

### Task 11: Connect the existing location UI to discovery jobs

**Files:**
- Modify: `web/src/types/index.ts`
- Create: `web/src/hooks/useDiscoveryJobs.ts`
- Modify: `web/src/components/drawers/ScrapeModal.tsx`
- Modify: `web/src/components/scrapers/BackgroundScrapeWidget.tsx`
- Modify: `web/src/components/sidebar/CompanyCard.tsx`
- Modify: `web/src/components/drawers/CompanyDetailDrawer.tsx`

**Interfaces:**
- Consumes: Task 2 and Task 10 endpoints.
- Produces: `useDiscoveryJob`, `useDiscoveryJobs`, `useTriggerDiscovery`, and `useCancelDiscovery` hooks.

- [ ] **Step 1: Add compile-time types and hook tests where the current frontend test setup permits**

Model all discovery statuses, source runs, presence values, arrangement values, posted/observed recency, and progress counters. If no frontend test runner exists, use TypeScript type-checking plus a production build as the runnable check rather than adding a test framework.

- [ ] **Step 2: Implement discovery hooks**

Poll every 2 seconds while pending/running and stop on completed/partial/failed/cancelled. Invalidate companies, clusters, company details, and technical jobs when counters or terminal status change.

- [ ] **Step 3: Reuse the current location controls**

Change the primary discovery action to `POST /api/v1/discovery/jobs` with selected latitude, longitude, and radius. Preserve the existing Go scrape-job UI/actions as a separate legacy control; do not reuse `/api/v1/jobs` for Python work.

- [ ] **Step 4: Add result labels**

Show presence separately from “recent technical jobs.” Use accessible text labels for arrangement badges and distinguish posted-within-14-days from recently observed.

- [ ] **Step 5: Verify frontend**

Run: `cd web && npm run type-check && npm run build`  
Expected: both exit 0.

- [ ] **Step 6: Commit**

```bash
git add web/src
git commit -m "feat: add nearby company discovery UI"
```

### Task 12: Package and verify the complete local stack

**Files:**
- Create: `python-discovery/Dockerfile`
- Create: `python-discovery/tests/fake_site/`
- Create: `python-discovery/tests/test_e2e.py`
- Modify: `docker-compose.yml`
- Modify: `.env.example`
- Modify: `Makefile`
- Modify: `README.md`

**Interfaces:**
- Consumes: all earlier tasks.
- Produces: local `python-discovery` Compose service and `make test-discovery`, `make logs-discovery` commands.

- [ ] **Step 1: Write the failing end-to-end test**

Start a local fake source with ordinary HTML, JSON-LD, a public ATS fixture, and one JavaScript-rendered page. Create a discovery job, run one worker cycle, and assert searchable companies, provenance, partial-source handling, presence, and recent technical-job labels through the Go API.

- [ ] **Step 2: Run and confirm the missing runtime wiring**

Run: `make test-discovery`  
Expected: FAIL because the image, Compose service, and target do not exist.

- [ ] **Step 3: Add the Python worker image and Compose service**

Use the official Playwright Python base compatible with the pinned Playwright version, install the package, run as a non-root user, depend on healthy PostgreSQL and Go API, and pass only required environment variables.

- [ ] **Step 4: Add Make targets and local documentation**

Document startup, configuration, adding a compliant adapter, source fixtures, and the difference between the legacy Go crawler and Python discovery worker.

- [ ] **Step 5: Run complete verification**

Run: `go test ./...`  
Expected: PASS.

Run: `cd python-discovery && python -m pytest -q`  
Expected: PASS.

Run: `cd web && npm run type-check && npm run build`  
Expected: PASS.

Run: `docker compose config`  
Expected: exits 0 and lists `db`, `app`, `crawler`, `python-discovery`, and `web`.

Run: `make test-discovery`  
Expected: PASS without external network access.

- [ ] **Step 6: Confirm protected packages are untouched**

Run: `git diff --name-only HEAD~11..HEAD -- internal/scraper internal/crawler cmd/crawler/main.go`  
Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add python-discovery/Dockerfile python-discovery/tests docker-compose.yml .env.example Makefile README.md
git commit -m "test: verify local Python discovery stack"
```

---

## Implementation Order and Stop Points

- Tasks 1–5 deliver a working queue and fake-source ingestion path; review before touching live-source adapters.
- Tasks 6–8 deliver ordinary-page and recent-job discovery; review source policy and fixture coverage.
- Task 9 adds browser cost only after ordinary crawling works.
- Tasks 10–11 expose the data without changing the existing Go crawler workflow.
- Task 12 is the release gate; do not call the feature complete until every verification command passes.
