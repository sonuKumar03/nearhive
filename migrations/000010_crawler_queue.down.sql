DROP INDEX IF EXISTS idx_scrape_jobs_queue;

ALTER TABLE scrape_jobs
    DROP COLUMN IF EXISTS attempts,
    DROP COLUMN IF EXISTS last_heartbeat_at,
    DROP COLUMN IF EXISTS worker_id,
    DROP COLUMN IF EXISTS radius_km,
    DROP COLUMN IF EXISTS lng,
    DROP COLUMN IF EXISTS lat;
