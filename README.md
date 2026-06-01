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
curl -s -X POST localhost:8080/demo/reset | jq .
curl -s -X POST localhost:8080/demo/seed | jq .
```

`/demo/seed` inserts three products: **GopherCon T-Shirt** ($24.99), **Go Programming Book** ($39.99), **Gopher Plush Toy** ($14.99).
Copy one of the returned `id` values — you'll need it for Scenarios 3 and 4.

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

**Talking points:**
- Count the prefixes: `handler →  service layer → order lookup → repository → database` — five clauses for one missing row.
- None of these prefixes tell the caller anything they couldn't figure out from the route and the root cause.
- The raw SQL error `sql: no rows in result set` reached an HTTP client. This is an internal detail that belongs at the database layer.
- Adding context is good; adding *every layer's name* is noise. The goal is signal, not a stack trace in a string.
- Ask the audience: *if this error shows up in a log aggregator at 3am, does the on-call engineer know what to do?*

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

**Talking points:**
- Open `services/services.go` and point to the imports: `database/sql` and `github.com/lib/pq` are infrastructure packages inside the business layer.
- The service calls `errors.Is(err, sql.ErrNoRows)` — it is making a decision based on a database concept. If you swap Postgres for MongoDB tomorrow, this code breaks.
- The error message itself contains `sql: no rows in result set` — a database driver string — verbatim in the HTTP response. The client now knows your storage technology.
- This is abstraction leakage: the contract of the persistence layer has leaked through the service layer all the way to the HTTP response.
- The fix is to translate at the boundary: `Get` should return `ErrProductNotFound`, and no layer above should ever import `database/sql`.

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

**Talking points:**
- The root cause is identical to Scenario 2: `p_ghost` doesn't exist, so `repo.Get` returns `sql.ErrNoRows`.
- But `AddProductToCart` reinterprets that as `"stock data unavailable"` — changing the *meaning* of the error.
- `writeError` checks `"unavailable"` before `"no rows"`, so this returns **503 Service Unavailable** instead of 404 Not Found.
- The client's retry logic will now hammer the server on a permanently missing product — exactly the wrong behavior.
- Point to `writeError` in `api/http.go`: the switch is order-dependent. Swapping two cases changes which HTTP status the whole system returns. That is fragile architecture encoded in string matching.
- The root fix: return a typed sentinel (`ErrProductNotFound`) from the repo, check it with `errors.Is`, and never relabel errors across layers.

---

### Scenario 4 — External Provider Error Leakage

Raw `FlutterwaveError` fields — internal code, region, transaction ref, retryable flag —
reach the HTTP response body with no abstraction.

```bash
# Step 1 — list products to pick an ID
curl -s localhost:8080/products | jq .

# Step 2 — add a product to cart
curl -s -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"<id from step 1>","quantity":1}' | jq .

# Step 3 — set payment mode to card declined (no server restart needed)
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"card_declined"}' | jq .

# Step 4 — checkout and watch the provider internals leak
curl -s -X POST localhost:8080/checkout | jq .
```

**Expected response:**
```json
{
  "error": "handler: POST /checkout failed: service layer: payment processing failed: flutterwave error FW-9082 region=eu-west retryable=false tx_ref=FW-TXN-84712947: CARD_DECLINED: insufficient_funds, issuer_code=05, network=VISA_EU"
}
```

**Talking points:**
- The response body contains `FW-9082`, `region=eu-west`, `tx_ref=FW-TXN-84712947`, `issuer_code=05`, `network=VISA_EU` — all internal fields of `FlutterwaveError`.
- This is your provider's internal topology and transaction references, live in an API response to an end user.
- Security risk: leaking provider codes, regions, and network names is useful information for attackers probing your payment stack.
- Portability risk: if you switch from Flutterwave to Stripe, every client parsing these codes breaks silently.
- The fix: define a `PaymentError` type at the service boundary — `{Code: "payment_declined", Retryable: false}` — and translate there. The provider is an implementation detail.
- Notice there is no `errors.Is` check for payment errors anywhere — the only reason the `writeError` switch catches this is the `FW-` string prefix. Rename the code format and 402 silently breaks.

```bash
# Reset payment mode when done
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"ok"}' | jq .
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
| `GET` | `/products/{id}` | Get product by ID |
| `POST` | `/cart/items` | Add product to cart |
| `POST` | `/checkout` | Checkout cart |
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
