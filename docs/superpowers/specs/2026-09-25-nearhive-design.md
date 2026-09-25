# NearHive — Design Specification

> **Project:** NearHive 🐝 — Multi-source tech company locator
> **Date:** 2026-09-25
> **Status:** Approved for implementation
> **License:** MIT (open source, self-hostable)

## 1. Purpose & Problem Statement

Google Maps is unreliable for discovering tech companies — many operate inside tech parks, coworking spaces, or shared hubs without individual map listings. NearHive solves this by scraping multiple data sources (OpenStreetMap, tech park tenant lists, business directories, government registrars, job portals), cross-referencing sightings, and producing a verified, confidence-scored database of tech companies within a configurable radius of any location.

### Success Criteria

- Given a lat/lng + radius, return tech companies with sub-second response times
- Cross-reference 4+ data sources to verify company locations
- Confidence scoring: users can filter by how reliably a location is verified
- Open source but self-hostable with private data per deployment
- Deployable on Railway/Heroku hobby tier (< 50MB RAM, single binary)

## 2. Scope

### In Scope (Phase 1)

- REST API with JWT authentication
- 4 scraper sources: OpenStreetMap, Tech Parks (config-driven), Google Places API, JustDial
- PostgreSQL + PostGIS for spatial queries
- 3-stage verification pipeline (normalize → merge & score → geo-verify)
- Scheduled background scraping (cron-based)
- Railway deployment (Dockerfile + Postgres addon)
- CLI subcommands: `serve`, `migrate`, `scrape`, `user`

### In Scope (Phase 2 — future)

- Additional scrapers: LinkedIn, Crunchbase, MCA/ROC, Naukri/Indeed
- Web dashboard with map visualization
- CLI client for terminal-based querying
- OAuth (Google/GitHub) authentication

### Out of Scope

- Real-time scraping (on-demand per request)
- Scraper proxy/rotation infrastructure
- Mobile apps
- Multi-tenant SaaS model

## 3. Tech Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Language | Go 1.23+ | Single binary, low memory (~20MB), goroutine-based concurrency for parallel scraping |
| HTTP Router | go-chi/chi v5 | Lightweight, stdlib-compatible, composable middleware |
| Database | PostgreSQL 16 + PostGIS 3.4 | `ST_DWithin` for radius queries on GIST spatial index, `pg_trgm` for fuzzy name matching |
| DB Access | jmoiron/sqlx | Struct scanning over `database/sql`, no ORM overhead |
| Migrations | golang-migrate/migrate v4 | SQL file-based, runs in code or standalone |
| JWT | golang-jwt/jwt v5 | De facto Go JWT library |
| Password Hashing | golang.org/x/crypto/bcrypt | Stdlib-adjacent, bcrypt cost 12 |
| Web Scraping | gocolly/colly v2 | Fast, clean API, built-in rate limiting and async |
| CLI | spf13/cobra | Industry standard Go CLI framework |
| CORS | rs/cors | Chi-compatible CORS middleware |
| Rate Limiting | golang.org/x/time/rate | Stdlib-adjacent, token bucket algorithm |
| Testing | stretchr/testify + testcontainers-go | Assertions + real PostGIS in integration tests |
| Deployment | Docker (multi-stage) on Railway | 15-20MB image, auto-deploy from GitHub |

## 4. Architecture

### 4.1 High-Level Overview

Single Go binary (modular monolith) with internal packages. All components compile into one executable. Communication is in-process via Go interfaces.

```
┌──────────────────────────────────────────────────────────┐
│                        NearHive                          │
│                                                          │
│  ┌──────────┐    ┌──────────────┐    ┌────────────────┐  │
│  │ REST API │    │  Scheduler   │    │  Verification  │  │
│  │ (Chi)    │    │  (cron)      │    │  Engine        │  │
│  └────┬─────┘    └──────┬───────┘    └───────┬────────┘  │
│       │                 │                    │            │
│       │          ┌──────▼───────┐            │            │
│       │          │   Scrape     │────────────┘            │
│       │          │ Orchestrator │                         │
│       │          └──────┬───────┘                         │
│       │                 │                                 │
│       │     ┌───────────┼───────────┐                    │
│       │   ┌─▼──┐   ┌───▼──┐   ┌───▼──┐                  │
│       │   │OSM │   │Tech  │   │Just  │  ...N scrapers    │
│       │   │    │   │Park  │   │Dial  │  (plugin iface)   │
│       │   └──┬─┘   └──┬───┘   └──┬───┘                   │
│       │      └─────────┼─────────┘                       │
│       │           ┌────▼─────┐                           │
│       │           │ Geocoder │ (Nominatim/Google)        │
│       │           └────┬─────┘                           │
│       └────────┬───────┘                                 │
│           ┌────▼──────────┐                              │
│           │  PostgreSQL   │                              │
│           │  + PostGIS    │                              │
│           └───────────────┘                              │
└──────────────────────────────────────────────────────────┘
```

### 4.2 Package Structure

```
nearhive/
├── cmd/
│   └── nearhive/
│       └── main.go                  # Entrypoint, dependency wiring, cobra commands
├── internal/
│   ├── api/
│   │   ├── router.go                # Chi router setup, route registration
│   │   ├── middleware.go            # Auth, rate-limit, CORS middleware
│   │   ├── handlers_auth.go        # POST /auth/register, /auth/login, /auth/refresh
│   │   ├── handlers_search.go      # GET /search, /search/suggestions
│   │   ├── handlers_company.go     # GET /companies/:id, /companies/:id/sightings
│   │   ├── handlers_jobs.go        # GET /jobs, POST /jobs/trigger, GET /jobs/:id
│   │   └── response.go             # JSON response helpers, error formatting
│   ├── auth/
│   │   ├── jwt.go                   # Token generation, validation, Claims struct
│   │   └── password.go             # bcrypt hash and compare
│   ├── config/
│   │   └── config.go               # Env-based config with defaults
│   ├── geocoder/
│   │   ├── geocoder.go             # Geocoder interface
│   │   ├── nominatim.go            # Nominatim (OSM) geocoder — primary
│   │   └── google.go               # Google Geocoding API — fallback
│   ├── model/
│   │   └── models.go               # Domain types: Company, Location, Sighting, User, etc.
│   ├── scheduler/
│   │   └── scheduler.go            # Cron-based job scheduling for periodic scrapes
│   ├── scraper/
│   │   ├── scraper.go              # Scraper interface, ScrapeRequest, ScrapeResult, Sighting
│   │   ├── orchestrator.go         # Fan-out to scrapers, geocode, persist
│   │   ├── ratelimit.go            # Per-source rate limiters
│   │   └── sources/
│   │       ├── osm.go              # OpenStreetMap Overpass API scraper
│   │       ├── techpark.go         # Config-driven tech park tenant scraper (Colly)
│   │       ├── google.go           # Google Places API scraper
│   │       └── justdial.go         # JustDial business directory scraper (Colly)
│   ├── store/
│   │   ├── store.go                # Repository interfaces (CompanyStore, LocationStore, etc.)
│   │   ├── postgres.go             # PostgreSQL implementation of all interfaces
│   │   └── queries.go              # Complex SQL: search, fuzzy matching, spatial
│   └── verifier/
│       ├── normalizer.go           # Company name normalization (suffix stripping, lowercasing)
│       ├── matcher.go              # Cross-source matching (domain → exact name → fuzzy)
│       ├── merger.go               # Sighting → company/location merging, confidence scoring
│       ├── geoverify.go            # Geocode missing coords, cluster nearby points
│       └── verifier.go             # Orchestrates the 3-stage verification pipeline
├── migrations/
│   ├── 000001_init_extensions.up.sql
│   ├── 000001_init_extensions.down.sql
│   ├── 000002_create_users.up.sql
│   ├── 000002_create_users.down.sql
│   ├── 000003_create_companies.up.sql
│   ├── 000003_create_companies.down.sql
│   ├── 000004_create_locations.up.sql
│   ├── 000004_create_locations.down.sql
│   ├── 000005_create_sightings.up.sql
│   ├── 000005_create_sightings.down.sql
│   ├── 000006_create_scrape_jobs.up.sql
│   ├── 000006_create_scrape_jobs.down.sql
│   ├── 000007_create_search_history.up.sql
│   └── 000007_create_search_history.down.sql
├── config/
│   └── techparks.yaml               # Tech park scraper configurations
├── Dockerfile
├── railway.toml
├── .github/
│   └── workflows/
│       └── ci.yml
├── go.mod
├── go.sum
├── .env.example
├── .gitignore
├── LICENSE
└── README.md
```

### 4.3 Dependency Flow

```
cmd/nearhive → internal/api → internal/store (interfaces)
                             → internal/auth
             → internal/scheduler → internal/scraper → internal/geocoder
                                                     → internal/store
             → internal/scraper/sources/* (implements scraper.Scraper)
             → internal/verifier → internal/store
                                 → internal/geocoder
             → internal/config
             → internal/model (used by all packages)
```

No circular dependencies. Every package depends on `model` for shared types and `store` interfaces for data access. Concrete implementations are wired in `cmd/nearhive/main.go`.

## 5. Data Model

### 5.1 PostgreSQL Schema

Required extensions: `postgis`, `pg_trgm`

#### users

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK, default `gen_random_uuid()` |
| email | VARCHAR(255) | UNIQUE, NOT NULL |
| password | VARCHAR(255) | NOT NULL (bcrypt hash) |
| created_at | TIMESTAMPTZ | DEFAULT NOW() |
| updated_at | TIMESTAMPTZ | DEFAULT NOW() |

#### companies

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK, default `gen_random_uuid()` |
| name | VARCHAR(500) | NOT NULL |
| normalized_name | VARCHAR(500) | NOT NULL, indexed, GIN trigram index |
| domain | VARCHAR(255) | indexed (dedup key) |
| industry | VARCHAR(255) | |
| employee_count | VARCHAR(50) | e.g. "50-200", "1000+" |
| description | TEXT | |
| verified | BOOLEAN | DEFAULT FALSE |
| created_at | TIMESTAMPTZ | DEFAULT NOW() |
| updated_at | TIMESTAMPTZ | DEFAULT NOW() |

#### locations

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK, default `gen_random_uuid()` |
| company_id | UUID | FK → companies(id) ON DELETE CASCADE |
| label | VARCHAR(100) | "HQ", "Branch", "Dev Center" |
| address | TEXT | NOT NULL |
| city | VARCHAR(255) | indexed |
| state | VARCHAR(255) | |
| country | VARCHAR(100) | DEFAULT 'IN' |
| pincode | VARCHAR(20) | |
| coords | GEOGRAPHY(POINT, 4326) | GIST indexed (spatial) |
| confidence | FLOAT | DEFAULT 0.0, range 0.0–1.0 |
| verified | BOOLEAN | DEFAULT FALSE |
| created_at | TIMESTAMPTZ | DEFAULT NOW() |
| updated_at | TIMESTAMPTZ | DEFAULT NOW() |

#### sightings

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK, default `gen_random_uuid()` |
| source | VARCHAR(100) | NOT NULL, indexed |
| source_url | TEXT | where we found this |
| company_name | VARCHAR(500) | NOT NULL, indexed |
| raw_address | TEXT | |
| lat | DOUBLE PRECISION | |
| lng | DOUBLE PRECISION | |
| metadata | JSONB | DEFAULT '{}', source-specific data |
| company_id | UUID | FK → companies(id), NULL until matched |
| location_id | UUID | FK → locations(id), NULL until matched |
| scraped_at | TIMESTAMPTZ | DEFAULT NOW() |

#### scrape_jobs

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| source | VARCHAR(100) | NOT NULL |
| status | VARCHAR(20) | 'pending', 'running', 'done', 'failed' |
| region | VARCHAR(255) | |
| sightings | INT | DEFAULT 0 |
| error | TEXT | |
| started_at | TIMESTAMPTZ | |
| finished_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | DEFAULT NOW() |

#### search_history

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| user_id | UUID | FK → users(id) ON DELETE CASCADE, indexed |
| query_lat | DOUBLE PRECISION | NOT NULL |
| query_lng | DOUBLE PRECISION | NOT NULL |
| radius_km | FLOAT | NOT NULL |
| result_count | INT | |
| searched_at | TIMESTAMPTZ | DEFAULT NOW() |

### 5.2 Key Design Decisions

- **`sightings` (raw) vs `companies`+`locations` (canonical):** Raw scraper output is immutable — never modified or deleted. The verifier merges sightings into canonical records. This provides a full audit trail and allows re-verification.
- **`GEOGRAPHY(POINT, 4326)`:** PostGIS geography type calculates distances in meters on a real sphere, not flat-earth approximation. Radius query: `ST_DWithin(coords, ST_MakePoint(lng, lat)::geography, radius_meters)`.
- **`confidence` score (0.0–1.0):** Composite score based on number of corroborating sources and source reliability weights. Allows API consumers to filter by verification strength.
- **`normalized_name`:** Enables deduplication across sources where the same company appears with different name variants ("Infosys Limited", "INFOSYS LTD", "infosys").
- **`domain` as dedup key:** Two sightings with the same website domain are almost certainly the same company, even with different name spellings.
- **`JSONB metadata` on sightings:** Each source returns different extra fields. JSONB keeps the schema clean while preserving source-specific data (LinkedIn: employee count, Crunchbase: funding round, MCA: CIN number).
- **User-scoped `search_history`:** Users only see their own search history. Company data itself is shared (scraped from public sources).

### 5.3 The Core Radius Query

```sql
SELECT c.id, c.name, c.domain, c.industry, c.employee_count,
       l.id AS location_id, l.label, l.address, l.city,
       l.confidence, l.verified,
       ST_Y(l.coords::geometry) AS lat,
       ST_X(l.coords::geometry) AS lng,
       ST_Distance(l.coords, ST_MakePoint($1, $2)::geography) AS distance_m
FROM locations l
JOIN companies c ON c.id = l.company_id
WHERE ST_DWithin(l.coords, ST_MakePoint($1, $2)::geography, $3)
  AND ($4::float IS NULL OR l.confidence >= $4)
  AND ($5::text IS NULL OR c.industry ILIKE '%' || $5 || '%')
  AND ($6::text IS NULL OR c.normalized_name % $6 OR c.name ILIKE '%' || $6 || '%')
ORDER BY distance_m
LIMIT $7 OFFSET $8;
```

Parameters: `$1`=lng, `$2`=lat, `$3`=radius_meters, `$4`=min_confidence, `$5`=industry, `$6`=name_query, `$7`=limit, `$8`=offset.

Uses GIST spatial index — sub-millisecond even with millions of rows.

## 6. Scraper Plugin System

### 6.1 Interface

```go
type Sighting struct {
    CompanyName string
    RawAddress  string
    Lat         float64           // 0 if unknown (geocoder fills later)
    Lng         float64
    SourceURL   string
    Metadata    map[string]any    // source-specific extras
}

type ScrapeRequest struct {
    Region   string
    Lat      float64
    Lng      float64
    RadiusKM float64
}

type ScrapeResult struct {
    Sightings []Sighting
    Errors    []error
}

type Scraper interface {
    Name() string
    Supports(region string) bool
    Scrape(ctx context.Context, req ScrapeRequest) (*ScrapeResult, error)
}
```

### 6.2 Orchestrator

The orchestrator fans out to all registered scrapers that support the target region. Goroutines are limited by a semaphore (`maxWorkers`, default 5). Each scraper result is geocoded (if missing coordinates) and persisted as raw sightings.

### 6.3 Phase 1 Scrapers

| Scraper | Source | Technique | Auth Required |
|---------|--------|-----------|--------------|
| `osm` | OpenStreetMap Overpass API | HTTP POST with Overpass QL | No |
| `techpark` | Tech park tenant pages | Colly + CSS selectors, config-driven via `config/techparks.yaml` | No |
| `google` | Google Places API | REST API with API key | Yes (API key) |
| `justdial` | JustDial business directory | Colly scraper | No |

### 6.4 Tech Park Config Format

Adding a new tech park requires zero code — just a YAML entry:

```yaml
# config/techparks.yaml
parks:
  - id: itpb
    name: "ITPB Whitefield"
    url: "https://www.itpb.com/tenants"
    region: "Bangalore"
    lat: 12.9854
    lng: 77.7366
    selectors:
      company_list: ".tenant-directory .card"
      name: ".card-title"
      address: ".card-body .address"
```

### 6.5 Rate Limiting

Each scraper source gets a per-source `rate.Limiter` (token bucket):

| Source | Rate | Rationale |
|--------|------|-----------|
| osm | 1 req / 2s | Overpass API fair use policy |
| techpark | 1 req / 3s | Polite crawling of static sites |
| google | 5 req / 1s burst | Paid API, higher budget |
| justdial | 1 req / 5s | Aggressive anti-scraping, be gentle |

## 7. Verification Engine

### 7.1 Three-Stage Pipeline

```
Raw Sightings → [Normalize & Match] → [Merge & Score] → [Geo-Verify] → Canonical Records
```

### 7.2 Stage 1: Normalize & Match

**Name normalization:** lowercase, strip common suffixes (`limited`, `ltd`, `pvt`, `private`, `inc`, `technologies`, `tech`, `solutions`, `software`, `systems`, `india`, `labs`, `group`), remove punctuation, collapse whitespace.

**Matching priority (highest to lowest):**

| Priority | Match Type | Confidence Boost | Example |
|----------|-----------|-----------------|---------|
| 1 | Exact domain match | +0.5 | Both sightings have `infosys.com` |
| 2 | Exact normalized name | +0.4 | `Normalize("Infosys Ltd") == Normalize("INFOSYS LIMITED")` |
| 3 | Fuzzy name (pg_trgm, similarity > 0.6) | +0.2 | `"wipro" ↔ "wipro technologies"` |
| 4 | Same address in same park | +0.3 | Both list "Tower B, ITPB Whitefield" |

Fuzzy matching uses PostgreSQL's `pg_trgm` extension with a GIN index — no external NLP dependency.

### 7.3 Stage 2: Merge & Score

When a sighting matches an existing company:
1. Check if its location is within 500m of an existing location for that company
2. If yes → boost existing location's confidence by `sourceWeight(source)`
3. If no → create a new location record (branch office)

**Source weights:**

| Source | Weight | Rationale |
|--------|--------|-----------|
| techpark | 0.40 | Official tenant list, very reliable |
| osm | 0.35 | Community-curated, generally accurate |
| google | 0.35 | Google Places data is solid |
| mca | 0.30 | Registered address, may be outdated |
| linkedin | 0.25 | Often city-level only |
| justdial | 0.25 | Decent but sometimes stale |
| crunchbase | 0.20 | Often only HQ city |
| jobportal | 0.15 | Weakest signal, inferred |

Confidence is capped at 1.0.

### 7.4 Stage 3: Geo-Verify

1. Geocode any locations with missing coordinates (address → lat/lng via Nominatim, fallback to Google)
2. Cluster locations for the same company within 500m → they're the same physical place → merge into highest-confidence record
3. No data is ever deleted — sightings are immutable, locations are soft-updated

### 7.5 Confidence Interpretation

| Score Range | Meaning |
|-------------|---------|
| 0.0–0.2 | Single weak source (one job portal mention) |
| 0.2–0.4 | Single decent source (one OSM or Google hit) |
| 0.4–0.6 | Multiple sources agree on city |
| 0.6–0.8 | Multiple sources agree on address |
| 0.8–1.0 | High confidence: 3+ sources agree, geocoded coordinates cluster |

### 7.6 Conflict Resolution

When sources disagree on location: both are kept as separate location records, each scored independently. The API returns both, ranked by confidence. No automatic deletion — the audit trail is always preserved.

## 8. API Design

### 8.1 Base URL & Auth

- Base: `/api/v1`
- Auth: Bearer JWT token in `Authorization` header
- Content-Type: `application/json`

### 8.2 Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/auth/register` | No | Create account (email + password) |
| `POST` | `/auth/login` | No | Returns JWT token |
| `POST` | `/auth/refresh` | Yes | Refresh expiring token |
| `GET` | `/search` | Yes | Core radius search |
| `GET` | `/search/suggestions` | Yes | Location name autocomplete |
| `GET` | `/companies/:id` | Yes | Full company detail + all locations |
| `GET` | `/companies/:id/sightings` | Yes | Raw sightings audit trail |
| `GET` | `/jobs` | Yes | List scrape job history |
| `POST` | `/jobs/trigger` | Yes | Manually trigger scrape for a region |
| `GET` | `/jobs/:id` | Yes | Job status + stats |
| `GET` | `/health` | No | Liveness check |
| `GET` | `/health/ready` | No | Readiness (DB + scrapers loaded) |

### 8.3 Search Endpoint

```
GET /api/v1/search?lat=12.9716&lng=77.5946&radius=15&min_confidence=0.3&page=1&limit=50
```

**Query params:**

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| lat | float | Yes | — | Center latitude |
| lng | float | Yes | — | Center longitude |
| radius | float | No | 15 | Radius in km (max 100) |
| min_confidence | float | No | 0.0 | Filter: 0.0–1.0 |
| industry | string | No | — | Industry keyword filter |
| q | string | No | — | Company name search (fuzzy) |
| page | int | No | 1 | Pagination page |
| limit | int | No | 50 | Results per page (max 100) |
| sort | string | No | distance | `distance`, `confidence`, `name` |

**Response shape:**

```json
{
  "meta": {
    "total": 142,
    "page": 1,
    "limit": 50,
    "center": { "lat": 12.9716, "lng": 77.5946 },
    "radius_km": 15
  },
  "companies": [
    {
      "id": "uuid",
      "name": "Company Name",
      "domain": "company.com",
      "industry": "IT Services",
      "employee_count": "1000+",
      "location": {
        "id": "uuid",
        "label": "HQ",
        "address": "Full address",
        "city": "Bangalore",
        "lat": 12.9854,
        "lng": 77.7366,
        "confidence": 0.85,
        "distance_km": 5.2,
        "sources": ["osm", "google", "techpark_itpb"],
        "verified": true
      }
    }
  ]
}
```

### 8.4 Error Response Format

```json
{
  "error": "descriptive message",
  "code": "VALIDATION_ERROR",
  "details": { "field": "radius", "message": "must be between 1 and 100 km" }
}
```

HTTP status codes: 200 (success), 201 (created), 400 (validation), 401 (unauthorized), 404 (not found), 429 (rate limited), 500 (server error).

## 9. Authentication & Security

### 9.1 Auth Flow

1. `POST /auth/register` — email + password → bcrypt hash (cost 12) → store in `users` table → return JWT
2. `POST /auth/login` — email + password → bcrypt compare → return JWT (24h expiry, HS256)
3. `POST /auth/refresh` — valid JWT → return new JWT with refreshed expiry
4. All protected endpoints check `Authorization: Bearer <token>` via middleware

### 9.2 JWT Claims

```go
type Claims struct {
    UserID uuid.UUID `json:"sub"`
    jwt.RegisteredClaims  // exp, iat
}
```

### 9.3 Security Measures

| Concern | Mitigation |
|---------|-----------|
| Passwords | bcrypt, cost 12 |
| JWT secret | `JWT_SECRET` env var, never hardcoded |
| Rate limiting | Per-IP: 10 req/min auth endpoints, 60 req/min search |
| SQL injection | Parameterized queries via sqlx |
| CORS | Configurable `ALLOWED_ORIGINS` env var |
| Search history | User-scoped via `user_id` in query |
| API keys | Environment variables, gitignored `.env` |

## 10. Geocoder

### 10.1 Interface

```go
type GeoResult struct {
    Lat     float64
    Lng     float64
    Address string  // formatted address from geocoder
}

type Geocoder interface {
    Geocode(ctx context.Context, address string) (*GeoResult, error)
    ReverseGeocode(ctx context.Context, lat, lng float64) (*GeoResult, error)
}
```

### 10.2 Implementations

- **Nominatim (primary):** Free, OSM-based, rate limited to 1 req/s. Good for Indian addresses.
- **Google Geocoding (fallback):** Requires API key, 200 free requests/day. Used when Nominatim returns no results.

Fallback logic: Nominatim first → if no result or error → Google → if no result → return error.

## 11. Scheduler

Cron-based scheduler using a goroutine with a ticker. Default schedule: daily at 3 AM (`0 3 * * *`). Configurable via `SCRAPE_SCHEDULE` env var.

For each configured region (`SCRAPE_REGIONS`):
1. Create a `scrape_jobs` record (status: pending)
2. Call `orchestrator.ScrapeRegion()` — fans out to all scrapers
3. Run verifier on new sightings
4. Update job record (status: done/failed, sightings count)

## 12. CLI Subcommands

```bash
nearhive serve                              # Start API server + scheduler
nearhive migrate up                         # Run pending migrations
nearhive migrate down                       # Rollback last migration
nearhive scrape --region=Bangalore --radius=20  # One-off scrape
nearhive user create --email=admin@x.com    # Create user interactively
```

## 13. Deployment

### 13.1 Dockerfile (multi-stage)

- Build stage: `golang:1.23-alpine`, compile with `CGO_ENABLED=0`, link flags `-s -w`
- Runtime stage: `alpine:3.20`, copy binary + migrations + config
- Final image: ~15-20MB

### 13.2 Railway (primary target)

- `railway.toml` with Dockerfile builder config
- PostgreSQL via Railway's one-click addon (auto-injects `DATABASE_URL`)
- Health check on `/health`
- Env vars set in Railway dashboard
- Auto-deploy on GitHub push

### 13.3 Heroku (alternative)

- `Procfile: web: nearhive serve`
- Heroku Postgres addon
- `DATABASE_URL` auto-injected

### 13.4 Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| PORT | No | 8080 | Server port |
| ENVIRONMENT | No | development | `development` or `production` |
| DATABASE_URL | Yes | — | PostgreSQL connection string |
| JWT_SECRET | Yes | — | JWT signing secret |
| NOMINATIM_URL | No | `https://nominatim.openstreetmap.org` | Nominatim base URL |
| GOOGLE_GEO_API_KEY | No | — | Google Geocoding API key |
| GOOGLE_PLACES_KEY | No | — | Google Places API key |
| MAX_SCRAPER_WORKERS | No | 5 | Max concurrent scraper goroutines |
| SCRAPE_SCHEDULE | No | `0 3 * * *` | Cron expression for scheduled scrapes |
| SCRAPE_REGIONS | No | `Bangalore` | Comma-separated region list |
| RATE_LIMIT_AUTH | No | 10 | Auth endpoint rate limit (req/min) |
| RATE_LIMIT_SEARCH | No | 60 | Search endpoint rate limit (req/min) |

## 14. Open Source + Data Privacy

| Layer | Public (repo) | Private (deployment) |
|-------|--------------|---------------------|
| Source code | ✅ MIT license on GitHub | — |
| Scraper configs (techparks.yaml) | ✅ Community-contributed | — |
| Database contents | — | 🔒 Your PostgreSQL instance |
| User accounts | — | 🔒 bcrypt hashed, JWT-gated |
| Search history | — | 🔒 Per-user scoped |
| API keys | — | 🔒 Environment variables |
| `.env.example` | ✅ Placeholder values | `.env` is gitignored |

Anyone can clone, configure, and self-host their own NearHive instance.

## 15. Testing Strategy

### 15.1 Test Pyramid

- **Unit tests (many, fast):** Normalizer, matcher, confidence scoring, JWT, handlers. Mocked store interfaces, no I/O.
- **Integration tests (moderate):** Store ↔ PostgreSQL via testcontainers-go with `postgis/postgis:16-3.4-alpine`. Verifier pipeline end-to-end with real DB.
- **Scraper tests (fixture-based):** Saved HTML/JSON responses in `testdata/` directories. HTTP test servers serve fixtures. No live website hits in CI.

### 15.2 CI

GitHub Actions: on push/PR, spin up PostGIS service container, run `go test ./... -v -race`, build binary.

## 16. Geographic Scope

**Phase 1:** India-focused. Initial `SCRAPE_REGIONS`: Bangalore, Pune, Hyderabad, Chennai, Gurgaon, Noida.
Tech park configs for major Indian tech parks (ITPB, Manyata, Hinjewadi, HITEC City, etc.).

**Future:** Extend globally by adding region configs and scrapers for other countries' business registries and tech park directories. The architecture is region-agnostic — scrapers declare which regions they `Support()`.
