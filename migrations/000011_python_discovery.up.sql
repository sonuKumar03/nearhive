CREATE TABLE IF NOT EXISTS discovery_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'partial', 'failed', 'cancelled')),
    lat DOUBLE PRECISION NOT NULL CHECK (lat >= -90 AND lat <= 90),
    lng DOUBLE PRECISION NOT NULL CHECK (lng >= -180 AND lng <= 180),
    radius_km DOUBLE PRECISION NOT NULL CHECK (radius_km > 0 AND radius_km <= 100),
    worker_id VARCHAR(100),
    lease_expires_at TIMESTAMPTZ,
    last_heartbeat_at TIMESTAMPTZ,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    error TEXT,
    company_count INT NOT NULL DEFAULT 0,
    job_count INT NOT NULL DEFAULT 0,
    evidence_count INT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_discovery_jobs_pending ON discovery_jobs (created_at ASC) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_discovery_jobs_user ON discovery_jobs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_discovery_jobs_lease ON discovery_jobs (lease_expires_at) WHERE status = 'running';

CREATE TABLE IF NOT EXISTS discovery_source_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    discovery_job_id UUID NOT NULL REFERENCES discovery_jobs(id) ON DELETE CASCADE,
    source VARCHAR(100) NOT NULL,
    source_family VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'partial', 'failed', 'cancelled')),
    attempts INT NOT NULL DEFAULT 0,
    company_count INT NOT NULL DEFAULT 0,
    job_count INT NOT NULL DEFAULT 0,
    evidence_count INT NOT NULL DEFAULT 0,
    error TEXT,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_discovery_source_runs_job_source UNIQUE (discovery_job_id, source)
);

CREATE INDEX IF NOT EXISTS idx_discovery_source_runs_job ON discovery_source_runs (discovery_job_id);

ALTER TABLE sightings
    ADD COLUMN IF NOT EXISTS source_family VARCHAR(100),
    ADD COLUMN IF NOT EXISTS source_record_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS content_hash VARCHAR(64),
    ADD COLUMN IF NOT EXISTS discovery_job_id UUID REFERENCES discovery_jobs(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX IF NOT EXISTS idx_sightings_source_record ON sightings (source, source_record_id)
    WHERE source_record_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_sightings_source_hash ON sightings (source, content_hash)
    WHERE content_hash IS NOT NULL AND source_record_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_sightings_discovery_job ON sightings (discovery_job_id);

ALTER TABLE locations
    ADD COLUMN IF NOT EXISTS presence_type VARCHAR(30) NOT NULL DEFAULT 'probable_office'
        CHECK (presence_type IN ('confirmed_office', 'probable_office', 'job_location_only'));

CREATE INDEX IF NOT EXISTS idx_locations_presence_type ON locations (presence_type);

CREATE TABLE IF NOT EXISTS technical_job_postings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    location_id UUID REFERENCES locations(id) ON DELETE SET NULL,
    discovery_job_id UUID REFERENCES discovery_jobs(id) ON DELETE SET NULL,
    source VARCHAR(100) NOT NULL,
    source_family VARCHAR(100) NOT NULL,
    source_job_id VARCHAR(255),
    canonical_url TEXT,
    title VARCHAR(500) NOT NULL,
    normalized_title VARCHAR(500) NOT NULL,
    description_excerpt TEXT,
    content_hash VARCHAR(64) NOT NULL,
    location_raw TEXT,
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION,
    work_arrangement VARCHAR(20) NOT NULL DEFAULT 'unknown' CHECK (work_arrangement IN ('in_office', 'hybrid', 'remote', 'unknown')),
    publication_state VARCHAR(30) NOT NULL DEFAULT 'observed_recently' CHECK (publication_state IN ('posted_recently', 'observed_recently', 'stale')),
    posted_at TIMESTAMPTZ,
    posted_at_confidence DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    technical_classification VARCHAR(100) NOT NULL,
    rule_version VARCHAR(50) NOT NULL,
    classification_reasons TEXT[] NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tech_jobs_source_job_id ON technical_job_postings (source, source_job_id)
    WHERE source_job_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_tech_jobs_source_hash ON technical_job_postings (source, content_hash)
    WHERE content_hash IS NOT NULL AND source_job_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_tech_jobs_company_posted ON technical_job_postings (company_id, posted_at DESC);
CREATE INDEX IF NOT EXISTS idx_tech_jobs_company_last_seen ON technical_job_postings (company_id, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_tech_jobs_work_arrangement ON technical_job_postings (work_arrangement);
CREATE INDEX IF NOT EXISTS idx_tech_jobs_discovery_job ON technical_job_postings (discovery_job_id);
