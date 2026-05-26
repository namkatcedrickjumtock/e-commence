# Tech Stacks — Gophercon Europe 2026 Demo

## Language & Runtime

| Choice | Why |
|--------|-----|
| **Go 1.22+** | Enhanced `net/http` ServeMux with method-based routing (`"GET /path"`) and `r.PathValue()` — no third-party router needed, keeps dependency count at 3 |

## HTTP Layer

| Choice | Why |
|--------|-----|
| **stdlib `net/http`** | Zero abstraction overhead. Attendees see exactly how errors map to HTTP responses without framework magic. The error flow is transparent. |

## Database

| Choice | Why |
|--------|-----|
| **PostgreSQL 14** | Standard relational DB. The e-commerce domain (products, cart, orders) maps naturally to SQL. |
| **sqlc v2** | Generates type-safe Go code from raw SQL. Keeps queries visible for the demo — no ORM to hide what's happening. |
| **`lib/pq`** | De facto PostgreSQL driver for Go. |

## Configuration & Tooling

| Choice | Why |
|--------|-----|
| **`ardanlabs/conf/v3`** | Struct-tag-based config parsing from environment vars. Minimal, well-known in the Go community. |
| **`joho/godotenv`** | Loads `.env` for local dev — zero config for attendees who just want to run the demo. |
| **Docker Compose** | Single command (`docker compose up -d`) to start Postgres. No manual DB setup during the talk. |
| **Makefile** | Convenience targets: `make run`, `make gen`, `make database`. Low ceremony. |

## Dependencies (total: 3)

```
github.com/ardanlabs/conf/v3  v3.1.8   — config
github.com/joho/godotenv       v1.5.1   — env loading
github.com/lib/pq              v1.10.9  — postgres driver
```

No web framework, no router, no ORM, no error-handling library. Every line of error logic is written explicitly for the demo.

## Same Stack, Two Versions

This project exists as two versions on the same tech stack:

| Version | Branch/Tag | Approach |
|---------|------------|----------|
| **The "Bad" Version** (Slide 18) | `v0.1` | Same stack, intentionally bad patterns — excessive `fmt.Errorf` wrapping at every layer, raw infrastructure errors leaking up, no sentinels, fragile string matching in handlers |
| **The Structured Version** (Slide 19) | `main` / `v1.0` | Same stack, layer-owned sentinel errors, clean propagation, centralized `errors.Is` HTTP mapping |

Both versions use identical dependencies, database, and routing. Only the error-handling discipline changes. This makes the point visually: **the framework didn't change — the architecture did.**

## Why No Test Framework?

The project has zero automated tests by design — it's a 10-minute live demo, not a production service. Error modes are exercised at runtime via the `PAYMENT_MODE` env var instead of unit tests. This lets the presenter toggle failure modes live on stage and show the resulting HTTP responses and error flows in real time.
