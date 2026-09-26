DO $$
DECLARE
    rec RECORD;
    keeper_id UUID;
BEGIN
    FOR rec IN
        SELECT normalized_name
        FROM companies
        WHERE normalized_name IS NOT NULL AND normalized_name != ''
        GROUP BY normalized_name
        HAVING COUNT(*) > 1
    LOOP
        SELECT id INTO keeper_id
        FROM companies
        WHERE normalized_name = rec.normalized_name
        ORDER BY created_at ASC, id ASC
        LIMIT 1;

        UPDATE locations
        SET company_id = keeper_id
        WHERE company_id IN (
            SELECT id FROM companies
            WHERE normalized_name = rec.normalized_name AND id != keeper_id
        );

        UPDATE sightings
        SET company_id = keeper_id
        WHERE company_id IN (
            SELECT id FROM companies
            WHERE normalized_name = rec.normalized_name AND id != keeper_id
        );

        DELETE FROM companies
        WHERE normalized_name = rec.normalized_name AND id != keeper_id;
    END LOOP;
END $$;

CREATE INDEX IF NOT EXISTS idx_scrape_jobs_created_at ON scrape_jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_scrape_jobs_status ON scrape_jobs(status);
CREATE INDEX IF NOT EXISTS idx_sightings_scraped_at ON sightings(scraped_at DESC);
CREATE INDEX IF NOT EXISTS idx_companies_name_trgm ON companies USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_companies_industry_trgm ON companies USING gin (industry gin_trgm_ops);
CREATE UNIQUE INDEX IF NOT EXISTS idx_companies_normalized_name ON companies(normalized_name) WHERE normalized_name IS NOT NULL AND normalized_name != '';
