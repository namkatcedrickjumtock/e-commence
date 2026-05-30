# e-commerce backend — GopherCon Europe 2026 Demo

**Talk:** Error Tracing: Lessons Learned From the Trenches
**Branch:** `v0.1` — Demo Part 1: The Bad Version

> This branch intentionally demonstrates poor error handling patterns in a layered architecture.
> Demo Part 2 (the structured version) lives on `main`.

## Layers

```
api/           HTTP handlers, request parsing, response serialization (presentation)
services/      business rules, use-case orchestration (business)
persistence/   PostgreSQL queries + Flutterwave payment mock (persistence)
cmd/           entry point, config, wiring
```

## Prerequisites

- Go 1.22+
- Docker
- jq (`brew install jq`)

## Quick start

```bash
# 1. Start PostgreSQL
docker compose up -d

# 2. Apply schema (once per fresh database)
make migrate

# 3. Copy env and run
cp .env.example .env
make run
```

The server listens on `:8080`.

---

## Demo — 4 bad error handling scenarios

### Setup (run once before any scenario)

```bash
curl -X POST localhost:8080/demo/reset
curl -X POST localhost:8080/demo/seed
```

---

### Scenario 1 — Excessive Wrapping

Every layer adds its own `fmt.Errorf` wrapper. The error chain becomes a paragraph.

```bash
curl localhost:8080/orders/o_ghost | jq .
```

**Expected response:**
```json
{
  "error": "handler: GET /orders/{id} failed: service layer: failed to retrieve order details: order lookup failed: repository: GetOrder query failed: order record not found in database: sql: no rows in result set"
}
```

Point to make: four layers, one missing row, zero useful signal.

---

### Scenario 2 — Abstraction Leakage

`services/services.go` and `api/http.go` both import `database/sql` and `github.com/lib/pq`.
Business logic branches on `sql.ErrNoRows` — an infrastructure detail.

```bash
curl localhost:8080/products/p_ghost | jq .
```

**Expected response:**
```json
{
  "error": "handler: GET /products/{id} failed: service: product not found in database catalog: sql query returned no rows: repository: GetProduct query failed: database error: sql: no rows in result set"
}
```

Point to make: `sql: no rows in result set` reached the HTTP client.
Open `services/services.go` and show the `import "database/sql"` line.

---

### Scenario 3 — Meaning Gets Reinterpreted

Same root cause as Scenario 2 (product doesn't exist) but through a different code path.
The service layer relabels `sql.ErrNoRows` as `"stock data unavailable"`.
`writeError` maps `"unavailable"` → **503**, not 404.

```bash
curl -i -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"p_ghost","quantity":1}'
```

**Expected response:**
```
HTTP/1.1 503 Service Unavailable

{
  "error": "handler: POST /cart/items failed: service: cart operation failed: inventory check failed: stock data unavailable: repository: GetProduct query failed: database error: sql: no rows in result set"
}
```

Point to make: the product just doesn't exist — this should be 404.
The meaning changed twice before reaching the client, and the wrong status was returned.

---

### Scenario 4 — External Provider Error Leakage

Raw `FlutterwaveError` fields — internal code, region, transaction ref, retryable flag —
reach the HTTP response body with no abstraction.

```bash
# Step 1 — set payment mode (no server restart needed)
curl -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"card_declined"}'

# Step 2 — add a product to cart (use the ID returned by /demo/seed)
curl -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"<id>","quantity":1}'

# Step 3 — checkout
curl -X POST localhost:8080/checkout | jq .
```

**Expected response:**
```json
{
  "error": "handler: POST /checkout failed: service layer: payment processing failed: flutterwave error FW-9082 region=eu-west retryable=false tx_ref=FW-TXN-84712947: CARD_DECLINED: insufficient_funds, issuer_code=05, network=VISA_EU"
}
```

Point to make: internal topology, transaction reference, issuer codes, and retry policy
all reached the API client. The payment layer has no error boundary.

```bash
# Reset payment mode when done
curl -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"ok"}'
```

---

## Makefile targets

```bash
make scenario-1     # excessive wrapping
make scenario-2     # abstraction leakage
make scenario-3     # meaning reinterpreted (shows wrong HTTP status)
make scenario-4     # provider error leakage
make scenario-all   # all four in sequence
```

---

## Demo management endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/demo/reset` | Clear all tables |
| `POST` | `/demo/seed` | Insert a demo product |
| `POST` | `/demo/payment-mode` | Change payment failure mode at runtime |

---

## All endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/products` | List all products |
| `POST` | `/products` | Create a product |
| `GET` | `/products/{id}` | Get product by ID |
| `GET` | `/products/{id}/inventory` | Check product stock |
| `POST` | `/cart/items` | Add product to cart |
| `POST` | `/checkout` | Checkout cart |
| `GET` | `/orders` | List all orders |
| `GET` | `/orders/{id}` | Get order by ID |

---

## Database

| Setting | Value |
|---------|-------|
| Container | `gophercon-demo` |
| Database | `gophercon` |
| Port | `5433` |
| User | `adminuser` |
| Password | `postgres` |
