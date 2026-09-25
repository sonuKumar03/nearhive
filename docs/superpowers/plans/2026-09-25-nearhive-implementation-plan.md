# NearHive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and deploy NearHive — a high-performance, modular Go web scraper and REST API for identifying, geolocating, deduplicating, and cross-verifying tech companies within a user-defined radius.

**Architecture:** A modular Go monolith structured around domain packages: pluggable scrapers coordinated by an orchestrator, a multi-stage NLP & spatial verification engine, PostGIS-backed spatial query store, JWT authentication, and a cron-based periodic crawler wrapped in a clean Chi REST API and Cobra CLI.

**Tech Stack:** Go 1.23+, PostgreSQL 16 with PostGIS 3.4 & pg_trgm, go-chi/chi/v5, jmoiron/sqlx, gocolly/colly/v2, golang-jwt/jwt/v5, golang.org/x/crypto/bcrypt, spf13/cobra, stretchr/testify.

**Spec:** [`docs/superpowers/specs/2026-09-25-nearhive-design.md`](file:///Users/sonukumar/project/office-locator/docs/superpowers/specs/2026-09-25-nearhive-design.md)

## Global Constraints

- Language: Go 1.23+ with strict standard error handling, no hidden panics.
- Code architecture: Modular monolith with explicit Go interfaces for all I/O components (`store`, `scraper`, `geocoder`).
- Database: All coordinates stored as `GEOGRAPHY(POINT, 4326)` and indexed via GIST.
- Name normalization: Lowercase, punctuation stripped, corporate suffixes removed.
- All terminal commands prefixed with `rtk ` per workspace instructions.
- Zero placeholder code: all steps provide concrete code and explicit test steps.

---

### Task 1: Project Scaffolding & Configuration Subsystem

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `.env.example`
- Create: `.gitignore`

**Interfaces:**
- Consumes: Environment variables (`os.Getenv`)
- Produces: `config.Config`, `config.Load() (*Config, error)`

- [ ] **Step 1: Initialize go.mod and install core dependencies**

Run:
```bash
rtk go mod init github.com/sonukumar/nearhive
rtk go get github.com/go-chi/chi/v5
rtk go get github.com/jmoiron/sqlx
rtk go get github.com/lib/pq
rtk go get github.com/golang-jwt/jwt/v5
rtk go get golang.org/x/crypto/bcrypt
rtk go get github.com/spf13/cobra
rtk go get github.com/stretchr/testify
rtk go get gopkg.in/yaml.v3
rtk go get github.com/google/uuid
```

- [ ] **Step 2: Create .gitignore and .env.example**

Create `.gitignore`:
```gitignore
bin/
dist/
*.exe
*.test
*.out
.env
.env.local
tmp/
```

Create `.env.example`:
```env
PORT=8080
ENVIRONMENT=development
DATABASE_URL=postgres://nearhive:password@localhost:5432/nearhive?sslmode=disable
JWT_SECRET=super-secret-jwt-key-replace-in-production
NOMINATIM_URL=https://nominatim.openstreetmap.org
GOOGLE_GEO_API_KEY=
GOOGLE_PLACES_KEY=
MAX_SCRAPER_WORKERS=5
SCRAPE_SCHEDULE=0 3 * * *
SCRAPE_REGIONS=Bangalore,Pune,Hyderabad
RATE_LIMIT_AUTH=10
RATE_LIMIT_SEARCH=60
```

- [ ] **Step 3: Write the failing unit test for configuration**

File: `internal/config/config_test.go`
```go
package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("DATABASE_URL", "postgres://localhost/test")
	_ = os.Setenv("JWT_SECRET", "test-secret")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, 5, cfg.MaxScraperWorkers)
	assert.Equal(t, "0 3 * * *", cfg.ScrapeSchedule)
	assert.Equal(t, "https://nominatim.openstreetmap.org", cfg.NominatimURL)
}

func TestLoad_MissingRequired(t *testing.T) {
	os.Clearenv()
	_, err := Load()
	assert.Error(t, err)
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `rtk go test ./internal/config/... -v`
Expected: FAIL ("undefined: Load")

- [ ] **Step 5: Implement configuration loader**

File: `internal/config/config.go`
```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port              string
	Environment       string
	DatabaseURL       string
	JWTSecret         string
	NominatimURL      string
	GoogleGeoAPIKey   string
	GooglePlacesKey   string
	MaxScraperWorkers int
	ScrapeSchedule    string
	ScrapeRegions     []string
	RateLimitAuth     int
	RateLimitSearch   int
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	cfg := &Config{
		Port:              getEnv("PORT", "8080"),
		Environment:       getEnv("ENVIRONMENT", "development"),
		DatabaseURL:       dbURL,
		JWTSecret:         jwtSecret,
		NominatimURL:      getEnv("NOMINATIM_URL", "https://nominatim.openstreetmap.org"),
		GoogleGeoAPIKey:   os.Getenv("GOOGLE_GEO_API_KEY"),
		GooglePlacesKey:   os.Getenv("GOOGLE_PLACES_KEY"),
		MaxScraperWorkers: getEnvInt("MAX_SCRAPER_WORKERS", 5),
		ScrapeSchedule:    getEnv("SCRAPE_SCHEDULE", "0 3 * * *"),
		ScrapeRegions:     splitAndTrim(getEnv("SCRAPE_REGIONS", "Bangalore,Pune,Hyderabad")),
		RateLimitAuth:     getEnvInt("RATE_LIMIT_AUTH", 10),
		RateLimitSearch:   getEnvInt("RATE_LIMIT_SEARCH", 60),
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `rtk go test ./internal/config/... -v`
Expected: PASS

- [ ] **Step 7: Commit configuration setup**

Run:
```bash
rtk git add go.mod go.sum .gitignore .env.example internal/config/
rtk git commit -m "feat(config): add environment configuration subsystem with tests"
```

---

### Task 2: Domain Models & Database Migrations

**Files:**
- Create: `internal/model/models.go`
- Create: `migrations/000001_init_extensions.up.sql`
- Create: `migrations/000001_init_extensions.down.sql`
- Create: `migrations/000002_create_users.up.sql`
- Create: `migrations/000002_create_users.down.sql`
- Create: `migrations/000003_create_companies.up.sql`
- Create: `migrations/000003_create_companies.down.sql`
- Create: `migrations/000004_create_locations.up.sql`
- Create: `migrations/000004_create_locations.down.sql`
- Create: `migrations/000005_create_sightings.up.sql`
- Create: `migrations/000005_create_sightings.down.sql`
- Create: `migrations/000006_create_scrape_jobs.up.sql`
- Create: `migrations/000006_create_scrape_jobs.down.sql`
- Create: `migrations/000007_create_search_history.up.sql`
- Create: `migrations/000007_create_search_history.down.sql`

**Interfaces:**
- Produces: `model.User`, `model.Company`, `model.Location`, `model.Sighting`, `model.ScrapeJob`, `model.SearchHistory`, `model.CompanySearchResult`

- [ ] **Step 1: Define shared domain entities**

File: `internal/model/models.go`
```go
package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type JSONMap map[string]any

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}

func (j *JSONMap) Scan(src any) error {
	if src == nil {
		*j = make(map[string]any)
		return nil
	}
	switch s := src.(type) {
	case []byte:
		return json.Unmarshal(s, j)
	case string:
		return json.Unmarshal([]byte(s), j)
	default:
		return errors.New("cannot scan type into JSONMap")
	}
}

type User struct {
	ID        uuid.UUID `db:"id" json:"id"`
	Email     string    `db:"email" json:"email"`
	Password  string    `db:"password" json:"-"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type Company struct {
	ID             uuid.UUID `db:"id" json:"id"`
	Name           string    `db:"name" json:"name"`
	NormalizedName string    `db:"normalized_name" json:"normalized_name"`
	Domain         *string   `db:"domain" json:"domain,omitempty"`
	Industry       *string   `db:"industry" json:"industry,omitempty"`
	EmployeeCount  *string   `db:"employee_count" json:"employee_count,omitempty"`
	Description    *string   `db:"description" json:"description,omitempty"`
	Verified       bool      `db:"verified" json:"verified"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

type Location struct {
	ID         uuid.UUID `db:"id" json:"id"`
	CompanyID  uuid.UUID `db:"company_id" json:"company_id"`
	Label      *string   `db:"label" json:"label,omitempty"`
	Address    string    `db:"address" json:"address"`
	City       *string   `db:"city" json:"city,omitempty"`
	State      *string   `db:"state" json:"state,omitempty"`
	Country    string    `db:"country" json:"country"`
	Pincode    *string   `db:"pincode" json:"pincode,omitempty"`
	Lat        float64   `db:"lat" json:"lat"`
	Lng        float64   `db:"lng" json:"lng"`
	Confidence float64   `db:"confidence" json:"confidence"`
	Verified   bool      `db:"verified" json:"verified"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

type Sighting struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	Source      string     `db:"source" json:"source"`
	SourceURL   *string    `db:"source_url" json:"source_url,omitempty"`
	CompanyName string     `db:"company_name" json:"company_name"`
	RawAddress  string     `db:"raw_address" json:"raw_address"`
	Lat         float64    `db:"lat" json:"lat"`
	Lng         float64    `db:"lng" json:"lng"`
	Metadata    JSONMap    `db:"metadata" json:"metadata"`
	CompanyID   *uuid.UUID `db:"company_id" json:"company_id,omitempty"`
	LocationID  *uuid.UUID `db:"location_id" json:"location_id,omitempty"`
	ScrapedAt   time.Time  `db:"scraped_at" json:"scraped_at"`
}

type ScrapeJob struct {
	ID         uuid.UUID  `db:"id" json:"id"`
	Source     string     `db:"source" json:"source"`
	Status     string     `db:"status" json:"status"`
	Region     *string    `db:"region" json:"region,omitempty"`
	Sightings  int        `db:"sightings" json:"sightings"`
	Error      *string    `db:"error" json:"error,omitempty"`
	StartedAt  *time.Time `db:"started_at" json:"started_at,omitempty"`
	FinishedAt *time.Time `db:"finished_at" json:"finished_at,omitempty"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
}

type SearchHistory struct {
	ID          uuid.UUID `db:"id" json:"id"`
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	QueryLat    float64   `db:"query_lat" json:"query_lat"`
	QueryLng    float64   `db:"query_lng" json:"query_lng"`
	RadiusKM    float64   `db:"radius_km" json:"radius_km"`
	ResultCount int       `db:"result_count" json:"result_count"`
	SearchedAt  time.Time `db:"searched_at" json:"searched_at"`
}

type CompanySearchResult struct {
	CompanyID      uuid.UUID `db:"company_id" json:"id"`
	Name           string    `db:"name" json:"name"`
	Domain         *string   `db:"domain" json:"domain,omitempty"`
	Industry       *string   `db:"industry" json:"industry,omitempty"`
	EmployeeCount  *string   `db:"employee_count" json:"employee_count,omitempty"`
	LocationID     uuid.UUID `db:"location_id" json:"location_id"`
	Label          *string   `db:"label" json:"label,omitempty"`
	Address        string    `db:"address" json:"address"`
	City           *string   `db:"city" json:"city,omitempty"`
	Lat            float64   `db:"lat" json:"lat"`
	Lng            float64   `db:"lng" json:"lng"`
	Confidence     float64   `db:"confidence" json:"confidence"`
	DistanceMeters float64   `db:"distance_m" json:"distance_meters"`
	Verified       bool      `db:"verified" json:"verified"`
}
```

- [ ] **Step 2: Create SQL migration files**

Create `migrations/000001_init_extensions.up.sql`:
```sql
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
```
Create `migrations/000001_init_extensions.down.sql`:
```sql
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS postgis;
```

Create `migrations/000002_create_users.up.sql`:
```sql
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
Create `migrations/000002_create_users.down.sql`:
```sql
DROP TABLE IF EXISTS users;
```

Create `migrations/000003_create_companies.up.sql`:
```sql
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
```
Create `migrations/000003_create_companies.down.sql`:
```sql
DROP TABLE IF EXISTS companies;
```

Create `migrations/000004_create_locations.up.sql`:
```sql
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
```
Create `migrations/000004_create_locations.down.sql`:
```sql
DROP TABLE IF EXISTS locations;
```

Create `migrations/000005_create_sightings.up.sql`:
```sql
CREATE TABLE IF NOT EXISTS sightings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source VARCHAR(100) NOT NULL,
    source_url TEXT,
    company_name VARCHAR(500) NOT NULL,
    raw_address TEXT,
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION,
    metadata JSONB DEFAULT '{}',
    company_id UUID REFERENCES companies(id) ON DELETE SET NULL,
    location_id UUID REFERENCES locations(id) ON DELETE SET NULL,
    scraped_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sightings_source ON sightings (source);
CREATE INDEX IF NOT EXISTS idx_sightings_company ON sightings (company_id);
CREATE INDEX IF NOT EXISTS idx_sightings_name ON sightings (company_name);
```
Create `migrations/000005_create_sightings.down.sql`:
```sql
DROP TABLE IF EXISTS sightings;
```

Create `migrations/000006_create_scrape_jobs.up.sql`:
```sql
CREATE TABLE IF NOT EXISTS scrape_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    region VARCHAR(255),
    sightings INT DEFAULT 0,
    error TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
Create `migrations/000006_create_scrape_jobs.down.sql`:
```sql
DROP TABLE IF EXISTS scrape_jobs;
```

Create `migrations/000007_create_search_history.up.sql`:
```sql
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
```
Create `migrations/000007_create_search_history.down.sql`:
```sql
DROP TABLE IF EXISTS search_history;
```

- [ ] **Step 3: Verify Go model compilation**

Run: `rtk go build ./internal/model/...`
Expected: PASS

- [ ] **Step 4: Commit domain models and migrations**

Run:
```bash
rtk git add internal/model/ migrations/
rtk git commit -m "feat(model): add domain models and database migrations for PostGIS and tables"
```

---

### Task 3: Store & Repository Layer

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/postgres.go`
- Create: `internal/store/queries.go`
- Create: `internal/store/store_mock.go`
- Create: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `*sqlx.DB`, `model.*`
- Produces: `store.Store`, `store.UserStore`, `store.CompanyStore`, `store.LocationStore`, `store.SightingStore`, `store.JobStore`

- [ ] **Step 1: Define repository interfaces**

File: `internal/store/store.go`
```go
package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
)

var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("record already exists")
)

type SearchOpts struct {
	MinConfidence *float64
	Industry      *string
	Query         *string
	Limit         int
	Offset        int
}

type UserStore interface {
	CreateUser(ctx context.Context, user *model.User) error
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

type CompanyStore interface {
	CreateCompany(ctx context.Context, c *model.Company) error
	GetCompanyByID(ctx context.Context, id uuid.UUID) (*model.Company, error)
	FindByDomain(ctx context.Context, domain string) (*model.Company, error)
	FindByNormalizedName(ctx context.Context, name string) (*model.Company, error)
	FindByFuzzyName(ctx context.Context, name string, threshold float64) (*model.Company, error)
	UpdateCompany(ctx context.Context, c *model.Company) error
}

type LocationStore interface {
	CreateLocation(ctx context.Context, l *model.Location) error
	GetLocationsByCompany(ctx context.Context, companyID uuid.UUID) ([]model.Location, error)
	FindNearbyLocation(ctx context.Context, companyID uuid.UUID, lat, lng float64, radiusMeters float64) (*model.Location, error)
	UpdateLocationConfidence(ctx context.Context, id uuid.UUID, confidence float64) error
	UpdateLocationCoords(ctx context.Context, id uuid.UUID, lat, lng float64) error
	Search(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) ([]model.CompanySearchResult, error)
	CountSearch(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) (int, error)
}

type SightingStore interface {
	SaveSightings(ctx context.Context, source string, sightings []model.Sighting) error
	GetSightingsByCompany(ctx context.Context, companyID uuid.UUID) ([]model.Sighting, error)
	LinkSighting(ctx context.Context, sightingID uuid.UUID, companyID, locationID uuid.UUID) error
}

type JobStore interface {
	CreateJob(ctx context.Context, job *model.ScrapeJob) error
	UpdateJob(ctx context.Context, job *model.ScrapeJob) error
	GetJobByID(ctx context.Context, id uuid.UUID) (*model.ScrapeJob, error)
	ListJobs(ctx context.Context, limit, offset int) ([]model.ScrapeJob, error)
}

type SearchHistoryStore interface {
	RecordSearch(ctx context.Context, h *model.SearchHistory) error
	GetHistoryByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.SearchHistory, error)
}

type Store interface {
	UserStore
	CompanyStore
	LocationStore
	SightingStore
	JobStore
	SearchHistoryStore
	Close() error
}
```

- [ ] **Step 2: Implement Postgres repository queries & methods**

File: `internal/store/postgres.go`
```go
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
```

- [ ] **Step 3: Create In-Memory Mock Store for Unit Testing**

File: `internal/store/store_mock.go`
```go
package store

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
)

type MockStore struct {
	mu            sync.RWMutex
	Users         map[uuid.UUID]*model.User
	UsersByEmail  map[string]*model.User
	Companies     map[uuid.UUID]*model.Company
	Locations     map[uuid.UUID]*model.Location
	Sightings     map[uuid.UUID]*model.Sighting
	Jobs          map[uuid.UUID]*model.ScrapeJob
	SearchHistory []model.SearchHistory
}

func NewMockStore() *MockStore {
	return &MockStore{
		Users:        make(map[uuid.UUID]*model.User),
		UsersByEmail: make(map[string]*model.User),
		Companies:    make(map[uuid.UUID]*model.Company),
		Locations:    make(map[uuid.UUID]*model.Location),
		Sightings:    make(map[uuid.UUID]*model.Sighting),
		Jobs:         make(map[uuid.UUID]*model.ScrapeJob),
	}
}

func (m *MockStore) Close() error { return nil }

func (m *MockStore) CreateUser(_ context.Context, u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	m.Users[u.ID] = u
	m.UsersByEmail[u.Email] = u
	return nil
}

func (m *MockStore) GetUserByEmail(_ context.Context, email string) (*model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.UsersByEmail[email]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MockStore) GetUserByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.Users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MockStore) CreateCompany(_ context.Context, c *model.Company) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	m.Companies[c.ID] = c
	return nil
}

func (m *MockStore) GetCompanyByID(_ context.Context, id uuid.UUID) (*model.Company, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.Companies[id]
	if !ok {
		return nil, ErrNotFound
	}
	return c, nil
}

func (m *MockStore) FindByDomain(_ context.Context, domain string) (*model.Company, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.Companies {
		if c.Domain != nil && *c.Domain == domain {
			return c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockStore) FindByNormalizedName(_ context.Context, name string) (*model.Company, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.Companies {
		if c.NormalizedName == name {
			return c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockStore) FindByFuzzyName(_ context.Context, name string, _ float64) (*model.Company, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.Companies {
		if strings.Contains(c.NormalizedName, name) || strings.Contains(name, c.NormalizedName) {
			return c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockStore) UpdateCompany(_ context.Context, c *model.Company) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Companies[c.ID] = c
	return nil
}

func (m *MockStore) CreateLocation(_ context.Context, l *model.Location) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	m.Locations[l.ID] = l
	return nil
}

func (m *MockStore) GetLocationsByCompany(_ context.Context, companyID uuid.UUID) ([]model.Location, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []model.Location
	for _, l := range m.Locations {
		if l.CompanyID == companyID {
			res = append(res, *l)
		}
	}
	return res, nil
}

func (m *MockStore) FindNearbyLocation(_ context.Context, companyID uuid.UUID, lat, lng float64, radiusMeters float64) (*model.Location, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, l := range m.Locations {
		if l.CompanyID == companyID {
			dist := haversineDistance(lat, lng, l.Lat, l.Lng)
			if dist <= radiusMeters {
				return l, nil
			}
		}
	}
	return nil, nil
}

func (m *MockStore) UpdateLocationConfidence(_ context.Context, id uuid.UUID, confidence float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if l, ok := m.Locations[id]; ok {
		l.Confidence = confidence
	}
	return nil
}

func (m *MockStore) UpdateLocationCoords(_ context.Context, id uuid.UUID, lat, lng float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if l, ok := m.Locations[id]; ok {
		l.Lat = lat
		l.Lng = lng
	}
	return nil
}

func (m *MockStore) Search(_ context.Context, lat, lng, radiusMeters float64, opts SearchOpts) ([]model.CompanySearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []model.CompanySearchResult

	for _, l := range m.Locations {
		c, ok := m.Companies[l.CompanyID]
		if !ok {
			continue
		}
		dist := haversineDistance(lat, lng, l.Lat, l.Lng)
		if dist <= radiusMeters {
			if opts.MinConfidence != nil && l.Confidence < *opts.MinConfidence {
				continue
			}
			results = append(results, model.CompanySearchResult{
				CompanyID:      c.ID,
				Name:           c.Name,
				Domain:         c.Domain,
				Industry:       c.Industry,
				EmployeeCount:  c.EmployeeCount,
				LocationID:     l.ID,
				Label:          l.Label,
				Address:        l.Address,
				City:           l.City,
				Lat:            l.Lat,
				Lng:            l.Lng,
				Confidence:     l.Confidence,
				DistanceMeters: dist,
				Verified:       l.Verified,
			})
		}
	}
	return results, nil
}

func (m *MockStore) CountSearch(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) (int, error) {
	res, err := m.Search(ctx, lat, lng, radiusMeters, opts)
	return len(res), err
}

func (m *MockStore) SaveSightings(_ context.Context, source string, sightings []model.Sighting) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range sightings {
		if s.ID == uuid.Nil {
			s.ID = uuid.New()
		}
		s.Source = source
		sCopy := s
		m.Sightings[s.ID] = &sCopy
	}
	return nil
}

func (m *MockStore) GetSightingsByCompany(_ context.Context, companyID uuid.UUID) ([]model.Sighting, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []model.Sighting
	for _, s := range m.Sightings {
		if s.CompanyID != nil && *s.CompanyID == companyID {
			res = append(res, *s)
		}
	}
	return res, nil
}

func (m *MockStore) LinkSighting(_ context.Context, sightingID, companyID, locationID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.Sightings[sightingID]; ok {
		s.CompanyID = &companyID
		s.LocationID = &locationID
	}
	return nil
}

func (m *MockStore) CreateJob(_ context.Context, job *model.ScrapeJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	m.Jobs[job.ID] = job
	return nil
}

func (m *MockStore) UpdateJob(_ context.Context, job *model.ScrapeJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Jobs[job.ID] = job
	return nil
}

func (m *MockStore) GetJobByID(_ context.Context, id uuid.UUID) (*model.ScrapeJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.Jobs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return j, nil
}

func (m *MockStore) ListJobs(_ context.Context, _, _ int) ([]model.ScrapeJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []model.ScrapeJob
	for _, j := range m.Jobs {
		res = append(res, *j)
	}
	return res, nil
}

func (m *MockStore) RecordSearch(_ context.Context, h *model.SearchHistory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SearchHistory = append(m.SearchHistory, *h)
	return nil
}

func (m *MockStore) GetHistoryByUser(_ context.Context, userID uuid.UUID, limit int) ([]model.SearchHistory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []model.SearchHistory
	for _, h := range m.SearchHistory {
		if h.UserID == userID {
			res = append(res, h)
			if len(res) >= limit {
				break
			}
		}
	}
	return res, nil
}

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Earth radius in meters
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}
```

- [ ] **Step 4: Write unit test for repository behavior**

File: `internal/store/store_test.go`
```go
package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestMockStore_CompanyAndLocationSearch(t *testing.T) {
	ctx := context.Background()
	mock := NewMockStore()

	companyID := uuid.New()
	c := &model.Company{
		ID:             companyID,
		Name:           "Tech Corp",
		NormalizedName: "tech corp",
	}
	err := mock.CreateCompany(ctx, c)
	assert.NoError(t, err)

	loc := &model.Location{
		CompanyID:  companyID,
		Address:    "Whitefield, Bangalore",
		Lat:        12.9854,
		Lng:        77.7366,
		Confidence: 0.8,
	}
	err = mock.CreateLocation(ctx, loc)
	assert.NoError(t, err)

	// Search within 5km from center close to location
	results, err := mock.Search(ctx, 12.9850, 77.7360, 5000, SearchOpts{})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Tech Corp", results[0].Name)

	// Search from far away (50km away) with 1km radius
	farResults, err := mock.Search(ctx, 13.5000, 78.0000, 1000, SearchOpts{})
	assert.NoError(t, err)
	assert.Empty(t, farResults)
}
```

- [ ] **Step 5: Run tests**

Run: `rtk go test ./internal/store/... -v`
Expected: PASS

- [ ] **Step 6: Commit store layer**

Run:
```bash
rtk git add internal/store/
rtk git commit -m "feat(store): implement repository interfaces, postgres backend, and mock store"
```

---

### Task 4: Authentication & Security Engine

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/jwt.go`
- Create: `internal/auth/auth_test.go`

**Interfaces:**
- Consumes: User credentials, JWT secret
- Produces: `auth.HashPassword`, `auth.CheckPasswordHash`, `auth.Manager`, `auth.Claims`

- [ ] **Step 1: Write failing tests for password and JWT handling**

File: `internal/auth/auth_test.go`
```go
package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPasswordHashing(t *testing.T) {
	password := "Secret123!"
	hash, err := HashPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)

	assert.True(t, CheckPasswordHash(password, hash))
	assert.False(t, CheckPasswordHash("WrongPassword", hash))
}

func TestJWTTokenGenerationAndValidation(t *testing.T) {
	mgr := NewManager("test-jwt-secret-key-1234567890123456", 1*time.Hour)
	userID := uuid.New()

	tokenStr, err := mgr.GenerateToken(userID)
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	claims, err := mgr.ValidateToken(tokenStr)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
}

func TestJWTTokenValidation_InvalidSecret(t *testing.T) {
	mgr1 := NewManager("secret-key-one-123456789012345678", 1*time.Hour)
	mgr2 := NewManager("secret-key-two-123456789012345678", 1*time.Hour)

	tokenStr, _ := mgr1.GenerateToken(uuid.New())
	_, err := mgr2.ValidateToken(tokenStr)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/auth/... -v`
Expected: FAIL ("undefined: HashPassword")

- [ ] **Step 3: Implement password hashing**

File: `internal/auth/password.go`
```go
package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
```

- [ ] **Step 4: Implement JWT manager**

File: `internal/auth/jwt.go`
```go
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

type Claims struct {
	UserID uuid.UUID `json:"sub"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Manager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (m *Manager) GenerateToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *Manager) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
```

- [ ] **Step 5: Run tests**

Run: `rtk go test ./internal/auth/... -v`
Expected: PASS

- [ ] **Step 6: Commit auth subsystem**

Run:
```bash
rtk git add internal/auth/
rtk git commit -m "feat(auth): implement bcrypt password hashing and JWT manager"
```

---

### Task 5: Geocoder Subsystem

**Files:**
- Create: `internal/geocoder/geocoder.go`
- Create: `internal/geocoder/nominatim.go`
- Create: `internal/geocoder/google.go`
- Create: `internal/geocoder/fallback.go`
- Create: `internal/geocoder/geocoder_test.go`

**Interfaces:**
- Produces: `geocoder.Geocoder`, `geocoder.GeoResult`, `geocoder.NewFallbackGeocoder()`

- [ ] **Step 1: Write failing test for geocoder fallback and response parsing**

File: `internal/geocoder/geocoder_test.go`
```go
package geocoder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNominatim_Geocode(t *testing.T) {
	mockResponse := `[
		{
			"lat": "12.9854",
			"lon": "77.7366",
			"display_name": "ITPB, Whitefield, Bangalore, Karnataka, India"
		}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResponse))
	}))
	defer srv.Close()

	geo := NewNominatim(srv.URL, srv.Client())
	res, err := geo.Geocode(context.Background(), "ITPB Whitefield")

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.InDelta(t, 12.9854, res.Lat, 0.0001)
	assert.InDelta(t, 77.7366, res.Lng, 0.0001)
	assert.Contains(t, res.Address, "ITPB")
}

func TestFallbackGeocoder_PrimaryFailsThenFallbackSucceeds(t *testing.T) {
	primarySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer primarySrv.Close()

	fallbackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"results": [{
				"formatted_address": "Google Office, Bangalore",
				"geometry": {"location": {"lat": 12.9716, "lng": 77.5946}}
			}],
			"status": "OK"
		}`))
	}))
	defer fallbackSrv.Close()

	primary := NewNominatim(primarySrv.URL, primarySrv.Client())
	fallback := NewGoogle(fallbackSrv.URL, "dummy-key", fallbackSrv.Client())

	chained := NewFallbackGeocoder(primary, fallback)
	res, err := chained.Geocode(context.Background(), "Google Bangalore")

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.InDelta(t, 12.9716, res.Lat, 0.0001)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/geocoder/... -v`
Expected: FAIL ("undefined: NewNominatim")

- [ ] **Step 3: Implement core geocoder interface and models**

File: `internal/geocoder/geocoder.go`
```go
package geocoder

import (
	"context"
	"errors"
)

var (
	ErrNoResults = errors.New("no geocoding results found")
)

type GeoResult struct {
	Lat     float64
	Lng     float64
	Address string
}

type Geocoder interface {
	Geocode(ctx context.Context, address string) (*GeoResult, error)
	ReverseGeocode(ctx context.Context, lat, lng float64) (*GeoResult, error)
}
```

- [ ] **Step 4: Implement Nominatim Geocoder**

File: `internal/geocoder/nominatim.go`
```go
package geocoder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Nominatim struct {
	baseURL string
	client  *http.Client
}

func NewNominatim(baseURL string, client *http.Client) *Nominatim {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://nominatim.openstreetmap.org"
	}
	return &Nominatim{
		baseURL: baseURL,
		client:  client,
	}
}

type nominatimResult struct {
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	DisplayName string `json:"display_name"`
}

func (n *Nominatim) Geocode(ctx context.Context, address string) (*GeoResult, error) {
	reqURL := fmt.Sprintf("%s/search?q=%s&format=json&limit=1", n.baseURL, url.QueryEscape(address))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NearHive/1.0 (contact@nearhive.local)")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned status: %d", resp.StatusCode)
	}

	var results []nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, ErrNoResults
	}

	lat, err := strconv.ParseFloat(results[0].Lat, 64)
	if err != nil {
		return nil, err
	}
	lng, err := strconv.ParseFloat(results[0].Lon, 64)
	if err != nil {
		return nil, err
	}

	return &GeoResult{
		Lat:     lat,
		Lng:     lng,
		Address: results[0].DisplayName,
	}, nil
}

func (n *Nominatim) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeoResult, error) {
	reqURL := fmt.Sprintf("%s/reverse?lat=%f&lon=%f&format=json", n.baseURL, lat, lng)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NearHive/1.0")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned status: %d", resp.StatusCode)
	}

	var result nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &GeoResult{
		Lat:     lat,
		Lng:     lng,
		Address: result.DisplayName,
	}, nil
}
```

- [ ] **Step 5: Implement Google Geocoder and Fallback Geocoder**

File: `internal/geocoder/google.go`
```go
package geocoder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Google struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewGoogle(baseURL, apiKey string, client *http.Client) *Google {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://maps.googleapis.com/maps/api/geocode/json"
	}
	return &Google{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  client,
	}
}

type googleGeocodeResponse struct {
	Status  string `json:"status"`
	Results []struct {
		FormattedAddress string `json:"formatted_address"`
		Geometry         struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
}

func (g *Google) Geocode(ctx context.Context, address string) (*GeoResult, error) {
	if g.apiKey == "" && g.baseURL == "https://maps.googleapis.com/maps/api/geocode/json" {
		return nil, errors.New("google geocoding api key is not configured")
	}

	reqURL := fmt.Sprintf("%s?address=%s&key=%s", g.baseURL, url.QueryEscape(address), g.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data googleGeocodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Status != "OK" || len(data.Results) == 0 {
		return nil, ErrNoResults
	}

	return &GeoResult{
		Lat:     data.Results[0].Geometry.Location.Lat,
		Lng:     data.Results[0].Geometry.Location.Lng,
		Address: data.Results[0].FormattedAddress,
	}, nil
}

func (g *Google) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeoResult, error) {
	reqURL := fmt.Sprintf("%s?latlng=%f,%f&key=%s", g.baseURL, lat, lng, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data googleGeocodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Status != "OK" || len(data.Results) == 0 {
		return nil, ErrNoResults
	}

	return &GeoResult{
		Lat:     data.Results[0].Geometry.Location.Lat,
		Lng:     data.Results[0].Geometry.Location.Lng,
		Address: data.Results[0].FormattedAddress,
	}, nil
}
```

File: `internal/geocoder/fallback.go`
```go
package geocoder

import "context"

type FallbackGeocoder struct {
	primary  Geocoder
	fallback Geocoder
}

func NewFallbackGeocoder(primary, fallback Geocoder) *FallbackGeocoder {
	return &FallbackGeocoder{
		primary:  primary,
		fallback: fallback,
	}
}

func (f *FallbackGeocoder) Geocode(ctx context.Context, address string) (*GeoResult, error) {
	if f.primary != nil {
		if res, err := f.primary.Geocode(ctx, address); err == nil && res != nil {
			return res, nil
		}
	}
	if f.fallback != nil {
		return f.fallback.Geocode(ctx, address)
	}
	return nil, ErrNoResults
}

func (f *FallbackGeocoder) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeoResult, error) {
	if f.primary != nil {
		if res, err := f.primary.ReverseGeocode(ctx, lat, lng); err == nil && res != nil {
			return res, nil
		}
	}
	if f.fallback != nil {
		return f.fallback.ReverseGeocode(ctx, lat, lng)
	}
	return nil, ErrNoResults
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `rtk go test ./internal/geocoder/... -v`
Expected: PASS

- [ ] **Step 7: Commit geocoder subsystem**

Run:
```bash
rtk git add internal/geocoder/
rtk git commit -m "feat(geocoder): implement Nominatim, Google API, and fallback geocoding pipeline"
```

---

### Task 6: Verification Engine

**Files:**
- Create: `internal/verifier/normalizer.go`
- Create: `internal/verifier/matcher.go`
- Create: `internal/verifier/merger.go`
- Create: `internal/verifier/geoverify.go`
- Create: `internal/verifier/verifier.go`
- Create: `internal/verifier/verifier_test.go`

**Interfaces:**
- Consumes: `store.Store`, `geocoder.Geocoder`, `model.Sighting`
- Produces: `verifier.Normalize`, `verifier.Matcher`, `verifier.Merger`, `verifier.Engine`

- [ ] **Step 1: Write failing tests for name normalization and company matching**

File: `internal/verifier/verifier_test.go`
```go
package verifier

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Infosys Limited", "infosys"},
		{"INFOSYS LTD", "infosys"},
		{"Wipro Technologies Pvt. Ltd.", "wipro"},
		{"Tata Consultancy Services (TCS) India", "tata consultancy services tcs"},
		{"Microsoft Corporation India Pvt Ltd", "microsoft"},
		{"ThoughtWorks Software Solutions", "thoughtworks"},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, Normalize(tc.input), "mismatch for %s", tc.input)
	}
}

func TestMatcher_DomainAndNameMatch(t *testing.T) {
	mockStore := store.NewMockStore()
	companyID := uuid.New()
	domain := "infosys.com"

	_ = mockStore.CreateCompany(context.Background(), &model.Company{
		ID:             companyID,
		Name:           "Infosys Limited",
		NormalizedName: "infosys",
		Domain:         &domain,
	})

	matcher := NewMatcher(mockStore)

	// 1. Domain match
	domainMatch, err := matcher.FindMatch(context.Background(), model.Sighting{
		CompanyName: "Infosys Careers",
		Metadata:    model.JSONMap{"website": "https://www.infosys.com/about"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, domainMatch)
	assert.Equal(t, companyID, domainMatch.CompanyID)
	assert.Equal(t, "domain", domainMatch.MatchType)

	// 2. Normalized name match
	nameMatch, err := matcher.FindMatch(context.Background(), model.Sighting{
		CompanyName: "INFOSYS LIMITED",
	})
	assert.NoError(t, err)
	assert.NotNil(t, nameMatch)
	assert.Equal(t, companyID, nameMatch.CompanyID)
	assert.Equal(t, "exact_name", nameMatch.MatchType)
}

func TestMerger_ConfidenceScoring(t *testing.T) {
	mockStore := store.NewMockStore()
	merger := NewMerger(mockStore)

	sighting1 := model.Sighting{
		CompanyName: "Swiggy",
		RawAddress:  "Koramangala, Bangalore",
		Lat:         12.9352,
		Lng:         77.6245,
		Source:      "osm",
	}

	match := &MatchResult{
		CompanyID: uuid.New(),
		MatchType: "new_company",
	}
	_ = mockStore.CreateCompany(context.Background(), &model.Company{
		ID:             match.CompanyID,
		Name:           sighting1.CompanyName,
		NormalizedName: "swiggy",
	})

	// First sighting from OSM (weight 0.35)
	err := merger.MergeSighting(context.Background(), sighting1, match)
	assert.NoError(t, err)

	locs, _ := mockStore.GetLocationsByCompany(context.Background(), match.CompanyID)
	assert.Len(t, locs, 1)
	assert.InDelta(t, 0.35, locs[0].Confidence, 0.01)

	// Second sighting from TechPark at same location (weight 0.40)
	sighting2 := model.Sighting{
		CompanyName: "Swiggy",
		RawAddress:  "Koramangala 4th Block",
		Lat:         12.9354, // ~25m away
		Lng:         77.6246,
		Source:      "techpark",
	}
	err = merger.MergeSighting(context.Background(), sighting2, match)
	assert.NoError(t, err)

	locsAfter, _ := mockStore.GetLocationsByCompany(context.Background(), match.CompanyID)
	assert.Len(t, locsAfter, 1) // Merged into existing location
	assert.InDelta(t, 0.75, locsAfter[0].Confidence, 0.01)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/verifier/... -v`
Expected: FAIL ("undefined: Normalize")

- [ ] **Step 3: Implement Name Normalization**

File: `internal/verifier/normalizer.go`
```go
package verifier

import (
	"regexp"
	"strings"
)

var (
	nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9\s]`)
	multipleSpacesRegex  = regexp.MustCompile(`\s+`)
	suffixes             = []string{
		" limited", " ltd", " pvt", " private",
		" inc", " incorporated", " corp", " corporation",
		" llp", " llc", " technologies", " tech",
		" solutions", " software", " systems",
		" india", " labs", " group",
	}
)

func Normalize(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))

	// Remove common corporate suffixes repeatedly until clean
	cleaned := true
	for cleaned {
		cleaned = false
		for _, s := range suffixes {
			if strings.HasSuffix(n, s) {
				n = strings.TrimSpace(strings.TrimSuffix(n, s))
				cleaned = true
			}
		}
	}

	n = nonAlphanumericRegex.ReplaceAllString(n, " ")
	n = multipleSpacesRegex.ReplaceAllString(n, " ")

	return strings.TrimSpace(n)
}
```

- [ ] **Step 4: Implement Matcher**

File: `internal/verifier/matcher.go`
```go
package verifier

import (
	"context"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

type MatchResult struct {
	CompanyID  uuid.UUID
	Confidence float64
	MatchType  string
}

type Matcher struct {
	store store.CompanyStore
}

func NewMatcher(s store.CompanyStore) *Matcher {
	return &Matcher{store: s}
}

func (m *Matcher) FindMatch(ctx context.Context, s model.Sighting) (*MatchResult, error) {
	// 1. Check domain match from metadata
	if domain := extractDomainFromMetadata(s.Metadata); domain != "" {
		if company, err := m.store.FindByDomain(ctx, domain); err == nil && company != nil {
			return &MatchResult{
				CompanyID:  company.ID,
				Confidence: 0.5,
				MatchType:  "domain",
			}, nil
		}
	}

	// 2. Check exact normalized name match
	normalized := Normalize(s.CompanyName)
	if normalized != "" {
		if company, err := m.store.FindByNormalizedName(ctx, normalized); err == nil && company != nil {
			return &MatchResult{
				CompanyID:  company.ID,
				Confidence: 0.4,
				MatchType:  "exact_name",
			}, nil
		}

		// 3. Check fuzzy match
		if company, err := m.store.FindByFuzzyName(ctx, normalized, 0.6); err == nil && company != nil {
			return &MatchResult{
				CompanyID:  company.ID,
				Confidence: 0.2,
				MatchType:  "fuzzy_name",
			}, nil
		}
	}

	return nil, nil
}

func extractDomainFromMetadata(meta model.JSONMap) string {
	if meta == nil {
		return ""
	}
	for _, key := range []string{"website", "url", "domain"} {
		if val, ok := meta[key].(string); ok && val != "" {
			u, err := url.Parse(val)
			if err == nil && u.Host != "" {
				host := strings.ToLower(u.Host)
				return strings.TrimPrefix(host, "www.")
			}
			return strings.ToLower(strings.TrimPrefix(val, "www."))
		}
	}
	return ""
}
```

- [ ] **Step 5: Implement Merger, GeoVerifier, and Engine Orchestrator**

File: `internal/verifier/merger.go`
```go
package verifier

import (
	"context"
	"math"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

type Merger struct {
	store store.Store
}

func NewMerger(s store.Store) *Merger {
	return &Merger{store: s}
}

func (m *Merger) MergeSighting(ctx context.Context, s model.Sighting, match *MatchResult) error {
	var companyID uuid.UUID

	if match == nil {
		// Create new company
		domain := extractDomainFromMetadata(s.Metadata)
		var domainPtr *string
		if domain != "" {
			domainPtr = &domain
		}
		newCompany := &model.Company{
			ID:             uuid.New(),
			Name:           s.CompanyName,
			NormalizedName: Normalize(s.CompanyName),
			Domain:         domainPtr,
		}
		if err := m.store.CreateCompany(ctx, newCompany); err != nil {
			return err
		}
		companyID = newCompany.ID
	} else {
		companyID = match.CompanyID
	}

	sourceWeight := getSourceWeight(s.Source)

	// Check if this location already exists for this company within 500m
	if s.Lat != 0 && s.Lng != 0 {
		existing, err := m.store.FindNearbyLocation(ctx, companyID, s.Lat, s.Lng, 500)
		if err != nil {
			return err
		}
		if existing != nil {
			newConf := math.Min(1.0, existing.Confidence+sourceWeight)
			if err := m.store.UpdateLocationConfidence(ctx, existing.ID, newConf); err != nil {
				return err
			}
			return m.store.LinkSighting(ctx, s.ID, companyID, existing.ID)
		}
	}

	// Create new location record
	newLoc := &model.Location{
		ID:         uuid.New(),
		CompanyID:  companyID,
		Address:    s.RawAddress,
		Lat:        s.Lat,
		Lng:        s.Lng,
		Confidence: sourceWeight,
		Verified:   sourceWeight >= 0.8,
	}
	if err := m.store.CreateLocation(ctx, newLoc); err != nil {
		return err
	}

	return m.store.LinkSighting(ctx, s.ID, companyID, newLoc.ID)
}

func getSourceWeight(source string) float64 {
	weights := map[string]float64{
		"techpark":  0.40,
		"osm":       0.35,
		"google":    0.35,
		"mca":       0.30,
		"linkedin":  0.25,
		"justdial":  0.25,
		"crunchbase": 0.20,
		"jobportal": 0.15,
	}
	if w, ok := weights[source]; ok {
		return w
	}
	return 0.10
}
```

File: `internal/verifier/geoverify.go`
```go
package verifier

import (
	"context"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/geocoder"
	"github.com/sonukumar/nearhive/internal/store"
)

type GeoVerifier struct {
	store    store.Store
	geocoder geocoder.Geocoder
}

func NewGeoVerifier(s store.Store, g geocoder.Geocoder) *GeoVerifier {
	return &GeoVerifier{store: s, geocoder: g}
}

func (gv *GeoVerifier) VerifyCompanyLocations(ctx context.Context, companyID uuid.UUID) error {
	locs, err := gv.store.GetLocationsByCompany(ctx, companyID)
	if err != nil {
		return err
	}

	for _, loc := range locs {
		if loc.Lat == 0 && loc.Lng == 0 && loc.Address != "" && gv.geocoder != nil {
			res, err := gv.geocoder.Geocode(ctx, loc.Address)
			if err == nil && res != nil {
				_ = gv.store.UpdateLocationCoords(ctx, loc.ID, res.Lat, res.Lng)
			}
		}
	}
	return nil
}
```

File: `internal/verifier/verifier.go`
```go
package verifier

import (
	"context"

	"github.com/sonukumar/nearhive/internal/geocoder"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

type Engine struct {
	matcher     *Matcher
	merger      *Merger
	geoverifier *GeoVerifier
}

func NewEngine(s store.Store, g geocoder.Geocoder) *Engine {
	return &Engine{
		matcher:     NewMatcher(s),
		merger:      NewMerger(s),
		geoverifier: NewGeoVerifier(s, g),
	}
}

func (e *Engine) ProcessSighting(ctx context.Context, s model.Sighting) error {
	match, err := e.matcher.FindMatch(ctx, s)
	if err != nil {
		return err
	}

	if err := e.merger.MergeSighting(ctx, s, match); err != nil {
		return err
	}

	if match != nil {
		return e.geoverifier.VerifyCompanyLocations(ctx, match.CompanyID)
	}
	return nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `rtk go test ./internal/verifier/... -v`
Expected: PASS

- [ ] **Step 7: Commit verification subsystem**

Run:
```bash
rtk git add internal/verifier/
rtk git commit -m "feat(verifier): implement name normalizer, company matcher, merger, and confidence scorer"
```

---

### Task 7: Scraper Infrastructure & OpenStreetMap Scraper

**Files:**
- Create: `internal/scraper/scraper.go`
- Create: `internal/scraper/ratelimit.go`
- Create: `internal/scraper/orchestrator.go`
- Create: `internal/scraper/sources/osm.go`
- Create: `internal/scraper/sources/osm_test.go`
- Create: `internal/scraper/sources/testdata/osm_bangalore_response.json`

**Interfaces:**
- Produces: `scraper.Scraper`, `scraper.Orchestrator`, `sources.NewOSMScraper`

- [ ] **Step 1: Create fixture data for OSM test**

File: `internal/scraper/sources/testdata/osm_bangalore_response.json`
```json
{
  "elements": [
    {
      "type": "node",
      "id": 101,
      "lat": 12.9854,
      "lon": 77.7366,
      "tags": {
        "name": "Mu Sigma",
        "office": "it",
        "addr:street": "ITPB Main Road",
        "addr:city": "Bangalore"
      }
    },
    {
      "type": "node",
      "id": 102,
      "lat": 12.9352,
      "lon": 77.6245,
      "tags": {
        "name": "Flipkart Internet Private Limited",
        "office": "company",
        "addr:city": "Bangalore",
        "website": "https://www.flipkart.com"
      }
    }
  ]
}
```

- [ ] **Step 2: Write failing test for OSM Scraper**

File: `internal/scraper/sources/osm_test.go`
```go
package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/stretchr/testify/assert"
)

func TestOSMScraper_ParseResponse(t *testing.T) {
	fixture, err := os.ReadFile("testdata/osm_bangalore_response.json")
	assert.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer srv.Close()

	osm := NewOSMScraperWithURL(srv.URL, srv.Client())
	res, err := osm.Scrape(context.Background(), scraper.ScrapeRequest{
		Region:   "Bangalore",
		Lat:      12.9716,
		Lng:      77.5946,
		RadiusKM: 15,
	})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, res.Sightings, 2)
	assert.Equal(t, "Mu Sigma", res.Sightings[0].CompanyName)
	assert.Equal(t, "Flipkart Internet Private Limited", res.Sightings[1].CompanyName)
	assert.Equal(t, 12.9854, res.Sightings[0].Lat)
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `rtk go test ./internal/scraper/sources/... -v`
Expected: FAIL ("undefined: NewOSMScraperWithURL")

- [ ] **Step 4: Implement Scraper Interfaces & Rate Limiting**

File: `internal/scraper/scraper.go`
```go
package scraper

import (
	"context"

	"github.com/sonukumar/nearhive/internal/model"
)

type ScrapeRequest struct {
	Region   string
	Lat      float64
	Lng      float64
	RadiusKM float64
}

type ScrapeResult struct {
	Sightings []model.Sighting
	Errors    []error
}

type Scraper interface {
	Name() string
	Supports(region string) bool
	Scrape(ctx context.Context, req ScrapeRequest) (*ScrapeResult, error)
}
```

File: `internal/scraper/ratelimit.go`
```go
package scraper

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiterRegistry struct {
	limiters map[string]*rate.Limiter
}

func NewRateLimiterRegistry() *RateLimiterRegistry {
	return &RateLimiterRegistry{
		limiters: map[string]*rate.Limiter{
			"osm":      rate.NewLimiter(rate.Every(2*time.Second), 1),
			"techpark": rate.NewLimiter(rate.Every(3*time.Second), 1),
			"google":   rate.NewLimiter(rate.Every(200*time.Millisecond), 5),
			"justdial": rate.NewLimiter(rate.Every(5*time.Second), 1),
		},
	}
}

func (r *RateLimiterRegistry) Wait(ctx context.Context, source string) error {
	limiter, ok := r.limiters[source]
	if !ok {
		return nil
	}
	return limiter.Wait(ctx)
}
```

- [ ] **Step 5: Implement OSM Scraper**

File: `internal/scraper/sources/osm.go`
```go
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
)

type OSMScraper struct {
	apiURL string
	client *http.Client
}

func NewOSMScraper() *OSMScraper {
	return NewOSMScraperWithURL("https://overpass-api.de/api/interpreter", nil)
}

func NewOSMScraperWithURL(apiURL string, client *http.Client) *OSMScraper {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &OSMScraper{
		apiURL: apiURL,
		client: client,
	}
}

func (o *OSMScraper) Name() string {
	return "osm"
}

func (o *OSMScraper) Supports(region string) bool {
	return true
}

type overpassElement struct {
	Type float64           `json:"lat"`
	Lat  float64           `json:"lat"`
	Lon  float64           `json:"lon"`
	Tags map[string]string `json:"tags"`
}

type overpassResponse struct {
	Elements []overpassElement `json:"elements"`
}

func (o *OSMScraper) Scrape(ctx context.Context, req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	radiusMeters := int(req.RadiusKM * 1000)
	if radiusMeters <= 0 {
		radiusMeters = 15000
	}

	query := fmt.Sprintf(`[out:json][timeout:25];(node["office"~"company|it|coworking"](around:%d,%f,%f););out center;`,
		radiusMeters, req.Lat, req.Lng)

	data := url.Values{}
	data.Set("data", query)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("User-Agent", "NearHive/1.0")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("overpass returned status %d", resp.StatusCode)
	}

	var opResp overpassResponse
	if err := json.NewDecoder(resp.Body).Decode(&opResp); err != nil {
		return nil, err
	}

	var sightings []model.Sighting
	for _, el := range opResp.Elements {
		name := el.Tags["name"]
		if name == "" {
			continue
		}
		var addressParts []string
		if street := el.Tags["addr:street"]; street != "" {
			addressParts = append(addressParts, street)
		}
		if city := el.Tags["addr:city"]; city != "" {
			addressParts = append(addressParts, city)
		}
		rawAddress := strings.Join(addressParts, ", ")
		if rawAddress == "" {
			rawAddress = req.Region
		}

		meta := make(model.JSONMap)
		if site := el.Tags["website"]; site != "" {
			meta["website"] = site
		}

		sightings = append(sightings, model.Sighting{
			Source:      o.Name(),
			CompanyName: name,
			RawAddress:  rawAddress,
			Lat:         el.Lat,
			Lng:         el.Lon,
			Metadata:    meta,
			ScrapedAt:   time.Now(),
		})
	}

	return &scraper.ScrapeResult{
		Sightings: sightings,
	}, nil
}
```

- [ ] **Step 6: Implement Scraper Orchestrator**

File: `internal/scraper/orchestrator.go`
```go
package scraper

import (
	"context"
	"sync"

	"github.com/sonukumar/nearhive/internal/geocoder"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/sonukumar/nearhive/internal/verifier"
)

type Orchestrator struct {
	scrapers    []Scraper
	store       store.Store
	geocoder    geocoder.Geocoder
	verifier    *verifier.Engine
	rateLimiter *RateLimiterRegistry
	maxWorkers  int
}

func NewOrchestrator(s store.Store, g geocoder.Geocoder, v *verifier.Engine, maxWorkers int) *Orchestrator {
	if maxWorkers <= 0 {
		maxWorkers = 5
	}
	return &Orchestrator{
		store:       s,
		geocoder:    g,
		verifier:    v,
		rateLimiter: NewRateLimiterRegistry(),
		maxWorkers:  maxWorkers,
	}
}

func (o *Orchestrator) Register(s Scraper) {
	o.scrapers = append(o.scrapers, s)
}

func (o *Orchestrator) ScrapeRegion(ctx context.Context, req ScrapeRequest) ([]model.Sighting, error) {
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		sightings []model.Sighting
		sem       = make(chan struct{}, o.maxWorkers)
	)

	for _, s := range o.scrapers {
		if !s.Supports(req.Region) {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}

		go func(scraper Scraper) {
			defer wg.Done()
			defer func() { <-sem }()

			_ = o.rateLimiter.Wait(ctx, scraper.Name())
			res, err := scraper.Scrape(ctx, req)
			if err != nil || res == nil {
				return
			}

			for i := range res.Sightings {
				if (res.Sightings[i].Lat == 0 || res.Sightings[i].Lng == 0) && o.geocoder != nil {
					if geo, err := o.geocoder.Geocode(ctx, res.Sightings[i].RawAddress); err == nil && geo != nil {
						res.Sightings[i].Lat = geo.Lat
						res.Sightings[i].Lng = geo.Lng
					}
				}
				if o.verifier != nil {
					_ = o.verifier.ProcessSighting(ctx, res.Sightings[i])
				}
			}

			_ = o.store.SaveSightings(ctx, scraper.Name(), res.Sightings)

			mu.Lock()
			sightings = append(sightings, res.Sightings...)
			mu.Unlock()
		}(s)
	}

	wg.Wait()
	return sightings, nil
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `rtk go test ./internal/scraper/... -v`
Expected: PASS

- [ ] **Step 8: Commit OSM scraper and orchestrator**

Run:
```bash
rtk git add internal/scraper/
rtk git commit -m "feat(scraper): implement scraper interfaces, rate limiter, orchestrator, and OSM scraper"
```

---

### Task 8: Tech Park (Config-Driven), JustDial & Google Places Scrapers

**Files:**
- Create: `config/techparks.yaml`
- Create: `internal/scraper/sources/techpark.go`
- Create: `internal/scraper/sources/justdial.go`
- Create: `internal/scraper/sources/google.go`
- Create: `internal/scraper/sources/techpark_test.go`
- Create: `internal/scraper/sources/testdata/techpark_itpb.html`

**Interfaces:**
- Produces: `sources.NewTechParkScraper`, `sources.NewJustDialScraper`, `sources.NewGooglePlacesScraper`

- [ ] **Step 1: Create sample tech park config and fixture HTML**

File: `config/techparks.yaml`
```yaml
parks:
  - id: itpb
    name: "ITPB Whitefield"
    url: "https://example.com/mock-itpb-tenants"
    region: "Bangalore"
    lat: 12.9854
    lng: 77.7366
    selectors:
      company_list: ".tenant-card"
      name: ".company-name"
      address: ".company-address"
```

File: `internal/scraper/sources/testdata/techpark_itpb.html`
```html
<div class="tenant-directory">
  <div class="tenant-card">
    <h3 class="company-name">Xerox Business Services</h3>
    <span class="company-address">Creator Building, 2nd Floor, ITPB</span>
  </div>
  <div class="tenant-card">
    <h3 class="company-name">Tata Elxsi</h3>
    <span class="company-address">Pioneer Building, ITPB</span>
  </div>
</div>
```

- [ ] **Step 2: Write failing test for Tech Park scraping**

File: `internal/scraper/sources/techpark_test.go`
```go
package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/stretchr/testify/assert"
)

func TestTechParkScraper(t *testing.T) {
	fixture, err := os.ReadFile("testdata/techpark_itpb.html")
	assert.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(fixture)
	}))
	defer srv.Close()

	cfg := TechParkConfig{
		ID:     "itpb",
		Name:   "ITPB",
		URL:    srv.URL,
		Region: "Bangalore",
		Lat:    12.9854,
		Lng:    77.7366,
		Selectors: TechParkSelectors{
			CompanyList: ".tenant-card",
			Name:        ".company-name",
			Address:     ".company-address",
		},
	}

	tpScraper := NewTechParkScraper([]TechParkConfig{cfg})
	res, err := tpScraper.Scrape(context.Background(), scraper.ScrapeRequest{Region: "Bangalore"})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, res.Sightings, 2)
	assert.Equal(t, "Xerox Business Services", res.Sightings[0].CompanyName)
	assert.Equal(t, 12.9854, res.Sightings[0].Lat)
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `rtk go test ./internal/scraper/sources/techpark_test.go -v`
Expected: FAIL ("undefined: TechParkConfig")

- [ ] **Step 4: Implement Tech Park scraper (with YAML loader)**

File: `internal/scraper/sources/techpark.go`
```go
package sources

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
	"gopkg.in/yaml.v3"
)

type TechParkSelectors struct {
	CompanyList string `yaml:"company_list"`
	Name        string `yaml:"name"`
	Address     string `yaml:"address"`
}

type TechParkConfig struct {
	ID        string            `yaml:"id"`
	Name      string            `yaml:"name"`
	URL       string            `yaml:"url"`
	Region    string            `yaml:"region"`
	Lat       float64           `yaml:"lat"`
	Lng       float64           `yaml:"lng"`
	Selectors TechParkSelectors `yaml:"selectors"`
}

type techParksYAML struct {
	Parks []TechParkConfig `yaml:"parks"`
}

type TechParkScraper struct {
	parks  []TechParkConfig
	client *http.Client
}

func LoadTechParksFromFile(filePath string) ([]TechParkConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var y techParksYAML
	if err := yaml.Unmarshal(data, &y); err != nil {
		return nil, err
	}
	return y.Parks, nil
}

func NewTechParkScraper(parks []TechParkConfig) *TechParkScraper {
	return &TechParkScraper{
		parks:  parks,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *TechParkScraper) Name() string {
	return "techpark"
}

func (t *TechParkScraper) Supports(region string) bool {
	for _, p := range t.parks {
		if strings.EqualFold(p.Region, region) {
			return true
		}
	}
	return false
}

func (t *TechParkScraper) Scrape(ctx context.Context, req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	var sightings []model.Sighting

	for _, park := range t.parks {
		if !strings.EqualFold(park.Region, req.Region) {
			continue
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, park.URL, nil)
		if err != nil {
			continue
		}
		httpReq.Header.Set("User-Agent", "Mozilla/5.0 (NearHive Company Research Bot)")

		resp, err := t.client.Do(httpReq)
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		doc.Find(park.Selectors.CompanyList).Each(func(i int, s *goquery.Selection) {
			name := strings.TrimSpace(s.Find(park.Selectors.Name).Text())
			if name == "" {
				return
			}
			addr := strings.TrimSpace(s.Find(park.Selectors.Address).Text())
			if addr == "" {
				addr = park.Name + ", " + park.Region
			}

			sightings = append(sightings, model.Sighting{
				Source:      t.Name(),
				SourceURL:   &park.URL,
				CompanyName: name,
				RawAddress:  addr,
				Lat:         park.Lat,
				Lng:         park.Lng,
				Metadata: model.JSONMap{
					"tech_park": park.Name,
				},
				ScrapedAt: time.Now(),
			})
		})
	}

	return &scraper.ScrapeResult{Sightings: sightings}, nil
}
```

- [ ] **Step 5: Implement JustDial and Google Places Scrapers**

File: `internal/scraper/sources/justdial.go`
```go
package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
)

type JustDialScraper struct {
	client *http.Client
}

func NewJustDialScraper() *JustDialScraper {
	return &JustDialScraper{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (j *JustDialScraper) Name() string {
	return "justdial"
}

func (j *JustDialScraper) Supports(region string) bool {
	return true
}

func (j *JustDialScraper) Scrape(ctx context.Context, req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	targetURL := fmt.Sprintf("https://www.justdial.com/%s/Software-Companies", url.PathEscape(req.Region))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := j.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &scraper.ScrapeResult{}, nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var sightings []model.Sighting
	doc.Find(".jsx-result-card, .resultbox").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find(".resultbox_title, .store-name").Text())
		if name == "" {
			return
		}
		addr := strings.TrimSpace(s.Find(".address-info, .resultbox_address").Text())
		sightings = append(sightings, model.Sighting{
			Source:      j.Name(),
			CompanyName: name,
			RawAddress:  addr + ", " + req.Region,
			ScrapedAt:   time.Now(),
		})
	})

	return &scraper.ScrapeResult{Sightings: sightings}, nil
}
```

File: `internal/scraper/sources/google.go`
```go
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
)

type GooglePlacesScraper struct {
	apiKey string
	client *http.Client
}

func NewGooglePlacesScraper(apiKey string) *GooglePlacesScraper {
	return &GooglePlacesScraper{
		apiKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (g *GooglePlacesScraper) Name() string {
	return "google"
}

func (g *GooglePlacesScraper) Supports(region string) bool {
	return g.apiKey != ""
}

type placesResponse struct {
	Results []struct {
		Name     string `json:"name"`
		Vicinity string `json:"vicinity"`
		Geometry struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
	Status string `json:"status"`
}

func (g *GooglePlacesScraper) Scrape(ctx context.Context, req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	if g.apiKey == "" {
		return &scraper.ScrapeResult{}, nil
	}

	radiusMeters := int(req.RadiusKM * 1000)
	if radiusMeters <= 0 {
		radiusMeters = 15000
	}

	apiURL := fmt.Sprintf("https://maps.googleapis.com/maps/api/place/nearbysearch/json?location=%f,%f&radius=%d&type=point_of_interest&keyword=%s&key=%s",
		req.Lat, req.Lng, radiusMeters, url.QueryEscape("software technology company"), g.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data placesResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var sightings []model.Sighting
	for _, p := range data.Results {
		sightings = append(sightings, model.Sighting{
			Source:      g.Name(),
			CompanyName: p.Name,
			RawAddress:  p.Vicinity,
			Lat:         p.Geometry.Location.Lat,
			Lng:         p.Geometry.Location.Lng,
			ScrapedAt:   time.Now(),
		})
	}

	return &scraper.ScrapeResult{Sightings: sightings}, nil
}
```

- [ ] **Step 6: Run tests to verify all scrapers compile and pass tests**

Run: `rtk go test ./internal/scraper/sources/... -v`
Expected: PASS

- [ ] **Step 7: Commit scraper modules**

Run:
```bash
rtk git add config/techparks.yaml internal/scraper/sources/
rtk git commit -m "feat(scrapers): add TechPark config-driven crawler, JustDial parser, and Google Places source"
```

---

### Task 9: REST API & Middlewares

**Files:**
- Create: `internal/api/response.go`
- Create: `internal/api/middleware.go`
- Create: `internal/api/handlers_auth.go`
- Create: `internal/api/handlers_search.go`
- Create: `internal/api/handlers_company.go`
- Create: `internal/api/handlers_jobs.go`
- Create: `internal/api/router.go`
- Create: `internal/api/api_test.go`

**Interfaces:**
- Consumes: `store.Store`, `auth.Manager`, `scraper.Orchestrator`
- Produces: `api.NewRouter(store, authMgr, orchestrator, secret)`

- [ ] **Step 1: Write failing test for Auth & Search API endpoints**

File: `internal/api/api_test.go`
```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/auth"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() (http.Handler, *store.MockStore, *auth.Manager) {
	mockStore := store.NewMockStore()
	authMgr := auth.NewManager("test-jwt-secret-very-long-enough-32bytes", 24*time.Hour)
	router := NewRouter(mockStore, authMgr, nil, "test-jwt-secret-very-long-enough-32bytes")
	return router, mockStore, authMgr
}

func TestAuthRegisterAndLogin(t *testing.T) {
	router, _, _ := setupTestRouter()

	// 1. Register
	regBody, _ := json.Marshal(map[string]string{
		"email":    "founder@nearhive.com",
		"password": "Password123!",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var regResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &regResp)
	assert.NotEmpty(t, regResp["token"])

	// 2. Login
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "founder@nearhive.com",
		"password": "Password123!",
	})
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var loginResp map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &loginResp)
	assert.NotEmpty(t, loginResp["token"])
}

func TestSearch_RequiresAuth(t *testing.T) {
	router, mockStore, authMgr := setupTestRouter()

	// Unauthenticated request should fail with 401
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/search?lat=12.9716&lng=77.5946&radius=15", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Authenticated request
	token, _ := authMgr.GenerateToken(uuid.New())
	// Seed one company
	cID := uuid.New()
	_ = mockStore.CreateCompany(nil, &model.Company{ID: cID, Name: "Infosys"})
	_ = mockStore.CreateLocation(nil, &model.Location{CompanyID: cID, Lat: 12.9716, Lng: 77.5946, Confidence: 0.9})

	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/search?lat=12.9716&lng=77.5946&radius=15", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var searchResp map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &searchResp)
	companies := searchResp["companies"].([]any)
	assert.Len(t, companies, 1)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/api/... -v`
Expected: FAIL ("undefined: NewRouter")

- [ ] **Step 3: Implement JSON response helpers & middlewares**

File: `internal/api/response.go`
```go
package api

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func JSONError(w http.ResponseWriter, status int, message string, code string, details any) {
	JSON(w, status, ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	})
}
```

File: `internal/api/middleware.go`
```go
package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/auth"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func AuthMiddleware(mgr *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				JSONError(w, http.StatusUnauthorized, "missing authorization header", "UNAUTHORIZED", nil)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := mgr.ValidateToken(tokenStr)
			if err != nil {
				JSONError(w, http.StatusUnauthorized, "invalid or expired token", "UNAUTHORIZED", nil)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserIDFromContext(ctx context.Context) uuid.UUID {
	val, ok := ctx.Value(UserIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return val
}
```

- [ ] **Step 4: Implement Auth, Search, Company, and Job Handlers**

File: `internal/api/handlers_auth.go`
```go
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/auth"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

type AuthHandler struct {
	store   store.UserStore
	authMgr *auth.Manager
}

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body", "VALIDATION_ERROR", nil)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || len(req.Password) < 6 {
		JSONError(w, http.StatusBadRequest, "email required and password must be >= 6 chars", "VALIDATION_ERROR", nil)
		return
	}

	hashed, err := auth.HashPassword(req.Password)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to process password", "INTERNAL_ERROR", nil)
		return
	}

	user := &model.User{
		ID:       uuid.New(),
		Email:    req.Email,
		Password: hashed,
	}

	if err := h.store.CreateUser(r.Context(), user); err != nil {
		JSONError(w, http.StatusConflict, "user already exists", "CONFLICT", nil)
		return
	}

	token, err := h.authMgr.GenerateToken(user.ID)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to generate token", "INTERNAL_ERROR", nil)
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"token": token,
		"user": map[string]any{
			"id":    user.ID,
			"email": user.Email,
		},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body", "VALIDATION_ERROR", nil)
		return
	}

	user, err := h.store.GetUserByEmail(r.Context(), strings.ToLower(req.Email))
	if err != nil || !auth.CheckPasswordHash(req.Password, user.Password) {
		JSONError(w, http.StatusUnauthorized, "invalid email or password", "UNAUTHORIZED", nil)
		return
	}

	token, err := h.authMgr.GenerateToken(user.ID)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to generate token", "INTERNAL_ERROR", nil)
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user": map[string]any{
			"id":    user.ID,
			"email": user.Email,
		},
	})
}
```

File: `internal/api/handlers_search.go`
```go
package api

import (
	"net/http"
	"strconv"

	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

type SearchHandler struct {
	store   store.Store
	history store.SearchHistoryStore
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")
	if latStr == "" || lngStr == "" {
		JSONError(w, http.StatusBadRequest, "lat and lng query parameters are required", "VALIDATION_ERROR", nil)
		return
	}

	lat, err1 := strconv.ParseFloat(latStr, 64)
	lng, err2 := strconv.ParseFloat(lngStr, 64)
	if err1 != nil || err2 != nil {
		JSONError(w, http.StatusBadRequest, "lat and lng must be valid numbers", "VALIDATION_ERROR", nil)
		return
	}

	radiusKM := 15.0
	if rStr := r.URL.Query().Get("radius"); rStr != "" {
		if rVal, err := strconv.ParseFloat(rStr, 64); err == nil && rVal > 0 && rVal <= 100 {
			radiusKM = rVal
		}
	}

	var opts store.SearchOpts
	if confStr := r.URL.Query().Get("min_confidence"); confStr != "" {
		if confVal, err := strconv.ParseFloat(confStr, 64); err == nil {
			opts.MinConfidence = &confVal
		}
	}
	if ind := r.URL.Query().Get("industry"); ind != "" {
		opts.Industry = &ind
	}
	if q := r.URL.Query().Get("q"); q != "" {
		opts.Query = &q
	}

	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if lVal, err := strconv.Atoi(lStr); err == nil && lVal > 0 && lVal <= 100 {
			limit = lVal
		}
	}
	opts.Limit = limit

	page := 1
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if pVal, err := strconv.Atoi(pStr); err == nil && pVal > 0 {
			page = pVal
		}
	}
	opts.Offset = (page - 1) * limit

	results, err := h.store.Search(r.Context(), lat, lng, radiusKM*1000, opts)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to query companies", "INTERNAL_ERROR", nil)
		return
	}

	total, _ := h.store.CountSearch(r.Context(), lat, lng, radiusKM*1000, opts)

	// Record search history for logged-in user
	userID := GetUserIDFromContext(r.Context())
	if userID != "" && h.history != nil {
		_ = h.history.RecordSearch(r.Context(), &model.SearchHistory{
			UserID:      userID,
			QueryLat:    lat,
			QueryLng:    lng,
			RadiusKM:    radiusKM,
			ResultCount: len(results),
		})
	}

	JSON(w, http.StatusOK, map[string]any{
		"meta": map[string]any{
			"total":     total,
			"page":      page,
			"limit":     limit,
			"radius_km": radiusKM,
			"center": map[string]float64{
				"lat": lat,
				"lng": lng,
			},
		},
		"companies": results,
	})
}
```

File: `internal/api/handlers_company.go`
```go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/store"
)

type CompanyHandler struct {
	store store.Store
}

func (h *CompanyHandler) GetCompany(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "invalid company id", "VALIDATION_ERROR", nil)
		return
	}

	company, err := h.store.GetCompanyByID(r.Context(), id)
	if err != nil {
		JSONError(w, http.StatusNotFound, "company not found", "NOT_FOUND", nil)
		return
	}

	locations, _ := h.store.GetLocationsByCompany(r.Context(), id)

	JSON(w, http.StatusOK, map[string]any{
		"company":   company,
		"locations": locations,
	})
}

func (h *CompanyHandler) GetSightings(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "invalid company id", "VALIDATION_ERROR", nil)
		return
	}

	sightings, err := h.store.GetSightingsByCompany(r.Context(), id)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to get sightings", "INTERNAL_ERROR", nil)
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"sightings": sightings,
	})
}
```

File: `internal/api/handlers_jobs.go`
```go
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/sonukumar/nearhive/internal/store"
)

type JobHandler struct {
	store        store.JobStore
	orchestrator *scraper.Orchestrator
}

type TriggerJobRequest struct {
	Region   string  `json:"region"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	RadiusKM float64 `json:"radius_km"`
}

func (h *JobHandler) TriggerJob(w http.ResponseWriter, r *http.Request) {
	var req TriggerJobRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Region == "" {
		req.Region = "Bangalore"
	}
	if req.RadiusKM <= 0 {
		req.RadiusKM = 15
	}

	job := &model.ScrapeJob{
		ID:        uuid.New(),
		Source:    "manual_trigger",
		Status:    "running",
		Region:    &req.Region,
		StartedAt: func() *time.Time { t := time.Now(); return &t }(),
	}
	_ = h.store.CreateJob(r.Context(), job)

	if h.orchestrator != nil {
		go func() {
			sightings, err := h.orchestrator.ScrapeRegion(r.Context(), scraper.ScrapeRequest{
				Region:   req.Region,
				Lat:      req.Lat,
				Lng:      req.Lng,
				RadiusKM: req.RadiusKM,
			})
			now := time.Now()
			job.FinishedAt = &now
			if err != nil {
				job.Status = "failed"
				errStr := err.Error()
				job.Error = &errStr
			} else {
				job.Status = "done"
				job.Sightings = len(sightings)
			}
			_ = h.store.UpdateJob(r.Context(), job)
		}()
	}

	JSON(w, http.StatusAccepted, job)
}

func (h *JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.store.ListJobs(r.Context(), 20, 0)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to list jobs", "INTERNAL_ERROR", nil)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "invalid job id", "VALIDATION_ERROR", nil)
		return
	}
	job, err := h.store.GetJobByID(r.Context(), id)
	if err != nil {
		JSONError(w, http.StatusNotFound, "job not found", "NOT_FOUND", nil)
		return
	}
	JSON(w, http.StatusOK, job)
}
```

File: `internal/api/router.go`
```go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sonukumar/nearhive/internal/auth"
	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/sonukumar/nearhive/internal/store"
)

func NewRouter(s store.Store, authMgr *auth.Manager, orchestrator *scraper.Orchestrator, jwtSecret string) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	authHandler := &AuthHandler{store: s, authMgr: authMgr}
	searchHandler := &SearchHandler{store: s, history: s}
	companyHandler := &CompanyHandler{store: s}
	jobHandler := &JobHandler{store: s, orchestrator: orchestrator}

	// Public health routes
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"ready": "true"})
	})

	// Public Auth endpoints
	r.Post("/api/v1/auth/register", authHandler.Register)
	r.Post("/api/v1/auth/login", authHandler.Login)

	// Protected routes
	r.Group(func(protected chi.Router) {
		protected.Use(AuthMiddleware(authMgr))

		protected.Get("/api/v1/search", searchHandler.Search)
		protected.Get("/api/v1/companies/{id}", companyHandler.GetCompany)
		protected.Get("/api/v1/companies/{id}/sightings", companyHandler.GetSightings)
		protected.Get("/api/v1/jobs", jobHandler.ListJobs)
		protected.Post("/api/v1/jobs/trigger", jobHandler.TriggerJob)
		protected.Get("/api/v1/jobs/{id}", jobHandler.GetJob)
	})

	return r
}
```

- [ ] **Step 5: Run API tests to verify all endpoints pass**

Run: `rtk go test ./internal/api/... -v`
Expected: PASS

- [ ] **Step 6: Commit REST API implementation**

Run:
```bash
rtk git add internal/api/
rtk git commit -m "feat(api): implement Chi router, JWT auth handlers, spatial search, company & job endpoints"
```

---

### Task 10: Scheduler & Background Workers

**Files:**
- Create: `internal/scheduler/scheduler.go`
- Create: `internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: `scraper.Orchestrator`, `store.JobStore`
- Produces: `scheduler.NewScheduler(orchestrator, regions, interval)`, `scheduler.Start()`, `scheduler.Stop()`

- [ ] **Step 1: Write failing test for background scheduler**

File: `internal/scheduler/scheduler_test.go`
```go
package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type dummyRunner struct {
	runs int32
}

func (d *dummyRunner) Run(ctx context.Context) error {
	atomic.AddInt32(&d.runs, 1)
	return nil
}

func TestScheduler_ExecutesInterval(t *testing.T) {
	d := &dummyRunner{}
	s := NewScheduler(10*time.Millisecond, d.Run)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(35 * time.Millisecond)
	s.Stop()

	assert.GreaterOrEqual(t, atomic.LoadInt32(&d.runs), int32(2))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/scheduler/... -v`
Expected: FAIL ("undefined: NewScheduler")

- [ ] **Step 3: Implement Scheduler**

File: `internal/scheduler/scheduler.go`
```go
package scheduler

import (
	"context"
	"sync"
	"time"
)

type JobFunc func(ctx context.Context) error

type Scheduler struct {
	interval time.Duration
	job      JobFunc
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func NewScheduler(interval time.Duration, job JobFunc) *Scheduler {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &Scheduler{
		interval: interval,
		job:      job,
		stopCh:   make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-ticker.C:
				if s.job != nil {
					_ = s.job(ctx)
				}
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}
```

- [ ] **Step 4: Run tests to verify it passes**

Run: `rtk go test ./internal/scheduler/... -v`
Expected: PASS

- [ ] **Step 5: Commit scheduler**

Run:
```bash
rtk git add internal/scheduler/
rtk git commit -m "feat(scheduler): implement robust background periodic scrape job scheduler"
```

---

### Task 11: CLI Commands & Wiring

**Files:**
- Create: `cmd/nearhive/main.go`

**Interfaces:**
- Produces: CLI root command with subcommands:
  - `nearhive serve`
  - `nearhive migrate`
  - `nearhive scrape`
  - `nearhive user create`

- [ ] **Step 1: Implement root command and dependency wiring**

File: `cmd/nearhive/main.go`
```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sonukumar/nearhive/internal/api"
	"github.com/sonukumar/nearhive/internal/auth"
	"github.com/sonukumar/nearhive/internal/config"
	"github.com/sonukumar/nearhive/internal/geocoder"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scheduler"
	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/sonukumar/nearhive/internal/scraper/sources"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/sonukumar/nearhive/internal/verifier"
)

var rootCmd = &cobra.Command{
	Use:   "nearhive",
	Short: "NearHive 🐝 — Scalable tech company locator and scraper",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(scrapeCmd())
	rootCmd.AddCommand(userCmd())
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the NearHive REST API and background worker",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("failed to load config: %v", err)
			}

			dbStore, err := store.NewPostgresStore(cfg.DatabaseURL)
			if err != nil {
				log.Fatalf("failed to connect to database: %v", err)
			}
			defer dbStore.Close()

			// Geocoders
			nom := geocoder.NewNominatim(cfg.NominatimURL, nil)
			var googleGeo *geocoder.Google
			if cfg.GoogleGeoAPIKey != "" {
				googleGeo = geocoder.NewGoogle("", cfg.GoogleGeoAPIKey, nil)
			}
			geo := geocoder.NewFallbackGeocoder(nom, googleGeo)

			// Verifier
			verifEngine := verifier.NewEngine(dbStore, geo)

			// Scrapers & Orchestrator
			orchestrator := scraper.NewOrchestrator(dbStore, geo, verifEngine, cfg.MaxScraperWorkers)
			orchestrator.Register(sources.NewOSMScraper())
			orchestrator.Register(sources.NewJustDialScraper())
			if cfg.GooglePlacesKey != "" {
				orchestrator.Register(sources.NewGooglePlacesScraper(cfg.GooglePlacesKey))
			}
			if parks, err := sources.LoadTechParksFromFile("config/techparks.yaml"); err == nil {
				orchestrator.Register(sources.NewTechParkScraper(parks))
			}

			// Auth Manager
			authMgr := auth.NewManager(cfg.JWTSecret, 24*time.Hour)

			// Background Scheduler (daily crawl)
			sched := scheduler.NewScheduler(24*time.Hour, func(ctx context.Context) error {
				for _, r := range cfg.ScrapeRegions {
					_, _ = orchestrator.ScrapeRegion(ctx, scraper.ScrapeRequest{Region: r, RadiusKM: 20})
				}
				return nil
			})
			schedCtx, schedCancel := context.WithCancel(context.Background())
			defer schedCancel()
			sched.Start(schedCtx)

			// HTTP Server
			router := api.NewRouter(dbStore, authMgr, orchestrator, cfg.JWTSecret)
			srv := &http.Server{
				Addr:         ":" + cfg.Port,
				Handler:      router,
				ReadTimeout:  15 * time.Second,
				WriteTimeout: 15 * time.Second,
			}

			go func() {
				log.Printf("🐝 NearHive API server listening on :%s", cfg.Port)
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("listen failed: %v", err)
				}
			}()

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
			<-stop

			log.Println("Shutting down NearHive...")
			sched.Stop()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
		},
	}
}

func scrapeCmd() *cobra.Command {
	var region string
	var radius float64

	cmd := &cobra.Command{
		Use:   "scrape",
		Short: "Run a one-off scrape for a specific region",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("config error: %v", err)
			}
			dbStore, err := store.NewPostgresStore(cfg.DatabaseURL)
			if err != nil {
				log.Fatalf("database error: %v", err)
			}
			defer dbStore.Close()

			nom := geocoder.NewNominatim(cfg.NominatimURL, nil)
			verifEngine := verifier.NewEngine(dbStore, nom)
			orchestrator := scraper.NewOrchestrator(dbStore, nom, verifEngine, cfg.MaxScraperWorkers)
			orchestrator.Register(sources.NewOSMScraper())
			orchestrator.Register(sources.NewJustDialScraper())

			log.Printf("Starting scrape for region: %s (radius: %.1f km)...", region, radius)
			sightings, err := orchestrator.ScrapeRegion(context.Background(), scraper.ScrapeRequest{
				Region:   region,
				RadiusKM: radius,
			})
			if err != nil {
				log.Fatalf("scrape failed: %v", err)
			}
			log.Printf("Scrape complete! Captured %d sightings.", len(sightings))
		},
	}
	cmd.Flags().StringVarP(&region, "region", "r", "Bangalore", "Region/City name")
	cmd.Flags().Float64VarP(&radius, "radius", "d", 15.0, "Radius in kilometers")
	return cmd
}

func userCmd() *cobra.Command {
	userRoot := &cobra.Command{Use: "user", Short: "User management commands"}
	var email, password string

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user account",
		Run: func(cmd *cobra.Command, args []string) {
			if email == "" || password == "" {
				log.Fatal("both --email and --password are required")
			}
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("config error: %v", err)
			}
			dbStore, err := store.NewPostgresStore(cfg.DatabaseURL)
			if err != nil {
				log.Fatalf("database error: %v", err)
			}
			defer dbStore.Close()

			hashed, err := auth.HashPassword(password)
			if err != nil {
				log.Fatalf("hash error: %v", err)
			}

			user := &model.User{
				ID:       uuid.New(),
				Email:    email,
				Password: hashed,
			}
			if err := dbStore.CreateUser(context.Background(), user); err != nil {
				log.Fatalf("create user failed: %v", err)
			}
			fmt.Printf("User created successfully: %s (ID: %s)\n", user.Email, user.ID)
		},
	}
	createCmd.Flags().StringVarP(&email, "email", "e", "", "User email")
	createCmd.Flags().StringVarP(&password, "password", "p", "", "User password")
	userRoot.AddCommand(createCmd)
	return userRoot
}
```

- [ ] **Step 2: Build binary to verify clean compilation**

Run: `rtk go build -o bin/nearhive ./cmd/nearhive`
Expected: PASS (produces `bin/nearhive` executable)

- [ ] **Step 3: Run help command on built binary**

Run: `bin/nearhive --help`
Expected: Displays NearHive commands (`serve`, `scrape`, `user`).

- [ ] **Step 4: Commit CLI entrypoint**

Run:
```bash
rtk git add cmd/nearhive/
rtk git commit -m "feat(cli): wire Cobra commands for serve, scrape, and user administration"
```

---

### Task 12: Deployment & Containerization Artifacts

**Files:**
- Create: `Dockerfile`
- Create: `railway.toml`
- Create: `.github/workflows/ci.yml`
- Create: `README.md`
- Create: `LICENSE`

**Interfaces:**
- Produces: Production Docker container, Railway deployment descriptor, GitHub Actions CI configuration, complete documentation.

- [ ] **Step 1: Create multi-stage Dockerfile**

File: `Dockerfile`
```dockerfile
# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o nearhive ./cmd/nearhive

# Final runtime image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /build/nearhive /usr/local/bin/nearhive
COPY --from=builder /build/migrations /app/migrations
COPY --from=builder /build/config /app/config

EXPOSE 8080

ENTRYPOINT ["nearhive"]
CMD ["serve"]
```

- [ ] **Step 2: Create Railway deployment configuration**

File: `railway.toml`
```toml
[build]
builder = "DOCKERFILE"
dockerfilePath = "./Dockerfile"

[deploy]
startCommand = "nearhive serve"
healthcheckPath = "/health"
healthcheckTimeout = 30
restartPolicyType = "ON_FAILURE"
restartPolicyMaxRetries = 3

[service]
internalPort = 8080
```

- [ ] **Step 3: Create GitHub Actions CI workflow**

File: `.github/workflows/ci.yml`
```yaml
name: CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    name: Test & Build
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true

      - name: Run unit tests
        run: go test ./... -v -race

      - name: Build binary
        run: go build -v ./cmd/nearhive
```

- [ ] **Step 4: Create LICENSE (MIT) and comprehensive README.md**

File: `LICENSE`
```text
MIT License

Copyright (c) 2026 Sonu Kumar

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

File: `README.md`
```markdown
# NearHive 🐝

> High-performance, modular tech company discovery and verification engine built in Go.

NearHive scrapes multiple independent sources (OpenStreetMap, Tech Park directories, Google Places, JustDial), normalizes corporate names, cross-verifies office locations, and provides spatial radius queries with sub-millisecond PostGIS accuracy.

## Features

- 📍 **Multi-Source Scraping**: Scrapes OSM Overpass API, config-driven tech park portals, JustDial, and Google Places.
- 🔍 **Verification & Deduplication**: 3-stage pipeline (name normalization, fuzzy/domain matching, and spatial clustering).
- 🧭 **High-Performance Spatial Search**: Powered by PostgreSQL + PostGIS `ST_DWithin` spatial indices.
- 🔐 **Secure & Private**: JWT authentication, bcrypt passwords, and user-scoped private search queries.
- 📦 **Minimal Footprint**: Single binary, compiles to ~15MB Docker image, fits easily in Railway or Heroku hobby tier.

## Getting Started

### Prerequisites

- Go 1.23+
- PostgreSQL 16+ with PostGIS 3.4 & pg_trgm extensions enabled

### Setup & Run

1. Clone the repository and install dependencies:
```bash
git clone https://github.com/sonukumar/nearhive.git
cd nearhive
go mod download
```

2. Copy `.env.example` to `.env` and configure your database:
```bash
cp .env.example .env
```

3. Build and run:
```bash
go build -o nearhive ./cmd/nearhive
./nearhive serve
```

## API Overview

- `POST /api/v1/auth/register` — Register a new account
- `POST /api/v1/auth/login` — Login and receive JWT token
- `GET /api/v1/search?lat=12.9716&lng=77.5946&radius=15` — Find companies within radius (km)
- `GET /api/v1/companies/:id` — Detailed company profile & verified branch locations
- `POST /api/v1/jobs/trigger` — Trigger scraping job for a region

## License

MIT
```

- [ ] **Step 5: Run complete test suite and build check**

Run: `rtk go test ./... -v`
Run: `rtk go build -o bin/nearhive ./cmd/nearhive`
Expected: ALL PASS

- [ ] **Step 6: Commit deployment artifacts**

Run:
```bash
rtk git add Dockerfile railway.toml .github/ README.md LICENSE
rtk git commit -m "chore(deploy): add multi-stage Dockerfile, Railway config, CI workflow, and documentation"
```
