# Tiny e-commerce backend (net/http) — layered errors demo

This repo is built for a 10–15 minute conference demo on **layered architecture and error ownership**.

> Errors are defined where they originate and propagated upward without noisy wrapping.

## Layers

```txt
api/           HTTP handlers, request parsing, response serialization (presentation)
services/      domain types, business errors, use-case orchestration (business)
persistence/   sqlc-generated PostgreSQL queries + Stripe payment mock (persistence)
cmd/           entry point, config, wiring
```

## Prerequisites

- Go 1.22+
- Docker (for PostgreSQL)

## Quick start

```bash
# Start PostgreSQL
docker compose up -d

# Run the app (reads .env or env vars)
cp .env.example .env
go run ./cmd
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/products` | List all products |
| POST | `/products` | Create a product |
| GET | `/products/{id}` | Get product by ID |
| GET | `/products/{id}/inventory` | Check product stock |
| POST | `/cart/items` | Add product to cart |
| POST | `/checkout` | Checkout cart |
| GET | `/orders` | List all orders |
| GET | `/orders/{id}` | Get order by ID |

### Examples — full flow

```bash
# ── Products ──────────────────────────────────────────────

# List all products
curl -sS localhost:8080/products

# Create a product
curl -sS -X POST localhost:8080/products \
  -H 'content-type: application/json' \
  -d '{"name":"Widget Pro","price_cents":3999,"stock":15}'

# Get a single product by ID
curl -sS localhost:8080/products/p_1

# Check product stock
curl -sS localhost:8080/products/p_1/inventory

# ── Cart ──────────────────────────────────────────────────

# Add product to cart
curl -sS -X POST localhost:8080/cart/items \
  -H 'content-type: application/json' \
  -d '{"product_id":"p_1","quantity":2}'

# ── Checkout ──────────────────────────────────────────────

# Checkout (returns order_id)
curl -sS -X POST localhost:8080/checkout

# ── Orders ────────────────────────────────────────────────

# List all orders
curl -sS localhost:8080/orders

# Get a single order by ID (replace o_1 with actual order_id)
curl -sS localhost:8080/orders/o_1
```

### Error-mode examples

```bash
# Start server in a specific failure mode
PAYMENT_MODE=card_declined go run ./cmd

# Then run checkout — expect HTTP 402
curl -sS -X POST localhost:8080/checkout

# Re-run checkout after switching to other modes:
#   processing_error  → 422
#   rate_limit        → 503
#   api_timeout       → 503
PAYMENT_MODE=api_timeout go run ./cmd
curl -sS -X POST localhost:8080/checkout
```

## Payment modes

Set `PAYMENT_MODE` env var:

| Mode | Behaviour | HTTP |
|------|-----------|------|
| `ok` | Payment succeeds | 201 |
| `card_declined` | Card declined | 402 |
| `processing_error` | Processing error | 422 |
| `rate_limit` | Rate limited | 503 |
| `api_timeout` | API timeout | 503 |

```bash
PAYMENT_MODE=card_declined go run ./cmd
```

## Error ownership by layer

| Error | Layer | HTTP |
|---|---|---|
| `invalid json` | presentation | 400 |
| `missing field` | presentation | 400 |
| `invalid product id` | presentation | 400 |
| `product not found` | business | 404 |
| `duplicate cart item` | business | 409 |
| `product out of stock` | business | 409 |
| `cart is empty` | business | 409 |
| `payment declined` | business | 402 |
| `database unavailable` | persistence | 503 |
| `stripe: processing error` | persistence | 422 |
| `stripe: too many requests` | persistence | 503 |
| `payment provider timeout` | persistence | 503 |

## Architecture approach

- **Sentinel errors** (`var ErrX = errors.New(...)`) are defined at the layer they originate
- Errors propagate **upward without wrapping** — no `fmt.Errorf("checkout failed: %w", err)`
- The API layer maps errors to HTTP status codes using `errors.Is`
- Infrastructure errors (Stripe, DB) pass through the business layer untouched
- Business-meaningful third-party errors (card declined) are translated to business sentinels via `%w`

## Error propagation example

When a card is declined by Stripe:

```txt
persistence.StripeProvider  →  returns fmt.Errorf("%w: ...", services.ErrPaymentDeclined)
                                  ↓
services.Checkout           →  errors.Is(err, ErrPaymentDeclined) → true → propagate
                                  ↓
api.writeError              →  errors.Is(err, services.ErrPaymentDeclined) → 402
```

When Stripe times out:

```txt
persistence.StripeProvider  →  returns fmt.Errorf("%w: ...", ErrPaymentProviderTimeout)
                                  ↓
services.Checkout           →  errors.Is(err, ErrPaymentDeclined) → false → propagate raw
                                  ↓
api.writeError              →  errors.Is(err, persistence.ErrPaymentProviderTimeout) → 503
```
