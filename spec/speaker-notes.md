# Speaker Notes — Error Tracing: Lessons Learned From the Trenches
**GopherCon Europe 2026 · Namkat Cedrick**

---

## Pre-slide — Verbal opener

*Walk to the lectern. Pause. Look at the audience.*

Guten Morgen, Berlin!

*[wait for reaction, smile]*

That's about the extent of my German, so we'll be in English from here.

Before I start — a quick fact about me. I run every day, 15 minutes. If I miss a day, I add those 15 minutes to the next day. It's been a real game-changer for my fitness. Tomorrow I'm supposed to run for 3 months.

*[beat]*

My name is Namkat Cedrick. I'm a software engineer at Iknite Inc, and I want to tell you a story.

---

## Slide 01 — Title

The title says "Lessons Learned From the Trenches." I mean that literally.

This is not a talk about what the Go spec says about errors. You already know that. This is about what happened when our team built something real, shipped it fast, and didn't think seriously about error handling — until production started telling us we had to.

*Transition: "Let me tell you who I am, then let's get into it."*

---

## Slide 02 — About Me

Software engineer at Iknite Inc. I also organize the West and Central African Gophers community, which has been one of the more rewarding things I've done outside of engineering work.

The code you're about to see — I wrote it. The problems I'm going to show you — I hit them on a real project at Iknite. The app on screen is a sample I built specifically for this talk to demonstrate those patterns cleanly. The e-commerce wrapper is just the vehicle. The mistakes in it are ones we actually made.

*Transition: "So here's what happened."*

---

## Slide 03 — The Overview · *What you're about to see*

At Iknite we were building a backend service — not e-commerce, but the error patterns we hit are the same regardless of what the domain is. And three specific things failed in ways we hadn't anticipated.

One: every layer was stamping its name on every error on the way out.

Two: our business logic was importing database drivers directly.

Three: our payment provider's internal error codes were reaching API clients.

The footer on this slide says "The fix wasn't discipline. It was architecture." That's the thesis. By the end of this talk you'll have seen exactly what that means in code.

*Transition: "But let me start with the assumption we had going in."*

---

## Slide 04 — The Illusion · *`if err != nil { return err }`*

We all write this. Every Go engineer in this room has written this exact pattern, probably today.

```go
if err != nil {
    return err  // probably wrap with some more context...
}
```

It looks like error handling. It compiles. The tests pass. But this is the illusion: returning an error is not the same as deciding what that error *means* for the layer receiving it. In a single-layer program, this is fine. In a layered system with persistence, business logic, and an API — this is the starting point of a problem.

*Transition: "Because here's what actually happens to that error."*

---

## Slide 05 — The Question · *What happens when an error travels up through the layers?*

You get `sql: no rows in result set` at the bottom of your stack. That's a fact — a row wasn't there. Simple.

But this error doesn't just teleport to the client. It travels. Through persistence. Through services. Through your API handler. And at every stop, a layer can do something to it: add a prefix, rename it, reinterpret its meaning, or just let its internals bleed straight through.

The question we had to sit with after our first production incident was simple: by the time this error reaches the browser — is it still telling the truth?

*Transition: "It usually isn't. And here's why."*

---

## Slide 06 — The Problem · *Errors don't stay where they start*

When errors travel through layers they fail in one of three ways.

They get **amplified** — buried under a chain of layer signatures so long the root cause is a footnote.

They get **misinterpreted** — a layer renames the error and now the meaning has changed, and so has the HTTP status code the client gets.

They get **leaked** — internal implementation details that were never meant to be public end up in an API response.

Every layer between the error origin and the client is a point where meaning can change. The rest of this talk is one concrete example of each of those three.

*Transition: "Let me show you the system I'm using to demonstrate this."*

---

## Slide 07 — The Setup · *Small E-commerce API*

Here's the sample app. Five endpoints. Three layers: presentation, business logic, and datastore. Postgres at the bottom. Three dependencies — no framework, no ORM.

The reason I built it this way is so every error decision is explicit code. There's nowhere to hide. When something goes wrong with error handling in this app, you can see exactly which line caused it. No magic, no middleware abstracting it away.

*Transition: "And here are the three patterns I want to walk through."*

---

## Slide 08 — The Map · *3 Recurring Anti-Patterns*

Three patterns. We hit all three on the same project, independently, at different points.

**Excessive wrapping** — every layer stamps its own signature on the error.

**Tight coupling between layers** — database contracts leaking into business logic.

**External provider leakage** — a third-party system's error schema becoming your API contract.

Every one of these is in the sample app on purpose. Let's go through them one at a time.

*Transition: "Starting with anti-pattern one."*

---

## Slide 09 — Anti-Pattern 01 Divider · *Excessive Wrapping*

"Every layer stamps its own signature onto the error on the way out."

---

## Slide 10 — Scenario 1 · Architecture · *GetOrder code*

Here's the code that produced the problem.

Persistence layer — `GetOrder` in `postgres.go`:
```
"repository: GetOrder query failed: order record not found in database: %w"
```

Service layer — `GetOrder` in `services.go`:
```
"service layer: failed to retrieve order details: order lookup failed: %w"
```

Two layers. Two `fmt.Errorf` calls. Both signing their name. Neither of these strings tells you anything you couldn't derive from the route that was called and the root cause at the bottom. The call stack already has this information. The error message is just duplicating it.

---

## Slide 11 — Scenario 1 · Error Chain · *When every layer adds its two cents*

This diagram shows what that looks like as the error travels up.

At the bottom — the origin: `sql: no rows in result set`. That's the actual fact.

Persistence wraps it. Service wraps that. API handler wraps that again. By the time the client gets the response, the error looks like a paragraph — and the actual cause is at the very end.

*Transition: "Here's exactly what that response looks like."*

---

## Slide 12 — Scenario 1 · Evidence · *curl /orders/o_ghost*

One curl request. One missing order. Look at the response.

`handler: GET /orders/{id} failed: service layer: failed to retrieve order details: order lookup failed: repository: GetOrder query failed: order record not found in database: sql: no rows in result set`

I've highlighted the layer signatures — `handler:`, `service layer:`, `order lookup failed:`, `repository:` — four of them, all in red, all before you get to the actual cause.

If you're an on-call engineer reading this at 3am in a log aggregator, you're parsing a paragraph to find five words of signal. The prefixes don't add information. They add noise.

*Transition: "So what's the right way to think about this?"*

---

## Slide 13 — Scenario 1 · Lesson · *Sweet spot — under vs. over wrapped*

The spectrum on screen shows this clearly. Both extremes are wrong.

Over-wrapped: every layer stamps its name. The root cause is buried under prefixes you already have in your call stack and your logs.

Under-wrapped: the error is so vague you don't know where it came from or what caused it.

The sweet spot in the middle: root cause plus minimal context. Enough to trace the origin — not so much that you've written the call graph in a string.

You already have logs. You already have a stack trace. The error message is not the place to reconstruct that.

*Transition: "That was anti-pattern one. The second one is more dangerous because it looks structural rather than cosmetic."*

---

## Slide 14 — Anti-Pattern 02 Divider · *Abstraction Leakage*

"Errors from one layer bleed into another."

This one came with a hidden cost we didn't see until we tried to swap out a dependency.

---

## Slide 15 — Scenario 2 · Evidence · *`sql: no rows in result set` in the HTTP response*

`curl /products/p_ghost`. Look at the response.

Right there, highlighted: `sql: no rows in result set`.

That is a PostgreSQL driver string in an HTTP API response. Your client now knows what database you're running. It also knows how to probe for absent records by reading driver-level strings that were never meant to be part of your API contract.

This isn't just an embarrassing message. It's a coupling problem that has real consequences when the system evolves.

*Transition: "Here's the line that caused it."*

---

## Slide 16 — Scenario 2 · Code · *`services/` importing `database/sql`*

Two panels. Left side: `services/services.go` — the imports.

`"database/sql"` — highlighted in red.
`"github.com/lib/pq"` — highlighted in red.

Our business layer is importing infrastructure packages. That's the problem in two lines.

Right side: the function that uses it — `errors.Is(err, sql.ErrNoRows)` — business logic branching on a database-level error type. The SQL driver's string became our API contract because the service layer was making decisions based on it.

*Transition: "And here's what that coupling actually costs you when the system changes."*

---

## Slide 17 — Scenario 2 · Lesson · *Swap the store, break the caller*

Swap Postgres for MongoDB. `errors.Is(err, sql.ErrNoRows)` — stops matching. Silently. No compile error, no test failure, just wrong behavior in production. The service layer isn't just *aware* of the database driver — it *depends* on it.

The goal of a layered architecture is independence. The service layer should be able to call the persistence layer without knowing what database is behind it. When your service layer imports `database/sql`, you've broken that independence and coupled two things that should never know about each other.

The fix is one rule: translate at the boundary. The persistence layer returns `ErrProductNotFound` — a domain sentinel. Nothing above persistence ever sees `sql.ErrNoRows` again.

*Transition: "Now the last pattern. This one had real security implications for us."*

---

## Slide 18 — Anti-Pattern 04 Divider · *External Provider Leakage*

"An external system's error contract bleeds into yours."

This is the payment one. This is the one that got our security team involved.

---

## Slide 19 — Scenario 4 · Evidence · *Stripe internals in the API*

`curl -X POST localhost:8080/checkout`. Look at what comes back.

`request_id=req_7NdMtHtVhh5i2J` — that's Stripe's internal request tracing ID.
`type=card_error` — error category.
`code=card_declined` — machine-readable code.
`decline_code=insufficient_funds` — the issuer's decline reason.
`charge=ch_3RJXsD2eZvKYlo2C0B5X3Y7z` — an internal charge reference.

All of this in an HTTP response to a browser.

A Stripe request ID and charge ID fingerprint your account. An attacker reading these responses knows your processor, can correlate payment attempts across users, and has a head start probing your setup. And notice — this is also a portability problem. The moment you switch from Stripe to Braintree, every client that's been parsing these field names breaks. Silently.

*Transition: "Here's the code that let this happen."*

---

## Slide 20 — Scenario 4 · Code · *StripeError returned directly*

Left panel: `persistence/stripe.go` — the `Charge` function.

It returns `&StripeError` directly — the full struct, every field, exactly as Stripe sent it. `RequestID`, `Type`, `Code`, `DeclineCode`, `ChargeID`, `Message` — all of it handed straight to the caller.

Right panel: `services/checkout.go` — the service layer.

```go
return nil, fmt.Errorf("payment processing failed: %w", err)
```

`%w` wraps the error — which means all of `StripeError`'s internals are preserved in the chain and eventually reach the HTTP response through `writeError`. Nobody translated. Nobody decided what the client should see. The provider's error schema became the API schema by accident.

*Transition: "Here's what that fix looks like."*

---

## Slide 21 — Scenario 4 · Lesson · *Translate at the boundary*

Left side shows what leaked — the full `StripeError` struct with all its fields. Right side shows the fix:

```go
func (p *StripeProvider) Charge(...) error {
    err := stripe.Charge(ctx, amount)
    if isDeclined(err) {
        return &PaymentError{
            Code:      "payment_declined",
            Retryable: false,
        }
    }
}
```

`StripeError` never leaves the persistence package. It's translated into `PaymentError` — a domain type that your callers can actually depend on. The footer says it clearly: "StripeError never leaves the persistence package. It becomes a domain type your clients can depend on."

Switch from Stripe to Braintree tomorrow — you update one translation function inside persistence. The service layer doesn't change. The API layer doesn't change. Your clients don't change. Your provider was always an implementation detail. Now it's treated like one.

This applies to any external integration — email providers, SMS gateways, storage APIs, auth services. The pattern is the same: translate at the boundary, keep provider internals out of your domain.

*Transition: "So what's the principle that ties all three of these together?"*

---

## Slide 22 — Takeaways · *This is not about perfect errors*

I want to be clear about what I'm not saying.

I'm not saying your error messages need to be beautiful. I'm not recommending a specific library or a lint rule for every case. This isn't about aesthetics.

Three things: **Preserve**, **Reduce**, **Enforce**.

**Preserve** — meaning, as errors cross boundaries. "Not found" stays "not found" all the way from the database to the HTTP response. No layer renames it.

**Reduce** — coupling between layers. The service layer should not know what database you're running, and it should not know which payment processor you use. That's the persistence layer's concern.

**Enforce** — architectural clarity. Every layer's `errors.go` is a contract. It says: these are the error conditions this layer can produce. Everything else is an implementation detail.

When you treat errors as part of your architecture from the start — not something you patch in after a production incident — you get a system that's easier to reason about, easier to debug, and easier to evolve.

*Transition: "One last thought before I let you go."*

---

## Slide 23 — Final Thought · *Simplicity does not scale automatically*

Go's error model is one of the simplest I've worked with. `error` is an interface with one method. `errors.Is`, `errors.As`, `fmt.Errorf` — elegant, minimal, powerful.

But simplicity doesn't inherit. A system with a handful of layers that all do `return fmt.Errorf("...: %w", err)` without thinking about what they're communicating is not simple. It's deferred complexity.

Architecture decides whether these primitives stay simple as the system grows. That's the work.

---

## Slide 24 — Thank You

Thank you.

The repo is open — `github.com/namkatcedrickjumtock/e-commence`. Both the original code and the refactored version are there. You can run the same requests, see the broken responses, and see the fixed ones side by side.

I'm happy to take questions.

*Danke schön.*

---

## Slide 25 — Empty

*Q&A buffer. No notes needed.*
