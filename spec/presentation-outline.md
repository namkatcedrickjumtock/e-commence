# Error Tracing: Lessons Learned From the Trenches

## Iknite Studio Engineering Retrospective

**Namkat Cedrick · Software Engineer, Iknite Inc**

**Total time: 30 minutes (~15 min talk, ~15 min live demo)**

---

## Slide 1 — Title

**Header strip:** `G O P H E R C O N  E U R O P E  2 0 2 6  ·  F R O M  T H E  T R E N C H E S`

**Title:** Error Tracing: Lessons Learned From the Trenches

**Slide text:**

- Title: Error Tracing: Lessons Learned From the Trenches
- Subtitle: Namkat Cedrick · Iknite Inc
- GopherCon Europe 2026 · Berlin

**Speaker notes · 30 seconds:**

Good morning, everyone. My name is Namkat Cedrick, I'm a software engineer at Iknite Inc, and it's a real pleasure to be here. Before we begin, I want to thank the organizers for this opportunity and thank all of you for choosing to spend the next half hour in this room.

I'll be honest with you — this is my first time presenting on a stage like this, so bear with me. The title is "Lessons Learned From the Trenches," and I mean the trenches. What I'm going to share today is a story from my own experience: how we at Iknite didn't treat errors as part of our architecture, how we kept kicking that can down the road, until one day it stopped being my problem alone and became a problem for our whole team.

**Transition → Slide 2:** "Let me tell you exactly what that looks like."

---

## Slide 2 — About Me

**Header strip:** `A B O U T  M E`

**Title:** About me

**Slide text:**

- Namkat Cedrick
- Software Engineer · Iknite Inc
- Founder · West / Central African Gophers
- github.com/namkatcedrickjumtock
- linkedin.com/in/namkatcedrick

**Speaker notes · 30 seconds:**

Before we dive in, let me introduce myself. I'm Namkat Cedrick, a software engineer at Iknite Inc. I organize the West and Central African Gophers community, and I've spent years building distributed systems in Go — including making every mistake you're about to see. The code I'm showing today isn't hypothetical — it's code I've written, debugged at 3am, and had to explain to a CTO.

**Transition → Slide 3:** "So here's the story of what happened at Iknite."

---

## Slide 3 — The Story

**Header strip:** `T H E  S T O R Y`

**Title:** What happened at Iknite

**Slide text:**

```
We built an e-commerce backend.
We shipped fast.
We didn't think about errors.

Four things broke in production:

  1  Every layer stamped its name on every error
  2  Our database driver strings leaked to HTTP responses
  3  A missing product returned 503 instead of 404
  4  Stripe's internal codes reached our API clients

The fix wasn't discipline — it was treating errors
as architecture from the start.
```

*This isn't a demo. This is what shipped to production.*

**Speaker notes · 1 minute:**

Here's the story. At Iknite, we built an e-commerce backend — five endpoints, three layers, a PostgreSQL database, and a Stripe integration. We shipped it fast, and it worked. The happy path was beautiful. But the moment something failed — a missing product, a declined card, a database timeout — the system fell apart. And not in subtle ways. We were returning 503 for products that didn't exist. We were leaking Stripe charge IDs to end users. Our error messages were paragraphs long. This is the story of how we dug ourselves out, the four anti-patterns we identified along the way, and the principle that ties the fix together.

**Transition → Slide 4:** "But the problem started somewhere simple. Something Go makes look easy."

---

## Slide 4 — The Illusion

**Header strip:** `W H E R E  I T  B E G A N`

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

Most of us know that Go was designed with simplicity in mind, especially the error handling philosophy. We carried that concept through our codebase: if something happens, wrap and return it to the caller and voilà, you're done. Well... [pause] Let me show you why that doesn't work in a layered system.

**Transition → Slide 5:** "So what actually happens when an error starts at the bottom and needs to reach the top?"

---

## Slide 5 — The Question

**Header strip:** `T H E  Q U E S T I O N`

**Slide text:**

```
What happens when an error
travels up through the layers?

sql: no rows in result set
        →
  persistence
        →
    services
        →
      api
        →
     client

By the time it reaches the top — is it still telling the truth?
```

**Speaker notes · 1 minute:**

Let's look at a concrete example from our codebase. You get a database error at the bottom of your stack — `sql: no rows in result set`. A record wasn't found. Simple.

But this error doesn't just teleport to the client. It travels. Through persistence. Through services. Through your API handler. And at each stop, a layer can do something to it: add a prefix, rename it, reinterpret its meaning, or let its internal details bleed through.

The question we sat with after our first production incident was: by the time this error reaches the top — is it still telling the truth?

**Transition → Slide 6:** "And here's what we found. Every boundary can corrupt meaning."

---

## Slide 6 — The Problem

**Header strip:** `T H E  P R O B L E M`

**Title:** Errors don't stay where they start

**Slide text:**

```
Amplified     Misinterpreted     Leaked
```

*Every layer between the error and the client can change what the error means — and whether the client gets the right response at all.*

**Speaker notes · 30 seconds:**

When errors occur in a particular layer, they don't stay there. They need to travel upstream. And as they travel through the layers, they can get amplified, misinterpreted, and impact everything upstream. That's the core problem we had to solve.

**Transition → Slide 7:** "Let me show you the architecture we were working with."

---

## Slide 7 — The Architecture

**Header strip:** `T H E  A R C H I T E C T U R E`

**Title:** The architecture we shipped

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
persistence/    → PostgreSQL + Stripe
cmd/            → wiring
```

*3 dependencies. No framework. No ORM. Every line of error logic was explicit. And it was broken.*

**Speaker notes · 1 minute:**

This is the architecture we shipped at Iknite. Five endpoints, three layers, three dependencies — no framework, no ORM. Everything was explicit. We were proud of how clean the happy path was. But we hadn't designed the error path at all. We just assumed errors would bubble up and somehow work out.

They didn't.

The architecture has three layers: handlers at the top, business logic in the middle, and persistence plus Stripe at the bottom. Every error had to travel through all three. And at every stop, something went wrong.

**Transition → Slide 8:** "Let me show you the first thing that broke."

---

┌─────────────────────────────────────────────────┐
│   WHAT BROKE: Original Code (v0.1)              │
│              Slides 8–15 · ~10 min               │
└─────────────────────────────────────────────────┘

---

## Slide 8 — Broken: Excessive Wrapping

**Header strip:** `W H A T  B R O K E  ·  1  O F  4`

**Title:** One missing row, five prefixes

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

[Switch to terminal] Let me show you what we saw in production. We request an order that doesn't exist and look at the response. One missing row — five clauses. Every layer stamped its own signature onto this error on the way out: `handler`, `service layer`, `order lookup`, `repository`, `database`. The root cause — `sql: no rows in result set` — is buried at the very end of a paragraph.

This was our first clue that something was wrong. When an engineer gets this error at 3am, they have to parse a paragraph to find the actual problem. None of these prefixes tell you anything you couldn't derive from the route and the root cause alone.

**Transition → Slide 9:** "Here's what we realized about the signal buried inside that noise."

---

## Slide 9 — What We Realized: Signal vs. Noise

**Header strip:** `W H A T  W E  R E A L I Z E D`

**Title:** Signal vs. noise

**Slide text:**

```
NOISE                                       SIGNAL
Every layer's name, retyped by hand.        The route that was called.
You already have a call stack for that.     The root cause at the bottom.
                                            Everything else is the layer
                                            congratulating itself for being involved.

persistence:  "repository: GetOrder query failed: order record not found in database: %w"
                         ↓
services:     "service layer: failed to retrieve order details: order lookup failed: %w"
                         ↓
api:          "handler: GET /orders/{id} failed: %w"
```

*Adding context is good. Adding every layer's name is noise.*

**Speaker notes · 1.5 minutes:**

Signal is the route that was called and the root cause at the bottom. Those two things are everything an on-call engineer needs to act at 3am. Noise is every layer's name stamped in between. You already have a call stack. You already have logs. The error message is not the place to reconstruct the call graph by hand.

Look at this chain: persistence stamps "repository: GetOrder query failed", service stamps "service layer: failed to retrieve order details", handler stamps "handler: GET /orders/{id} failed". Three layers, each adding its signature for no reason. Over-wrapping doesn't add information — it buries the signal under noise.

**Transition → Slide 10:** "That was the first thing we found. The second was worse."

---

## Slide 10 — Broken: Abstraction Leakage

**Header strip:** `W H A T  B R O K E  ·  2  O F  4`

**Title:** Our database leaked into our HTTP response

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

[Switch to terminal] Here's another one from our logs. See `sql: no rows in result set` in the HTTP response? That's a PostgreSQL driver message — live in an API response to an end user. Your client now knows what database you run. It also knows whether or not this product exists, because it's reading a driver-level string that was never meant to be public.

[Switch to editor] Here's why. Our `services/services.go` line 6: `import "database/sql"`. Line 9: `import "github.com/lib/pq"`. Our business layer was importing infrastructure packages directly. `errors.Is(err, sql.ErrNoRows)` — business logic branching on a database-level error type. The SQL driver's string was our API contract.

**Transition → Slide 11:** "Here's what that coupling cost us."

---

## Slide 11 — What We Realized: Swap the Store, Break the Caller

**Header strip:** `W H A T  W E  R E A L I Z E D`

**Title:** Swap the store, break the caller

**Slide text:**

```
  Postgres  →  MongoDB
                  ✕
       service breaks — no more sql.ErrNoRows

services/services.go:6     import "database/sql"      ← business layer importing infrastructure
services/services.go:66    if errors.Is(err, sql.ErrNoRows) { ... }  ← business logic, database concept
```

*Each layer's errors should be independent of every other layer.*
*We learned: translate at the boundary. The repo returns ErrProductNotFound — nothing above persistence imports database/sql.*

**Speaker notes · 1.5 minutes:**

This was the real cost of our abstraction leakage: coupling that looked invisible in the code but would have broken at runtime. Swap Postgres for MongoDB tomorrow and `errors.Is(err, sql.ErrNoRows)` stops matching — silently. The service layer isn't just aware of the database driver; it depends on it.

The goal of a layered architecture is independence: each layer's errors should be self-contained. You should be able to swap persistence without touching services. Error handling is part of that contract. When your service layer imports `database/sql`, you've broken layer independence and coupled two layers that should never know about each other. The fix is one sentence: translate at the boundary. The repo returns `ErrProductNotFound` — a domain sentinel. Nothing above persistence ever imports `database/sql` again.

**Transition → Slide 12:** "And then we found something even worse — where the error's meaning actually changed."

---

## Slide 12 — Broken: Meaning Reinterpretation

**Header strip:** `W H A T  B R O K E  ·  3  O F  4`

**Title:** Same root cause, wrong status code

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

[Switch to terminal] Same root cause as before — `p_ghost` doesn't exist. But this time we're adding it to a cart. The service layer receives `sql.ErrNoRows` and relabels it as "stock data unavailable". [Point to response] Look at the HTTP status: 503 Service Unavailable, not 404 Not Found. We were telling clients a missing product was a temporary server error. Clients that retry on 503 would hammer our servers forever on a permanently missing product.

**Transition → Slide 13:** "Here's how this relabeling broke our HTTP contract."

---

## Slide 13 — What We Realized: A Status Code Is a Promise

**Header strip:** `W H A T  W E  R E A L I Z E D`

**Title:** A status code is a promise

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

**Transition → Slide 14:** "Now the last one — and this one had real security implications."

---

## Slide 14 — Broken: External Provider Leakage

**Header strip:** `W H A T  B R O K E  ·  4  O F  4`

**Title:** Stripe's internals, in our API

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
  "error": "handler: POST /checkout failed: service layer: payment processing failed: stripe error request_id=req_7NdMtHtVhh5i2J type=card_error code=card_declined decline_code=insufficient_funds charge=ch_3RJXsD2eZvKYlo2C0B5X3Y7z: Your card has insufficient funds."
}
```

**Why it matters:**

- **Security:** Stripe request IDs and charge references fingerprint your account for attackers probing your payment stack
- **Portability:** Switch from Stripe to Braintree and every client parsing these fields breaks silently
- **Coupling:** Your HTTP contract now depends on Stripe's internal error format and field names

**Speaker notes · 1.5 minutes:**

[Switch to terminal] This was the one that got our security team's attention. We set the provider to card declined and checked out. Look at the response: `request_id=req_7NdMtHtVhh5i2J`, `type=card_error`, `code=card_declined`, `decline_code=insufficient_funds`, `charge=ch_3RJXsD2eZvKYlo2C0B5X3Y7z`. These are Stripe's internal request tracing ID, error category, machine-readable code, issuer decline reason, and internal charge reference — all in an HTTP response to an end user.

Security: a Stripe request ID and charge ID fingerprint your account. An attacker with this knows your processor, can correlate payment attempts, and has a head start probing your setup. Portability: switch to Braintree or Adyen and every client parsing these field names breaks silently. And notice: the only reason `writeError` catches this at all is a fragile `strings.Contains` check on `"stripe"`. Rename the provider tomorrow and the 402 status silently breaks.

This isn't just payments. Email providers, SMS gateways, storage APIs, auth services — any external integration. Your provider is an implementation detail. We learned to treat it like one.

**Transition → Slide 15:** "Here's exactly what leaked and how we sealed it."

---

## Slide 15 — What We Did: Translate at the Boundary

**Header strip:** `W H A T  W E  D I D`

**Title:** Translate at the boundary

**Slide text:**

```
StripeError {
    RequestID:   "req_7NdMtHtVhh5i2J"               ← Stripe request tracing ID
    Type:        "card_error"                        ← provider error category
    Code:        "card_declined"                     ← machine-readable code
    DeclineCode: "insufficient_funds"                ← issuer decline reason
    ChargeID:    "ch_3RJXsD2eZvKYlo2C0B5X3Y7z"     ← internal charge reference
    Message:     "Your card has insufficient funds." ← Stripe's raw message
}
```

*None of this should reach the client. We translate to a domain error at the persistence boundary.*

```go
// persistence — StripeError never leaves this package
func (p *StripeProvider) Charge(ctx, amount) error {
    err := stripe.Charge(ctx, amount)
    if isDeclined(err) {
        return &PaymentError{Code: "payment_declined", Retryable: false}
    }
    ...
}
```

**Speaker notes · 1.5 minutes:**

Let me break down what leaked: Stripe's request tracing ID, error category, machine-readable code, issuer decline reason, and internal charge reference. This was Stripe's internal error schema, exposed as our public API contract.

The fix is the same principle applied to external systems: the persistence layer owns the translation. `StripeError` never leaves the persistence package. It becomes `PaymentError{Code: "payment_declined", Retryable: false}` — a domain type your clients can depend on. Switch from Stripe to Braintree? You update one translation function. The service layer doesn't change. The API layer doesn't change. Your clients don't change. Your provider was always an implementation detail — now it's treated like one.

**Transition → Slide 16:** "So what was the principle that tied all of this together?"

---

## Slide 16 — The Principle We Adopted

**Header strip:** `T H E  P R I N C I P L E`

**Title:** Errors are architecture

**Slide text:**

```
Presentation   owns  → invalid request, malformed payload, bad params
Domain         owns  → business rule failures, domain invariants
Infrastructure owns  → database failures, external system failures
```

*The layer where an error originates defines its meaning. Propagate upward unchanged. Translate only at boundaries.*

**Speaker notes · 1 minute:**

So what was the fix? Errors are architecture. Each layer owns its own errors. The presentation layer owns invalid requests, malformed payloads, bad params. The domain layer owns business rule failures and domain invariants. The infrastructure layer owns database failures and external system failures. The layer where an error originates defines its meaning. Propagate upward unchanged. Translate only at boundaries.

Three rules we adopted:

1. Own your errors at the boundary — the persistence layer translates `sql.ErrNoRows` to `ErrProductNotFound`, and no layer above ever sees a database error.
2. Don't reinterpret meaning — "not found" stays "not found" all the way to the HTTP handler.
3. External systems are implementation details — translate their errors at the service boundary, your clients should never see provider codes.

**Transition → Slide 17:** "Here's what that looked like in our codebase."

---

## Slide 17 — Before & After

**Header strip:** `B E F O R E  ·  A F T E R`

**Title:** Same codebase. Errors as architecture.

**Slide text — Before (our original code):**

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

**Slide text — After (the refactor):**

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

Here's what changed in our codebase. In the original code, the persistence layer wraps without translating — `sql.ErrNoRows` propagates raw. In the refactored version, persistence translates `sql.ErrNoRows` to `ErrProductNotFound` at the boundary. The database driver never leaves the persistence package. Services no longer import `database/sql` — they just propagate errors unchanged. No wrapping, no reinterpretation. And `writeError` in the API layer now uses `errors.Is` on typed sentinels instead of string matching. No order-dependent switch cases. No fragile string comparisons. And for external providers, `StripeError` gets translated to `PaymentError` at the persistence boundary — no provider internals reach the client.

**Transition → Slide 18:** "Let me be precise about what actually changed."

---

## Slide 18 — What Actually Changed

**Header strip:** `W H A T  C H A N G E D ?`

**Title:** What actually changed?

**Slide text:**


| Before (original code)                         | After (the refactor)                                     |
| ---------------------------------------------- | -------------------------------------------------------- |
| `database/sql` imported in business logic      | No infrastructure imports in services                    |
| `sql.ErrNoRows` propagates raw to HTTP         | Translated to `ErrProductNotFound` at the boundary       |
| Service relabels "not found" as "unavailable"  | "Not found" stays "not found" through all layers         |
| `writeError` uses `strings.Contains` matching  | `writeError` uses `errors.Is` on typed sentinels         |
| `StripeError` leaks to client                  | Translated to `PaymentError` at the boundary             |
| Each layer wraps with `fmt.Errorf`             | Errors propagate unchanged unless a layer adds meaning   |


*Same stack. Same endpoints. Same database. The difference: the refactored version treats errors as architecture — owned at layer boundaries, not bolted on after.*

**Speaker notes · 1 minute:**

Let me be clear about what changed and what didn't. Same stack. Same endpoints. Same database. Same three dependencies. The only thing that changed is that we started treating errors as architecture — owned at layer boundaries, translated at boundaries, propagated upward unchanged. The persistence layer translates errors at the boundary instead of letting them leak. The service layer propagates errors instead of wrapping them. The API layer maps sentinels with `errors.Is` instead of string matching. The external provider translates its internal errors to a domain type. That's it.

**Transition → Slide 19:** "Now let me prove it. Same scenarios, after the refactor."

---

┌─────────────────────────────────────────────────┐
│          AFTER THE REFACTOR (v0.2)               │
│              Slide 19 · ~4 min                   │
└─────────────────────────────────────────────────┘

---

## Slide 19 — After the Refactor

**Header strip:** `T H E  F I X  I N  A C T I O N`

**Title:** Same scenarios, after the refactor

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

**Expected response (refactored):**

```json
{
  "error": "order not found"
}
```

**Speaker notes:** "Let me switch to the refactored branch and run the same request. [Run command] Same missing order, same root cause. But now the response is just: 'order not found.' The persistence layer translated `sql.ErrNoRows` to `ErrOrderNotFound`, the service layer propagated it unchanged, and the API layer mapped it to 404 via `errors.Is`. No paragraph, no prefixes, no database driver strings."

### Scenario 3 fix — Meaning reinterpretation → Correct status code (~1 min)

```bash
curl -i -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"p_ghost","quantity":1}'
```

**Expected response (refactored):**

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

**Expected response (refactored):**

```json
{
  "error": "payment failed: payment_declined"
}
```

**Speaker notes:** [Run commands interactively] "Now let's set the provider to card declined and checkout again. [Run commands] Look at the response: 'payment failed: payment_declined.' No request_id. No charge references. No Stripe internals. The `StripeError` was translated to a `PaymentError` at the persistence boundary — the service layer and the HTTP layer never saw provider internals. Your client gets a clean, portable error. Switch providers tomorrow and your API contract doesn't change."

**Transition → Slide 20:** "Three things we took away from this."

---

## Slide 20 — What We Learned

**Header strip:** `W H A T  W E  L E A R N E D`

**Title:** This is not about perfect errors

**Slide text:**

```
This is not about perfect errors

  Preserve   meaning, as errors cross boundaries
  Reduce     coupling between layers
  Enforce    architectural clarity
```

**Speaker notes · 1 minute:**

I want to be clear about what I'm not saying. I'm not saying your error messages need to be beautiful. I'm not saying you need an error-handling library, a framework, or a lint rule for every case.

Three things we took away. Preserve meaning as errors cross boundaries — "not found" stays "not found." Reduce coupling between layers — the service layer should not need to know what database you're running. Enforce architectural clarity — every layer's `errors.go` is a contract. It says: these are the error conditions this layer can produce. Everything else is an implementation detail.

When you treat errors as architecture from the start, you get three things in return: a system that's easier to reason about, a system that's easier to debug at 3am in a log aggregator, and a system that's easier to evolve — because your layers are genuinely decoupled.

**Transition → Slide 21:** "One final thought."

---

## Slide 21 — Close

**Header strip:** `F I N A L  T H O U G H T`

**Title:** Simplicity does not scale automatically

**Slide text:**

```
Simplicity does not scale automatically

Go gives us simple primitives. Architecture decides whether
they stay simple at scale.

Thank you
Namkat Cedrick  |  @namkatcedrickjumtock  |  github.com/namkatcedrickjumtock/e-commence
```

**Speaker notes · 30 seconds + Q&A:**

Go gives us simple primitives. `error` is an interface with one method. `errors.Is`, `errors.As`, `fmt.Errorf` — elegant, minimal, powerful. But simplicity doesn't scale automatically. Architecture determines whether these primitives stay simple as the system grows.

The repo is open source — clone it, run the original code on `v0.1`, run the refactored version on `v0.2`, and see the difference yourself. Both branches have the same endpoints, the same database, the same three dependencies. Only the error discipline changed.

Thank you for your time. Questions?

---

## Time Budget Summary


| Section                                 | Slides | Mode  | Time        | Cumulative  |
| --------------------------------------- | ------ | ----- | ----------- | ----------- |
| Title + About Me                        | 1–2    | Talk  | 1 min       | 0:00–1:00   |
| The Story                               | 3      | Talk  | 1 min       | 1:00–2:00   |
| The Illusion → The Problem              | 4–6    | Talk  | 2.5 min     | 2:00–4:30   |
| The Architecture                        | 7      | Talk  | 1 min       | 4:30–5:30   |
| Broken 1: Over-wrapping                 | 8–9    | Demo  | 3 min       | 5:30–8:30   |
| Broken 2: Abstraction Leakage           | 10–11  | Demo  | 3 min       | 8:30–11:30  |
| Broken 3: Meaning Reinterpretation      | 12–13  | Demo  | 3 min       | 11:30–14:30 |
| Broken 4: External Provider Leakage     | 14–15  | Demo  | 3 min       | 14:30–17:30 |
| The Principle                           | 16     | Talk  | 1 min       | 17:30–18:30 |
| Before & After / What Changed           | 17–18  | Talk  | 3 min       | 18:30–21:30 |
| After the Refactor                      | 19     | Demo  | 4 min       | 21:30–25:30 |
| What We Learned + Close                 | 20–21  | Talk  | 1.5 min     | 25:30–27:00 |
| Buffer for Q&A                          | —      | —     | 3 min       | 27:00–30:00 |
| **Total**                               |        |       | **~30 min** |             |

---

## After the Refactor — Quick Reference

### Switch to refactored branch

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
#           No request_id, no charge ref, no Stripe codes

# Reset
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"ok"}'
```

---

## Original Code — Quick Reference

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


| From → To     | Transition Line                                                                                       |
| ------------- | ----------------------------------------------------------------------------------------------------- |
| Slide 1 → 2   | "Let me tell you exactly what that looks like."                                                       |
| Slide 2 → 3   | "So here's the story of what happened at Iknite."                                                     |
| Slide 3 → 4   | "But the problem started somewhere simple. Something Go makes look easy."                             |
| Slide 4 → 5   | "So what actually happens when an error starts at the bottom and needs to reach the top?"             |
| Slide 5 → 6   | "And here's what we found. Every boundary can corrupt meaning."                                       |
| Slide 6 → 7   | "Let me show you the architecture we were working with."                                              |
| Slide 7 → 8   | [switch to terminal] "Let me show you the first thing that broke."                                    |
| Slide 8 → 9   | "Here's what we realized about the signal buried inside that noise."                                  |
| Slide 9 → 10  | "That was the first thing we found. The second was worse."                                            |
| Slide 10 → 11 | "Here's what that coupling cost us."                                                                  |
| Slide 11 → 12 | "And then we found something even worse — where the error's meaning actually changed."                |
| Slide 12 → 13 | "Here's how this relabeling broke our HTTP contract."                                                 |
| Slide 13 → 14 | "Now the last one — and this one had real security implications."                                     |
| Slide 14 → 15 | "Here's exactly what leaked and how we sealed it."                                                    |
| Slide 15 → 16 | "So what was the principle that tied all of this together?"                                           |
| Slide 16 → 17 | "Here's what that looked like in our codebase."                                                       |
| Slide 17 → 18 | "Let me be precise about what actually changed."                                                      |
| Slide 18 → 19 | "Now let me prove it. Same scenarios, after the refactor."                                            |
| Slide 19 → 20 | "Three things we took away from this."                                                                |
| Slide 20 → 21 | "Thank you."                                                                                          |

