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

## CLI Usage

```bash
# Start API server and background scheduler
./nearhive serve

# Execute a one-off scrape for a region
./nearhive scrape --region=Bangalore --radius=20

# Create an initial user account
./nearhive user create --email=admin@nearhive.com --password=SecretPassword123!
```

## API Overview

- `POST /api/v1/auth/register` — Register a new account
- `POST /api/v1/auth/login` — Login and receive JWT token
- `GET /api/v1/search?lat=12.9716&lng=77.5946&radius=15` — Find companies within radius (km)
- `GET /api/v1/companies/:id` — Detailed company profile & verified branch locations
- `GET /api/v1/companies/:id/sightings` — Raw scrape sightings audit trail
- `POST /api/v1/jobs/trigger` — Trigger scraping job for a region
- `GET /health` — Health check endpoint

## Deployment

### Railway (Recommended)

1. Connect your repository to Railway.
2. Add the **PostgreSQL** service in Railway.
3. PostGIS is enabled automatically when running migrations.
4. Set required environment variables: `JWT_SECRET`, `PORT=8080`.
5. Deploy using the included `Dockerfile` and `railway.toml`.

## License

MIT
