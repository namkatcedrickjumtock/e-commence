# Roadmap — Gophercon Europe 2026 Demo

The demo is structured around three consecutive slides. Each phase below maps to one slide.

## Phase 1: Demo Setup (Slide 17) — *done*

The foundation — a small e-commerce backend using stdlib only.

- [x] `net/http` server with layered architecture
- [x] Products, cart, checkout, orders endpoints
- [x] PostgreSQL + sqlc for data access
- [x] Fake payment provider with multiple failure modes
- [x] Domain types shared across layers (`Product`, `CartItem`, `Order`)

## Phase 2: The "Bad" Version (Slide 18) — *to build*

A deliberately broken version used as the "before" in the talk. Shows what happens when error handling has no discipline.

- [ ] Excessive wrapping at every layer (`fmt.Errorf("layer X: %w", err)`)
- [ ] Infrastructure leakage — DB/HTTP errors bubble up raw
- [ ] Unreadable error chains (`err.Error()` produces paragraphs)
- [ ] No sentinel errors — just string comparisons
- [ ] HTTP layer depends on database error semantics
- [ ] Business meaning buried under wrapping noise

This version exists as a separate branch / tag so the presenter can toggle between it and the structured version during the demo.

## Phase 3: The Structured Version (Slide 19) — *done (current codebase)*

The "after" version. Layer-owned errors, clean propagation, centralized mapping.

- [x] Sentinel errors defined at their origin layer (`api/`, `services/`, `persistence/`)
- [x] Errors propagate upward without wrapping unless a layer adds meaning
- [x] Centralized `writeError` maps errors to HTTP status codes via `errors.Is`
- [x] Business-meaningful infrastructure errors (card declined) are translated via `%w`
- [x] Each layer's `errors.go` is an audit trail of every error it can produce

## Future

- [ ] **OpenTelemetry tracing** — show error spans across layers in a trace viewer
- [ ] **Structured logging** — emit layer + error info in JSON logs for live `tail`
- [ ] **Error-visualisation endpoint** — `GET /debug/error-flow` that returns a JSON trace of how each error propagates
- [ ] **Demo automation** — `make demo` target that runs a sequence of curl commands with `jq` formatting
- [ ] **Dockerise the Go app** so attendees can run the full stack with one `docker compose up`
