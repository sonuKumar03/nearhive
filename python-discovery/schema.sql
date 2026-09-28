-- Fresh local schema for Python-owned discovery. Run on an empty PostGIS database.
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Shared authentication identity; the Go API writes only this table.
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(500) NOT NULL,
    normalized_name VARCHAR(500) NOT NULL,
    domain VARCHAR(255),
    industry TEXT,
    employee_count TEXT,
    description TEXT,
    verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX companies_domain_unique ON companies (lower(domain)) WHERE domain IS NOT NULL;
CREATE UNIQUE INDEX companies_nameless_domain_unique ON companies (normalized_name) WHERE domain IS NULL;
CREATE INDEX companies_name_trgm ON companies USING gin (normalized_name gin_trgm_ops);

CREATE TABLE locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    label TEXT,
    address TEXT NOT NULL,
    city TEXT,
    state TEXT,
    country TEXT NOT NULL DEFAULT 'India',
    pincode TEXT,
    coords geography(Point, 4326),
    presence_type VARCHAR(30) NOT NULL DEFAULT 'probable_office'
        CHECK (presence_type IN ('confirmed_office', 'probable_office', 'job_location_only')),
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1),
    verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX locations_company ON locations (company_id);
CREATE INDEX locations_coords ON locations USING gist (coords);

CREATE TABLE discovery_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'completed', 'partial', 'failed', 'cancelled')),
    lat DOUBLE PRECISION NOT NULL CHECK (lat BETWEEN -90 AND 90),
    lng DOUBLE PRECISION NOT NULL CHECK (lng BETWEEN -180 AND 180),
    radius_km DOUBLE PRECISION NOT NULL CHECK (radius_km > 0 AND radius_km <= 100),
    worker_id VARCHAR(100),
    lease_expires_at TIMESTAMPTZ,
    last_heartbeat_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    error TEXT,
    company_count INTEGER NOT NULL DEFAULT 0,
    job_count INTEGER NOT NULL DEFAULT 0,
    evidence_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX discovery_jobs_pending ON discovery_jobs (created_at) WHERE status = 'pending';
CREATE INDEX discovery_jobs_lease ON discovery_jobs (lease_expires_at) WHERE status = 'running';
CREATE INDEX discovery_jobs_user ON discovery_jobs (user_id, created_at DESC);

CREATE TABLE discovery_source_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    discovery_job_id UUID NOT NULL REFERENCES discovery_jobs(id) ON DELETE CASCADE,
    source VARCHAR(100) NOT NULL,
    source_family VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'completed', 'partial', 'failed', 'cancelled')),
    attempts INTEGER NOT NULL DEFAULT 0,
    company_count INTEGER NOT NULL DEFAULT 0,
    job_count INTEGER NOT NULL DEFAULT 0,
    evidence_count INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (discovery_job_id, source)
);

CREATE TABLE sightings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source VARCHAR(100) NOT NULL,
    source_family VARCHAR(100),
    source_record_id VARCHAR(255),
    content_hash VARCHAR(64),
    discovery_job_id UUID REFERENCES discovery_jobs(id) ON DELETE SET NULL,
    source_url TEXT,
    company_name VARCHAR(500) NOT NULL,
    raw_address TEXT,
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION,
    metadata JSONB NOT NULL DEFAULT '{}',
    company_id UUID REFERENCES companies(id) ON DELETE SET NULL,
    location_id UUID REFERENCES locations(id) ON DELETE SET NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    scraped_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((lat IS NULL) = (lng IS NULL)),
    CHECK (lat IS NULL OR lat BETWEEN -90 AND 90),
    CHECK (lng IS NULL OR lng BETWEEN -180 AND 180)
);
CREATE UNIQUE INDEX sightings_source_record ON sightings (source, source_record_id)
    WHERE source_record_id IS NOT NULL;
CREATE UNIQUE INDEX sightings_source_hash ON sightings (source, content_hash)
    WHERE content_hash IS NOT NULL AND source_record_id IS NULL;

CREATE TABLE technical_job_postings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    discovery_job_id UUID REFERENCES discovery_jobs(id) ON DELETE SET NULL,
    source VARCHAR(100) NOT NULL,
    source_family VARCHAR(100) NOT NULL,
    source_job_id VARCHAR(255),
    canonical_url TEXT,
    title VARCHAR(500) NOT NULL,
    normalized_title VARCHAR(500) NOT NULL,
    description_excerpt TEXT,
    content_hash VARCHAR(64) NOT NULL,
    work_arrangement VARCHAR(20) NOT NULL DEFAULT 'unknown'
        CHECK (work_arrangement IN ('in_office', 'hybrid', 'remote', 'unknown')),
    publication_state VARCHAR(30) NOT NULL DEFAULT 'observed_recently'
        CHECK (publication_state IN ('posted_recently', 'observed_recently', 'stale')),
    posted_at TIMESTAMPTZ,
    posted_at_confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (posted_at_confidence BETWEEN 0 AND 1),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_active BOOLEAN NOT NULL DEFAULT true,
    technical_classification VARCHAR(100) NOT NULL,
    rule_version VARCHAR(50) NOT NULL,
    classification_reasons TEXT[] NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX technical_jobs_source_id ON technical_job_postings (source, source_job_id)
    WHERE source_job_id IS NOT NULL;
CREATE UNIQUE INDEX technical_jobs_source_hash ON technical_job_postings (source, content_hash)
    WHERE content_hash IS NOT NULL AND source_job_id IS NULL;
CREATE INDEX technical_jobs_company ON technical_job_postings (company_id, last_seen_at DESC);

CREATE TABLE job_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES technical_job_postings(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL DEFAULT 0 CHECK (ordinal >= 0),
    location_raw TEXT NOT NULL DEFAULT '',
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    coordinate_source VARCHAR(30) NOT NULL DEFAULT 'unknown'
        CHECK (coordinate_source IN ('structured', 'provider', 'geocoded', 'inferred', 'unknown')),
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1),
    is_active BOOLEAN NOT NULL DEFAULT true,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_id, ordinal),
    CHECK ((latitude IS NULL) = (longitude IS NULL)),
    CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180)
);
CREATE INDEX job_locations_coords ON job_locations USING gist (
    (ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)::geography)
) WHERE is_active AND latitude IS NOT NULL AND longitude IS NOT NULL;

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

-- The Go HTTP API can read discovery data and manage shared user accounts.
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'nearhive_api') THEN
        CREATE ROLE nearhive_api LOGIN PASSWORD 'nearhive-api-dev';
    END IF;
END $$;
GRANT CONNECT ON DATABASE nearhive TO nearhive_api;
GRANT USAGE ON SCHEMA public TO nearhive_api;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO nearhive_api;
GRANT INSERT, UPDATE ON users TO nearhive_api;
