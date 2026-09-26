# Standalone Crawler Service Implementation Plan

**Date:** 2026-09-26  
**Status:** In Progress  
**Spec Reference:** [`docs/superpowers/specs/2026-09-26-standalone-crawler-service-infra-spec.md`](../specs/2026-09-26-standalone-crawler-service-infra-spec.md)  
**Branch:** `feature/standalone-crawler-service`  

---

## Architecture Overview

Split NearHive from a monolithic API+scraper binary into two decoupled Go services communicating via PostgreSQL:
1. **`nearhive-api`** (`cmd/nearhive`): Lightweight REST API serving spatial search, clustering, authentication, and job dispatching.
2. **`nearhive-crawler`** (`cmd/crawler`): Standalone worker daemon consuming jobs with `FOR UPDATE SKIP LOCKED`, executing scraping pipelines, listening to real-time `LISTEN / NOTIFY` cancellations, and persisting verified sightings.

---

## Tasks Breakdown

- [x] **Task 1: Database Migration for Queue & Coordinates**
  - Create `migrations/000010_crawler_queue.up.sql` and `.down.sql`:
    - Add `lat FLOAT8`, `lng FLOAT8`, `radius_km FLOAT8`, `worker_id VARCHAR(100)`, `last_heartbeat_at TIMESTAMPTZ`, `attempts INT DEFAULT 0` to `scrape_jobs`.
    - Add index `idx_scrape_jobs_queue ON scrape_jobs(status, created_at ASC) WHERE status = 'pending'`.
  - Update `internal/model/models.go` (`ScrapeJob` struct).
  - Update `internal/store/postgres_job.go` (and `mock_store.go`) to persist and retrieve new fields.

- [x] **Task 2: PostgreSQL Queue & IPC Engine (`internal/queue`)**
  - Implement `internal/queue/queue.go`:
    - `DequeueNextJob(ctx context.Context, workerID string) (*model.ScrapeJob, error)` using `FOR UPDATE SKIP LOCKED`.
    - `SendCancelNotification(ctx context.Context, jobID uuid.UUID) error` using `pg_notify`.
    - `ListenForCancelNotifications(ctx context.Context, onCancel func(jobID uuid.UUID)) error` using `pq.Listener` or `pgx/v5/pgconn`.
    - `Heartbeat(ctx context.Context, jobID uuid.UUID, workerID string) error`.
  - Unit tests for queue logic in `internal/queue/queue_test.go`.

- [x] **Task 3: Standalone Crawler Worker Daemon (`internal/crawler`)**
  - Implement `internal/crawler/worker.go`:
    - Long-running worker loop polling for jobs or triggered by notifications.
    - Concurrent scraper execution via `Orchestrator`.
    - Live cancellation subscription via PostgreSQL listener.
    - Periodic heartbeat updater to detect dead workers.

- [x] **Task 4: Standalone Crawler Entrypoint (`cmd/crawler/main.go`)**
  - Wire database, geocoder, rate limiters, scrapers (OSM, Wikidata, TechParks, Google Places, JustDial), and verifier.
  - Listen for OS signals (`SIGINT`, `SIGTERM`) for graceful draining.

- [x] **Task 5: Update API Server & Handlers**
  - In `internal/api/handlers_jobs.go`:
    - `TriggerJob`: Enqueues job with `status = 'pending'`, stores `lat`, `lng`, `radius_km`.
    - `CancelJob`: Calls queue `SendCancelNotification` for immediate cross-process cancellation.
  - In `cmd/nearhive/main.go`:
    - Support running in API-only mode when `crawler` is separate.

- [x] **Task 6: Docker & Compose Infrastructure**
  - Create `Dockerfile.api`.
  - Create `Dockerfile.crawler`.
  - Update `docker-compose.yml` to define 4 services: `db`, `api`, `crawler`, `web`.
  - Update `Makefile` with `make run-api` and `make run-crawler`.

- [x] **Task 7: End-to-End Verification**
  - Run all Go unit & integration tests (`rtk go test ./...`).
  - Run Next.js production build (`rtk pnpm --dir web build`).
  - Verify Docker compose builds.
