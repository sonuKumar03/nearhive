# Location-Based Job Discovery Upgrade Implementation Plan

> 🟠 **PARTIALLY SUPERSEDED (2026-09-28).** The job-location and radius rules here
> remain valid, but ownership moved: Python writes evidence directly and Go no longer
> validates or ingests. Any Go ingestion/validation steps in this plan are superseded by
> the [Python-Owned Discovery and Go Read API Plan](2026-09-28-python-owned-discovery-architecture.md).
> The "Architecture" paragraph below (line 12) contradicts this — Go is NOT the only
> canonical writer; Python owns all canonical scraped-data writes. Do not execute the
> Go ingestion tasks.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the existing Python discovery worker into a location-first job engine that discovers employers around a requested coordinate, follows their public careers surfaces, and returns trustworthy recent technical jobs whose actual job location is inside the requested radius.

**Architecture:** Keep the current ownership boundaries. Python performs bounded source discovery, careers/ATS enrichment, classification, and job-location resolution. The Go API remains the only canonical writer and exposes spatial job search. PostgreSQL/PostGIS remains the search engine. The web app adds a small Jobs result mode beside the current Companies mode.

**Tech Stack:** Python 3.12, httpx, Scrapy, existing Greenhouse/Lever/JSON-LD adapters, Go 1.26, PostgreSQL/PostGIS 17 with pg_trgm, Next.js 15, React 19, TanStack Query 5.

**Spec:** `docs/superpowers/specs/2026-09-27-python-company-discovery-design.md`

## Global Constraints

- Preserve the existing boundary: Python may lease discovery jobs directly, but canonical companies, locations, evidence, and jobs are written only through the Go ingestion API.
- A nearby job must have explicit valid coordinates inside the requested radius. A nearby company office alone does not make every job at that company nearby.
- Remote jobs do not appear in location-radius results. A later explicit remote mode may expose them without pretending they are local.
- “Recent” means a trustworthy `posted_at` within 14 days. `observed_recently` is retained as evidence but is not returned as a recently posted job.
- Preserve structured coordinates from JSON-LD or a provider before attempting address geocoding. Never replace stronger location evidence with weaker inferred data.
- Keep every network fan-out bounded: maximum candidate sites per discovery job, existing per-site page/depth limits, deduplicated ATS targets, and cached location lookups.
- Reuse the current URL safety, redirect validation, robots, timeout, and Playwright controls for all derived source targets.
- Do not copy the comparison repository's Django, Elasticsearch, Redis cache, or Celery stack. Postgres/PostGIS already covers the required query shape.
- Do not treat a static company catalog as live job evidence. A licensed/current catalog may later seed candidate domains through `ConfiguredDirectorySource`, but it is not part of this plan.
- CI uses saved fixtures and local fake HTTP services only; it must not call OSM, company sites, ATS providers, or a public geocoder.

## Review Focus

- **False locality:** remote jobs, jobs outside the radius, and jobs with unresolved locations must not enter spatial results; covered in Tasks 2 and 3.
- **Unbounded crawling:** one dense OSM result must not cause unlimited company-site or ATS requests; covered in Task 1.
- **Duplicate providers/jobs:** configured and dynamically discovered ATS targets must collapse to one provider request and one canonical job row; covered in Tasks 1 and 5.
- **Unsafe derived URLs:** company and ATS links discovered from public pages must pass the existing public HTTP/HTTPS and redirect checks; covered in Tasks 1 and 5.
- **Misleading freshness:** missing, future, or weak publication dates must not satisfy the 14-day job-search filter; covered in Tasks 2, 3, and 5.

---

## File Map

### Python discovery

- `python-discovery/nearhive_discovery/worker.py` — run candidate, company-site, and ATS stages in order while retaining the current lease/cancellation lifecycle.
- `python-discovery/nearhive_discovery/settings.py` — add bounded site-enrichment and geocoder settings.
- `python-discovery/nearhive_discovery/sources/company_site.py` — expose deduplicated Greenhouse/Lever targets found during the existing bounded crawl.
- `python-discovery/nearhive_discovery/sources/greenhouse.py` — accept dynamically discovered board tokens without changing evidence semantics.
- `python-discovery/nearhive_discovery/sources/lever.py` — accept dynamically discovered company tokens without changing evidence semantics.
- `python-discovery/nearhive_discovery/location.py` — resolve non-remote job location strings with the already-installed `httpx`, an in-process cache, and a bounded request rate.
- `python-discovery/tests/test_worker.py` — verify staged handoff, target limits, deduplication, cancellation, and partial failures.
- `python-discovery/tests/test_company_site.py` — verify careers-link and ATS-target recognition without following arbitrary third-party links.
- `python-discovery/tests/test_location.py` — verify coordinate preservation, remote exclusion, cache behavior, failures, and bounds.
- `python-discovery/tests/fixtures/` and `python-discovery/tests/fake_site/server.py` — local company, careers, ATS, and geocoder responses for integration coverage.

### Go API and PostGIS

- `migrations/000012_technical_job_spatial_search.up.sql` / `.down.sql` — add partial spatial and title indexes for searchable technical jobs.
- `internal/model/discovery.go` — add the technical-job spatial search projection.
- `internal/store/store.go` — add focused technical-job search/count methods.
- `internal/store/store_mock.go` — implement the new methods for API tests.
- `internal/store/discovery.go` — query recent, active, geolocated technical jobs with PostGIS.
- `internal/store/discovery_test.go` — prove radius, freshness, remote, title, and pagination behavior.
- `internal/api/handlers_search.go` — expose `GET /api/v1/search/jobs` using the current coordinate/radius validation rules.
- `internal/api/router.go` and `internal/api/api_test.go` — register and test the authenticated route.

### Web and operations

- `web/src/types/index.ts` — add job-search response types.
- `web/src/hooks/useNearbyJobs.ts` — query the new endpoint with the current map center, radius, text, page, and limit.
- `web/src/components/sidebar/JobCard.tsx` — render job, company, location, arrangement, age, and source link.
- `web/src/components/sidebar/Sidebar.tsx` — add a Companies/Jobs mode switch and mode-specific empty/loading copy.
- `web/src/app/page.tsx` — connect the existing location/search state to the job query; leave the current company map markers unchanged in this iteration.
- `.env.example`, `docker-compose.yml`, and `README.md` — document location resolver controls and the separate Python worker deployment.

---

### Task 1: Connect nearby company discovery to official sites and ATS adapters

**Files:**
- Modify: `python-discovery/nearhive_discovery/worker.py`
- Modify: `python-discovery/nearhive_discovery/settings.py`
- Modify: `python-discovery/nearhive_discovery/sources/company_site.py`
- Modify: `python-discovery/nearhive_discovery/sources/greenhouse.py`
- Modify: `python-discovery/nearhive_discovery/sources/lever.py`
- Modify: `python-discovery/tests/test_worker.py`
- Modify: `python-discovery/tests/test_company_site.py`
- Modify: `python-discovery/tests/test_greenhouse.py`
- Modify: `python-discovery/tests/test_lever.py`

**Interfaces:**
- Produces: a bounded staged flow: candidate sources → unique company domains → one `CompanySiteSource` run → deduplicated Greenhouse/Lever targets → one run per ATS provider.
- Produces: `CompanySiteSource.discovered_ats_targets`, containing only validated provider/token/company tuples observed in public anchor URLs.
- Produces: `Settings.max_company_sites`, defaulting to a conservative fixed ceiling such as 50.
- Preserves: the existing `Source.run(job) -> EvidenceBatch | Iterable | AsyncIterable` protocol and ingestion contract version.

- [ ] **Step 1: Write failing staged-worker tests**

Add tests where an OSM batch contains duplicate domains, a domain exposes a careers link and Greenhouse/Lever URLs, and configured ATS targets overlap discovered targets. Assert each company site and provider token runs once, the site ceiling is honored, and cancellation stops before the next derived stage.

- [ ] **Step 2: Run the focused tests and confirm the missing handoff**

Working directory: `python-discovery`  
Run: `rtk uv run pytest -q tests/test_worker.py tests/test_company_site.py tests/test_greenhouse.py tests/test_lever.py`  
Expected: FAIL because `get_default_sources()` currently creates only static sources and `CompanySiteSource` does not report ATS targets.

- [ ] **Step 3: Detect provider targets during the existing site crawl**

Recognize only canonical public Greenhouse and Lever job URL shapes. Extract the board/company token and retain the associated company name/domain already known from candidate evidence. Do not follow unrelated cross-domain anchors. Pass every derived URL through the current URL-safety checks before using it.

- [ ] **Step 4: Make the worker run three explicit stages**

Extract the repeated “run source, stream batches, submit, record source run” body from `_process_job` into one private helper. Consume async iterators incrementally instead of collecting every batch in memory before the first submission. Use the helper for:

1. configured candidate sources (`open_dataset` and `public_directory`), collecting unique public domains from emitted company evidence;
2. one bounded `CompanySiteSource` over those domains, collecting JSON-LD jobs and ATS targets;
3. the existing Greenhouse and Lever adapters, each receiving the union of configured and discovered targets.

Keep heartbeat, lease-loss, cancellation, partial-success, Playwright cleanup, and source-run recording behavior unchanged. Do not introduce a workflow framework or generic DAG.

- [ ] **Step 5: Run Python unit tests**

Working directory: `python-discovery`  
Run: `rtk uv run pytest -q tests --ignore=tests/test_e2e.py`  
Expected: PASS, including worker lifecycle and URL-policy tests.

- [ ] **Step 6: Commit**

```bash
rtk git add python-discovery/nearhive_discovery python-discovery/tests
rtk git commit -m "feat: discover jobs from nearby company sites"
```

### Task 2: Resolve trustworthy job coordinates before ingestion

**Files:**
- Create: `python-discovery/nearhive_discovery/location.py`
- Create: `python-discovery/tests/test_location.py`
- Modify: `python-discovery/nearhive_discovery/worker.py`
- Modify: `python-discovery/nearhive_discovery/settings.py`
- Modify: `python-discovery/tests/test_worker.py`
- Modify: `.env.example`

**Interfaces:**
- Produces: `resolve_job_location(job, resolver) -> TechnicalJobEvidence`, preserving frozen evidence via `dataclasses.replace`.
- Produces: a small async resolver backed by `httpx`, keyed by normalized `location_raw`, with success and no-result caching for the worker process.
- Produces: metadata values `location_resolution=structured|geocoded|unresolved` and `location_provider` when applicable.
- Produces: `Settings.max_job_geocodes`, defaulting to a conservative per-discovery-job ceiling such as 50.
- Preserves: coordinates already extracted from JSON-LD or an ATS provider.

- [ ] **Step 1: Write failing location-resolution tests**

Cover: structured coordinates are untouched; remote jobs are never geocoded and have no local coordinates; identical normalized strings call the fake geocoder once; invalid/out-of-range results are discarded; timeout/no-result leaves the job ingestible but spatially unsearchable; a valid result adds coordinates and provenance.

- [ ] **Step 2: Run and confirm failure**

Working directory: `python-discovery`  
Run: `rtk uv run pytest -q tests/test_location.py tests/test_worker.py`  
Expected: FAIL because no location resolver is present in the batch path.

- [ ] **Step 3: Implement the minimum resolver**

Use the installed `httpx`; do not add a geocoding package. Configure the base URL, per-job lookup ceiling, and minimum request interval; apply a short timeout, URL-encode the query, request one result, validate latitude/longitude ranges, and cache both successful and empty results. Resolve only non-remote technical jobs with non-empty `location_raw` and no existing coordinates.

- [ ] **Step 4: Resolve each outgoing job batch**

Apply resolution after source extraction and before `client.submit`. A resolver failure must not fail the source or discard the job. Preserve the raw location and record `unresolved` metadata so coverage can be measured.

- [ ] **Step 5: Run Python tests**

Working directory: `python-discovery`  
Run: `rtk uv run pytest -q tests --ignore=tests/test_e2e.py`  
Expected: PASS with no public network access.

- [ ] **Step 6: Commit**

```bash
rtk git add python-discovery/nearhive_discovery python-discovery/tests .env.example
rtk git commit -m "feat: resolve technical job locations"
```

### Task 3: Add PostGIS technical-job search

**Files:**
- Create: `migrations/000012_technical_job_spatial_search.up.sql`
- Create: `migrations/000012_technical_job_spatial_search.down.sql`
- Modify: `internal/model/discovery.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_mock.go`
- Modify: `internal/store/discovery.go`
- Modify: `internal/store/discovery_test.go`
- Modify: `internal/api/handlers_search.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/api_test.go`

**Interfaces:**
- Produces: `TechnicalJobSearchOpts{Query, WorkArrangement, Limit, Offset}`.
- Produces: `TechnicalJobStore.SearchTechnicalJobs(ctx, lat, lng, radiusMeters, opts)` and `CountTechnicalJobSearch(...)`.
- Produces: authenticated `GET /api/v1/search/jobs?lat=&lng=&radius=&q=&work_arrangement=&page=&limit=`.
- Produces: response `{meta: {total, page, limit, radius_km, center}, jobs: [...]}` with job, company, distance, source, and location fields.

- [ ] **Step 1: Write failing store tests**

Insert recent inside-radius, recent outside-radius, remote, stale, weak-date, inactive, and title-mismatch jobs. Assert only the recent active non-remote job with valid coordinates is returned; verify distance order, arrangement/title filters, count, limit, and offset.

- [ ] **Step 2: Run and confirm missing store methods**

Run: `rtk go test ./internal/store -run 'TestTechnicalJobSpatialSearch' -v`  
Expected: FAIL because the spatial search methods and indexes do not exist.

- [ ] **Step 3: Add the smallest useful indexes and query**

Create a partial GiST expression index on `ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography` for active rows with both coordinates, plus a pg_trgm GIN index on `normalized_title`. Query using the identical geography expression so PostgreSQL can use the spatial index. Require:

- `is_active = TRUE`;
- `work_arrangement <> 'remote'`;
- `publication_state = 'posted_recently'`;
- non-null `posted_at` within 14 days and positive `posted_at_confidence`;
- `ST_DWithin(..., radiusMeters)`.

Order by distance ascending, then `posted_at` descending. Do not add Elasticsearch or an application cache.

- [ ] **Step 4: Add and test the API route**

Reuse the current search handler's coordinate, radius, page, and limit rules. Reject unknown arrangement values and invalid coordinates with `VALIDATION_ERROR`. Keep this route JWT-protected like company search.

Run: `rtk go test ./internal/api -run 'TestTechnicalJobSearch' -v`  
Expected: PASS for auth, validation, filters, response shape, and pagination.

- [ ] **Step 5: Run migration, store, and API tests**

Run: `rtk go test ./migrations ./internal/store ./internal/api -v`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add migrations/000012_technical_job_spatial_search.* internal/model internal/store internal/api
rtk git commit -m "feat: search technical jobs by location"
```

### Task 4: Expose nearby jobs in the existing search UI

**Files:**
- Modify: `web/src/types/index.ts`
- Create: `web/src/hooks/useNearbyJobs.ts`
- Create: `web/src/components/sidebar/JobCard.tsx`
- Modify: `web/src/components/sidebar/Sidebar.tsx`
- Modify: `web/src/app/page.tsx`

**Interfaces:**
- Produces: `TechnicalJobSearchResult` and `TechnicalJobSearchResponse` matching Task 3.
- Produces: `useNearbyJobs(params, enabled)` with query key `['nearby-jobs', params]`.
- Produces: an accessible Companies/Jobs switch in the existing sidebar.
- Preserves: existing company search, company selection, detail drawer, and map markers.

- [ ] **Step 1: Add the response types and query hook**

Build query parameters with `URLSearchParams`, reuse `fetchApi`, and enable the query only in Jobs mode with valid coordinates. Invalidate `nearby-jobs` when discovery completes or is cancelled, beside the existing company/job invalidations.

- [ ] **Step 2: Add the minimal job result card**

Show title, company, resolved location, distance, work arrangement, posted date, and provider. The canonical job URL is an explicit external link with safe `rel` attributes. Do not add a second drawer, saved jobs, salary parsing, or map-marker redesign.

- [ ] **Step 3: Add the sidebar mode switch**

Keep one text field: it filters company names in Companies mode and job titles in Jobs mode. Provide mode-specific count, loading text, and empty state. Keep keyboard focus and `aria-pressed`/tab semantics clear.

- [ ] **Step 4: Verify frontend types and build**

Working directory: `web`  
Run: `rtk npm run type-check` and then `rtk npm run build`  
Expected: PASS; Companies mode remains the default and existing map behavior is unchanged.

- [ ] **Step 5: Commit**

```bash
rtk git add web/src
rtk git commit -m "feat: show nearby technical job results"
```

### Task 5: Prove the complete local flow and document deployment

**Files:**
- Modify: `python-discovery/tests/fake_site/server.py`
- Add/Modify: `python-discovery/tests/fixtures/`
- Modify: `python-discovery/tests/test_e2e.py`
- Modify: `docker-compose.yml`
- Modify: `.env.example`
- Modify: `README.md`

**Interfaces:**
- Proves: coordinate → OSM candidate → official site → ATS/JSON-LD job → location resolution → Go ingestion → `GET /api/v1/search/jobs`.
- Documents: a separate Python worker service using `python-discovery/Dockerfile`, the same PostgreSQL database, `NEARHIVE_API_URL`, and the shared discovery worker token.

- [ ] **Step 1: Extend the local fake source chain**

Serve one nearby company with an official careers page, duplicate ATS links, one recent local technical job, one stale job, one remote job, and a fake geocoder result. Keep every response deterministic.

- [ ] **Step 2: Write the failing end-to-end assertion**

Run one discovery job and query the new spatial endpoint. Assert one local recent job is returned once, its distance is inside the requested radius, and stale/remote/unresolved jobs are absent. Assert the discovery source runs show candidate, company-site, and ATS stages.

- [ ] **Step 3: Run focused and complete verification**

Run:

```bash
rtk go test ./internal/store ./internal/api ./migrations -v
# from python-discovery/
rtk uv run pytest -q
# from web/
rtk npm run type-check
rtk npm run build
```

Expected: PASS. If the Compose worker is already running against the same test database, stop only that worker before database-backed Python tests so it cannot lease test jobs.

- [ ] **Step 4: Verify the Compose path**

Run: `rtk make up`  
Then trigger one discovery job against the local fake services and call `/api/v1/search/jobs` with its coordinates.  
Expected: the Python worker reaches `completed` or `partial`, source runs are visible, and the local recent job is returned.

- [ ] **Step 5: Document production worker deployment**

Document a separate long-running service rooted at `python-discovery/` rather than changing the current Go `railway.toml`. It must share `DATABASE_URL`, point `NEARHIVE_API_URL` at the Go API, share the worker token, and run exactly one worker replica initially. Add replicas only after lease and source-rate measurements justify them.

- [ ] **Step 6: Commit**

```bash
rtk git add python-discovery/tests docker-compose.yml .env.example README.md
rtk git commit -m "test: verify location-based job discovery"
```

---

## Deferred Until Measurements Justify Them

- **Static company imports:** add only when a maintained, licensed dataset materially improves candidate-domain coverage in a target market. Feed it through `ConfiguredDirectorySource`; do not add a Django service.
- **Additional ATS adapters:** add the next provider only after source-run logs show missed companies and identify the dominant provider.
- **Elasticsearch/OpenSearch:** consider only when PostGIS plus pg_trgm misses measured relevance or latency targets at production scale.
- **Redis/application caching:** consider only after query profiling shows repeated expensive reads that PostgreSQL indexes do not solve.
- **Celery/Kafka/workflow orchestration:** the existing PostgreSQL lease queue is sufficient until measured throughput or isolation requirements prove otherwise.

## Success Criteria

- A discovery job can start from only `lat`, `lng`, and `radius_km` and reach official careers/ATS data for bounded nearby company domains.
- Every job returned by `/api/v1/search/jobs` is active, technical, non-remote, trustworthy within 14 days, geolocated, and inside the requested radius.
- Duplicate company domains, ATS links, provider records, and batch retries do not duplicate canonical jobs.
- Existing company search and per-company technical-job endpoints remain compatible.
- The complete fixture-driven Go, Python, and web verification suite passes without public network calls.
