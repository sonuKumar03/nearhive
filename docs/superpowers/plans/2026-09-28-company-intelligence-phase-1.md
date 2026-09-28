# Company Intelligence Phase 1 — Provenance and Canonical Dataset Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every canonical company, location, and job row source-traceable provenance (`data_sources` → `ingestion_runs` → `source_records` → entity links), extend company identity (aliases, domains), add role/skill taxonomy, generalize job fields, extend `job_locations`, and add explicit remote eligibility.

**Architecture:** Python discovery owns all writes; `python-discovery/schema.sql` is the single schema source (Compose initializes fresh volumes from it — existing rows are disposable, so there are no ALTER-style migrations and no backfill). `persistence.py` extends its transactional `persist()` to write provenance rows in the same transaction as canonical rows. Go stays read-only; its queries keep working because no existing column or table is removed.

**Tech Stack:** PostgreSQL 17 / PostGIS 3.5, Python 3.12 + psycopg 3, pytest against `NEARHIVE_TEST_DATABASE_URL`.

**Spec:** `docs/superpowers/specs/2026-09-28-company-intelligence-database-design.md` (§5–§6.11; authoritative ownership per `docs/superpowers/plans/2026-09-28-python-owned-discovery-architecture.md`)

## Global Constraints

- Spec §4.2: UUID PKs; `TIMESTAMPTZ` UTC; ISO 3166-1 alpha-2 `country_code` (e.g. `'IN'`); `GEOGRAPHY(POINT, 4326)`; lowercase stable slugs; `VARCHAR + CHECK` for state sets (no PG enums); JSONB only for source payloads/breakdowns.
- Spec §5.3: `source_records` is append/update; unique on `(source_id, record_type, external_id)` when `external_id` present, else `(source_id, record_type, normalized_payload_hash)`.
- Spec §6.7: India eligibility = active `global` scope row or active `country` scope row with `scope_code='IN'`; never inferred from the word "remote".
- Deviation recorded in the spec audit: no backfill/dual-write — the schema initializes fresh and `persist()` writes provenance going forward (per the authoritative plan: "Existing scraped rows are disposable").
- All Python writes go through `PostgresPersistence.persist()` in one transaction; validation failures reject the batch before any write.
- Test DB: `NEARHIVE_TEST_DATABASE_URL` (see `make test-discovery`); tests must create their own unique companies via uuid suffixes like existing `test_persistence.py` tests.

## Review Focus

- An unknown `source` value is auto-registered in `data_sources` on first delivery (never a rejection); only blank/oversized `source` fails validation (existing rule) — pinned in Task 3 test `test_provenance_rows_written_and_idempotent` asserting the data_sources row exists after persist.
- Duplicate delivery of the same source record must update `last_seen_at` without duplicating `source_records` or canonical rows — pinned in Task 3 test `test_provenance_rows_written_and_idempotent`.
- Remote jobs with no `job_remote_eligibility` row must not count as India-eligible — pinned in Task 7 test `test_remote_without_scope_not_india_eligible`.
- Inferred job-location coordinates must never overwrite `structured`/`provider` coordinates on conflict — pinned in Task 6 test `test_inferred_does_not_overwrite_provider`.
- Existing Go reads (`/api/v1/companies/{id}/technical-jobs`, `/api/v1/search/jobs`) must still return data after the schema change — pinned in Task 8 via `make test`.

---

### Task 1: Provenance registry schema

**Files:**
- Modify: `python-discovery/schema.sql`

**Interfaces:**
- Produces: tables `data_sources`, `ingestion_runs`, `source_records`, `company_source_links`, `location_source_links`, `job_source_links` — exact columns below; later tasks and `persistence.py` rely on these names.

- [ ] **Step 1: Add the six provenance tables to `schema.sql`** (after `discovery_source_runs`, before `sightings`)

```sql
CREATE TABLE data_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    source_type VARCHAR(50) NOT NULL CHECK (source_type IN
        ('official_site', 'official_ats', 'open_dataset', 'licensed_import', 'company_submitted', 'user_submitted')),
    source_family VARCHAR(100) NOT NULL,
    trust_tier SMALLINT NOT NULL DEFAULT 3 CHECK (trust_tier BETWEEN 1 AND 5),
    license_name VARCHAR(255),
    terms_url TEXT,
    refresh_interval_seconds INTEGER,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX data_sources_type ON data_sources (source_type, enabled);

CREATE TABLE ingestion_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    data_source_id UUID NOT NULL REFERENCES data_sources(id),
    discovery_job_id UUID REFERENCES discovery_jobs(id) ON DELETE SET NULL,
    adapter_version VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'completed', 'partial', 'failed')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    accepted_count INTEGER NOT NULL DEFAULT 0,
    rejected_count INTEGER NOT NULL DEFAULT 0,
    cursor_state JSONB NOT NULL DEFAULT '{}',
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ingestion_runs_source ON ingestion_runs (data_source_id, started_at DESC);
CREATE INDEX ingestion_runs_status ON ingestion_runs (status, started_at);
CREATE INDEX ingestion_runs_discovery_job ON ingestion_runs (discovery_job_id);

CREATE TABLE source_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    data_source_id UUID NOT NULL REFERENCES data_sources(id),
    ingestion_run_id UUID REFERENCES ingestion_runs(id) ON DELETE SET NULL,
    record_type VARCHAR(30) NOT NULL CHECK (record_type IN
        ('company', 'location', 'job', 'review', 'compensation', 'interview', 'question')),
    external_id VARCHAR(500),
    source_url TEXT,
    normalized_payload JSONB NOT NULL DEFAULT '{}',
    normalized_payload_hash VARCHAR(64) NOT NULL,
    source_updated_at TIMESTAMPTZ,
    source_deleted_at TIMESTAMPTZ,
    validation_state VARCHAR(20) NOT NULL DEFAULT 'valid'
        CHECK (validation_state IN ('valid', 'invalid', 'pending')),
    validation_errors TEXT,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (external_id IS NOT NULL OR normalized_payload_hash IS NOT NULL)
);
CREATE UNIQUE INDEX source_records_external
    ON source_records (data_source_id, record_type, external_id) WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX source_records_hash
    ON source_records (data_source_id, record_type, normalized_payload_hash) WHERE external_id IS NULL;
CREATE INDEX source_records_seen ON source_records (data_source_id, record_type, last_seen_at DESC);

CREATE TABLE company_source_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    source_record_id UUID NOT NULL REFERENCES source_records(id) ON DELETE CASCADE,
    relation VARCHAR(50) NOT NULL,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, source_record_id)
);

CREATE TABLE location_source_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
    source_record_id UUID NOT NULL REFERENCES source_records(id) ON DELETE CASCADE,
    relation VARCHAR(50) NOT NULL,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (location_id, source_record_id)
);

CREATE TABLE job_source_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES technical_job_postings(id) ON DELETE CASCADE,
    source_record_id UUID NOT NULL REFERENCES source_records(id) ON DELETE CASCADE,
    relation VARCHAR(50) NOT NULL,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_id, source_record_id)
);
```

Note: `technical_job_postings` must be declared before `source_records`-linking tables that reference it, or links added after — place these tables after all entity tables in the file. `source_records` itself only references `data_sources`/`ingestion_runs`, so table order: `data_sources`, `ingestion_runs`, then (after entity tables) `source_records` and the three link tables.

- [ ] **Step 2: Verify schema applies cleanly**

Run: `docker compose down -v && make up && sleep 8 && docker compose exec -T db psql -U postgres -d nearhive -c '\dt'`
Expected: all six new tables listed; `make ps` shows all services healthy.

- [ ] **Step 3: Commit**

```bash
git add python-discovery/schema.sql
git commit -m "feat(schema): add provenance registry (sources, runs, records, links)"
```

---

### Task 2: Identity, taxonomy, job, and eligibility schema

**Files:**
- Modify: `python-discovery/schema.sql`

**Interfaces:**
- Consumes: `companies`, `locations`, `technical_job_postings`, `job_locations` (existing tables).
- Produces: `company_aliases`, `company_domains`, `role_families`, `skills`, `skill_aliases`, `job_skills`, `job_remote_eligibility`; new columns on `companies`, `locations`, `technical_job_postings`, `job_locations` as listed. Task 5–7 write these columns/tables.

- [ ] **Step 1: Extend entity tables in `schema.sql`**

Append to the end of the file in this order: (a) companies/locations ALTERs (block 1 below), (b) taxonomy/identity CREATE TABLE IF NOT EXISTS blocks (referenced by the job ALTERs, so they must come before them), then (c) job and job_locations ALTERs (block 2 below). Everything goes at the end so fresh volumes and re-runs both work:

```sql
-- (a) entity extensions: companies and locations

ALTER TABLE companies
    ADD COLUMN IF NOT EXISTS legal_name VARCHAR(500),
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'acquired', 'merged', 'closed', 'unknown')),
    ADD COLUMN IF NOT EXISTS identity_confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (identity_confidence BETWEEN 0 AND 1),
    ADD COLUMN IF NOT EXISTS first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS merged_into_company_id UUID REFERENCES companies(id),
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE locations
    ADD COLUMN IF NOT EXISTS location_type VARCHAR(30) NOT NULL DEFAULT 'unknown'
        CHECK (location_type IN ('headquarters', 'office', 'coworking', 'registered', 'job_only', 'unknown')),
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'unverified'
        CHECK (status IN ('active', 'inactive', 'unverified')),
    ADD COLUMN IF NOT EXISTS country_code VARCHAR(2) NOT NULL DEFAULT 'IN',
    ADD COLUMN IF NOT EXISTS timezone_name VARCHAR(64),
    ADD COLUMN IF NOT EXISTS first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS address_hash VARCHAR(64);
CREATE INDEX IF NOT EXISTS locations_company_status ON locations (company_id, status);
CREATE INDEX IF NOT EXISTS locations_country_city ON locations (country_code, city);

-- (b) taxonomy and identity tables

CREATE TABLE IF NOT EXISTS role_families (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(50) NOT NULL UNIQUE,
    parent_family_id UUID REFERENCES role_families(id),
    display_name VARCHAR(100) NOT NULL,
    is_technical BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO role_families (slug, display_name, is_technical) VALUES
    ('engineering', 'Engineering', true),
    ('data', 'Data', true),
    ('product', 'Product', true),
    ('design', 'Design', true),
    ('security', 'Security', true),
    ('it', 'IT', true),
    ('sales', 'Sales', false),
    ('marketing', 'Marketing', false),
    ('operations', 'Operations', false),
    ('finance', 'Finance', false),
    ('hr', 'HR', false),
    ('legal', 'Legal', false),
    ('customer_support', 'Customer Support', false),
    ('other', 'Other', false)
ON CONFLICT (slug) DO NOTHING;

CREATE TABLE IF NOT EXISTS company_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    alias VARCHAR(500) NOT NULL,
    normalized_alias VARCHAR(500) NOT NULL,
    alias_type VARCHAR(30) NOT NULL CHECK (alias_type IN ('brand', 'former_name', 'legal_name', 'source_name')),
    country_code VARCHAR(2),
    is_active BOOLEAN NOT NULL DEFAULT true,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, normalized_alias, alias_type)
);

CREATE TABLE IF NOT EXISTS company_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    normalized_domain VARCHAR(255) NOT NULL,
    domain_type VARCHAR(30) NOT NULL DEFAULT 'primary' CHECK (domain_type IN ('primary', 'secondary', 'legacy')),
    is_primary BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    verified_at TIMESTAMPTZ,
    verification_method VARCHAR(50),
    source_record_id UUID,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, normalized_domain)
);
CREATE UNIQUE INDEX company_domains_one_primary
    ON company_domains (company_id) WHERE is_primary AND is_active;

CREATE TABLE IF NOT EXISTS skills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(200) NOT NULL,
    category VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS skill_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    alias VARCHAR(200) NOT NULL,
    UNIQUE (skill_id, alias)
);

CREATE TABLE IF NOT EXISTS job_skills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES technical_job_postings(id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    source_record_id UUID,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1),
    UNIQUE (job_id, skill_id)
);

CREATE TABLE IF NOT EXISTS job_remote_eligibility (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES technical_job_postings(id) ON DELETE CASCADE,
    scope_type VARCHAR(20) NOT NULL CHECK (scope_type IN ('global', 'country', 'region', 'timezone')),
    scope_code VARCHAR(100) NOT NULL,
    source_record_id UUID,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1),
    is_active BOOLEAN NOT NULL DEFAULT true,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_id, scope_type, scope_code)
);
CREATE INDEX job_remote_eligibility_scope ON job_remote_eligibility (scope_type, scope_code, is_active);
CREATE INDEX job_remote_eligibility_job ON job_remote_eligibility (job_id, is_active);

-- (c) entity extensions: jobs and job_locations

ALTER TABLE technical_job_postings
    ADD COLUMN IF NOT EXISTS role_family_id UUID REFERENCES role_families(id),
    ADD COLUMN IF NOT EXISTS seniority VARCHAR(20) NOT NULL DEFAULT 'unknown'
        CHECK (seniority IN ('intern', 'entry', 'mid', 'senior', 'lead', 'manager', 'director', 'executive', 'unknown')),
    ADD COLUMN IF NOT EXISTS employment_type VARCHAR(20) NOT NULL DEFAULT 'unknown'
        CHECK (employment_type IN ('full_time', 'part_time', 'contract', 'internship', 'temporary', 'unknown')),
    ADD COLUMN IF NOT EXISTS state VARCHAR(20) NOT NULL DEFAULT 'open'
        CHECK (state IN ('open', 'closed', 'stale', 'unknown')),
    ADD COLUMN IF NOT EXISTS activity_confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (activity_confidence BETWEEN 0 AND 1);
CREATE INDEX IF NOT EXISTS jobs_state_seen ON technical_job_postings (company_id, state, last_seen_at DESC);

ALTER TABLE job_locations
    ADD COLUMN IF NOT EXISTS location_kind VARCHAR(30) NOT NULL DEFAULT 'unknown'
        CHECK (location_kind IN ('office', 'stated_job_location', 'inferred', 'unknown')),
    ADD COLUMN IF NOT EXISTS company_location_id UUID REFERENCES locations(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS city TEXT,
    ADD COLUMN IF NOT EXISTS state_province TEXT,
    ADD COLUMN IF NOT EXISTS country_code VARCHAR(2) NOT NULL DEFAULT 'IN',
    ADD COLUMN IF NOT EXISTS postal_code TEXT;
```

The SQL block above contains the full table definitions for `role_families` (with the 14 seeded families via `INSERT ... ON CONFLICT (slug) DO NOTHING`; technical: engineering, data, product, design, security, it), `company_aliases` (unique `(company_id, normalized_alias, alias_type)`), `company_domains` (unique `(company_id, normalized_domain)`; partial unique index `(company_id) WHERE is_primary AND is_active`), `skills`/`skill_aliases`/`job_skills`, and `job_remote_eligibility` (unique `(job_id, scope_type, scope_code)`; indexes `(scope_type, scope_code, is_active)` and `(job_id, is_active)`) — per spec §6.2, §6.4, and §6.7.

- [ ] **Step 2: Verify schema applies cleanly and role families are seeded**

Run: `docker compose down -v && make up && sleep 8 && docker compose exec -T db psql -U postgres -d nearhive -c "SELECT slug, is_technical FROM role_families ORDER BY slug" && docker compose exec -T db psql -U postgres -d nearhive -c '\d technical_job_postings'`
Expected: 14 seeded families with the 5 technical ones flagged true; new job columns present.

- [ ] **Step 3: Commit**

```bash
git add python-discovery/schema.sql
git commit -m "feat(schema): company identity, taxonomy, canonical job fields, remote eligibility"
```

---

### Task 3: Source registration and provenance writes in persistence

**Files:**
- Modify: `python-discovery/nearhive_discovery/persistence.py`
- Modify: `python-discovery/nearhive_discovery/contracts.py` (add `source_family`-compatible fields if needed — none expected; `EvidenceBatch` already carries `source`, `source_family`)
- Test: `python-discovery/tests/test_persistence.py`

**Interfaces:**
- Consumes: `EvidenceBatch` (existing), schema tables from Tasks 1–2.
- Produces:
  - `PostgresPersistence._register_source(source: str, source_family: str) -> str` — returns `data_sources.id`; creates the row (slug = source, `source_type='open_dataset'` default per-family mapping: `official_site`→`official_site`, `job_ats`→`official_ats`, else `open_dataset`) if missing.
  - `PostgresPersistence._upsert_source_record(data_source_id: str, ingestion_run_id: str, record_type: str, external_id: str | None, source_url: str | None, payload: dict) -> str` — returns `source_records.id`; conflict updates `last_seen_at = EXCLUDED.last_seen_at, normalized_payload = EXCLUDED.normalized_payload`. Hash: `_content_hash(json.dumps(payload, sort_keys=True))`.
  - `PostgresPersistence._upsert_ingestion_run(data_source_id: str, discovery_job_id: str | None, adapter_version: str = "v1") -> str` — returns `ingestion_runs.id`; at the end of `persist()` the run is updated with accepted/rejected counts and `status='completed'` / `finished_at`.
  - After Task 3, every `persist()` writes: 1 ingestion run, 1 company source_record + `company_source_links` per company evidence, 1 job source_record + `job_source_links` per job evidence. Batch validation (existing `validate()`) additionally requires `batch.source` to resolve via `_register_source` — unknown sources auto-register (first delivery registers them); validation failure only if `source` is blank/oversized (existing rule).

- [ ] **Step 1: Write the failing test** in `tests/test_persistence.py`, modeled on the existing idempotency test:

```python
def test_provenance_rows_written_and_idempotent() -> None:
    # build a batch (unique uuid suffix), persist twice
    # assert: one data_sources row for source, one ingestion_runs row (completed),
    # one company source_record and one job source_record — NOT two after second persist,
    # one company_source_links row and one job_source_links row,
    # and second persist bumped source_records.last_seen_at
```

- [ ] **Step 2: Run to verify failure**

Run: `cd python-discovery && NEARHIVE_TEST_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/nearhive_test uv run pytest -q tests/test_persistence.py::test_provenance_rows_written_and_idempotent`
Expected: FAIL — tables exist (Task 1) but provenance rows are not written (counts 0).

- [ ] **Step 3: Implement** the three private helpers and wire them into `persist()` inside the existing `with self.conn.transaction():` block: `_register_source` first, `_upsert_ingestion_run`, then per company/job record upserts and link inserts (`relation='evidence'`, `confidence=1.0`). Finish by updating the ingestion run counts.

- [ ] **Step 4: Run to verify pass** (same command; expect PASS) and the full persistence suite: `uv run pytest -q tests/test_persistence.py`

- [ ] **Step 5: Commit**

```bash
git add python-discovery/nearhive_discovery/persistence.py python-discovery/tests/test_persistence.py
git commit -m "feat(discovery): write source records and entity links with canonical evidence"
```

---

### Task 4: Company identity writes — aliases and domains

**Files:**
- Modify: `python-discovery/nearhive_discovery/persistence.py`
- Test: `python-discovery/tests/test_persistence.py`

**Interfaces:**
- Consumes: `company_aliases`, `company_domains` (Task 2); `_upsert_company` (existing, returns company id).
- Produces: `_upsert_company` extended to also: insert a `source_name` alias when a new company is created (alias = name, normalized via `_normalized_name`), and upsert into `company_domains` (`normalized_domain` = cleaned domain, `domain_type='primary'`, `is_primary=true`) when a domain exists. `CompanyEvidence.name`/`domain` flow unchanged.

- [ ] **Step 1: Write the failing test** `test_company_alias_and_domain_written`:

```python
# persist a batch with a company (name + domain); assert one company_aliases row
# with alias_type='source_name' and one company_domains row with is_primary=true;
# re-persisting must not add duplicate rows
```

- [ ] **Step 2: Run to verify failure** (`uv run pytest -q tests/test_persistence.py::test_company_alias_and_domain_written`) — Expected: FAIL, zero alias/domain rows.
- [ ] **Step 3: Implement** inside `_upsert_company` (after company id is resolved): insert alias on creation; upsert domain row on every call (conflict on `(company_id, normalized_domain)` → update `is_primary`, `last_seen_at`-equivalent updated timestamp).
- [ ] **Step 4: Run to verify pass** plus full `tests/test_persistence.py`.
- [ ] **Step 5: Commit**

```bash
git add python-discovery/nearhive_discovery/persistence.py python-discovery/tests/test_persistence.py
git commit -m "feat(discovery): write company aliases and domains during identity resolution"
```

---

### Task 5: Location lifecycle fields on write

**Files:**
- Modify: `python-discovery/nearhive_discovery/persistence.py`
- Test: `python-discovery/tests/test_persistence.py`

**Interfaces:**
- Consumes: `locations` new columns (Task 2).
- Produces: `_upsert_company_location` sets `location_type='office'`, `status='unverified'`, `country_code='IN'`, `address_hash = _content_hash(normalized address)` and dedupes by `(company_id, address_hash)` first (fallback to existing 500 m spatial dedupe when hash is absent); updates `last_seen_at` on the matched row.

- [ ] **Step 1: Write the failing test** `test_location_lifecycle_fields_written`:

```python
# persist a batch with a geocoded company; assert the location row has
# location_type='office', status='unverified', country_code='IN', non-null address_hash;
# persisting the same address again matches the SAME location id (no duplicate)
```

- [ ] **Step 2: Run to verify failure.** Expected: FAIL — defaults are `unknown`/`unverified` missing, dedupe returns same row only by proximity.
- [ ] **Step 3: Implement** the changes in `_upsert_company_location`.
- [ ] **Step 4: Run to verify pass** plus full persistence suite.
- [ ] **Step 5: Commit**

```bash
git add python-discovery/nearhive_discovery/persistence.py python-discovery/tests/test_persistence.py
git commit -m "feat(discovery): lifecycle fields and address-hash dedupe for company locations"
```

---

### Task 6: Canonical job fields and enriched job locations

**Files:**
- Modify: `python-discovery/nearhive_discovery/persistence.py`
- Test: `python-discovery/tests/test_persistence.py`

**Interfaces:**
- Consumes: `technical_job_postings`/`job_locations` new columns (Task 2); `TechnicalJobEvidence` (existing — carries `work_arrangement`, `posted_at`, `location_raw`, `lat`/`lng`).
- Produces: `_upsert_job` sets `role_family_id` = the `engineering` role_family id (classification stays technical; full classifier integration is deferred), `seniority='unknown'`, `employment_type='unknown'`, `state` mapped from `publication_state` (`posted_recently`/`observed_recently`→`open`, `stale`→`stale`), `activity_confidence = posted_at_confidence`. Job location rows get `location_kind='stated_job_location'` when `location_raw` is set, `'inferred'` when only coordinates were geocoded (existing `coordinate_source='inferred'`), and `country_code='IN'`.

- [ ] **Step 1: Write the failing test** `test_job_canonical_fields_and_location_kind`:

```python
# persist a job with location_raw + coords; assert state='open', role_family_id is the
# engineering family, job row has seniority='unknown';
# assert the job_locations row has location_kind='stated_job_location', country_code='IN'
# persist a coords-only job (no location_raw); assert location_kind='inferred'
```

- [ ] **Step 2: Run to verify failure.** Expected: FAIL — fields at defaults.
- [ ] **Step 3: Implement** in `_upsert_job` (both the job INSERT/UPDATE column list and the job_locations INSERT).
- [ ] **Step 4: Write the conflict test** `test_inferred_does_not_overwrite_provider`: persist a job with provider coords (`lat/lng`, `coordinate_source='provider'`), then persist the same job id with an inferred-only update attempt; assert the existing latitude/longitude are unchanged.

- [ ] **Step 5: Run to verify both pass.** Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add python-discovery/nearhive_discovery/persistence.py python-discovery/tests/test_persistence.py
git commit -m "feat(discovery): canonical job state and enriched job-location writes"
```

---

### Task 7: Remote eligibility writes

**Files:**
- Modify: `python-discovery/nearhive_discovery/contracts.py` (add `remote_scopes: list[str] = field(default_factory=list)` to `TechnicalJobEvidence` — list of scope codes, e.g. `["GLOBAL"]`, `["IN"]`; `scope_type` derived: `"GLOBAL"` → `global`, ISO-2 uppercase → `country`)
- Modify: `python-discovery/nearhive_discovery/persistence.py`
- Test: `python-discovery/tests/test_persistence.py`

**Interfaces:**
- Consumes: `job_remote_eligibility` (Task 2), `job_source_links` (Task 3).
- Produces: `_upsert_job` writes one `job_remote_eligibility` row per scope on `remote` jobs (`is_active=true`, `confidence=0.8`, `source_record_id` = the job's source record); clears existing active scopes when `remote_scopes` is empty. India-eligible = active `global` row or active `country` row with `scope_code='IN'` (spec §6.7).

- [ ] **Step 1: Write the failing test** `test_remote_eligibility_scopes_written`:

```python
# persist a remote job with remote_scopes=["GLOBAL"]; assert one eligibility row
# scope_type='global', scope_code='GLOBAL';
# persist a remote job with remote_scopes=["IN"]; assert scope_type='country', scope_code='IN';
# persist a remote job with empty scopes; assert no active rows remain
```

- [ ] **Step 2: Run to verify failure.** Expected: FAIL — no rows.
- [ ] **Step 3: Implement** the scope derivation helper `_remote_scope_rows(scopes: list[str]) -> list[tuple[str, str]]` and the eligibility upsert/delete in `_upsert_job` (only when `work_arrangement == 'remote'`).
- [ ] **Step 4: Write the negative test** `test_remote_without_scope_not_india_eligible`: remote job with empty `remote_scopes`; assert zero eligibility rows (Review Focus item).

- [ ] **Step 5: Run to verify all pass** plus full persistence suite.
- [ ] **Step 6: Commit**

```bash
git add python-discovery/nearhive_discovery/contracts.py python-discovery/nearhive_discovery/persistence.py python-discovery/tests/test_persistence.py
git commit -m "feat(discovery): explicit remote-eligibility scope writes"
```

---

### Task 8: Read-side verification and full-suite gate

**Files:**
- Modify: `Makefile` (only if `test-discovery` needs the new test file added — check; current target already lists `tests/test_persistence.py`)
- Test: full suites

**Interfaces:**
- Consumes: everything above.
- Produces: confidence that Go reads still work over the extended schema.

- [ ] **Step 1: Run the Python discovery suite**

Run: `make test-discovery`
Expected: PASS including `test_persistence.py`, `test_operations_api.py`, `test_queue.py`.

- [ ] **Step 2: Run Go tests**

Run: `make test`
Expected: PASS (Go store code compiles against the extended schema; no columns removed).

- [ ] **Step 3: Live smoke check through Go**

Run: `docker compose down -v && make up`, launch a discovery run via the web (localhost:3000), then:

```bash
curl -s "http://localhost:8080/api/v1/search?lat=12.97&lng=77.59&radius_km=50" | head -c 400
curl -s "http://localhost:8080/api/v1/search/jobs?lat=12.97&lng=77.59&radius_km=50" | head -c 400
```

Expected: valid JSON responses; company/job data written by Python visible through Go.

- [ ] **Step 4: Verify provenance chain end-to-end**

Run: `docker compose exec -T db psql -U postgres -d nearhive -c "SELECT s.record_type, d.slug FROM source_records s JOIN data_sources d ON d.id = s.data_source_id LIMIT 10"`
Expected: company/job rows attributed to the configured sources (osm, greenhouse, or company_site).

- [ ] **Step 5: Commit any Makefile/doc fix-ups**

```bash
git add -A && git commit -m "chore(discovery): phase 1 read-side verification pass"
```
(only if something changed; otherwise skip)

---

## Deferred from Phase 1 (recorded deviations)

- `company_merge_candidates` review queue (spec §6.10): current identity resolution is domain/normalized-name keyed with no fuzzy merging, so no ambiguous-merge path exists yet; introduce alongside `company_aliases`-based matching in a follow-up.
- Separate `identity_rule_version` / per-domain confidence computation (spec §6.2): identity confidence stays at defaults until multiple sources contribute.

## Out of scope (deferred to later plans)

- Phase 2 hiring projections and the blended candidate query (spec §7).
- Salary/review/interview schemas (spec §9).
- Go API changes — read endpoints already work over the extended schema.
- Backfill of historical sightings (data disposable; authoritative plan §Delivery-1).
