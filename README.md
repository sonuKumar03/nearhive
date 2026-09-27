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
# 1. Start full app (React Web + Go Backend API + PostGIS DB + Discovery Worker) in background
make up

# 2. View live logs
make logs           # All services
make logs-web       # Next.js React frontend
make logs-app       # Go NearHive application
make logs-discovery # Python company discovery worker
make logs-crawler   # Go crawler daemon

# 3. Run tests
make test           # Go unit and integration tests
make test-discovery # Python discovery worker end-to-end suite

# 4. Stop containers when done
make down

# 5. Stop and wipe database volume for a clean slate
make clean
```

The stack will be available at:
- **React Frontend**: [http://localhost:3000](http://localhost:3000)
- **Go Backend API**: [http://localhost:8080](http://localhost:8080)
- **Health Check**: [http://localhost:8080/health](http://localhost:8080/health)
- **PostGIS Database**: `localhost:5432` (`postgres:postgres`)

### Manual Native Setup (Without Docker)

1. Ensure Go 1.23+, Python 3.12+, `uv`, and PostgreSQL with PostGIS are installed.
2. Copy configuration:
```bash
cp .env.example .env
```
3. Run backend natively:
```bash
make local-run
```
4. Run discovery worker natively:
```bash
cd python-discovery
uv run nearhive-discovery
```

## Python Company Discovery Worker

NearHive features a dedicated Python discovery stack (`python-discovery/`) running alongside the Go core API.

### Difference Between Legacy Go Crawler and Python Discovery Worker

- **Legacy Go Crawler (`cmd/crawler`)**: Scheduled, periodic batch scraping daemon. Crawls broad geospatial regions via OpenStreetMap (OSM Overpass), JustDial, and configured tech parks (`config/techparks.yaml`).
- **Python Discovery Worker (`nearhive_discovery`)**: Reactive, on-demand queue processor. Listens to `discovery_jobs` claimed via PostgreSQL row-level locks, crawls target company websites, extracts Schema.org JSON-LD microdata, fetches public ATS job postings (Greenhouse, Lever), executes client-side JavaScript via headless Playwright, and streams canonical evidence batches back to the Go API (`POST /api/v1/internal/discovery/batches`).

### Configuration

The Python discovery worker is configured via environment variables and YAML:
- `DATABASE_URL`: PostgreSQL connection string with PostGIS support.
- `NEARHIVE_API_URL`: Go API base URL (e.g. `http://app:8080` in Docker, `http://localhost:8080` locally).
- `DISCOVERY_WORKER_TOKEN` / `NEARHIVE_WORKER_TOKEN`: Shared secret token for authenticating batch ingestion.
- `PLAYWRIGHT_CONTEXTS`: Concurrency limit for headless browser contexts (default `2`).
- `config/python_sources.yaml`: Declarative CSS/XPath selector definitions for public directories.

### Adding a Compliant Adapter

Custom discovery adapters implement the `BaseSourceAdapter` protocol:

```python
from collections.abc import Iterable
from nearhive_discovery.contracts import DiscoveryJob, EvidenceBatch, CompanyEvidence
from nearhive_discovery.http import validate_public_url
from nearhive_discovery.sources.base import BaseSourceAdapter

class CustomDirectoryAdapter(BaseSourceAdapter):
    name = "custom_directory"
    source_family = "public_directory"  # canonical family for presence evaluation

    def run(self, job: DiscoveryJob) -> Iterable[EvidenceBatch]:
        # 1. Enforce strict SSRF protection before any HTTP fetch
        normalized = validate_public_url("https://example.com/directory")
        
        # 2. Extract and emit canonical evidence batches
        yield EvidenceBatch(
            contract_version=1,
            discovery_job_id=str(job.id),
            source=self.name,
            source_family=self.source_family,
            companies=[CompanyEvidence(name="Acme Tech", domain="acme.example.com")],
            jobs=[],
        )
```

Compliance requirements:
- **SSRF Defense**: All external URLs must be validated with `validate_public_url()`.
- **Bounded Resource Ceilings**: Strict pagination limits, max crawl depths, and request timeouts.
- **Canonical Ingestion**: Evidence must use canonical contracts (`CompanyEvidence`, `TechnicalJobEvidence`).

### Source Fixtures & Offline Testing

To protect third-party services and guarantee deterministic CI execution:
- Static HTML/JSON fixtures are stored in `python-discovery/tests/fixtures/`.
- Local fake servers (`python-discovery/tests/fake_site/`) provide in-memory HTTP endpoints for HTML, JSON-LD, ATS, and Playwright SPA rendering during tests.
- Local and CI test suites run with zero live network calls.

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
- `POST /api/v1/discovery/jobs` — Create a reactive company discovery scan job
- `GET /api/v1/discovery/jobs/:id` — Retrieve discovery job progress and statistics
- `POST /api/v1/discovery/jobs/:id/cancel` — Cancel an in-progress discovery job
- `POST /api/v1/internal/discovery/batches` — Worker evidence batch ingestion endpoint
- `GET /api/v1/search?lat=12.9716&lng=77.5946&radius=15` — Find companies within radius (km) with presence types
- `GET /api/v1/companies/:id` — Detailed company profile & verified branch locations
- `GET /api/v1/companies/:id/sightings` — Raw scrape sightings audit trail
- `GET /api/v1/companies/:id/technical-jobs` — Active and recent technical job postings
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
