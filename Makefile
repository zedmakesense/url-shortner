include .env
export

MIGRATE_DSN = postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

.PHONY: dev-up dev-down dev-down-force dev-logs dev-migrate-up dev-migrate-down dev-migrate-version \
	prod-up prod-down prod-down-force prod-logs prod-migrate-up prod-migrate-down prod-migrate-version \
	help run lint

.DEFAULT_GOAL := help

dev-up:
	docker compose -f dev-compose.yml up --build -d

run:
	cd backend && air

lint:
	cd backend && golangci-lint run ./...

dev-down:
	docker compose -f dev-compose.yml down

dev-down-force:
	docker compose -f dev-compose.yml down -v

dev-logs:
	docker compose -f dev-compose.yml logs

dev-migrate-up:
	docker compose -f dev-compose.yml run --rm migrate -path=/migrations -database="$(MIGRATE_DSN)" up

dev-migrate-down:
	docker compose -f dev-compose.yml run --rm migrate -path=/migrations -database="$(MIGRATE_DSN)" down 1

dev-migrate-version:
	docker compose -f dev-compose.yml run --rm migrate -path=/migrations -database="$(MIGRATE_DSN)" version

prod-up:
	docker compose -f prod-compose.yml up --build -d

prod-down:
	docker compose -f prod-compose.yml down

prod-down-force:
	docker compose -f prod-compose.yml down -v

prod-logs:
	docker compose -f prod-compose.yml logs -f app

prod-migrate-up:
	docker compose -f prod-compose.yml run --rm migrate -path=/migrations -database="$(MIGRATE_DSN)" up

prod-migrate-down:
	docker compose -f prod-compose.yml run --rm migrate -path=/migrations -database="$(MIGRATE_DSN)" down 1

prod-migrate-version:
	docker compose -f prod-compose.yml run --rm migrate -path=/migrations -database="$(MIGRATE_DSN)" version

help:
	@echo "=== Dev Environment ==="
	@echo "  make dev-up               - Start dev containers (builds if needed)"
	@echo "  make run                  - Start the go api server"
	@echo "  make dev-down             - Stop dev containers"
	@echo "  make dev-down-force       - Stop dev containers and remove volumes"
	@echo "  make dev-logs             - Stream dev container logs"
	@echo "  make dev-migrate-up       - Run pending migrations"
	@echo "  make dev-migrate-down     - Rollback last migration"
	@echo "  make dev-migrate-version  - Show current migration version"
	@echo ""
	@echo "=== Prod Environment ==="
	@echo "  make prod-up              - Start prod containers (builds if needed)"
	@echo "  make prod-down            - Stop prod containers"
	@echo "  make prod-down-force      - Stop prod containers and remove volumes"
	@echo "  make prod-logs            - Stream prod container logs"
	@echo "  make prod-migrate-up      - Run pending migrations"
	@echo "  make prod-migrate-down    - Rollback last migration"
	@echo "  make prod-migrate-version - Show current migration version"
