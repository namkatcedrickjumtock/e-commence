# Tiny e-commerce backend (net/http) — layered errors demo

This repo is intentionally small and built for a 10–15 minute conference demo.

The point is **layered architecture and error ownership**:

> Errors are defined where they originate and propagated upward without noisy wrapping.

## Layers

```txt
/cmd
/internal
  /api            HTTP (presentation)
  /application    use-cases (orchestration)
  /domain         entities + business errors + ports
  /infrastructure in-memory storage + fake payment provider
```

`sqlc` integration is also included under `internal/domain/sql/` + `sqlc.yaml` to demonstrate a production-style persistence boundary.

## Running

```bash
go run ./cmd/server
```

Environment:
- `DEMO_PAYMENT_MODE`: `ok` (default), `decline`, `timeout`

## Endpoints

### Add product to cart

```bash
curl -sS -X POST localhost:8080/cart/items \
  -H 'content-type: application/json' \
  -d '{"product_id":"p_1","quantity":1}'
```

### Checkout

```bash
curl -sS -X POST localhost:8080/checkout
```

To demo payment timeout:

```bash
DEMO_PAYMENT_MODE=timeout go run ./cmd/server
```

## Error responses (shape)

Errors are returned as:

```json
{ "error": "product out of stock" }
```

The exact message is the **layer-owned error contract** (e.g. `domain.ErrProductOutOfStock`).

## sqlc (optional, for persistence demo)

`sqlc` isn’t a runtime dependency; it only generates Go code.

Install `sqlc`:

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

Generate:

```bash
sqlc generate
```

Inputs:
- `internal/domain/sql/schema.sql`
- `internal/domain/sql/queries.sql`
- `sqlc.yaml`

Output (configured):
- `internal/domain/db`

### Notes (stdlib-only runtime)

This demo intentionally keeps the runtime **standard-library only**. That means:

- The `sqlc`-generated code will compile (it uses `database/sql`), but
- you still need a real database driver (third-party import) to actually connect at runtime.

For the conference demo, the app runs against the in-memory infrastructure store, and `sqlc` exists to showcase the **persistence boundary + error translation** approach.

