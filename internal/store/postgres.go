package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/sonukumar/nearhive/internal/model"
)

type PostgresStore struct {
	db *sqlx.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
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

// CompanyStore implementation
func (s *PostgresStore) CreateCompany(ctx context.Context, c *model.Company) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	query := `INSERT INTO companies (id, name, normalized_name, domain, industry, employee_count, description, verified, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	_, err := s.db.ExecContext(ctx, query, c.ID, c.Name, c.NormalizedName, c.Domain, c.Industry, c.EmployeeCount, c.Description, c.Verified, c.CreatedAt, c.UpdatedAt)
	return err
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

func (s *PostgresStore) FindByDomain(ctx context.Context, domain string) (*model.Company, error) {
	var c model.Company
	query := `SELECT id, name, normalized_name, domain, industry, employee_count, description, verified, created_at, updated_at
	          FROM companies WHERE domain = $1 LIMIT 1`
	err := s.db.GetContext(ctx, &c, query, domain)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *PostgresStore) FindByNormalizedName(ctx context.Context, name string) (*model.Company, error) {
	var c model.Company
	query := `SELECT id, name, normalized_name, domain, industry, employee_count, description, verified, created_at, updated_at
	          FROM companies WHERE normalized_name = $1 LIMIT 1`
	err := s.db.GetContext(ctx, &c, query, name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *PostgresStore) FindByFuzzyName(ctx context.Context, name string, threshold float64) (*model.Company, error) {
	var c model.Company
	query := `SELECT id, name, normalized_name, domain, industry, employee_count, description, verified, created_at, updated_at
	          FROM companies 
	          WHERE similarity(normalized_name, $1) > $2
	          ORDER BY similarity(normalized_name, $1) DESC
	          LIMIT 1`
	err := s.db.GetContext(ctx, &c, query, name, threshold)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *PostgresStore) UpdateCompany(ctx context.Context, c *model.Company) error {
	c.UpdatedAt = time.Now()
	query := `UPDATE companies SET name = $1, normalized_name = $2, domain = $3, industry = $4,
	          employee_count = $5, description = $6, verified = $7, updated_at = $8
	          WHERE id = $9`
	_, err := s.db.ExecContext(ctx, query, c.Name, c.NormalizedName, c.Domain, c.Industry, c.EmployeeCount, c.Description, c.Verified, c.UpdatedAt, c.ID)
	return err
}

// LocationStore implementation
func (s *PostgresStore) CreateLocation(ctx context.Context, l *model.Location) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	now := time.Now()
	l.CreatedAt = now
	l.UpdatedAt = now
	query := `INSERT INTO locations (id, company_id, label, address, city, state, country, pincode, coords, confidence, verified, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, ST_SetSRID(ST_MakePoint($9, $10), 4326)::geography, $11, $12, $13, $14)`
	_, err := s.db.ExecContext(ctx, query, l.ID, l.CompanyID, l.Label, l.Address, l.City, l.State, l.Country, l.Pincode, l.Lng, l.Lat, l.Confidence, l.Verified, l.CreatedAt, l.UpdatedAt)
	return err
}

func (s *PostgresStore) GetLocationsByCompany(ctx context.Context, companyID uuid.UUID) ([]model.Location, error) {
	var locs []model.Location
	query := `SELECT id, company_id, label, address, city, state, country, pincode,
	                 ST_Y(coords::geometry) as lat, ST_X(coords::geometry) as lng,
	                 confidence, verified, created_at, updated_at
	          FROM locations WHERE company_id = $1`
	err := s.db.SelectContext(ctx, &locs, query, companyID)
	return locs, err
}

func (s *PostgresStore) FindNearbyLocation(ctx context.Context, companyID uuid.UUID, lat, lng float64, radiusMeters float64) (*model.Location, error) {
	var l model.Location
	query := `SELECT id, company_id, label, address, city, state, country, pincode,
	                 ST_Y(coords::geometry) as lat, ST_X(coords::geometry) as lng,
	                 confidence, verified, created_at, updated_at
	          FROM locations 
	          WHERE company_id = $1 
	            AND ST_DWithin(coords, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4)
	          ORDER BY ST_Distance(coords, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography) ASC
	          LIMIT 1`
	err := s.db.GetContext(ctx, &l, query, companyID, lng, lat, radiusMeters)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &l, err
}

func (s *PostgresStore) UpdateLocationConfidence(ctx context.Context, id uuid.UUID, confidence float64) error {
	query := `UPDATE locations SET confidence = $1, updated_at = NOW() WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, confidence, id)
	return err
}

func (s *PostgresStore) UpdateLocationCoords(ctx context.Context, id uuid.UUID, lat, lng float64) error {
	query := `UPDATE locations SET coords = ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, updated_at = NOW() WHERE id = $3`
	_, err := s.db.ExecContext(ctx, query, lng, lat, id)
	return err
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
	       l.verified
	FROM locations l
	JOIN companies c ON c.id = l.company_id
	WHERE ST_DWithin(l.coords, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
	  AND ($4::float IS NULL OR l.confidence >= $4)
	  AND ($5::text IS NULL OR c.industry ILIKE '%' || $5 || '%')
	  AND ($6::text IS NULL OR c.name ILIKE '%' || $6 || '%' OR c.normalized_name % $6)
	ORDER BY distance_m ASC
	LIMIT $7 OFFSET $8
	`

	var results []model.CompanySearchResult
	err := s.db.SelectContext(ctx, &results, query, lng, lat, radiusMeters, opts.MinConfidence, opts.Industry, opts.Query, limit, opts.Offset)
	return results, err
}

func (s *PostgresStore) CountSearch(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) (int, error) {
	query := `
	SELECT COUNT(*)
	FROM locations l
	JOIN companies c ON c.id = l.company_id
	WHERE ST_DWithin(l.coords, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
	  AND ($4::float IS NULL OR l.confidence >= $4)
	  AND ($5::text IS NULL OR c.industry ILIKE '%' || $5 || '%')
	  AND ($6::text IS NULL OR c.name ILIKE '%' || $6 || '%' OR c.normalized_name % $6)
	`
	var count int
	err := s.db.GetContext(ctx, &count, query, lng, lat, radiusMeters, opts.MinConfidence, opts.Industry, opts.Query)
	return count, err
}

// SightingStore implementation
func (s *PostgresStore) SaveSightings(ctx context.Context, source string, sightings []model.Sighting) error {
	if len(sightings) == 0 {
		return nil
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareNamedContext(ctx, `
		INSERT INTO sightings (id, source, source_url, company_name, raw_address, lat, lng, metadata, company_id, location_id, scraped_at)
		VALUES (:id, :source, :source_url, :company_name, :raw_address, :lat, :lng, :metadata, :company_id, :location_id, :scraped_at)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range sightings {
		if s.ID == uuid.Nil {
			s.ID = uuid.New()
		}
		if s.Source == "" {
			s.Source = source
		}
		if s.ScrapedAt.IsZero() {
			s.ScrapedAt = time.Now()
		}
		if _, err := stmt.ExecContext(ctx, s); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *PostgresStore) GetSightingsByCompany(ctx context.Context, companyID uuid.UUID) ([]model.Sighting, error) {
	var sightings []model.Sighting
	query := `SELECT id, source, source_url, company_name, raw_address, lat, lng, metadata, company_id, location_id, scraped_at
	          FROM sightings WHERE company_id = $1 ORDER BY scraped_at DESC`
	err := s.db.SelectContext(ctx, &sightings, query, companyID)
	return sightings, err
}

func (s *PostgresStore) LinkSighting(ctx context.Context, sightingID uuid.UUID, companyID, locationID uuid.UUID) error {
	query := `UPDATE sightings SET company_id = $1, location_id = $2 WHERE id = $3`
	_, err := s.db.ExecContext(ctx, query, companyID, locationID, sightingID)
	return err
}

// JobStore implementation
func (s *PostgresStore) CreateJob(ctx context.Context, job *model.ScrapeJob) error {
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}
	query := `INSERT INTO scrape_jobs (id, source, status, region, sightings, error, started_at, finished_at, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := s.db.ExecContext(ctx, query, job.ID, job.Source, job.Status, job.Region, job.Sightings, job.Error, job.StartedAt, job.FinishedAt, job.CreatedAt)
	return err
}

func (s *PostgresStore) UpdateJob(ctx context.Context, job *model.ScrapeJob) error {
	query := `UPDATE scrape_jobs SET status = $1, sightings = $2, error = $3, started_at = $4, finished_at = $5 WHERE id = $6`
	_, err := s.db.ExecContext(ctx, query, job.Status, job.Sightings, job.Error, job.StartedAt, job.FinishedAt, job.ID)
	return err
}

func (s *PostgresStore) GetJobByID(ctx context.Context, id uuid.UUID) (*model.ScrapeJob, error) {
	var job model.ScrapeJob
	query := `SELECT id, source, status, region, sightings, error, started_at, finished_at, created_at FROM scrape_jobs WHERE id = $1`
	err := s.db.GetContext(ctx, &job, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &job, err
}

func (s *PostgresStore) ListJobs(ctx context.Context, limit, offset int) ([]model.ScrapeJob, error) {
	var jobs []model.ScrapeJob
	query := `SELECT id, source, status, region, sightings, error, started_at, finished_at, created_at FROM scrape_jobs ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	err := s.db.SelectContext(ctx, &jobs, query, limit, offset)
	return jobs, err
}

// SearchHistoryStore implementation
func (s *PostgresStore) RecordSearch(ctx context.Context, h *model.SearchHistory) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	if h.SearchedAt.IsZero() {
		h.SearchedAt = time.Now()
	}
	query := `INSERT INTO search_history (id, user_id, query_lat, query_lng, radius_km, result_count, searched_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.db.ExecContext(ctx, query, h.ID, h.UserID, h.QueryLat, h.QueryLng, h.RadiusKM, h.ResultCount, h.SearchedAt)
	return err
}

func (s *PostgresStore) GetHistoryByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.SearchHistory, error) {
	var history []model.SearchHistory
	query := `SELECT id, user_id, query_lat, query_lng, radius_km, result_count, searched_at
	          FROM search_history WHERE user_id = $1 ORDER BY searched_at DESC LIMIT $2`
	err := s.db.SelectContext(ctx, &history, query, userID, limit)
	return history, err
}
