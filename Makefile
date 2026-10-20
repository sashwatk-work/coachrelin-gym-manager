SHELL := /bin/bash

.PHONY: help db-up db-down db-reset migrate seed api web test test-go test-go-db test-web build up down production-config production-up production-down

REPO_ROOT := $(CURDIR)
ENV_FILE ?= $(REPO_ROOT)/docker/.env
PRODUCTION_ENV_FILE ?= $(REPO_ROOT)/docker/.env.production
PRODUCTION_PROJECT ?= coachrelin-production

DATABASE_URL ?= postgres://gym:gym@127.0.0.1:5432/gym?sslmode=disable
JWT_SECRET   ?= dev-secret-change-me
PORT         ?= 8080

-include $(ENV_FILE)

export DATABASE_URL JWT_SECRET PORT

LOCAL_COMPOSE = docker compose --env-file "$(ENV_FILE)" -f docker/compose.yaml
PRODUCTION_COMPOSE = docker compose --project-name "$(PRODUCTION_PROJECT)" --env-file "$(PRODUCTION_ENV_FILE)" -f docker/compose.yaml -f docker/compose.production.yaml

help:
	@echo "Targets:"
	@echo "  db-up               Start Postgres in Docker"
	@echo "  db-down             Stop Docker services"
	@echo "  db-reset            Stop Docker services and delete local Postgres files"
	@echo "  migrate             Run DB migrations"
	@echo "  seed                Optional sample members for Coach Relin Fitness"
	@echo "  api                 Run the Go API (port $(PORT))"
	@echo "  web                 Run the Vite dev server (port 5173)"
	@echo "  up                  Build and run postgres + backend + frontend in Docker"
	@echo "  down                Stop that stack"
	@echo "  test                Run Go unit tests + frontend typecheck/build"
	@echo "  test-go-db          HTTP integration + RLS tests (needs db-up)"
	@echo "  production-config   Validate docker/compose.production.yaml (needs docker/.env.production)"
	@echo "  production-up       Start Nginx, frontend, backend, and private PostgreSQL"
	@echo "  production-down     Stop production without deleting data"

db-up:
	@test -f "$(ENV_FILE)" || { echo "Copy docker/.env.example to docker/.env"; exit 1; }
	$(LOCAL_COMPOSE) up -d postgres
	@echo "Waiting for Postgres..."
	@until $(LOCAL_COMPOSE) exec -T postgres pg_isready -U postgres -d gym >/dev/null 2>&1; do sleep 1; done
	@echo "Postgres is ready."

db-down:
	$(LOCAL_COMPOSE) down

db-reset:
	$(LOCAL_COMPOSE) down
	rm -rf "$(REPO_ROOT)/data/postgres"

migrate:
	cd backend && go run ./cmd/api -migrate

seed:
	cd backend && go run ./cmd/api -seed

api:
	cd backend && go run ./cmd/api

web:
	cd frontend && npm run dev

test: test-go test-web

test-go:
	cd backend && go test ./...

# Integration tests (HTTP handlers against a live Postgres + RLS isolation
# checks) are skipped unless TEST_DATABASE_URL is set. They create throwaway
# tenants and delete them when done.
test-go-db:
	cd backend && TEST_DATABASE_URL="$(DATABASE_URL)" go test -count=1 ./...

test-web:
	cd frontend && npm run typecheck && npm run build

build:
	cd backend && go build -o bin/api ./cmd/api
	cd frontend && npm run build

up:
	@test -f "$(ENV_FILE)" || { echo "Copy docker/.env.example to docker/.env"; exit 1; }
	$(LOCAL_COMPOSE) up -d --build

down:
	$(LOCAL_COMPOSE) down

production-config:
	@test -f "$(PRODUCTION_ENV_FILE)" || { echo "Copy docker/.env.production.example to docker/.env.production"; exit 1; }
	$(PRODUCTION_COMPOSE) config --quiet

production-up: production-config
	$(PRODUCTION_COMPOSE) up -d --build --remove-orphans postgres backend frontend nginx

production-down:
	@test -f "$(PRODUCTION_ENV_FILE)" || { echo "Copy docker/.env.production.example to docker/.env.production"; exit 1; }
	$(PRODUCTION_COMPOSE) down --remove-orphans
