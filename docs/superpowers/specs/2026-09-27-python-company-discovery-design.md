# Python Company Discovery and Job Enrichment Design

**Status:** Approved design, awaiting implementation-plan review  
**Date:** 2026-09-27

## Summary

NearHive will add a Python discovery worker alongside the existing Go crawler. Users choose their current location or a point on the map and a radius. Python gathers company evidence from open datasets, public directories, company websites, and permitted public job sources; Go remains responsible for authenticated APIs, canonical company records, verification, PostGIS search, and presentation.

The existing Go crawler under `internal/scraper` and `internal/crawler` remains unchanged. Focused additions to the Go API, store, models, and verifier accept richer Python evidence and replace the current tech-company-only assumption with independent-source verification. A company stays discoverable even when it has no recent technical jobs.

## Goals

- Discover as many companies as practical around an arbitrary user-selected point and radius.
- Accept ordinary HTML, structured data, JSON endpoints, and JavaScript-rendered pages.
- Enrich discovered companies with technical jobs posted within the previous 14 days.
- Label jobs as `in_office`, `hybrid`, `remote`, or `unknown`.
- Distinguish confirmed offices, probable offices, and job-location-only evidence.
- Preserve raw provenance and prevent syndicated data from masquerading as independent verification.
- Run locally through Docker Compose and scale later by adding Python worker replicas.
- Keep source adapters isolated so one site change does not affect the rest of the pipeline.

## Non-goals

- Guarantee complete coverage of every company worldwide. Public-source coverage is inherently incomplete.
- Crawl the entire web or scrape general search-engine result pages.
- Bypass authentication, CAPTCHAs, robots directives, rate limits, or access controls.
- Replace the existing Go crawler, API, verifier ownership, or PostGIS search layer.
- Store every non-technical job. The initial job scope is technical roles only.
- Add Redis, Kafka, an LLM classifier, proxy rotation, or a workflow engine before measured need.

## Current System

The Go crawler currently registers OpenStreetMap, Wikidata, JustDial, optional Google Places, and configured tech-park sources. Each source emits a `Sighting`. The verifier then:

1. applies a tech-company name/metadata classifier;
2. matches by domain or normalized/fuzzy name;
3. merges nearby locations;
4. adds fixed source weights to location confidence.

This is a useful base but does not satisfy the new goal. It filters out non-tech employers, has limited source breadth, and treats repeated source sightings as additive confidence rather than grouping evidence by independent source family.

## Architecture

```text
Location UI
    |
    v
Go discovery-job API ---> PostgreSQL discovery_jobs
                              |
                              v
                       Python worker pool
                         |           |
                    HTTP/Scrapy   Playwright fallback
                         |           |
                         +-----+-----+
                               |
                  normalized evidence batches
                               |
                               v
                    Go internal ingestion API
                               |
                    matcher + verifier + store
                               |
                               v
             companies / locations / sightings / jobs
                               |
                               v
                    existing PostGIS search API
```

### Ownership boundaries

- **Go API:** authentication, request validation, discovery-job creation/status/cancellation, ingestion authentication, canonical persistence, matching, verification, and search.
- **Python worker:** source selection, fetching, browser rendering, parsing, normalization into the ingestion contract, technical-role classification, and source-run reporting.
- **PostgreSQL:** durable job queue and canonical data store.
- **Frontend:** location/radius input, progress, company results, evidence state, and recent-job labels.

Python may claim and update discovery queue leases directly in PostgreSQL. It may not write canonical companies, locations, sightings, or jobs directly; those writes pass through Go validation and verification.

## Request and Data Flow

1. The user chooses “near me” or a map point and a radius.
2. The authenticated Go endpoint validates coordinates/radius and inserts a `discovery_jobs` row.
3. A Python worker atomically claims the oldest eligible job with `FOR UPDATE SKIP LOCKED`, records a lease and heartbeat, and creates one source-run row per selected adapter.
4. Candidate adapters discover company names, locations, domains, and evidence URLs.
5. Enrichment follows only bounded likely pages: homepage, contact, locations, about, careers, and relevant sitemap entries.
6. Job adapters collect technical postings and their work arrangement. Only jobs with a trustworthy publication date within 14 days qualify as recently posted. Jobs without a reliable publication date remain “recently observed,” not “recently posted.”
7. Python sends bounded, idempotent evidence batches to the internal Go endpoint.
8. Go validates, deduplicates, matches, stores provenance, recalculates company/location confidence, and upserts job postings.
9. The frontend polls discovery-job progress and refreshes nearby results while sources finish independently.
10. Completion records partial failures instead of discarding successful source results.

## Python Service Structure

The implementation should start with this minimal package layout:

```text
python-discovery/
  pyproject.toml
  nearhive_discovery/
    worker.py
    queue.py
    contracts.py
    settings.py
    http.py
    classify.py
    sources/
      base.py
      jsonld.py
      company_site.py
      greenhouse.py
      lever.py
      configured_directory.py
  tests/
    fixtures/
```

Do not add abstract factories or multiple framework layers. A source adapter needs only a stable name, source family, support predicate, and asynchronous discovery method. Shared HTTP policy, normalization, and output contracts live outside adapters.

Scrapy is the default fetch/crawl engine. Playwright is a fallback for pages that require browser rendering; it is not launched for ordinary HTML or JSON responses.

## Source Strategy

### Stage 1: Candidate discovery

- Consume companies and evidence already stored by the Go crawler.
- Query explicitly configured public business, industry, technology-park, chamber, or registry pages where their policies permit automated access.
- Use public location-filtered job or ATS pages as company-discovery signals.
- Support reusable JSON-LD extraction for `Organization`, `LocalBusiness`, `PostalAddress`, and coordinates.

### Stage 2: Official-site enrichment

- Prefer a canonical company domain from existing evidence.
- Crawl a bounded set of likely pages and relevant sitemap URLs.
- Extract official names, domains, addresses, phone numbers, coordinates, career links, and structured data.
- Prevent SSRF by accepting only HTTP/HTTPS URLs, resolving only public network addresses, and rechecking redirects.

### Stage 3: Job enrichment

- Follow public career links discovered from official company sites.
- Add dedicated adapters for public Greenhouse and Lever job boards.
- Parse standard `JobPosting` JSON-LD before custom HTML selectors.
- Add site-specific job-board adapters only after confirming their public access rules and robots policy.
- Deduplicate syndicated postings using source IDs, canonical URLs, and normalized content fingerprints.

Each adapter declares a source family such as `official_site`, `open_dataset`, `public_directory`, or `job_ats`. Multiple domains that syndicate the same underlying record keep the same evidence lineage and do not count as independent confirmation.

## Crawling Policy

- Read and honor `robots.txt` before crawling a host.
- Use an identifiable NearHive user agent with contact information.
- Apply per-domain concurrency, request delay, timeout, response-size, and page-count limits.
- Retry only transient failures with bounded exponential backoff and jitter.
- Never bypass authentication, CAPTCHAs, paywalls, or explicit blocking.
- Never submit forms or perform state-changing browser actions.
- Cache robots decisions and short-lived fetch results within a worker process.
- Record blocked, skipped, timed-out, and parse-failed outcomes per source.

## Ingestion Contract

Python sends versioned batches. A batch contains:

- `contract_version`
- `discovery_job_id`
- `source`
- `source_family`
- `observed_at`
- company sightings
- technical-job sightings

A company sighting contains the source record ID, evidence URL, observed name, canonical domain when known, address text, coordinates when known, phone when public, and source-specific metadata.

A job sighting contains the source job ID, canonical URL, company identity fields, title, description excerpt or normalized text hash, location text, publication time and its provenance, first/last observation times, work arrangement, technical classification, and classification reasons.

The Go endpoint rejects unknown contract versions, oversized batches, invalid coordinates, unsupported URL schemes, invalid timestamps, and malformed required fields. It returns per-record acceptance or rejection so one bad record does not force the worker to resend an entire successful batch.

## Database Changes

### `discovery_jobs`

Separate from `scrape_jobs` so the existing Go crawler cannot claim Python work.

- ID and requesting user
- latitude, longitude, radius
- status: `pending`, `running`, `completed`, `partial`, `failed`, or `cancelled`
- worker ID, lease expiry, heartbeat, and attempt count
- company/job/evidence counters
- error summary and timestamps

The pending queue has a partial index ordered by creation time. Lease recovery returns abandoned jobs to `pending` until the configured attempt limit is reached.

### `discovery_source_runs`

- discovery job and adapter name
- source family
- status and attempt count
- evidence/company/job counts
- error summary, duration, and timestamps

The unique key is `(discovery_job_id, source)`.

### Existing `sightings`

Add source family, source record ID, stable content hash, discovery job ID, first-seen time, and last-seen time. A partial unique index on `(source, source_record_id)` makes repeated ingestion idempotent when a source supplies stable IDs. Hash-based idempotency covers sources without IDs.

### Existing `locations`

Add `presence_type` constrained to:

- `confirmed_office`
- `probable_office`
- `job_location_only`

Existing rows default to `probable_office`; future verification recalculates them from evidence.

### `technical_job_postings`

- canonical company ID
- source, source family, source job ID, and canonical URL
- title and normalized title
- public location text and optional coordinates
- arrangement: `in_office`, `hybrid`, `remote`, or `unknown`
- publication time, publication-time confidence, first seen, last seen, and active state
- technical classification, rule version, and reasons
- content hash and compact source metadata

Use unique source IDs when available and a source-plus-content-hash fallback otherwise. Do not copy full copyrighted job descriptions when a title, short excerpt/hash, and source URL are sufficient.

## Matching and Verification

All candidate companies are retained. The old tech-company classifier is not used as an admission gate for Python sightings.

### Matching order

1. Exact canonical domain.
2. Exact normalized name near the same location.
3. Name plus matching public phone, address, or official-site evidence.
4. Fuzzy name only marks a possible match; it cannot merge records automatically.

Automatic merges must be explainable and repeatable. Ambiguous candidates remain separate rather than risking a destructive merge.

### Location evidence

- `confirmed_office`: official company evidence plus another independent family, or two independent non-syndicated families agreeing spatially.
- `probable_office`: one credible location source.
- `job_location_only`: a recent job names the location but no credible office evidence exists.

Remote-only jobs enrich a known company but never make a distant company appear nearby. In-office or hybrid jobs may create `job_location_only` evidence when their location can be geocoded inside the selected radius.

### Confidence

Confidence is derived from unique evidence families, evidence type, agreement, and recency—not raw page count. Repeated sightings refresh `last_seen_at` but do not repeatedly add weight. Verification rules should return both a score and machine-readable reasons for UI/debugging.

## Technical Job Classification

The first version uses deterministic title/category rules, not an LLM. It recognizes software engineering, data, infrastructure, security, QA/automation, product engineering, technical support, technical leadership, and related roles. Negative rules exclude clearly non-technical uses of ambiguous words.

Work arrangement uses explicit structured fields and page text. If evidence conflicts or is absent, store `unknown`; do not infer remote/hybrid from weak hints.

“Recently hiring” means at least one technical posting with a trustworthy `posted_at >= now() - 14 days`. A missing publication date supports only a “recently observed” label based on `first_seen_at`.

## API Changes

Authenticated user endpoints:

- `POST /api/v1/discovery/jobs`
- `GET /api/v1/discovery/jobs`
- `GET /api/v1/discovery/jobs/{id}`
- `POST /api/v1/discovery/jobs/{id}/cancel`
- extend nearby-company responses with presence and recent technical-job summaries
- add company technical-job listing endpoint

Internal worker endpoint:

- `POST /api/v1/internal/discovery/batches`

The internal endpoint uses a dedicated worker token, constant-time credential comparison, request-size limits, timeouts, and structured audit logs. It is not exposed through the public frontend.

## Frontend Changes

- Reuse the existing “near me” and picked-location controls.
- Route new discovery actions to the discovery-job endpoints without changing the existing Go scrape-job controls.
- Show per-source progress and partial failure without blocking available results.
- Display company presence status separately from hiring status.
- Display recent technical-job count and arrangement badges.
- Distinguish “posted within 14 days” from “recently observed; publication date unavailable.”

The discovery concept remains map/list based; no swipe or matching UI is required.

## Queueing, Concurrency, and Cancellation

- Claim jobs with `FOR UPDATE SKIP LOCKED` and a finite lease.
- Heartbeat while work is active; reclaim expired leases.
- Check cancellation between source/page batches and before ingestion.
- Keep global and per-domain concurrency separately configurable.
- Limit Playwright browser contexts independently from HTTP concurrency.
- Batch ingestion by record count and payload size.
- Scale by running additional identical Python workers against the same PostgreSQL queue.

Redis or another broker becomes justified only when PostgreSQL queue contention or workload isolation is measured as a bottleneck.

## Failure Handling

- A source failure marks its source run failed but allows other adapters to finish.
- A job with usable results and one or more source failures ends as `partial`.
- Validation failures return per-record errors and remain visible in source-run metrics.
- Transient network failures retry; deterministic parse/validation failures do not.
- Browser crashes recreate only the affected browser context and bounded source batch.
- Database or ingestion outages preserve the lease and retry within the attempt budget.
- Logs must exclude tokens, private credentials, and unnecessary job-description content.

## Observability

Use structured logs with job ID, source, source family, URL host, duration, outcome, and record counts. Track at least:

- queue depth and lease age
- jobs completed/partial/failed
- request and parse outcomes by source
- robots-blocked and policy-skipped URLs
- companies, locations, and jobs accepted/rejected
- duplicate and ambiguous-match counts
- HTTP versus Playwright request counts
- ingestion latency and batch size

Local logs and database source-run rows are sufficient initially; a separate metrics platform is deferred.

## Testing

- Parser tests use saved HTML/JSON fixtures and never require live third-party sites.
- Contract tests share JSON fixtures between Python and Go.
- Unit tests cover URL safety, normalization, technical-role classification, work arrangement, publication-time confidence, and source lineage.
- PostgreSQL integration tests cover leasing, heartbeat expiry, idempotent ingestion, cancellation, and independent-family confidence.
- Go verifier tests prove non-tech companies are retained and syndicated evidence does not inflate confidence.
- One Docker Compose end-to-end test uses a local fake HTML/JSON/JavaScript source, creates a location job, runs Python discovery, ingests evidence, and queries the resulting company and job.
- CI does not scrape live websites.

## Local Development

Docker Compose adds a `python-discovery` service with PostgreSQL access and the internal worker token. The existing Go API, Go crawler, frontend, and database services remain. Make targets should wrap build, run, test, and logs rather than requiring developers to remember raw container commands.

Configuration includes:

- worker concurrency and lease duration
- HTTP and Playwright concurrency
- per-domain delay, timeout, response-size, and page limits
- enabled source adapters
- technical-job freshness window, defaulting to 14 days
- internal ingestion URL and token

Safe defaults belong in versioned example configuration; secrets remain in local environment files.

## Delivery Phases

### Phase 1: Contracts and queue

- Add migrations, Go discovery-job APIs, internal ingestion contract, and Python worker skeleton.
- Implement leasing, heartbeat, cancellation, source-run tracking, and a fake source.
- Prove the full local path without touching the existing Go crawler.

### Phase 2: Company discovery

- Add JSON-LD, configured-directory, and official-company-site adapters.
- Store all companies and source evidence.
- Update matching and independent-source verification.
- Surface presence states in nearby search.

### Phase 3: Technical jobs

- Add Greenhouse, Lever, generic `JobPosting`, and bounded careers-page adapters.
- Add technical-role and work-arrangement rules.
- Store and display jobs posted within 14 days.

### Phase 4: JavaScript fallback

- Add Playwright only to adapters with fixture-proven rendering requirements.
- Add browser pool limits, JavaScript fixtures, and browser failure recovery.

### Phase 5: Hardening and coverage

- Measure source yield and failure rates.
- Add new compliant source adapters in priority geographies.
- Tune matching thresholds from reviewed ambiguous cases.
- Add worker replicas only when local throughput demonstrates need.

## Acceptance Criteria

- A user can choose current or picked coordinates and radius and start a Python discovery job.
- The existing Go crawler still builds, tests, and runs without changes to its scraper/crawler packages.
- At least one ordinary HTML/JSON source and one JavaScript fixture source complete through the same adapter contract.
- Partial source failure still produces searchable companies and a `partial` job status.
- Re-ingesting the same source records does not create duplicate companies, sightings, or jobs.
- Every result exposes its evidence URLs, source family, and observation time.
- Nearby results distinguish confirmed, probable, and job-location-only presence.
- Technical jobs expose arrangement and publication-time confidence.
- Only trustworthy postings from the previous 14 days count as recently hiring.
- Remote-only evidence does not create a false nearby company location.
- Multiple worker processes can claim distinct jobs without double-processing.
- The complete local end-to-end test runs without contacting third-party websites.

## Deferred Decisions

The following are intentionally deferred until measurements justify them; they are not implementation blockers:

- Redis/Kafka or an external workflow engine
- proxy rotation or distributed browser infrastructure
- machine-learning/LLM classification
- automatic fuzzy company merges
- country-specific registry adapters beyond the first configured sources
- long-term job-description archiving
- production deployment and autoscaling configuration

