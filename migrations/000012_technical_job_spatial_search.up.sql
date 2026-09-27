CREATE INDEX IF NOT EXISTS idx_tech_jobs_spatial ON technical_job_postings
    USING GIST ((ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography))
    WHERE is_active = TRUE AND lat IS NOT NULL AND lng IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tech_jobs_normalized_title_trgm ON technical_job_postings
    USING GIN (normalized_title gin_trgm_ops);
