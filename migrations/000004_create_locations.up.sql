CREATE TABLE IF NOT EXISTS locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    label VARCHAR(100),
    address TEXT NOT NULL,
    city VARCHAR(255),
    state VARCHAR(255),
    country VARCHAR(100) DEFAULT 'IN',
    pincode VARCHAR(20),
    coords GEOGRAPHY(POINT, 4326),
    confidence FLOAT DEFAULT 0.0,
    verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_locations_company ON locations (company_id);
CREATE INDEX IF NOT EXISTS idx_locations_city ON locations (city);
CREATE INDEX IF NOT EXISTS idx_locations_coords ON locations USING GIST (coords);
