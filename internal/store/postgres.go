package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/sonukumar/nearhive/internal/model"
)

type PostgresStore struct {
	db *sqlx.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		// If server does not have SSL enabled (e.g. Railway private network, local Docker), fallback to sslmode=disable
		if strings.Contains(err.Error(), "SSL is not enabled on the server") && strings.Contains(dsn, "sslmode=require") {
			fallbackDSN := strings.Replace(dsn, "sslmode=require", "sslmode=disable", 1)
			if dbFallback, errFallback := sqlx.Connect("postgres", fallbackDSN); errFallback == nil {
				dbFallback.SetMaxOpenConns(25)
				dbFallback.SetMaxIdleConns(5)
				dbFallback.SetConnMaxLifetime(5 * time.Minute)
				return &PostgresStore{db: dbFallback}, nil
			}
		}
		// If server requires SSL (e.g. Neon, Supabase, AWS RDS), fallback to sslmode=require
		if strings.Contains(err.Error(), "does not support SSL-off") && strings.Contains(dsn, "sslmode=disable") {
			fallbackDSN := strings.Replace(dsn, "sslmode=disable", "sslmode=require", 1)
			if dbFallback, errFallback := sqlx.Connect("postgres", fallbackDSN); errFallback == nil {
				dbFallback.SetMaxOpenConns(25)
				dbFallback.SetMaxIdleConns(5)
				dbFallback.SetConnMaxLifetime(5 * time.Minute)
				return &PostgresStore{db: dbFallback}, nil
			}
		}
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) DB() *sql.DB {
	return s.db.DB
}

// UserStore implementation
func (s *PostgresStore) CreateUser(ctx context.Context, u *model.User) error {
	query := `INSERT INTO users (id, email, password, created_at, updated_at) 
	          VALUES ($1, $2, $3, $4, $5)`
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, query, u.ID, u.Email, u.Password, u.CreatedAt, u.UpdatedAt)
	return err
}

func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	query := `SELECT id, email, password, created_at, updated_at FROM users WHERE email = $1`
	err := s.db.GetContext(ctx, &u, query, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func (s *PostgresStore) GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var u model.User
	query := `SELECT id, email, password, created_at, updated_at FROM users WHERE id = $1`
	err := s.db.GetContext(ctx, &u, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func (s *PostgresStore) GetCompanyByID(ctx context.Context, id uuid.UUID) (*model.Company, error) {
	var c model.Company
	query := `SELECT id, name, normalized_name, domain, industry, employee_count, description, verified, created_at, updated_at
	          FROM companies WHERE id = $1`
	err := s.db.GetContext(ctx, &c, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *PostgresStore) GetLocationsByCompany(ctx context.Context, companyID uuid.UUID) ([]model.Location, error) {
	var locs []model.Location
	query := `SELECT id, company_id, label, address, city, state, country, pincode,
	                 ST_Y(coords::geometry) as lat, ST_X(coords::geometry) as lng,
	                 confidence, presence_type, verified, created_at, updated_at
	          FROM locations WHERE company_id = $1`
	err := s.db.SelectContext(ctx, &locs, query, companyID)
	return locs, err
}

type searchCompanyRow struct {
	model.CompanySearchResult
	ArrangementsRaw pq.StringArray `db:"arrangements_raw"`
}

func (s *PostgresStore) Search(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) ([]model.CompanySearchResult, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
	SELECT c.id AS company_id, c.name, c.domain, c.industry, c.employee_count,
	       l.id AS location_id, l.label, l.address, l.city,
	       ST_Y(l.coords::geometry) AS lat, ST_X(l.coords::geometry) AS lng,
	       l.confidence,
	       ST_Distance(l.coords, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) AS distance_m,
	       COALESCE(l.presence_type, 'probable_office') AS presence_type,
	       COALESCE(j.recent_technical_job_count, 0) AS recent_technical_job_count,
	       COALESCE(j.arrangements, '{}') AS arrangements_raw,
	       l.verified
	FROM locations l
	JOIN companies c ON c.id = l.company_id
	LEFT JOIN LATERAL (
	    SELECT 
	        COUNT(*)::int AS recent_technical_job_count,
	        ARRAY_AGG(DISTINCT tj.work_arrangement ORDER BY tj.work_arrangement) AS arrangements
	    FROM technical_job_postings tj
	    WHERE tj.company_id = c.id
	      AND tj.is_active = TRUE
	      AND tj.posted_at IS NOT NULL
	      AND tj.posted_at >= NOW() - INTERVAL '14 days'
	      AND (tj.publication_state = 'posted_recently' OR (tj.posted_at_confidence > 0 AND tj.publication_state NOT IN ('observed_recently', 'stale')))
	) j ON TRUE
	WHERE l.presence_type <> 'job_location_only'
	  AND ST_DWithin(l.coords, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
	  AND ($4::float IS NULL OR l.confidence >= $4)
	  AND ($5::text IS NULL OR c.industry ILIKE '%' || $5 || '%')
	  AND ($6::text IS NULL OR c.name ILIKE '%' || $6 || '%' OR c.normalized_name % $6)
	ORDER BY distance_m ASC
	LIMIT $7 OFFSET $8
	`

	var rows []searchCompanyRow
	err := s.db.SelectContext(ctx, &rows, query, lng, lat, radiusMeters, opts.MinConfidence, opts.Industry, opts.Query, limit, opts.Offset)
	if err != nil {
		return nil, err
	}

	results := make([]model.CompanySearchResult, len(rows))
	for i, row := range rows {
		results[i] = row.CompanySearchResult
		if len(row.ArrangementsRaw) > 0 {
			arrs := make([]model.WorkArrangement, len(row.ArrangementsRaw))
			for j, a := range row.ArrangementsRaw {
				arrs[j] = model.WorkArrangement(a)
			}
			results[i].Arrangements = arrs
		}
	}
	return results, nil
}

func (s *PostgresStore) CountSearch(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) (int, error) {
	query := `
	SELECT COUNT(*)
	FROM locations l
	JOIN companies c ON c.id = l.company_id
	WHERE l.presence_type <> 'job_location_only'
	  AND ST_DWithin(l.coords, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
	  AND ($4::float IS NULL OR l.confidence >= $4)
	  AND ($5::text IS NULL OR c.industry ILIKE '%' || $5 || '%')
	  AND ($6::text IS NULL OR c.name ILIKE '%' || $6 || '%' OR c.normalized_name % $6)
	`
	var count int
	err := s.db.GetContext(ctx, &count, query, lng, lat, radiusMeters, opts.MinConfidence, opts.Industry, opts.Query)
	return count, err
}

func (s *PostgresStore) ClusterSearch(ctx context.Context, lat, lng, radiusMeters float64, k int) ([]model.SpatialCluster, error) {
	if k <= 0 {
		k = 20
	}
	if k > 100 {
		k = 100
	}

	query := `
	WITH matched_locations AS (
		SELECT l.coords::geometry AS geom
		FROM locations l
		WHERE l.presence_type <> 'job_location_only'
		  AND ST_DWithin(l.coords, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
	),
	clustered AS (
		SELECT 
			geom,
			ST_ClusterKMeans(geom, $4) OVER() AS cluster_id
		FROM matched_locations
	)
	SELECT 
		cluster_id,
		COUNT(*) AS count,
		ST_Y(ST_Centroid(ST_Collect(geom))) AS lat,
		ST_X(ST_Centroid(ST_Collect(geom))) AS lng
	FROM clustered
	GROUP BY cluster_id
	ORDER BY count DESC
	`

	var clusters []model.SpatialCluster
	err := s.db.SelectContext(ctx, &clusters, query, lng, lat, radiusMeters, k)
	if err != nil {
		return nil, err
	}
	if clusters == nil {
		clusters = []model.SpatialCluster{}
	}
	return clusters, nil
}

// ClusterGridSearch groups only visible offices into map-sized cells.
func (s *PostgresStore) ClusterGridSearch(ctx context.Context, lat, lng, radiusMeters float64, view ClusterViewport) ([]model.SpatialCluster, error) {
	// A cell spans one 256-pixel map tile at the requested zoom.
	cellMeters := 40075016.68557849 / float64(uint64(1)<<uint(view.Zoom))
	query := `
	WITH visible AS (
		SELECT ST_Y(l.coords::geometry) AS lat,
		       ST_X(l.coords::geometry) AS lng,
		       FLOOR(ST_X(ST_Transform(l.coords::geometry, 3857)) / $8) AS cell_x,
		       FLOOR(ST_Y(ST_Transform(l.coords::geometry, 3857)) / $8) AS cell_y
		FROM locations l
		JOIN companies c ON c.id = l.company_id
		WHERE l.presence_type <> 'job_location_only'
		  AND ST_DWithin(l.coords, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		  AND ST_Intersects(l.coords::geometry, ST_MakeEnvelope($4, $5, $6, $7, 4326))
		  AND ($9::text IS NULL OR c.name ILIKE '%' || $9 || '%' OR c.normalized_name % $9)
	), grouped AS (
		SELECT COUNT(*)::int AS count, AVG(lat) AS lat, AVG(lng) AS lng,
		       MIN(lng) AS west, MIN(lat) AS south,
		       MAX(lng) AS east, MAX(lat) AS north
		FROM visible
		GROUP BY cell_x, cell_y
	)
	SELECT (ROW_NUMBER() OVER (ORDER BY count DESC) - 1)::int AS cluster_id,
	       count, lat, lng, west, south, east, north
	FROM grouped
	ORDER BY count DESC`
	var clusters []model.SpatialCluster
	err := s.db.SelectContext(ctx, &clusters, query, lng, lat, radiusMeters,
		view.West, view.South, view.East, view.North, cellMeters, view.Query)
	if clusters == nil {
		clusters = []model.SpatialCluster{}
	}
	return clusters, err
}

func (s *PostgresStore) GetSightingsByCompany(ctx context.Context, companyID uuid.UUID) ([]model.Sighting, error) {
	var sightings []model.Sighting
	query := `SELECT id, source, source_family, source_record_id, content_hash, discovery_job_id, source_url, company_name, raw_address, lat, lng, metadata, company_id, location_id, first_seen_at, last_seen_at, scraped_at
	          FROM sightings WHERE company_id = $1 ORDER BY scraped_at DESC`
	err := s.db.SelectContext(ctx, &sightings, query, companyID)
	return sightings, err
}
