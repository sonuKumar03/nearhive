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
