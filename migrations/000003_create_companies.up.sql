CREATE TABLE IF NOT EXISTS companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(500) NOT NULL,
    normalized_name VARCHAR(500) NOT NULL,
    domain VARCHAR(255),
    industry VARCHAR(255),
    employee_count VARCHAR(50),
    description TEXT,
    verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_companies_normalized ON companies (normalized_name);
CREATE INDEX IF NOT EXISTS idx_companies_domain ON companies (domain);
CREATE INDEX IF NOT EXISTS idx_companies_trgm ON companies USING GIN (normalized_name gin_trgm_ops);
