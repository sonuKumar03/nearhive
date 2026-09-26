CREATE INDEX IF NOT EXISTS idx_scrape_jobs_created_at ON scrape_jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_scrape_jobs_status ON scrape_jobs(status);
CREATE INDEX IF NOT EXISTS idx_sightings_scraped_at ON sightings(scraped_at DESC);
CREATE INDEX IF NOT EXISTS idx_companies_name_trgm ON companies USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_companies_industry_trgm ON companies USING gin (industry gin_trgm_ops);
CREATE UNIQUE INDEX IF NOT EXISTS idx_companies_normalized_name ON companies(normalized_name) WHERE normalized_name IS NOT NULL AND normalized_name != '';
