CREATE TABLE IF NOT EXISTS sightings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source VARCHAR(100) NOT NULL,
    source_url TEXT,
    company_name VARCHAR(500) NOT NULL,
    raw_address TEXT,
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION,
    metadata JSONB DEFAULT '{}',
    company_id UUID REFERENCES companies(id) ON DELETE SET NULL,
    location_id UUID REFERENCES locations(id) ON DELETE SET NULL,
    scraped_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sightings_source ON sightings (source);
CREATE INDEX IF NOT EXISTS idx_sightings_company ON sightings (company_id);
CREATE INDEX IF NOT EXISTS idx_sightings_name ON sightings (company_name);
