# NearHive 🐝

> High-performance, modular tech company discovery and verification engine built in Go.

NearHive scrapes multiple independent sources (OpenStreetMap, Tech Park directories, Google Places, JustDial), normalizes corporate names, cross-verifies office locations, and provides spatial radius queries with sub-millisecond PostGIS accuracy.

## Features

- 📍 **Multi-Source Scraping**: Scrapes OSM Overpass API, config-driven tech park portals, JustDial, and Google Places.
- 🔍 **Verification & Deduplication**: 3-stage pipeline (name normalization, fuzzy/domain matching, and spatial clustering).
- 🧭 **High-Performance Spatial Search**: Powered by PostgreSQL + PostGIS `ST_DWithin` spatial indices.
- 🔐 **Secure & Private**: JWT authentication, bcrypt passwords, and user-scoped private search queries.
- 📦 **Minimal Footprint**: Single binary, compiles to ~15MB Docker image, fits easily in Railway or Heroku hobby tier.

## Live Deployment

- **Base URL**: `https://nearhive-production.up.railway.app`
- **Health Check**: `GET https://nearhive-production.up.railway.app/health`

## Quick Start (Local Development)

The easiest way to run NearHive locally is via Docker and Make — no Go or PostgreSQL installation required:

```bash
# 1. Start full app & PostGIS database in background
make up

# 2. View live logs
make logs-app

# 3. Stop containers when done
make down

# 4. Stop and wipe database volume for a clean slate
make clean
```

The app will be available at:
- **Web App / Dashboard**: [http://localhost:8080](http://localhost:8080)
- **Health Check**: [http://localhost:8080/health](http://localhost:8080/health)
- **PostGIS Database**: `localhost:5432` (`postgres:postgres`)

### Manual Native Setup (Without Docker)

1. Ensure Go 1.23+ and PostgreSQL with PostGIS are installed.
2. Copy configuration:
```bash
cp .env.example .env
```
3. Run natively:
```bash
make local-run
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
