.PHONY: install-tools check-env gen database
 
install-tools:
	@if [ ! $$(which go) ]; then \
		echo "golang not found."; \
		echo "Try installing Go..."; \
		exit 1; \
	fi
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@if [ ! $$(which migrate) ]; then \
		echo "The 'migrate' command was not found in your path. You most likely need to add \$$$${HOME}/go/bin to your PATH."; \
		exit 1; \
	fi

run: check-env database
	go run ./cmd/api
 
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
 