DROP TABLE IF EXISTS technical_job_postings;

DROP INDEX IF EXISTS idx_locations_presence_type;
ALTER TABLE locations DROP COLUMN IF EXISTS presence_type;

DROP INDEX IF EXISTS idx_sightings_discovery_job;
DROP INDEX IF EXISTS idx_sightings_source_hash;
DROP INDEX IF EXISTS idx_sightings_source_record;
ALTER TABLE sightings
    DROP COLUMN IF EXISTS last_seen_at,
    DROP COLUMN IF EXISTS first_seen_at,
    DROP COLUMN IF EXISTS discovery_job_id,
    DROP COLUMN IF EXISTS content_hash,
    DROP COLUMN IF EXISTS source_record_id,
    DROP COLUMN IF EXISTS source_family;

DROP TABLE IF EXISTS discovery_source_runs;
DROP TABLE IF EXISTS discovery_jobs;
