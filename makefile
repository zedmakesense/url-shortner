include .env
export

ENV ?= dev
COMPOSE_FILE = $(ENV)-compose.yml
MIGRATE_DSN = postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

.PHONY: compose dev prod up down down-force logs migrate-up migrate-down migrate-version help

compose:
	@ENV=$$(echo '$(MAKECMDGOALS)' | grep -oE '^(dev|prod)' | head -1); \
	if [ -z "$$ENV" ]; then ENV="$(ENV)"; fi; \
	ACTIONS=$$(echo '$(MAKECMDGOALS)' | sed 's/dev\|prod//g'); \
	case "$$ACTIONS" in \
		*migrate-up*) GOAL=migrate-up ;; \
		*migrate-down*) GOAL=migrate-down ;; \
		*migrate-version*) GOAL=migrate-version ;; \
		*up*) GOAL=up ;; \
		*down-force*) GOAL=down-force ;; \
		*down*) GOAL=down ;; \
		*logs*) GOAL=logs ;; \
		*) GOAL=up ;; \
	esac; \
	case "$$GOAL" in \
		up) docker compose -f $${ENV}-compose.yml up --build -d ;; \
		down) docker compose -f $${ENV}-compose.yml down ;; \
		down-force) docker compose -f $${ENV}-compose.yml down -v ;; \
		logs) docker compose -f $${ENV}-compose.yml logs -f app ;; \
		migrate-up) docker compose -f $${ENV}-compose.yml run --rm migrate -path=/migrations -database="$(MIGRATE_DSN)" up ;; \
		migrate-down) docker compose -f $${ENV}-compose.yml run --rm migrate -path=/migrations -database="$(MIGRATE_DSN)" down 1 ;; \
		migrate-version) docker compose -f $${ENV}-compose.yml run --rm migrate -path=/migrations -database="$(MIGRATE_DSN)" version ;; \
	esac

dev prod up down down-force logs migrate-up migrate-down migrate-version: compose

help:
	@echo "make dev [up|down|down-force|logs]"
	@echo "make prod [up|down|down-force|logs]"
	@echo "make dev|prod migrate-up|migrate-down|migrate-version"
