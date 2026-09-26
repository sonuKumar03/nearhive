.PHONY: help up dev down stop restart logs logs-app logs-db logs-web clean clean-volume ps build test local-run

# Default target
all: help

help: ## Show this help message
	@echo "🐝 NearHive Local Development Commands:"
	@echo ""
	@echo "  make up          Start full app (React Web + Go API + PostGIS DB) in background"
	@echo "  make dev         Alias for 'make up' with live rebuild"
	@echo "  make down        Stop all running containers"
	@echo "  make stop        Alias for 'make down'"
	@echo "  make restart     Restart all services"
	@echo "  make logs        Stream logs from all services"
	@echo "  make logs-web    Stream logs from the Next.js React frontend"
	@echo "  make logs-app    Stream logs from the Go NearHive application"
	@echo "  make logs-crawler Stream logs from the standalone crawler daemon"
	@echo "  make logs-db     Stream logs from PostGIS database"
	@echo "  make ps          Show container status"
	@echo "  make clean       Stop containers and WIPE all database volumes (fresh start)"
	@echo "  make build       Rebuild Docker images"
	@echo "  make test        Run Go unit and integration tests"
	@echo "  make local-run   Build and run NearHive binary natively (outside Docker)"
	@echo ""

up: ## Start containers in background
	docker compose up --build -d
	@echo ""
	@echo "🐝 NearHive stack is booting up!"
	@echo "👉 React Frontend: http://localhost:3000"
	@echo "👉 Backend API:    http://localhost:8080"
	@echo "👉 Health Check:   http://localhost:8080/health"
	@echo "👉 Run 'make logs' to watch startup output"

dev: up ## Start development environment

down: ## Stop containers
	docker compose down

stop: down ## Alias for down

restart: ## Restart containers
	docker compose restart

logs: ## Follow all container logs
	docker compose logs -f

logs-web: ## Follow web frontend logs
	docker compose logs -f web

logs-app: ## Follow application logs
	docker compose logs -f app

logs-crawler: ## Follow standalone crawler logs
	docker compose logs -f crawler

logs-db: ## Follow database logs
	docker compose logs -f db

ps: ## View running containers
	docker compose ps

clean: ## Stop and wipe database volume for a clean slate
	docker compose down -v
	@echo "🧹 Cleaned up containers and wiped database volume 'nearhive_pgdata'."

clean-volume: clean ## Alias for clean

build: ## Rebuild docker images
	docker compose build

test: ## Run unit and package tests
	go test -v ./...

local-run: ## Build and run locally with native Go
	go build -o bin/nearhive ./cmd/nearhive
	./bin/nearhive serve
