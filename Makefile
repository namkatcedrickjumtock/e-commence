.PHONY: install-tools check-env run database migrate gen

DB_USER ?= adminuser
DB_PASS ?= postgres
DB_NAME ?= gophercon
DB_PORT ?= 5433

install-tools:
	@if [ ! $$(which go) ]; then \
		echo "golang not found. Install from https://go.dev/dl/"; \
		exit 1; \
	fi
	@if [ ! $$(which docker) ]; then \
		echo "docker not found. Install from https://docs.docker.com/get-docker/"; \
		exit 1; \
	fi
	@if [ ! $$(which jq) ]; then \
		echo "installing jq..."; \
		brew install jq; \
	fi
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

run: check-env database gen
	go run ./cmd

check-env:
	go mod tidy
	@if [ ! -f .env ]; then \
		echo ".env not found. Copying from .env.example..."; \
		cp .env.example .env; \
	fi

gen:
	sqlc generate

database:
	docker compose up -d

migrate:
	@echo "==> Applying schema..."
	@docker exec -i gophercon-demo psql -U $(DB_USER) -d $(DB_NAME) < db/schema.sql
	@echo "==> Schema applied."
