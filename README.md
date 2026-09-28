# NearHive

NearHive discovers nearby tech companies and recent technical jobs. Python owns discovery, its PostgreSQL schema, and all scraped-data writes. Go serves authentication and read APIs. The Next.js web app uses both APIs through same-origin rewrites.

## Local setup

```sh
make up
make ps
```

The web app runs at `http://localhost:3000`, Go at `http://localhost:8080`, Python discovery at `http://localhost:8090`, and PostGIS at `localhost:5432`. On a new volume, Compose initializes the database from [`python-discovery/schema.sql`](python-discovery/schema.sql). Go uses a database role with read access to discovery tables and write access to `users`; Python uses the writer connection.

To reset disposable local data and reinitialize the Python schema:

```sh
make clean
make up
```

## API ownership

| Service | Routes | Responsibility |
| --- | --- | --- |
| Go | `/api/v1/auth/*`, `/api/v1/search*`, `/api/v1/companies/*` | Accounts and reads |
| Python | `/api/v1/discovery/jobs*` | Create, list, inspect, and cancel discovery runs |

Discovery jobs accept `lat`, `lng`, and `radius_km`. Their client status is `queued`, `in_progress`, `completed`, `failed`, or `cancelled`. The Python worker claims runs from PostgreSQL, gathers company and job evidence, and writes the canonical tables directly. Company offices and job locations are distinct, so job evidence does not imply a nearby office.

The Go crawler, scrape command, scheduled crawl, scrape routes, and batch ingestion endpoint are retired. Configure Python sources in [`config/python_sources.yaml`](config/python_sources.yaml). See [`python-discovery/README.md`](python-discovery/README.md) for worker development and the isolated test database.
