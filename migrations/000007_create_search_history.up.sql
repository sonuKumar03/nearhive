CREATE TABLE IF NOT EXISTS search_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    query_lat DOUBLE PRECISION NOT NULL,
    query_lng DOUBLE PRECISION NOT NULL,
    radius_km FLOAT NOT NULL,
    result_count INT DEFAULT 0,
    searched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_search_user ON search_history (user_id);
