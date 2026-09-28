# Python-Owned Discovery and Go Read API Plan

**Status:** Local Python-owned cutover implemented; deployment configuration remains to be reviewed before any deployment  
**Date:** 2026-09-28

## Current implementation checkpoint

- Python writes validated company, sighting, job, and job-location evidence directly to PostgreSQL. It owns authenticated create/list/status/cancel operations and the lease worker.
- Compose initializes an empty PostGIS volume from `python-discovery/schema.sql`. The local database was reset and the four services started successfully.
- Go serves auth and company/job reads. Its scraper CLI, scheduled crawler, crawler packages, scrape routes, and batch ingestion route are removed. Its discovery database role has read-only privileges; only `users` remains writable.
- The web uses Python for discovery operations and Go for search/details. It shows coarse lifecycle status only.
- Focused Python checks, Go checks, web build, and an authenticated Compose smoke check passed. Python-written company/job evidence was read through Go, and a Go-role write to `companies` was denied.

## Goal

Python owns company/job discovery, scraping, normalization, validation, deduplication, and canonical database writes. Go serves authenticated data to web/mobile clients through nearby-company and nearby-job queries. Users can query companies and jobs independently.

Keep PostgreSQL/PostGIS as the shared canonical store. Keep the Python scraper as one deployable service and Go as one read API. Preserve source provenance and safe, bounded crawling. The Python discovery app defines the write model; build the database schema and Go read paths from it on a fresh local database. Existing rows and API contracts impose no upgrade or compatibility requirements.

## Target boundaries

- **Python discovery service:** accepts or schedules discovery work; leases and records runs; selects sources; fetches/parses pages; validates and normalizes company, office, and job evidence; resolves identity and duplicates; writes canonical records and provenance transactionally; records partial failures and run metrics.
- **Go API:** authentication and read-only company/job/search endpoints, pagination/filter validation, and PostGIS-backed queries. It does not crawl, schedule scraping, ingest scraper batches, validate scraped records, merge identities, or mutate discovery data.
- **PostgreSQL/PostGIS:** shared canonical database. Python is the only writer for scraped company/job data and discovery-run state. Go uses read-only DB credentials for these tables. User/auth tables may retain their existing write path if needed for login.
- **Web/mobile clients:** call the Go read API for company/job data. Users can launch discovery and see its coarse lifecycle status (`queued`, `in_progress`, `completed`, `failed`, or `cancelled`) through the Python-owned operation API. Do not expose per-source progress, logs, or detailed monitoring in the UI.

Keep validation at the Python persistence boundary. Store raw source observations separately from chosen canonical fields so decisions can be reprocessed and audited. Preserve independent nearby-company and nearby-job queries: remote-only jobs do not make companies nearby, and a job is nearby only when its own eligible location is within the requested radius.

## Findings at planning time

- `python-discovery/` already has OSM/directory candidates, official-site enrichment, Greenhouse/Lever adapters, JSON-LD parsing, location resolution, URL policy, a Postgres lease queue, and the three-stage worker.
- The current Python worker sends evidence to Go's `POST /api/v1/internal/discovery/batches`; Go validates, verifies, merges, and persists it. Go also owns discovery-job APIs, crawler jobs, scraper/verifier packages, and nearby company/job reads.
- PostgreSQL already has company, location, sighting, discovery-run, and job tables with PostGIS and spatial indexes. Existing Go nearby search and Python scrape discovery APIs can anchor migration.
- The current plans assume Go-owned canonical storage and must not be executed as written. In particular, the 2026-09-27 plan and 2026-09-28 job-search upgrade task lists include Go ingestion/validation work that conflicts with this target.

## Keep, change, remove

| Area | Plan |
|---|---|
| Python adapters, HTTP safety, parsing, queue leasing, and source-run reporting | Keep; make Python the authoritative validation and persistence path |
| `companies`, `locations`, `sightings`, `technical_job_postings`, PostGIS | Keep the proven concepts; revise schema/ownership for direct Python writes and separate spatial job locations |
| Go authentication, read handlers, nearby queries, response models, and PostGIS search | Keep; remove write access to scraped-data tables |
| Go internal discovery ingestion endpoint, verifier/matcher/merger, and Python batch client | Remove after Python persistence is complete |
| Go crawler executable, crawler worker, scraper/orchestrator/sources, legacy scrape-job routes and UI | Retire after source coverage has moved to Python and client controls are resolved |
| Duplicate Go and Python queues for scraping | Collapse to one Python-owned queue/run model; do not keep compatibility tables or dual writes |
| `technical_job_postings` naming | Retain the technical-only scope and schema; do not generalize for all roles |

Do not delete auth, user accounts, read API, frontend search, source provenance, or PostGIS. Inspect dependencies before removing a legacy component and remove its UI/routes/config/docs with it.

## Delivery sequence

### 1. Apply confirmed product decisions and inventory current paths

- Treat Python as the only scraper and canonical scraped-data writer; Go is read-only for that dataset.
- Users can launch discovery. Python owns the launch operation and exposes only coarse run status for the UI; detailed monitoring is out of scope.
- Keep job discovery technical-only.
- Existing scraped rows are disposable. No legacy data import, backfill, dual-write, or old-worker compatibility is needed.
- Inventory all Go routes, scheduled jobs, services, env vars, deployment manifests, and UI controls tied to scraping. Mark each as migrate, retire, or retain.

### 2. Make Python validation and persistence canonical

- Define the minimal canonical schema and constraints for company identity, office locations, source observations, jobs, and independent job locations. Initialize it on an empty database from the Python-owned model.
- Move deterministic validation, normalization, domain/name matching, idempotency, and canonical upserts into Python. Keep raw source payloads bounded and only where needed for provenance/reprocessing; do not store full copyrighted job descriptions.
- Make a source observation and its canonical upsert atomic/idempotent. Preserve source family, source record ID/hash, URL, observed time, and validation outcomes.
- Keep the PostgreSQL lease queue with `FOR UPDATE SKIP LOCKED`; avoid Redis, Kafka, Celery, multiple services, or a new repository unless measurements require them.
- Give Python read/write DB access. Give Go a read-only DB role for scraped data, enforced in deployment/configuration as well as code conventions.

### 3. Move discovery operations and remove the Go ingestion bridge

- Have Python own discovery request/run creation, cancellation, lease heartbeat/recovery, internal per-source state, and coarse user-visible lifecycle status (`queued`, `in_progress`, `completed`, `failed`, or `cancelled`). Keep per-source state out of the UI.
- Update Python worker to validate and persist directly; remove the Go ingestion HTTP client and the internal batch route after parity is demonstrated.
- Keep ingestion validation strict: reject unsafe URLs, invalid coordinates/timestamps, invalid enum values, oversized content, and malformed identities; quarantine/reject records with explicit reasons instead of silently weakening rules.
- Preserve bounded concurrency, robots/rate policy, SSRF protections, retries only for transient failures, and per-source isolation.

### 4. Reduce Go to the client-facing read API

- Keep JWT/auth middleware if clients need accounts. Keep nearby company and nearby job GET endpoints, company details, and only useful read projections.
- Make read queries use the new canonical schema. Company proximity comes from office evidence; job proximity comes from job-specific location evidence and excludes remote-only/unresolved locations from spatial results.
- Remove Go scrape/discovery mutation routes, queue workers, scheduling, verifier, matching/merge code, source adapters, and write interfaces once no read path depends on them.
- Configure Go's scraped-data DB connection with read-only permissions. Avoid duplicating validation/matching logic in Go.

### 5. Keep the client surface simple

- Expose separate Companies and Jobs queries with shared location/radius input and independent result counts, filters, loading, and empty states.
- The UI calls Python to launch a run and retrieve its coarse status; it queries discovered companies and jobs from Go. Show lifecycle status only, without per-source progress or detailed run monitoring.
- Keep existing map/list presentation where useful; do not add separate databases, search engines, caches, or UI frameworks for this change.

### 6. Cut over and retire legacy paths

- Existing scraped data can be dropped; make the clean schema usable from an empty database with no import or compatibility layer for old rows.
- Switch Compose and deployment configs to run the Python discovery service and Go read API with appropriate DB roles. Remove obsolete Go crawler service/configuration and worker secrets.
- Remove superseded plan steps and update README/API docs so no instructions describe Go as canonical writer or scraper.
- Rebuild the local development database when cutting over. Keep a reproducible schema initialization path; do not add upgrade migrations, data backfills, or dual-write transitions for disposable rows.

## Minimal verification policy

Do not add a broad test suite or production mock data. Keep only checks that protect the ownership boundary and real data integrity:

- one Python persistence check for invalid input rejection and idempotent retry;
- one database-backed path proving a valid nearby company and a separately located job can be queried, while remote/unresolved jobs are excluded spatially;
- one Go API check proving the two read queries return the expected real-schema projections and cannot write scraped data (prefer DB-role enforcement over a mock-only claim);
- a fresh-schema/compose smoke check for the actual local deployment path.

Use test-only fixtures/local endpoints where source responses are needed. Never seed mock companies/jobs from production application startup or normal runtime code. Do not add live-site tests.

## Completion criteria

- Python alone schedules/fetches/validates/deduplicates/persists scraped companies, locations, evidence, and jobs.
- Go cannot mutate scraped-data tables and serves only client-facing reads for that data.
- Nearby companies and nearby jobs are independently queryable and locality rules are based on the appropriate evidence.
- There is one scraping queue/worker model and no active Go crawler or ingestion bridge.
- The app runs locally with no production-path mock records and no unnecessary infrastructure dependencies.

## Confirmed decisions

- Users may launch discovery and see only coarse lifecycle status in the UI. Detailed monitoring is not required.
- Job discovery remains technical-only.
- Existing scraped data is disposable; no preservation/backfill path is needed.
