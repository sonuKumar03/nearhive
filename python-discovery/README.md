# NearHive Python Discovery

Python discovery worker, operations API, and canonical writes for NearHive
company and technical job discovery.

## Local development

From the repository root, run `make up` and check the services with `make ps`.
The discovery operations API listens on `http://localhost:8090`. It accepts
the same bearer JWT as the Go login API:

- `POST /api/v1/discovery/jobs` with `{"lat": 12.97, "lng": 77.59, "radius_km": 10}`
- `GET /api/v1/discovery/jobs`
- `GET /api/v1/discovery/jobs/{id}`
- `POST /api/v1/discovery/jobs/{id}/cancel`

`GET /health` is unauthenticated and used by the platform health probe.

The worker writes validated evidence directly to PostgreSQL. Company offices
and job locations are stored separately so a job location does not make the
company nearby.

Create a separate test database once from the Python discovery schema:

```sh
docker compose exec -T db createdb -U postgres nearhive_test
docker compose exec -T db psql -U postgres -v ON_ERROR_STOP=1 -d nearhive_test < python-discovery/schema.sql
```

In `python-discovery/`, use the project environment and test database:

```sh
uv sync --locked --extra dev
NEARHIVE_TEST_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/nearhive_test \
  .venv/bin/python -m pytest -q tests/test_persistence.py
NEARHIVE_TEST_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/nearhive_test \
  .venv/bin/python -m pytest -q tests/test_operations_api.py
```

The local Compose database uses the credentials above. Keep checks that write
records on the isolated `nearhive_test` database.
