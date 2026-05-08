.PHONY: install-tools check-env run database

install-tools:
	@if [ ! $$(which go) ]; then \
		echo "golang not found."; \
		echo "Try installing Go..."; \
		exit 1; \
	fi

run: check-env
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
