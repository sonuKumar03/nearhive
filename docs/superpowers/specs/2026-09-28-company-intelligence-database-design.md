# NearHive Company Intelligence Database Design

> 🟡 **DRAFT / FORWARD-LOOKING — reconciled 2026-09-28.** This umbrella spec is not yet
> an implementation plan. Ownership has settled: ingestion is Python-only and Go
> has read-only access to discovery tables (Go writes only `users`). The "internal
> worker token" ingestion path referenced in §12 is retired — the Python operations
> API authenticates user JWTs only. The body has been audited against the landed
> [Python-Owned Discovery and Go Read API Plan](../plans/2026-09-28-python-owned-discovery-architecture.md)
> implementation (master, PR #28); §3.2, §6.6, §6.11, and §12 were corrected accordingly.

**Status:** Draft for review  
**Date:** 2026-09-28  
**Scope:** Canonical dataset, ingestion provenance, hiring projections, user preferences, moderation, reviews, compensation, and interview intelligence  
**Out of scope:** Frontend redesign, swipe interactions, a dedicated SPA, recommendation-model training, Elasticsearch, Redis, Kafka, and data-vendor selection

## 1. Purpose

NearHive needs a trustworthy dataset that can answer four progressively broader questions:

1. Which companies have a real presence near a coordinate?
2. Which companies are actively hiring nearby, and which offer India-eligible remote work?
3. Which companies has a user saved or dismissed?
4. What do trustworthy reviews, compensation submissions, and interview experiences say about a company?

The primary delivery remains nearby company and hiring discovery. Community intelligence is built on the same identity, provenance, and moderation foundations without delaying the primary dataset.

## 2. Confirmed Product Decisions

- India-first, with ISO country and currency fields that permit global expansion.
- Target approximately 1 million companies, 10 million historical jobs, and 100,000 active users.
- Technical roles launch first; all-role support must not require another schema redesign.
- Nearby and India-eligible remote opportunities form one queryable candidate dataset.
- Company hiring status is derived and multi-label.
- Reviews, compensation, and interview information may combine user, company/admin, licensed, and public sources.
- User contributions appear anonymous publicly but remain account-linked internally.
- PostgreSQL/PostGIS is the canonical store and initial search engine.
- Query-oriented projection tables provide low-latency reads; canonical writes remain normalized.
- Salary cohorts require at least five distinct approved contributors before publication.

## 3. Current Repository Audit

### 3.1 Foundations to retain

| Existing capability | Current implementation | Decision |
|---|---|---|
| Company identity | `companies` | Extend with aliases and multiple domains |
| Spatial offices | `locations` with PostGIS geography and GiST | Retain physical table; extend semantics and observation dates |
| Jobs | `technical_job_postings` with multi-row `job_locations` | Generalize to canonical job postings before non-technical ingestion |
| Evidence | `sightings`, discovery source fields, first/last seen | Migrate toward registered sources and durable source records |
| Discovery operations | `discovery_jobs`, `discovery_source_runs` | Retain as operational run history |
| Users | Email/password/JWT users | Extend lifecycle; remove shared-guest identity before personal data |
| Nearby search | `ST_DWithin` over office locations (companies) and `job_locations` (jobs, at `GET /api/v1/search/jobs`) | Retain and add blended local+remote candidate queries with cursor pagination |
| Text search | pg_trgm company indexes | Retain and extend to normalized job titles |
| Work arrangement | `in_office`, `hybrid`, `remote`, `unknown` | Retain |
| Job freshness | publication state, posted/seen timestamps | Separate active, recently posted, closed, and stale rules |

### 3.2 Gaps (audited 2026-09-28 against master)

- Remote eligibility has no country, region, or time-zone representation (only `work_arrangement` exists).
- The physical table name and model assume every future job is technical (`technical_job_postings`).
- Source names are strings rather than registered source identities with terms, trust, and refresh policy.
- Canonical field selection cannot be reproduced consistently when sources conflict.
- Company nearby search attaches company-wide job counts, not job locality. (A job-locality search exists at `GET /api/v1/search/jobs` via `ST_DWithin` over `job_locations`, but there is no blended local+remote candidate query, role-family filter, or cursor pagination.)
- No saved/dismissed state, contributor identity model, moderation queue, or audit trail exists.
- No review, compensation, interview, question, or privacy-safe aggregate schema exists.
- The frontend auto-uses one shared guest account (`guest@nearhive.com`, auto-logged in by `web/src/hooks/useAuth.ts`), which cannot safely own personal preferences or community content.

Closed since the draft was written: multi-row `job_locations` now exists with ordinal, coordinates, and a GiST index, so the earlier "one location per job" gap is resolved at the storage level; §6.6's remaining work is enriching it with structured address fields, provenance, and lifecycle columns.

## 4. Architectural Model

```text
External sources and user/company submissions
                    |
                    v
        source_records + ingestion_runs
                    |
                    v
      normalized observations and identity links
                    |
                    v
 companies -- locations -- job_postings -- job_locations
                    |
                    v
 hiring/reputation/salary/interview projection tables
                    |
                    v
  location, hiring, and company-intelligence queries
```

### 4.1 Canonical versus observational data

- **Canonical tables** contain the current best identity and state used by product queries.
- **Source records** retain what a particular source asserted and when it was observed.
- **Link tables** explain which source records support each canonical entity.
- **Projection tables** are disposable, reproducible read models. They never become the only copy of user or source facts.
- **Operational tables** describe ingestion and refresh execution, not product truth.

### 4.2 Database conventions

- UUID primary keys.
- UTC `TIMESTAMPTZ` timestamps.
- ISO 3166-1 alpha-2 country codes and ISO 4217 currency codes.
- Monetary values stored as `BIGINT` minor units; no floating-point money.
- PostgreSQL `GEOGRAPHY(POINT, 4326)` for searchable coordinates.
- Lowercase stable slugs for taxonomies and classifications.
- Text plus `CHECK` constraints for small evolving state sets; avoid PostgreSQL enums that complicate rollout.
- JSONB only for source payloads, rule explanations, and compact projection breakdowns—not for core relationships.
- Soft deletion only where audit, moderation, or user recovery requires it. Canonical source observations remain append-only.

## 5. Cross-Phase Source and Provenance Foundation

These tables are introduced in Phase 1 and reused by every later phase.

### 5.1 `data_sources`

Registry for every origin of imported or submitted data.

| Column | Type | Rules |
|---|---|---|
| `id` | UUID | Primary key |
| `slug` | VARCHAR | Unique stable identifier |
| `name` | VARCHAR | Display name |
| `source_type` | VARCHAR | `official_site`, `official_ats`, `open_dataset`, `licensed_import`, `company_submitted`, `user_submitted` |
| `source_family` | VARCHAR | Correlation family used by evidence verification |
| `trust_tier` | SMALLINT | 1–5; entity-specific rules still decide confidence |
| `license_name` | VARCHAR nullable | Human-readable license or agreement |
| `terms_url` | TEXT nullable | Terms or source policy reference |
| `refresh_interval_seconds` | INTEGER nullable | Expected observation cadence |
| `enabled` | BOOLEAN | Disable without erasing history |
| timestamps | TIMESTAMPTZ | Created and updated |

Indexes: unique `slug`; `(source_type, enabled)`.

### 5.2 `ingestion_runs`

One execution of a source adapter or import.

Required fields: source, discovery job when applicable, adapter/rule version, status, start/end, accepted/rejected counts, cursor/checkpoint, and bounded error summary. Existing `discovery_source_runs` remains the user-visible discovery execution record and may reference `ingestion_runs` rather than being replaced. All ingestion runs are initiated by the Python worker.

Indexes: `(source_id, started_at DESC)`, `(status, started_at)`, optional `discovery_job_id`.

### 5.3 `source_records`

Append/update record for a source-native entity.

Required fields:

- `source_id`, `ingestion_run_id`
- `record_type`: company, location, job, review, compensation, interview, question
- `external_id` when supplied
- canonical source URL
- raw payload checksum and normalized payload checksum
- bounded `normalized_payload JSONB`
- first seen, last seen, source-updated, and source-deleted timestamps
- validation state and validation errors

Uniqueness:

- `(source_id, record_type, external_id)` when an external ID exists.
- `(source_id, record_type, normalized_payload_hash)` otherwise.

Raw full responses may be stored outside the main database or retained temporarily. The core database retains normalized payloads, checksums, and lineage needed to reproduce canonical decisions.

### 5.4 Entity-source links

Use explicit foreign-keyed tables rather than a polymorphic entity ID:

- `company_source_links(company_id, source_record_id, relation, confidence)`
- `location_source_links(location_id, source_record_id, relation, confidence)`
- `job_source_links(job_id, source_record_id, relation, confidence)`

Each has a unique pair constraint and observation timestamps. Later community records reference `source_records` directly through their shared content parent.

### 5.5 Confidence rules

Confidence is not one universal company score. Store separate confidence for:

- company identity;
- physical location;
- job identity;
- job activity/freshness;
- work arrangement;
- compensation provenance.

Each computed value records a rule version and machine-readable reason list. Official does not automatically mean spatially correct: an official careers page may be authoritative for a job but not prove a physical office.

## 6. Phase 1 — Canonical Company and Job Dataset

### 6.1 Objective

Produce a stable, source-traceable company/location/job dataset that supports multiple job locations, India-eligible remote roles, technical-first classification, and future role families.

### 6.2 Company identity

#### Extend `companies`

Retain the current table and fields. Add:

- `legal_name` nullable;
- `status`: active, acquired, merged, closed, unknown;
- `identity_confidence` and `identity_rule_version`;
- `primary_source_record_id` nullable;
- `first_seen_at`, `last_seen_at`;
- `merged_into_company_id` nullable self-reference;
- `deleted_at` only for erroneous records, not ordinary closed companies.

The current normalized-name uniqueness constraint must be relaxed before global expansion because unrelated companies may share a normalized name. Uniqueness comes from verified identity evidence, not name alone.

#### `company_aliases`

Fields: company, alias, normalized alias, alias type (`brand`, `former_name`, `legal_name`, `source_name`), country scope, source record, active dates. Unique on `(company_id, normalized_alias, alias_type)`.

#### `company_domains`

Fields: company, normalized domain, domain type, is primary, verified at, verification method, source record, active dates. A partial unique index permits one active primary domain per company. A domain can transfer during acquisitions, so do not impose global historical uniqueness without active-date semantics.

### 6.3 Company locations

Keep the current physical `locations` table to minimize disruption; treat it as the canonical company-location table. Add:

- `location_type`: headquarters, office, coworking, registered, job_only, unknown;
- `status`: active, inactive, unverified;
- ISO `country_code`; retain current country text temporarily during migration;
- `timezone` nullable;
- `first_seen_at`, `last_seen_at`, `closed_at`;
- `location_rule_version` and confidence reasons;
- normalized address hash for deduplication.

The existing GiST coordinate index remains. Add `(company_id, status)` and `(country_code, city)` indexes. A unique exact-coordinate constraint is not valid because several companies may share a building.

### 6.4 Role and skill taxonomy

#### `role_families`

Seed stable slugs for engineering, data, product, design, security, IT, sales, marketing, operations, finance, HR, legal, customer support, and other. Fields include parent family, display name, `is_technical`, and active flag.

Technical-first ingestion initially admits only technical families to active discovery. Non-technical families exist so later adapters do not require schema work.

#### `skills`

Canonical skill slug/name plus optional category. `skill_aliases` maps source spellings to canonical skills.

### 6.5 Canonical jobs

Generalize `technical_job_postings` into canonical `job_postings`. A table rename is allowed only in the same release that updates every Go query and ingestion path; otherwise add the new general fields first and defer the physical rename until non-technical ingestion begins.

Required canonical fields:

- company and discovery lineage;
- title and normalized title;
- role family, technical flag, classifier version, reasons;
- seniority: intern, entry, mid, senior, lead, manager, director, executive, unknown;
- employment type: full_time, part_time, contract, internship, temporary, unknown;
- work arrangement: existing four values;
- canonical URL and canonical fingerprint;
- state: open, closed, stale, unknown;
- source-posted, first-seen, last-seen, source-closed, and canonical-closed timestamps;
- posted-time confidence and activity confidence;
- description excerpt and content hash;
- created/updated timestamps.

The existing source fields remain during migration but canonical source relationships move to `job_source_links`.

#### Job deduplication order

1. Same source and stable external job ID.
2. Same normalized canonical URL.
3. Same company plus provider-independent fingerprint of normalized title, stable locations, and description content.
4. Otherwise create a new job; never fuzzy-merge jobs solely by title.

### 6.6 Multiple job locations

#### `job_locations`

| Field group | Contents |
|---|---|
| Identity | job ID, ordinal, location kind |
| Canonical link | optional company `location_id` |
| Raw evidence | raw and normalized location text |
| Address | city, state, country code, postal code |
| Spatial | geography point, coordinate confidence |
| Provenance | source record, resolution method, resolver version |
| Lifecycle | first seen, last seen, active flag |

Location kind: `office`, `stated_job_location`, `inferred`, `unknown`. Inferred coordinates never overwrite stronger structured/provider coordinates.

Indexes: GiST on coordinates where active; `(job_id, active)`; `(country_code, city)`.

The `job_locations` table already exists with `ordinal`, raw location text, coordinates, `coordinate_source`, confidence, and lifecycle flags; this phase extends it with structured address fields (city, state, country code, postal code), source-record provenance, and a location `location_kind`. No dual-write from legacy job columns is needed — the current schema has no per-job location columns outside `job_locations`.

### 6.7 Remote eligibility

#### `job_remote_eligibility`

One job may have several explicit scopes.

- `scope_type`: global, country, region, timezone;
- `scope_code`: `GLOBAL`, ISO country, normalized region, or IANA time-zone identifier;
- source record and confidence;
- first/last seen and active flag.

India-eligible means an active `global` row or an active country row with `IN`. Do not infer India eligibility from the word “remote” alone.

Indexes: `(scope_type, scope_code, active, job_id)` and `(job_id, active)`.

### 6.8 Job skills

`job_skills(job_id, skill_id, source_record_id, confidence)` with a unique job/skill pair. Skills are enrichment, not required for a job to be canonical.

### 6.9 Activity and freshness rules

Keep these distinct:

- **Open:** source explicitly reports open and the source was successfully checked within its refresh SLA.
- **Recently posted:** trustworthy posted timestamp is within 14 days.
- **Recently observed:** seen recently but posted time is absent or weak.
- **Stale:** refresh SLA exceeded or repeated checks cannot confirm the posting.
- **Closed:** source explicitly closes/removes the job or the configured missing-observation threshold is reached.

Hiring projections count confirmed open jobs. When a source cannot expose open/closed state, a trustworthy post within 14 days may count with lower activity confidence. Merely observing an undated page does not establish “recently hiring.”

### 6.10 Merge review

`company_merge_candidates` stores left/right company IDs, domain/name/location signals, generated score, reasons, status, reviewer, and decision timestamps. Domain or registration identity may auto-merge under deterministic rules; fuzzy name-only matches require review.

### 6.11 Phase 1 migration and rollout

1. Add source registry, runs, records, and link tables.
2. Register currently active sources: OSM and Greenhouse are configured in `config/python_sources.yaml`; Lever is implemented but unconfigured; the company-site crawl and JSON-LD extraction are pipeline enrichment stages that register as sources when they produce evidence.
3. Backfill source records and links from current source strings, sightings, and technical jobs.
4. Add company alias/domain and location lifecycle fields.
5. Add taxonomies and general job fields.
6. Extend `job_locations` and add remote eligibility.
7. Backfill and compare old versus new read results.
8. Switch canonical readers to the new relationships.
9. Remove legacy columns only in a later cleanup after rollback windows expire.

### 6.12 Phase 1 acceptance

- Existing company and technical-job APIs retain their data after migration.
- Every new canonical job links to at least one source record.
- A job supports zero, one, or many physical locations and zero or many remote scopes.
- India-remote eligibility is explicit and reproducible.
- Duplicate delivery updates observation time without duplicating canonical companies/jobs.
- Ambiguous company identity creates a review candidate instead of an automatic fuzzy merge.

## 7. Phase 2 — Hiring Classification and Discovery Projections

### 7.1 Objective

Return blended nearby and India-remote hiring candidates without joining all canonical, observation, and community tables at request time.

### 7.2 Projection tables

#### `company_hiring_summary`

One row per company:

- open and recently-posted job counts;
- technical open counts;
- in-office, hybrid, remote, and unknown counts;
- role-family counts as bounded JSONB;
- latest trustworthy job and verification timestamps;
- activity confidence;
- projection version and refreshed timestamp.

#### `company_location_hiring_summary`

One row per company/location:

- active in-office and hybrid counts;
- recently-posted counts;
- technical counts;
- role-family counts;
- latest job/verification timestamps;
- confidence and refresh version.

Only jobs explicitly linked to the location or spatially resolved within the configured matching distance contribute. A company office does not localize an unrelated job automatically.

#### `company_remote_hiring_summary`

One row per company/country scope:

- active remote counts;
- technical and role-family counts;
- most recent job/verification timestamps;
- confidence and refresh version.

India queries use country code `IN`; global-eligible jobs are included in the recomputation.

#### `company_discovery_projection`

One compact company row containing fields needed by discovery ranking: canonical name/domain/industry/size, company confidence, active hiring totals, latest hiring timestamp, and later reputation/compensation summary pointers. It contains no raw source payloads or review bodies.

### 7.3 Derived multi-label classification

Labels are query output derived from projections:

- `hiring_nearby_in_office`: nearby location summary has at least one qualifying in-office job.
- `hiring_nearby_hybrid`: nearby location summary has at least one qualifying hybrid job.
- `hiring_remote_india`: India remote summary has at least one qualifying remote job.
- `mixed_hiring`: at least two of the preceding labels apply.
- `nearby_not_hiring`: company has an active nearby location but no qualifying active hiring fact.
- `insufficient_data`: location or job sources have not been checked within their expected freshness window.

Because “nearby” depends on the request coordinate and radius, nearby labels are not stored as universal company attributes.

### 7.4 Candidate query

1. Use `ST_DWithin` on active company locations and active job locations.
2. Join only compact location/company hiring projections.
3. Read `IN` and global rows from remote hiring summaries.
4. `UNION ALL` local and remote candidates.
5. Group by company, retaining matched labels and minimum distance.
6. Apply role-family, arrangement, confidence, and hiring-state filters.
7. Order through a stable, versioned ranking expression.
8. Paginate with an opaque cursor containing ranking version, sort values, and company ID.

The initial relevance expression uses deterministic features only: qualifying hiring count, freshness, confidence, distance decay for local facts, and later reputation completeness. Personalized behavior is deferred, but saved/dismissed state can filter results after Phase 3.

### 7.5 Indexes

- Existing GiST company-location index.
- Partial GiST job-location index where active coordinates exist.
- Partial job indexes for open/recent jobs.
- `(company_id, state, last_seen_at DESC)` on canonical jobs.
- `(role_family_id, state, last_seen_at DESC)` for filtering.
- pg_trgm GIN on normalized company and job titles.
- Primary/unique keys on every projection grain.
- `(country_code, active_remote_count DESC, refreshed_at)` on remote summaries.
- Refresh queue claim index described below.

### 7.6 Incremental refresh

#### `company_projection_refresh_queue`

One row per dirty company, upserted by canonical job/company/location changes.

Fields: company ID primary key, reasons array or JSONB, first/last requested timestamps, available time, attempts, lease owner/expiry, last error.

A worker claims with `FOR UPDATE SKIP LOCKED`, recomputes all company projections in one transaction, then deletes the queue row. Repeated changes collapse into one refresh request. No complex database triggers recompute aggregates; application ingestion explicitly marks affected companies.

Provide an idempotent full rebuild command that creates replacement projections, validates counts, and swaps or truncates/repopulates safely.

### 7.7 Service targets

- Normal projection freshness: under five minutes.
- Candidate database query: p95 at or below 200 ms at target data volume, excluding network latency.
- Cursor pages remain stable for one ranking-version window.
- Projection failure never corrupts canonical data; it causes visible staleness and retry.

### 7.8 Phase 2 acceptance

- A nearby company with no local job is distinguishable from a nearby company hiring locally.
- Remote-only companies eligible in India appear without fabricated distance.
- Remote jobs with unknown eligibility do not appear as India-eligible.
- Stale, closed, and weak undated observations do not inflate active hiring labels.
- Rebuilding projections produces the same facts as incremental updates.
- Target-volume query plans use spatial/partial indexes and meet the latency budget.

## 8. Phase 3 — User Preferences and Content Governance

### 8.1 Objective

Provide safe ownership, saving/dismissal, and moderation infrastructure before collecting community company intelligence.

### 8.2 User lifecycle

Extend `users` with account status, verified-at, disabled-at, and deleted-at fields. The shared `guest@nearhive.com` behavior must not own persistent preferences or submissions. Anonymous browsing may remain, but state-changing operations require a unique authenticated user.

### 8.3 `user_company_preferences`

One row per user/company when non-neutral state exists.

- state: saved or dismissed;
- notification preference;
- created, updated, saved, dismissed timestamps;
- optional source context identifying where the action occurred.

Primary key `(user_id, company_id)`. State transitions update the row; no duplicate saved rows. An optional append-only interaction stream is deferred until a recommendation system has a proven use for it.

### 8.4 `user_job_preferences`

Query-ready user intent, not behavioral inference:

- role-family selections;
- seniority selections;
- work-arrangement selections;
- maximum commute radius;
- remote-country eligibility preference;
- optional salary floor with currency;
- updated timestamp and schema version.

Use normalized child rows for multi-select role and arrangement preferences if direct indexing is required; do not hide every preference in one JSON document.

### 8.5 Shared content parent

#### `community_content`

Every review, compensation submission, interview experience, and interview question has a shared moderation identity.

Fields:

- content type;
- internal contributor user ID, nullable only after compliant anonymization;
- origin: user, company, admin, licensed import, public import;
- optional source record;
- moderation state: pending, published, flagged, rejected, removed;
- public anonymity flag, fixed true for user contributions in the first release;
- automated validation/PII flags;
- submission, publication, update, removal timestamps;
- soft-delete reason;
- row version for safe edits.

Public responses never expose contributor user ID.

### 8.6 Moderation

#### `content_reports`

Reporter, content ID, reason code, bounded details, state, created/resolved timestamps. Limit one open report per reporter/content/reason.

#### `moderation_actions`

Append-only actions containing content, moderator, prior/new state, reason code, bounded note, rule version, and timestamp. Updates to moderated content create a new pending revision rather than silently altering published text.

#### `content_revisions`

Content ID, revision number, structured payload, checksum, author, and timestamp. Public reads use the currently published revision.

### 8.7 Privacy and abuse boundaries

- Rate limiting is enforced by the API, supported by indexed contributor/time queries; no new rate-limit database is required initially.
- Automated validation rejects secrets, direct contact information, and obviously identifying personal data before publication.
- Account deletion removes personal profile access and follows an explicit retention/anonymization workflow; moderation evidence needed for abuse prevention is minimized and access-controlled.
- Salary exact values are never returned through public endpoints.
- Moderation tables are not exposed through ordinary company APIs.
- Legal retention details must be reviewed before production; this spec defines the required deletion/anonymization capability, not legal advice.

### 8.8 Phase 3 acceptance

- Two users can independently save or dismiss the same company.
- Shared guest traffic cannot mutate persistent user-owned data.
- Public content contains no contributor identifier.
- Every moderation state change has an immutable actor, reason, and timestamp.
- Removed content disappears publicly but remains auditable to authorized moderators.
- A deleted user can be anonymized without deleting canonical companies or aggregate statistics.

## 9. Phase 4 — Reviews, Compensation, and Interview Intelligence

### 9.1 Objective

Store structured, source-aware company intelligence and publish only privacy-safe aggregates.

### 9.2 Company reviews

#### `company_reviews`

One-to-one with `community_content` plus:

- company;
- optional company location and role family;
- employment relationship: current employee, former employee, contractor, candidate, unknown;
- tenure-month band rather than exact dates where privacy requires;
- overall, culture, work-life, management, compensation, and growth ratings, each 1–5 nullable except overall;
- title, pros, cons, and advice text;
- experience month/year;
- recommended-company boolean nullable.

Validation constrains rating ranges and text lengths. Only published current revisions contribute to summaries.

#### `company_reputation_summary`

One row per company with published count, overall/dimension averages, recommendation rate, last contribution date, and refreshed timestamp. Show count with every aggregate. Do not calculate a public rating from fewer than the configured minimum approved reviews; initial default is three.

### 9.3 Compensation records

#### `compensation_records`

One-to-one with `community_content` plus:

- company, role family, normalized title, seniority;
- experience-year band;
- optional company/job location, country code, and arrangement;
- currency and original pay period;
- base minimum/maximum;
- variable/bonus minimum/maximum;
- annualized equity minimum/maximum;
- other compensation minimum/maximum;
- stated total minimum/maximum;
- normalized annual total minimum/maximum;
- compensation year;
- normalization rule version and confidence.

For an individual user submission, minimum and maximum may be equal. Company or licensed salary bands retain their range. Annualization never overwrites original values.

#### `company_salary_summary`

Grain: company, role family, seniority, country/location cohort, arrangement, currency, and compensation year window.

Store contributor count, record count, p25, median, p75, optional min/max suppressed against outliers, component medians, refresh version, and published flag. Publish only when at least five distinct approved user contributors exist, or when a separately labeled verified company/licensed band is being shown. Imported records never inflate user-contributor anonymity counts.

### 9.4 Interview experiences

#### `interview_experiences`

One-to-one with `community_content` plus:

- company, role family, normalized title, seniority, optional location;
- application channel;
- process start month/year;
- duration band;
- difficulty 1–5;
- experience sentiment;
- outcome: offer, rejected, withdrawn, pending, undisclosed;
- offer accepted nullable;
- process summary.

Exact interview dates are not required publicly.

#### `interview_stages`

Ordered stages per experience: application, recruiter, assessment, technical, system design, manager, panel, HR, other. Store sequence, mode, duration band, and notes.

#### `interview_questions`

Canonical normalized question text, question type, role family, and content hash. It contains no contributor identity.

#### `interview_question_mentions`

Links a published interview experience to a canonical question with stage and optional context. Duplicate wording maps to the same question only after deterministic normalization; semantic/LLM merging is deferred.

#### `company_interview_summary`

Company/role-family aggregates: experience count, median difficulty, outcome distribution, common stage distribution, recent activity, and refresh version. Show sample size and suppress low-count breakdowns where they risk contributor identification.

### 9.5 Multi-origin records

- User content uses the internal contributor and moderation flow.
- Company/admin records identify the asserting organization and verification state.
- Licensed/public imports require a registered source and source record.
- Conflicting facts coexist as observations. Canonical projections choose values by source type, freshness, and validation rules without deleting disagreement.
- Public output labels user aggregates, company-provided bands, and imported facts distinctly.

### 9.6 Phase 4 acceptance

- Reviews expose sample size, moderation state, and dimension averages without author identity.
- Salary summaries do not publish below the five-contributor threshold.
- Original and annualized compensation values remain reproducible.
- Company-provided and user-reported salary data cannot be mistaken for one another.
- Interview questions link to approved experiences and deduplicate exact normalized duplicates.
- Removed or revised contributions stop affecting projections after refresh.

## 10. Projection Refresh and Consistency

All Phase 2 and Phase 4 summaries use the same dirty-company queue pattern.

Canonical and community transactions must:

1. write canonical/submission changes;
2. upsert the affected company into the refresh queue in the same transaction;
3. commit;
4. allow a worker to recompute all dependent projections idempotently.

The worker records projection version and source watermark. Readers may expose `refreshed_at` and a stale indicator. Failed refreshes retry with bounded backoff and become visible operational failures after the attempt ceiling. Canonical writes never roll back because a projection refresh later fails.

## 11. Data Retention and Lifecycle

Initial defaults, adjustable after operational/legal review:

- Canonical companies and identity/merge history: durable.
- Canonical jobs, including closed jobs: durable at the target 10-million-row scale; exclude closed rows from active partial indexes.
- Source normalized observations and checksums: durable while they support canonical history.
- Full raw HTTP/import payloads: 30 days unless licensing, debugging, or dispute policy requires less.
- Ingestion operational logs/errors: 90 days, retaining aggregate run metadata longer.
- Published community content: until author deletion, moderation removal, or source/license expiry.
- Deleted-account contributor link: anonymize after the defined recovery/security window while retaining non-identifying aggregate eligibility where lawful.
- Moderation actions: durable, access-controlled, and minimized.
- Search history: define a separate product retention policy before using it for personalization; it is not required for dataset correctness.

No initial time partitioning is required. Add partitioning only after measured vacuum, index, or retention costs justify it; source records and interaction events are the likely first candidates, not companies.

## 12. Security and Access Boundaries

- Ingestion is Python-only. The Python operations API and worker authenticate with user JWTs signed by the shared `JWT_SECRET`; there is no separate worker token. Production should support secret rotation and narrowly scoped credentials.
- User-owned writes derive `user_id` from authentication, never request bodies.
- Moderation and company-verified submissions require explicit roles, introduced before Phase 3/4 endpoints.
- Public/company APIs select explicit columns and never serialize internal user IDs, moderation notes, raw payloads, or source credentials.
- Source URLs and imported content pass the existing SSRF and bounded-fetch controls.
- Database roles should separate application reads/writes, migration rights, worker ingestion, and analytics/reporting access.
- Sensitive exports and moderator queries require audit logging.

## 13. Failure Handling

- Duplicate source delivery is idempotent through source identities and checksums.
- Partial ingestion commits valid records and records per-record rejection details.
- Source outages age confidence and freshness; they do not delete canonical companies immediately.
- Projection lag is visible and repairable.
- Ambiguous company merges wait for review.
- Geocoding failure preserves raw job location but prevents false spatial inclusion.
- Currency or period normalization failure preserves the original compensation record but excludes it from normalized aggregates.
- Moderation failure leaves content pending rather than public.

## 14. Verification Strategy

### Schema and migration

- Forward and backward migration tests on an existing populated schema.
- Constraint tests for every state, rating, currency, country, and money invariant.
- Backfill comparison reports for companies, locations, jobs, and current search results.
- Rollback rehearsals before removing legacy job-location columns.

### Identity and ingestion

- Repeated source records remain idempotent.
- Company domain matching and ambiguous merge behavior.
- Multi-source job deduplication and closure.
- Multiple locations and remote-scope eligibility.
- Provenance reconstruction from canonical rows to source records.

### Classification and projections

- Local office without a local job.
- Local in-office, local hybrid, India remote, mixed, not hiring, and insufficient-data cases.
- Stale, undated, closed, and conflicting source cases.
- Incremental refresh versus full rebuild equivalence.
- Failure/retry and dirty-row collapse.

### Community privacy and moderation

- Public serialization contains no contributor IDs.
- Five-contributor salary threshold across edits/removals.
- Reports and append-only moderation actions.
- Account deletion/anonymization behavior.
- Revision publication and projection removal.

### Performance

- Representative target-volume fixtures or generated data.
- `EXPLAIN (ANALYZE, BUFFERS)` assertions/manual gates for core candidate queries.
- p50/p95 measurements for local, remote, blended, saved, and filtered queries.
- Projection refresh throughput and backlog recovery.
- Index size, vacuum behavior, and write amplification measurements before adding partitions or secondary stores.

CI must use fixtures and local fake sources; it must not scrape or geocode public services.

## 15. Phase Dependencies and Delivery Gates

| Phase | Depends on | Ships independently when |
|---|---|---|
| 1. Canonical dataset | Current schema | Existing data migrated, provenance complete, multi-location/remote jobs queryable |
| 2. Hiring projections | Phase 1 | Rebuild/incremental results match and candidate query meets latency/correctness gates |
| 3. Preferences/governance | Phase 1 identity and auth | Personal state is isolated and moderation/audit/privacy controls pass |
| 4. Company intelligence | Phase 3 governance; Phase 1 companies/taxonomy | Review, salary, and interview projections obey provenance and privacy thresholds |

Phase 1 and Phase 2 deliver the primary product goal. Phase 3 must precede accepting public community submissions. Phase 4 may begin with licensed/company data only after provenance exists, but user submissions wait for Phase 3.

## 16. Explicit Non-Goals and Deferred Decisions

- No frontend or SPA design in these phases.
- No swipe-event schema until a real recommendation product is designed.
- No opaque ML classifier or LLM-based company/job merge.
- No public exact salary-submission endpoint.
- No scraping of sources without an approved access/license policy.
- No Elasticsearch/OpenSearch until PostgreSQL misses measured relevance or latency goals.
- No Redis until measured repeated reads justify cache invalidation complexity.
- No Kafka/Celery/workflow engine while the PostgreSQL lease/refresh queues meet throughput.
- No table partitioning without measured maintenance pressure.
- No global launch-specific tax, labor-law, or localization rules in the India-first release.

## 17. Required Follow-Up Plans

After this specification is approved, create four implementation plans in order:

1. Canonical company/job/provenance migration and ingestion plan.
2. Hiring classifications, projection refresh, and candidate-query plan.
3. User preferences, unique identity, moderation, and privacy plan.
4. Reviews, compensation, interview intelligence, and aggregate-projection plan.

Each plan must include exact migrations, Go/Python interfaces, backfill and rollback steps, fixture changes, query/index verification, and an independently shippable acceptance gate. Implementation must not begin from this umbrella specification alone.
