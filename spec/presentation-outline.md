# Error Tracing: Lessons Learned From the Trenches

## GopherCon Europe 2026 · Berlin
**Namkat Cedrick · Software Engineer, Iknite Inc**

**Total time: 30 minutes (~15 min talk, ~15 min live demo)**

---

## Slide 1 — Title

**Title:** Error Tracing: Lessons Learned From the Trenches

**Slide text:**
- Title: Error Tracing: Lessons Learned From the Trenches
- Subtitle: Namkat Cedrick · Iknite Inc
- GopherCon Europe 2026 · Berlin

**Speaker notes · 30 seconds:**

Good morning, everyone. My name is Namkat Cedrick, I'm a software engineer at Iknite Inc, and it's a real pleasure to be here. Before we begin, I want to thank the organizers for this opportunity and thank all of you for choosing to spend the next half hour in this room.

I'll be honest with you — this is my first time presenting on a stage like this, so bear with me. The title is "Lessons Learned From the Trenches," and I mean the trenches. What I'm going to share today is a story from my own experience: how I didn't treat errors as part of my architecture, how I kept kicking that can down the road, until one day it stopped being my problem alone and became a problem for my whole team.

**Transition → Slide 2:** "Let me tell you exactly what that looks like."

---

## Slide 2 — Thesis / What We'll Cover

**Title:** Your error handling is probably lying to you.

**Slide text:**

> Over-wrapping error messages
>
> Letting your database leak into your business logic
>
> Reinterpreting errors so the wrong HTTP status fires
>
> Exposing your provider's internals to every client

**Bottom line:** *Same codebase. Same stack. Only the error discipline changes.*

**Speaker notes · 1 minute:**

Everything you'll see came out of a real layered backend that worked beautifully in the happy path and fell apart the moment something failed. Here's the thesis, in one line: in a layered system, handling errors as part of your architecture isn't a detail you bolt on at the end — it is architecture. And we're going to use a sample layered backend API to test that claim in action.

These four patterns — over-wrapping error messages, letting your database leak into your business logic, reinterpreting errors so the wrong status code fires, and letting your external provider's internals reach every client — these aren't theoretical. I've hit every single one in production.

**Transition → Slide 3:** "And it starts with something Go makes look deceptively simple."

---

## Slide 3 — The Illusion

**Title:** `if err != nil { return err }`

**Slide text:**

```go
// This is fine... right?
func GetUser(ctx context.Context, id string) (*User, error) {
    user, err := db.Query(ctx, id)
    if err != nil {
        return err
    }
    return user, nil
}
```

*Go makes it look easy. Production disagrees.*

**Speaker notes · 1 minute:**

Most of us know that Go was designed with simplicity in mind, especially the error handling philosophy. I've always carried that concept in the back of my mind: if something happens, wrap and return it to the caller and voilà, you're done. Well... [pause] Let me show you why that doesn't work in a layered system.

**Transition → Slide 4:** "So what actually happens when an error starts at the bottom and needs to reach the top?"

---

## Slide 4 — The Question

**Title:** What happens to this error?

**Slide text:**

```
Repository  ──→  Service  ──→  Handler
   ↓                ↓              ↓
sql.ErrNoRows    ???           ???  → HTTP ???
```

*Who owns the meaning of this error?*

**Speaker notes · 1 minute:**

Let's look at a concrete example. In your layered system, you get a database error — say, while trying to get a user by ID. The question is: how does this specific error message get contextualized and persisted through the layers? Because...

**Transition → Slide 5:** [click] revealing the three problems on the next slide

---

## Slide 5 — The Problem

**Title:** Errors don't stay where they start

**Slide text:**

```
Amplified     Misinterpreted     Leaked
```

*Every layer between the error and the client can change what the error means — and whether the client gets the right response at all.*

**Speaker notes · 30 seconds:**

When errors occur in a particular layer, they don't stay there. They need to travel upstream. And as they travel through the layers, they can get amplified, misinterpreted, and impact everything upstream. That's the core problem we're going to explore.

**Transition → Slide 6:** "Let me show you the four specific ways this breaks."

---

## Slide 6 — The Anti-Pattern Map

**Title:** 4 patterns that break layered systems

**Slide text — 2×2 grid:**

| | |
|---|---|
| **1. Over-wrapping** — every layer stamps the error, the real cause is buried | **2. Abstraction Leakage** — `database/sql` in your business logic |
| **3. Meaning Reinterpretation** — "not found" becomes "unavailable" | **4. External Provider Leakage** — provider internals reach the client |

*Every one of these came from production. The demo app reproduces them on purpose.*

**Speaker notes · 1 minute:**

Here are four anti-patterns that I encountered in real life. I'm using a demo application for demonstration purposes, but make no mistake — these are patterns I've seen in production systems. Let me note that anti-pattern 4 is not specific to payments. Any time you integrate with an external system — an email provider, an SMS gateway, cloud storage, a third-party auth service — the same leakage pattern applies.

**Transition → Slide 7:** "Let me show you the app we'll use to demonstrate all four."

---

## Slide 7 — The Demo App

**Title:** The setup

**Slide text:**

```
GET  /products          → List products
GET  /products/{id}     → Get product
POST /cart/items        → Add to cart
POST /checkout          → Pay + place order
GET  /orders/{id}       → Get order
```

```
api/            → handlers, routing, writeError
services/       → business logic
persistence/    → PostgreSQL + external provider
cmd/            → wiring
```

*3 dependencies. No framework. No ORM. Every line of error logic is explicit.*

**Speaker notes · 1 minute:**

Let me introduce the demo app. It's a minimal e-commerce backend with five endpoints: list products, get a product, add to cart, checkout, and get an order. Three dependencies — no framework, no ORM, no error handling library. Every line of error logic is explicit on purpose. The architecture has three layers: handlers at the top, business logic in the middle, and persistence plus an external provider at the bottom.

I'm going to demo this in two passes. First, the bad version — where all four anti-patterns live. Then, after we talk about the fix, I'll switch to the good version and show you the same scenarios with the error discipline applied. Let's see what happens when things fail.

**Transition → Slide 8:** [switch to terminal] "Let's start with Scenario 1."

---

┌─────────────────────────────────────────────────┐
│           LIVE DEMO: v0.1 (Bad Version)         │
│              Slides 8–15 · ~10 min              │
└─────────────────────────────────────────────────┘

---

## Slide 8 — Demo: Scenario 1 (Over-wrapping)

**Title:** Scenario 1: One missing row, five prefixes

**Slide text:**

```bash
curl localhost:8080/orders/o_ghost
```

```json
{
  "error": "handler: GET /orders/{id} failed: service layer: failed to retrieve order details: order lookup failed: repository: GetOrder query failed: order record not found in database: sql: no rows in result set"
}
```

**Speaker notes · 1.5 minutes:**

[Switch to terminal] Let's run the app on the v0.1 branch and request an order that doesn't exist. [Run command] Look at that. One missing row produces an error with five prefixes — handler, service layer, order lookup, repository, database. Every layer added its own stamp to the message. The real cause — `sql: no rows in result set` — is buried at the very end of a paragraph. None of these prefixes tell the caller anything they couldn't figure out from the route and the root cause.

**Transition → Slide 9:** "Let me show you exactly how this chain builds up."

---

## Slide 9 — Scenario 1 Post-Mortem

**Title:** The wrapping chain

**Slide text:**

```
persistence:  "repository: GetOrder query failed: order record not found in database: %w"
                         ↓
services:     "service layer: failed to retrieve order details: order lookup failed: %w"
                         ↓
api:          "handler: GET /orders/{id} failed: %w"
```

*Every layer adds noise. The root cause is buried at the end of a paragraph.*

**Speaker notes · 1.5 minutes:**

Look at how each layer wraps: persistence adds "repository: GetOrder query failed", service adds "service layer: failed to retrieve order details", handler adds "handler: GET /orders/{id} failed". That's three layers each stamping the error with redundant context. If this shows up in a log aggregator at 3am, the on-call engineer has to read through five clauses to find the actual cause. Over-wrapping doesn't add information — it buries it.

**Transition → Slide 10:** "That's anti-pattern 1. Let me show you anti-pattern 2."

---

## Slide 10 — Demo: Scenario 2 (Abstraction Leakage)

**Title:** Scenario 2: Your database just leaked into your HTTP response

**Slide text:**

```bash
curl localhost:8080/products/p_ghost
```

```json
{
  "error": "handler: GET /products/{id} failed: service: product not found in database catalog: sql query returned no rows: repository: GetProduct query failed: database error: sql: no rows in result set"
}
```

**Speaker notes · 1.5 minutes:**

[Switch to terminal] Now let's request a product that doesn't exist. [Run command] The string `sql: no rows in result set` — that's a PostgreSQL driver message, live in an HTTP response to a client. Your client now knows your storage technology. The SQL driver's string format is now part of your API contract.

**Transition → Slide 11:** "Let me show you where this leak originates."

---

## Slide 11 — Scenario 2 Post-Mortem

**Title:** Where the leak happens

**Slide text:**

```
services/services.go:6     import "database/sql"       ← business layer
services/services.go:9      import "github.com/lib/pq"  ← business layer

services/services.go:66     if !errors.Is(err, sql.ErrNoRows) { ... }
services/services.go:90     if errors.Is(err, sql.ErrNoRows) { ... }
services/services.go:122    if errors.Is(err, sql.ErrNoRows) { ... }
services/services.go:142    if errors.As(err, &pgErr) && pgErr.Code == "23505" { ... }
```

*The contract of the persistence layer has leaked all the way to the HTTP response.*

**Speaker notes · 1.5 minutes:**

[Switch to editor] Let me show you where the leak happens. `services/services.go` line 6: `import "database/sql"`. Line 9: `import "github.com/lib/pq"`. Your business logic is importing database packages directly. And look at line 66: `errors.Is(err, sql.ErrNoRows)`. Line 142: `errors.As(err, &pgErr) && pgErr.Code == "23505"`. That's business logic branching on database error types. Swap Postgres for MongoDB tomorrow — this code breaks.

**Transition → Slide 12:** "Now let me show you what happens when that same leak gets worse."

---

## Slide 12 — Demo: Scenario 3 (Meaning Reinterpretation)

**Title:** Scenario 3: Same root cause, wrong status code

**Slide text:**

```bash
curl -i -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"p_ghost","quantity":1}'
```

```
HTTP/1.1 503 Service Unavailable

{"error": "handler: POST /cart/items failed: service: cart operation failed: inventory check failed: stock data unavailable: repository: GetProduct query failed: database error: sql: no rows in result set"}
```

**Speaker notes · 1.5 minutes:**

[Switch to terminal] Same root cause as Scenario 2 — `p_ghost` doesn't exist. But this time we're adding it to a cart. The service layer receives `sql.ErrNoRows` and relabels it as "stock data unavailable". [Point to response] Look at the HTTP status: 503 Service Unavailable, not 404 Not Found. A missing product just told the client "try again later." That's the wrong response.

**Transition → Slide 13:** "Let me show you how this relabeling breaks the HTTP contract."

---

## Slide 13 — Scenario 3 Post-Mortem

**Title:** How "not found" became "unavailable"

**Slide text:**

```
persistence:  sql.ErrNoRows                    ← "product not found"
                      ↓
services:      "stock data unavailable: %w"     ← relabeled!
                      ↓
api writeError: strings.Contains(msg, "unavailable") → 503  ← matches first
                strings.Contains(msg, "no rows")    → 404  ← never reached
```

*The error's meaning changed at the service layer. The HTTP mapping became order-dependent and fragile.*

**Speaker notes · 1.5 minutes:**

The error's meaning changed at the service layer. "Product not found" became "stock data unavailable". Then `writeError` checks strings in order — "unavailable" matches before "no rows" — so the client gets 503 instead of 404. A client that retries on 503 will hammer your server on a permanently missing product. That's exactly the wrong behavior. And notice: the `writeError` switch is order-dependent. Swap two cases and the entire system returns different HTTP statuses. That's not architecture — that's luck.

**Transition → Slide 14:** "Now the last anti-pattern — and this one has real security implications."

---

## Slide 14 — Demo: Scenario 4 (External Provider Leakage)

**Title:** Scenario 4: Your provider's internals just hit production

**Slide text:**

```bash
# Set the external provider failure mode
curl -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"card_declined"}'

curl -X POST localhost:8080/checkout | jq .
```

```json
{
  "error": "handler: POST /checkout failed: service layer: payment processing failed: flutterwave error FW-9082 region=eu-west retryable=false tx_ref=FW-TXN-84712947: CARD_DECLINED: insufficient_funds, issuer_code=05, network=VISA_EU"
}
```

**Speaker notes · 1.5 minutes:**

[Switch to terminal] Now let's switch the external provider to a failure mode and try to check out. [Run commands] The HTTP response contains `FW-9082`, `region=eu-west`, `tx_ref=FW-TXN-84712947`, `issuer_code=05`, `network=VISA_EU`. These are the provider's internal codes, deployment regions, and transaction references.

And again — this isn't just about payments. This applies to any external system integration. An email provider, an SMS gateway, cloud storage, a third-party auth service — the same pattern. Your provider's internals are now your API contract. Switch providers and every client parsing these codes breaks silently.

**Transition → Slide 15:** "Let me break down exactly what leaked."

---

## Slide 15 — Scenario 4 Post-Mortem

**Title:** What leaked and why it matters

**Slide text:**

```
FlutterwaveError {
    Code:      "FW-9082"              ← internal provider code
    Region:    "eu-west"              ← deployment topology
    Retryable: false                  ← internal retry policy
    TxRef:     "FW-TXN-84712947"     ← transaction reference
    Message:   "CARD_DECLINED:
                insufficient_funds,
                issuer_code=05,
                network=VISA_EU"     ← acquirer internals
}
```

**Why it matters:**

- **Security:** Provider codes + regions = reconnaissance goldmine
- **Portability:** Switch providers → every client breaks silently
- **Coupling:** Your HTTP contract depends on your provider's string format

**Speaker notes · 1.5 minutes:**

Let me break down what actually leaked. The internal provider code, the deployment region, the retry policy, the transaction reference, and the raw provider message containing acquirer codes and network names. Why does this matter? Security: provider codes and regions are reconnaissance goldmine for attackers probing your stack. Portability: switch from one provider to another and every client parsing these codes breaks silently. Coupling: your HTTP contract depends on your provider's string format. This is architectural debt you pay every time the provider changes anything.

**Transition → Slide 16:** "So what's the fix? Let's talk about the principle first, then I'll show you the good version."

---

## Slide 16 — The Principle

**Title:** Errors are architecture

**Slide text:**

```
Presentation   owns  → invalid request, malformed payload, bad params
Domain         owns  → business rule failures, domain invariants
Infrastructure owns  → database failures, external system failures
```

*The layer where an error originates defines its meaning. Propagate upward unchanged. Translate only at boundaries.*

**Speaker notes · 1 minute:**

So what's the fix? Errors are architecture. Each layer owns its own errors. The presentation layer owns invalid requests, malformed payloads, bad params. The domain layer owns business rule failures and domain invariants. The infrastructure layer owns database failures and external system failures. The layer where an error originates defines its meaning. Propagate upward unchanged. Translate only at boundaries.

Three rules:
1. Own your errors at the boundary — the persistence layer translates `sql.ErrNoRows` to `ErrProductNotFound`, and no layer above ever sees a database error.
2. Don't reinterpret meaning — "not found" stays "not found" all the way to the HTTP handler.
3. External systems are implementation details — translate their errors at the service boundary, your clients should never see provider codes.

**Transition → Slide 17:** "Here's what that looks like in code."

---

## Slide 17 — The Fix: Before & After

**Title:** Same codebase. Different discipline.

**Slide text — Before (v0.1):**

```go
// services — imports database/sql, relabels errors, wraps everything
func (s *service) GetProduct(ctx, id) (*Product, error) {
    product, err := s.repo.Get(ctx, id)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, fmt.Errorf("service: product not found in database catalog: %w", err)
    }
    ...
}
```

**Slide text — After (v0.2):**

```go
// persistence — translates at the boundary
func (r *PostgresRepo) Get(ctx, id) (*Product, error) {
    p, err := r.q.GetProduct(ctx, id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, ErrProductNotFound    // domain sentinel
        }
        return nil, ErrDatabaseUnavailable
    }
    ...
}

// services — no database imports, no wrapping
func (s *service) GetProduct(ctx, id) (*Product, error) {
    product, err := s.repo.Get(ctx, id)
    if err != nil {
        return nil, err    // propagate unchanged
    }
    return product, nil
}

// api — centralized errors.Is mapping
func (h *Handler) writeError(w, err) {
    switch {
    case errors.Is(err, persistence.ErrProductNotFound):  status = 404
    case errors.Is(err, persistence.ErrOrderNotFound):    status = 404
    case errors.Is(err, persistence.ErrProductOutOfStock): status = 409
    case errors.Is(err, &persistence.PaymentError{}):     // checks Retryable
    ...
    }
}
```

**Speaker notes · 2 minutes:**

Here's what changes. In v0.1, the persistence layer wraps without translating — `sql.ErrNoRows` propagates raw. In v0.2, persistence translates `sql.ErrNoRows` to `ErrProductNotFound` at the boundary. The database driver never leaves the persistence package. Services no longer import `database/sql` — they just propagate errors unchanged. No wrapping, no reinterpretation. And `writeError` in the API layer now uses `errors.Is` on typed sentinels instead of string matching. No order-dependent switch cases. No fragile string comparisons. And for external providers, `FlutterwaveError` gets translated to `PaymentError` at the persistence boundary — no provider internals reach the client.

**Transition → Slide 18:** "Let me be precise about what actually changed."

---

## Slide 18 — What Actually Changed

**Title:** What actually changed?

**Slide text:**

| Before (v0.1) | After (v0.2) |
|---|---|
| `database/sql` imported in business logic | No infrastructure imports in services |
| `sql.ErrNoRows` propagates raw to HTTP | Translated to `ErrProductNotFound` at the boundary |
| Service relabels "not found" as "unavailable" | "Not found" stays "not found" through all layers |
| `writeError` uses `strings.Contains` matching | `writeError` uses `errors.Is` on typed sentinels |
| `FlutterwaveError` leaks to client | Translated to `PaymentError` at the boundary |
| Each layer wraps with `fmt.Errorf` | Errors propagate unchanged unless a layer adds meaning |

*Same stack. Same endpoints. Same database. Only the error discipline changed.*

**Speaker notes · 1 minute:**

Let me be clear about what changed and what didn't. Same stack. Same endpoints. Same database. Same three dependencies. The only thing that changed is the error discipline. The persistence layer translates errors at the boundary instead of letting them leak. The service layer propagates errors instead of wrapping them. The API layer maps sentinels with `errors.Is` instead of string matching. The external provider translates its internal errors to a domain type. That's it.

**Transition → Slide 19:** "Now let me prove it. Let me switch to v0.2 and run the same scenarios."

---

┌─────────────────────────────────────────────────┐
│          LIVE DEMO: v0.2 (Good Version)          │
│              Slides 19 · ~4 min                  │
└─────────────────────────────────────────────────┘

---

## Slide 19 — v0.2 Demo: The Good Version

**Title:** Let's see the fix in action

**Slide text:**

```
git checkout v0.2
make run
```

**Demo flow (~4 minutes):**

### Scenario 1 fix — Over-wrapping → Clean errors (~1 min)

```bash
curl localhost:8080/orders/o_ghost | jq .
```

**Expected response (v0.2):**
```json
{
  "error": "order not found"
}
```

**Speaker notes:** "Let me switch to the v0.2 branch and run the same request. [Run command] Same missing order, same root cause. But now the response is just: 'order not found.' The persistence layer translated `sql.ErrNoRows` to `ErrOrderNotFound`, the service layer propagated it unchanged, and the API layer mapped it to 404 via `errors.Is`. No paragraph, no prefixes, no database driver strings."

### Scenario 3 fix — Meaning reinterpretation → Correct status code (~1 min)

```bash
curl -i -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"p_ghost","quantity":1}'
```

**Expected response (v0.2):**
```
HTTP/1.1 404 Not Found

{"error": "adding to cart: product not found"}
```

**Speaker notes:** "Same product ID that doesn't exist, same `p_ghost`. But now we get 404 Not Found — not 503. The error wasn't relabeled. The persistence layer returned `ErrProductNotFound`, the service layer propagated it, and `writeError` matched it cleanly with `errors.Is`. The client gets the right status code."

### Scenario 4 fix — External provider leakage → Translated error (~2 min)

```bash
# Setup: seed products and add one to cart
curl -s -X POST localhost:8080/demo/reset | jq .
curl -s -X POST localhost:8080/demo/seed | jq .

# Add a product to cart (use an ID from seed output)
curl -s -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"<ID from seed>","quantity":1}'

# Set external provider to failure mode
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"card_declined"}'

# Checkout and see the translated error
curl -s -X POST localhost:8080/checkout | jq .

# Reset
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"ok"}'
```

**Expected response (v0.2):**
```json
{
  "error": "payment failed: payment_declined"
}
```

**Speaker notes:** [Run commands interactively] "Now let's set the provider to card declined and checkout again. [Run commands] Look at the response: 'payment failed: payment_declined.' No FW-9082. No eu-west. No transaction references. No acquirer internals. The `FlutterwaveError` was translated to a `PaymentError` at the persistence boundary — the service layer and the HTTP layer never saw provider internals. Your client gets a clean, portable error. Switch providers tomorrow and your API contract doesn't change."

**Transition → Slide 20:** "Three things to take home."

---

## Slide 20 — Takeaways

**Title:** 3 things to take home

**Slide text:**

> **1. Own your errors at the boundary**
> The layer where an error originates defines its meaning. Translate there, propagate elsewhere.

> **2. Never reinterpret meaning across layers**
> "Not found" stays "not found." Don't relabel it "unavailable" — you'll break the HTTP contract.

> **3. External systems are implementation details**
> Any provider — payment, email, SMS, storage — should be translated at the boundary. Your clients should never see their internals.

**Speaker notes · 1 minute:**

Three things to take home. First: own your errors at the boundary. The layer where an error originates defines its meaning — translate there, propagate elsewhere. `sql.ErrNoRows` becomes `ErrProductNotFound` at the persistence layer, and nothing above ever sees a database error again. Second: never reinterpret meaning across layers. "Not found" stays "not found." Don't relabel it "unavailable." The meaning an error is born with is the meaning it should die with. Third: external systems are implementation details. Any provider — payment, email, SMS, storage — should be translated at the boundary. Your clients should never see their internals.

**Transition → Slide 21:** "Thank you."

---

## Slide 21 — Close

**Title:** Thank you

**Slide text:**

```
github.com/namkatcedrickjumtock/e-commence
```

*Questions?*

**Speaker notes · 30 seconds + Q&A:**

The repo is open source — you can clone it, run the bad version on the `v0.1` branch, run the good version on the `v0.2` branch, and see the difference for yourself. Both branches have the same endpoints, the same database, the same three dependencies. Only the error discipline changed. Thank you for your time. Questions?

---

## Time Budget Summary

| Section | Slides | Mode | Time | Cumulative |
|---|---|---|---|---|
| Introduction + thesis | 1–2 | Talk | 1.5 min | 0:00–1:30 |
| The Illusion → The Problem | 3–5 | Talk | 2.5 min | 1:30–4:00 |
| Anti-pattern map | 6 | Talk | 1 min | 4:00–5:00 |
| Demo app setup | 7 | Talk | 1 min | 5:00–6:00 |
| Scenario 1: Over-wrapping | 8–9 | Demo | 3 min | 6:00–9:00 |
| Scenario 2: Abstraction Leakage | 10–11 | Demo | 3 min | 9:00–12:00 |
| Scenario 3: Meaning Reinterpretation | 12–13 | Demo | 3 min | 12:00–15:00 |
| Scenario 4: External Provider Leakage | 14–15 | Demo | 3 min | 15:00–18:00 |
| The Principle | 16 | Talk | 1 min | 18:00–19:00 |
| Before & After / What Changed | 17–18 | Talk | 3 min | 19:00–22:00 |
| v0.2 Demo: Good Version | 19 | Demo | 4 min | 22:00–26:00 |
| Takeaways + Close | 20–21 | Talk | 1.5 min | 26:00–27:30 |
| Buffer for Q&A | — | — | 2.5 min | 27:30–30:00 |
| **Total** | | | **~30 min** | |

---

## v0.2 Demo Quick Reference

### Switch to v0.2 branch
```bash
git checkout v0.2
make run
```

### Scenario 1 fix — Over-wrapping → Clean errors
```bash
curl localhost:8080/orders/o_ghost | jq .
# Expected: {"error": "order not found"}  → 404
```

### Scenario 3 fix — Meaning reinterpretation → Correct 404
```bash
curl -i -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"p_ghost","quantity":1}'
# Expected: HTTP/1.1 404 Not Found
#           {"error": "adding to cart: product not found"}
```

### Scenario 4 fix — External provider leakage → Translated error
```bash
# Setup
curl -s -X POST localhost:8080/demo/reset | jq .
curl -s -X POST localhost:8080/demo/seed | jq .

# Add to cart (use actual product ID from seed output)
curl -s -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"<ID>","quantity":1}'

# Set failure mode
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"card_declined"}'

# Checkout
curl -s -X POST localhost:8080/checkout | jq .
# Expected: {"error": "payment failed: payment_declined"}  → 402
#           No FW-9082, no region, no tx_ref, no acquirer codes

# Reset
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"ok"}'
```

---

## v0.1 Demo Quick Reference (Bad Version)

### Setup (run once before demo)
```bash
git checkout v0.1
docker compose up -d
make migrate
cp .env.example .env
make run
```

### Reset + Seed (run before each scenario)
```bash
curl -s -X POST localhost:8080/demo/reset | jq .
curl -s -X POST localhost:8080/demo/seed | jq .
```

### Scenario 1 — Over-wrapping
```bash
curl localhost:8080/orders/o_ghost | jq .
```

### Scenario 2 — Abstraction Leakage
```bash
curl localhost:8080/products/p_ghost | jq .
```

### Scenario 3 — Meaning Reinterpretation
```bash
curl -i -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"p_ghost","quantity":1}'
```

### Scenario 4 — External Provider Leakage
```bash
# Step 1: Seed + get product ID
curl -s -X POST localhost:8080/demo/reset | jq .
curl -s -X POST localhost:8080/demo/seed | jq .

# Step 2: Add to cart (use actual product ID from seed output)
curl -s -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"<ID from step 1>","quantity":1}'

# Step 3: Set external provider to failure mode
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"card_declined"}'

# Step 4: Checkout and see leaked internals
curl -s -X POST localhost:8080/checkout | jq .

# Reset when done
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"ok"}'
```

---

## Transition Cheat Sheet (Quick Reference During Talk)

| From → To | Transition Line |
|---|---|
| Slide 1 → 2 | "Let me tell you exactly what that looks like." |
| Slide 2 → 3 | "And it starts with something Go makes look deceptively simple." |
| Slide 3 → 4 | "So what actually happens when an error starts at the bottom and needs to reach the top?" |
| Slide 4 → 5 | [click to reveal the three problems] |
| Slide 5 → 6 | "Let me show you the four specific ways this breaks." |
| Slide 6 → 7 | "Let me show you the app we'll use to demonstrate all four." |
| Slide 7 → 8 | [switch to terminal] "Let's start with Scenario 1." |
| Slide 8 → 9 | "Let me show you exactly how this chain builds up." |
| Slide 9 → 10 | "That's anti-pattern 1. Let me show you anti-pattern 2." |
| Slide 10 → 11 | "Let me show you where this leak originates." |
| Slide 11 → 12 | "Now let me show you what happens when that same leak gets worse." |
| Slide 12 → 13 | "Let me show you how this relabeling breaks the HTTP contract." |
| Slide 13 → 14 | "Now the last anti-pattern — and this one has real security implications." |
| Slide 14 → 15 | "Let me break down exactly what leaked." |
| Slide 15 → 16 | "So what's the fix? Let's talk about the principle first, then I'll show you the good version." |
| Slide 16 → 17 | "Here's what that looks like in code." |
| Slide 17 → 18 | "Let me be precise about what actually changed." |
| Slide 18 → 19 | "Now let me prove it. Let me switch to v0.2 and run the same scenarios." |
| Slide 19 → 20 | "Three things to take home." |
| Slide 20 → 21 | "Thank you." |