# Error Tracing: Lessons Learned From the Trenches
## GopherCon Europe 2026 · Berlin — Speaker Outline v2

**Namkat Cedrick · Software Engineer, Iknite Inc**
**Total time: 30 minutes (~15 min talk, ~15 min live demo)**

> This outline maps to the 20-slide Claude Design deck.
> Live demo sections are marked with ┌─ blocks and happen in the terminal — no slide shown.

---

## Slide 1 — Title

**Header strip:** `G O P H E R C O N  E U R O P E  2 0 2 6  ·  F R O M  T H E  T R E N C H E S`

**Slide text:**
```
Error Tracing
Lessons Learned From the Trenches

Error handling as architecture in a layered Go system.

Namkat Cedrick
Software Engineer · Iknite Inc
Organizer · West / Central African Gophers
linkedin.com/in/namkatcedrick · github.com/namkatcedrickjumtock
```

**Speaker notes · 30 seconds:**

Good morning, everyone. My name is Namkat Cedrick, I'm a software engineer at Iknite Inc, and it's a real pleasure to be here in Berlin. I want to thank the organizers for this opportunity and thank all of you for choosing to spend the next half hour in this room.

I'll be honest with you — this is my first time presenting on a stage like this. The title is "Lessons Learned From the Trenches," and I mean the trenches. What I'm going to share today is a story from my own experience: how I didn't treat errors as part of my architecture, how I kept kicking that can down the road, until one day it stopped being my problem alone and became a problem for my whole team.

**Transition → Slide 2:** "And it starts with something Go makes look deceptively simple."

---

## Slide 2 — The Illusion

**Header strip:** `W H E R E  I T  B E G I N S`

**Slide text:**
```
The illusion — at least the one I tell myself

"Go error handling is simple."
                               the whole story

 8  if err != nil {
 9      return err
10  }
```

**Speaker notes · 1 minute:**

Most of us know that Go was designed with simplicity in mind. I've always carried that concept in the back of my mind: if something happens, wrap and return it to the caller and voilà, you're done.

And for small programs? It holds. A CLI tool. A short script. Two layers. It holds beautifully.

But `if err != nil { return err }` is not the whole story. It's the beginning of the story. Because in a layered system, errors don't just return — they *travel*. They cross boundaries. And every boundary is a place where meaning can change.

**Transition → Slide 3:** "So what actually happens to an error as it travels up through the layers?"

---

## Slide 3 — The Question

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

Let's look at a concrete example. You get a database error at the bottom of your stack — `sql: no rows in result set`. A record wasn't found. Simple.

But this error doesn't just teleport to the client. It travels. Through persistence. Through services. Through your API handler. And at each stop, a layer can do something to it: add a prefix, rename it, reinterpret its meaning, or let its internal details bleed through.

The question I want you to sit with for the next fifteen minutes is: by the time this error reaches the top — is it still telling the truth?

**Transition → Slide 4:** "And here's what I've found. Every boundary can corrupt meaning."

---

## Slide 4 — The Problem

**Header strip:** `T H E  P R O B L E M`

**Slide text:**
```
Every boundary can corrupt meaning

add context       the good instinct — orient the reader
dilute · relabel  the same instinct, run too far
leak internals    infrastructure escapes its layer

The problem isn't that errors exist.
It's that nobody decided what happens to one when it crosses a line.
```

**Speaker notes · 1 minute:**

There are three things that happen to errors at boundaries, and two of them are problems.

Adding context — good. Telling the reader which operation failed, on what data — that's useful. That's signal.

Diluting or relabeling — dangerous. When a layer receives "product not found" and emits "stock data unavailable," it changed the *meaning* of the error. The caller downstream now has a different understanding of reality than the database had.

Leaking internals — architectural debt. When a database driver string, or a payment provider's internal code, reaches your HTTP response, you've made your implementation detail part of your API contract.

The root cause isn't that individual developers made bad choices. It's that nobody drew the line. Nobody said: *this layer owns these errors, and nothing crosses that line unchanged.*

**Transition → Slide 5:** "Here are the four specific patterns I see this become. Over and over."

---

## Slide 5 — The Map

**Header strip:** `T H E  M A P`

**Slide text:**
```
Four ways errors go wrong

  1  Excessive wrapping
  2  Abstraction leakage
  3  Meaning reinterpreted
  4  Provider internals leak
```

**Speaker notes · 1 minute:**

Here are four anti-patterns. I'm using a demo app to show all four in action — a minimal e-commerce backend: products, cart, checkout, orders. Three dependencies. No framework, no ORM. Every line of error logic is explicit on purpose.

I'm going to show you the bad version first — where all four live — and then after we talk about the fix, switch to the good version and run the same scenarios.

Let me note that anti-pattern 4 is not specific to payments. Any time you integrate with an external system — email, SMS, storage, auth — the same leakage pattern applies.

**Transition → Slide 6:** [switch to terminal, then return to slide] "Let's start with the first one."

---

┌─────────────────────────────────────────────────┐
│         DEMO SETUP — run before Slide 6          │
│  git checkout v0.1 && make run                   │
│  curl -s -X POST localhost:8080/demo/reset | jq  │
│  curl -s -X POST localhost:8080/demo/seed  | jq  │
│  (copy a returned product id for scenarios 3+4)  │
└─────────────────────────────────────────────────┘

---

## Slide 6 — Anti-Pattern 01: Intro Card

**Slide text:**
```
01
Excessive Wrapping

The good instinct — add context — runs away with you,
and every layer stamps its own name on the way out.
```

**Speaker notes · 20 seconds:**

The first pattern. Every layer adds a prefix. The good instinct — add context — runs away with you. Let me show you what that looks like in practice.

**Transition → Slide 7:** [switch to terminal] "Watch what happens when I request an order that doesn't exist."

---

## Slide 7 — Anti-Pattern 1 · Evidence

**Header strip:** `A N T I - P A T T E R N  1  ·  T H E  E V I D E N C E`

**Slide text:**
```
One missing row, five clauses

GET /orders/o_ghost · HTTP 500         V0.1

handler: GET /orders/{id} failed: service layer: failed to retrieve
order details: order lookup failed: repository: GetOrder query failed:
order record not found in database: sql: no rows in result set
```

**Live demo commands:**
```bash
curl localhost:8080/orders/o_ghost | jq .
```

**Speaker notes · 1.5 minutes:**

[Switch to terminal] Let me run this. [Run command] One missing row. Five prefixes — `handler`, `service layer`, `order lookup`, `repository`, `database`. Every layer stamped its own name onto this error on the way out.

Count them: five clauses for a row that doesn't exist. The real cause — `sql: no rows in result set` — is buried at the very end of a paragraph.

None of these prefixes tell the caller anything they couldn't figure out from the route and the root cause. If this shows up in a log aggregator at 3am, the on-call engineer has to read through five clauses to find what actually failed.

**Transition → Slide 8:** "Let me show you why this is noise, not signal."

---

## Slide 8 — Anti-Pattern 1 · Lesson

**Header strip:** `A N T I - P A T T E R N  1  ·  T H E  L E S S O N`

**Slide text:**
```
Signal vs. noise

NOISE                               SIGNAL
Every layer's name is just the      The route and the root cause.
call stack, retyped by hand.        Everything between is the layer
You already have stack traces       congratulating itself for being
for that.                           involved.

Adding context is good. Adding every layer's name is noise.
```

**Speaker notes · 1 minute:**

Here's the mental model I use. Signal: the route that was called, and the root cause. Those two things together are everything an engineer needs to act. Noise: every layer's name, stamped in between. You already have a call stack. You already have logs. The error message is not the place to reconstruct the call graph.

Over-wrapping doesn't add information — it buries it. The fix isn't to stop wrapping. It's to wrap *only when a layer genuinely adds meaning*. The persistence layer telling you it was a GetOrder lookup? Signal. The service layer telling you it was *also* involved? Noise.

**Transition → Slide 9:** "That's anti-pattern 1. Anti-pattern 2 is quieter — and in some ways more dangerous."

---

## Slide 9 — Anti-Pattern 02: Intro Card

**Slide text:**
```
02
Abstraction Leakage

Quieter, more dangerous — because the code
looks completely reasonable.
```

**Speaker notes · 20 seconds:**

This one is my favorite to show because when you look at the code, it seems totally fine. A simple `errors.Is` check. Completely idiomatic Go. But it's not fine.

**Transition → Slide 10:** [switch to terminal and editor] "Let me show you."

---

## Slide 10 — Anti-Pattern 2 · Evidence

**Header strip:** `A N T I - P A T T E R N  2  ·  T H E  E V I D E N C E`

**Slide text:**
```
database/sql in the business layer

services/services.go

import "database/sql"
import "github.com/lib/pq"

// business logic, deciding on a
// database concept:
if errors.Is(err, sql.ErrNoRows) {
    …
}

→ Infrastructure packages, imported inside the business layer.
→ The driver string rides all the way out — the client
  knows you run Postgres.

The persistence contract has leaked through services and out the front door.
```

**Live demo commands:**
```bash
curl localhost:8080/products/p_ghost | jq .
# Response contains: "sql: no rows in result set"
```

**Speaker notes · 1.5 minutes:**

[Switch to terminal] Let me request a product that doesn't exist. [Run command] See the response? `sql: no rows in result set` — a PostgreSQL driver message, live in an HTTP response. Your client now knows you run Postgres. The SQL driver's string format is part of your API contract.

[Switch to editor] Here's why. `services/services.go` line 6: `import "database/sql"`. Line 9: `import "github.com/lib/pq"`. Your business layer is importing infrastructure packages. And look at the condition: `errors.Is(err, sql.ErrNoRows)` — business logic branching on a database concept.

**Transition → Slide 11:** "What happens when you swap the store?"

---

## Slide 11 — Anti-Pattern 2 · Lesson

**Header strip:** `A N T I - P A T T E R N  2  ·  T H E  L E S S O N`

**Slide text:**
```
If we swap the store, we break the caller

  Postgres  →  MongoDB
                  ✕
           service breaks — no more sql.ErrNoRows

The fix is one sentence: translate at the boundary.
The repo returns ErrProductNotFound;
nothing above persistence imports database/sql.
```

**Speaker notes · 1 minute:**

Swap Postgres for MongoDB tomorrow and this code breaks. Not because MongoDB is wrong — because the service layer was never supposed to know which database you're using. That's the persistence layer's job.

The fix is one sentence: translate at the boundary. `Get` returns `ErrProductNotFound` — a domain sentinel. Nothing above persistence imports `database/sql`. The service layer doesn't care what the storage engine returned. It cares that the product wasn't found. Those are different things.

**Transition → Slide 12:** "Anti-pattern 3 builds on this. Same root cause — but now the meaning changes."

---

## Slide 12 — Anti-Pattern 03: Intro Card

**Slide text:**
```
03
Meaning Gets Reinterpreted

A layer doesn't just wrap the error — it changes
what the error means.
```

**Speaker notes · 20 seconds:**

This one has a runtime consequence. It's not just a code smell — it breaks your clients.

**Transition → Slide 13:** [switch to terminal] "Same product ID. Same root cause. Different path through the code."

---

## Slide 13 — Anti-Pattern 3 · Evidence

**Header strip:** `A N T I - P A T T E R N  3  ·  T H E  E V I D E N C E`

**Slide text:**
```
A missing product returns 503

root cause               →  relabeled in services  →  writeError matches "unavailable"
product p_ghost                "stock data                          503
  doesn't exist              unavailable"

A permanent 404 became a temporary 503 —
so the client's retry logic hammers a product
that will never exist.
```

**Live demo commands:**
```bash
curl -i -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"p_ghost","quantity":1}'
# HTTP/1.1 503 Service Unavailable
```

**Speaker notes · 1.5 minutes:**

[Switch to terminal] Same product ID, `p_ghost`. Same root cause — the product doesn't exist. But this time we're adding it to a cart instead of fetching it directly. [Run command] Look at that status code: 503 Service Unavailable. Not 404.

Here's why. `AddProductToCart` receives `sql.ErrNoRows` and relabels it: "stock data unavailable." Then `writeError` checks strings in order — `"unavailable"` matches before `"no rows"` — so the client gets 503.

A client that retries on 503 will now hammer your server on a product that will never exist. That's exactly the wrong behavior.

**Transition → Slide 14:** "Because a status code is a promise."

---

## Slide 14 — Anti-Pattern 3 · Lesson

**Header strip:** `A N T I - P A T T E R N  3  ·  T H E  L E S S O N`

**Slide text:**
```
A status code is a promise

THE CONTRACT                    THE BREACH
404 means stop.                 Relabel a not-found as "unavailable"
503 means retry.                and a final answer becomes an
The client obeys, literally.    infinite retry.

A layer may enrich an error — never change its category.
```

**Speaker notes · 1 minute:**

404 means: this resource doesn't exist. Stop asking. 503 means: I'm temporarily unavailable. Try again. The client obeys these codes literally — that's the whole point of HTTP semantics.

When you relabel a "not found" as "unavailable," you've broken a contract. You told the client something false. And in this case you told it: keep trying. Against a product that will never exist.

The rule I use: a layer may enrich an error — add context, add specificity — but it may never change its *category*. Not found stays not found. Unavailable stays unavailable. These are different meanings.

**Transition → Slide 15:** "And now the last one — this one should make the security folks in the room sit up."

---

## Slide 15 — Anti-Pattern 04: Intro Card

**Slide text:**
```
04
Provider Internals Leak

The one that should make the security folks
in the room sit up.
```

**Speaker notes · 20 seconds:**

Payment providers. External APIs. SMS gateways. Anything you integrate with. The same pattern.

**Transition → Slide 16:** [switch to terminal] "Let me set the payment provider to a failure mode and checkout."

---

## Slide 16 — Anti-Pattern 4 · Evidence

**Header strip:** `A N T I - P A T T E R N  4  ·  T H E  E V I D E N C E`

**Slide text:**
```
Flutterwave's topology, in your API

POST /checkout · HTTP 500                                              V0.1

…payment processing failed: flutterwave error FW-9082 region=eu-west
retryable=false tx_ref=FW-TXN-84712947: CARD_DECLINED: insufficient_funds,
issuer_code=05, network=VISA_EU

security   provider codes, regions and networks —
           a gift to anyone probing your stack.

portability  migrate off Flutterwave and every client
             parsing these breaks silently.
```

**Live demo commands:**
```bash
# Step 1 — add a product to cart (use id from seed output)
curl -s -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"<id>","quantity":1}' | jq .

# Step 2 — set provider to failure mode
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"card_declined"}' | jq .

# Step 3 — checkout and see the leak
curl -s -X POST localhost:8080/checkout | jq .

# Reset when done
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"ok"}' | jq .
```

**Speaker notes · 1.5 minutes:**

[Switch to terminal] Let me add a product to the cart first, then switch the payment provider to a failure mode, then checkout. [Run commands]

Look at the response body. `FW-9082`. `region=eu-west`. `tx_ref=FW-TXN-84712947`. `issuer_code=05`. `network=VISA_EU`. These are the provider's internal error code, deployment region, transaction reference, and acquirer network name — all in an HTTP response to an end user.

Two problems. Security: provider codes and regions are reconnaissance for attackers probing your payment stack. They know your provider. They know your region. They know it's Visa and it's the EU acquirer. Portability: switch from Flutterwave to Stripe tomorrow and every client parsing these codes breaks silently — they never even notice until their code starts failing.

**Transition → Slide 17:** "The fix is always the same: translate at the boundary."

---

## Slide 17 — Anti-Pattern 4 · Lesson

**Header strip:** `A N T I - P A T T E R N  4  ·  T H E  L E S S O N`

**Slide text:**
```
Translate external integration
errors at the boundaries
```

**Speaker notes · 1.5 minutes:**

The persistence layer is the only layer that should ever see a `FlutterwaveError`. At that boundary, you translate it to a `PaymentError` — your domain type. `{Code: "payment_declined", Retryable: false}`. That's it. Everything above persistence sees a domain concept, not a vendor concept.

Switch providers tomorrow? You update the translation in one place. The service layer doesn't change. The API layer doesn't change. Your clients don't change. Because your API contract was never "here's what Flutterwave said" — it was "here's what happened to your payment."

And again — this isn't just payments. Email providers, SMS gateways, cloud storage, auth services. Any external integration. Your provider is an implementation detail. Treat it like one.

---

┌─────────────────────────────────────────────────┐
│          TRANSITION — switch to v0.2             │
│  "Let me now show you what the fix looks like."  │
│  git checkout v0.2                               │
│  make run                                        │
└─────────────────────────────────────────────────┘

**Speaker notes during switch:**

I'm going to switch branches now to v0.2 — the same app, same stack, same database, same three dependencies. The only thing that changed is the error discipline. Let me run the same scenarios.

---

## Slide 18 — What Changed?

**Header strip:** `W H A T  C H A N G E D ?`

**Slide text:**
```
Not the errors — the architecture

  easier to   reason about
  easier to   debug — at 3am, in a log aggregator
  easier to   evolve — swap a database or a payment provider
```

**Live demo (v0.2 — run all three during this slide):**

```bash
# Scenario 1 fix — clean error, no prefixes
curl localhost:8080/orders/o_ghost | jq .
# Expected: {"error": "order not found"}  → 404

# Scenario 3 fix — correct status code
curl -i -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"p_ghost","quantity":1}'
# Expected: HTTP/1.1 404 Not Found
#           {"error": "adding to cart: product not found"}

# Scenario 4 fix — translated payment error
curl -s -X POST localhost:8080/demo/reset | jq .
curl -s -X POST localhost:8080/demo/seed  | jq .
# (add product to cart, set card_declined, checkout)
curl -s -X POST localhost:8080/checkout | jq .
# Expected: {"error": "payment failed: payment_declined"}  → 402
```

**Speaker notes · 2.5 minutes:**

[Run Scenario 1] Same missing order. Response: "order not found." That's it. The persistence layer translated `sql.ErrNoRows` to `ErrOrderNotFound`, the service layer propagated it unchanged, and the API layer mapped it to 404 via `errors.Is`. No paragraph, no prefixes, no database driver strings.

[Run Scenario 3] Same product ID, `p_ghost`, same missing product. But now: 404 Not Found — not 503. The error wasn't relabeled. The meaning traveled through the layers intact.

[Run Scenario 4] Same failure mode. Response: "payment failed: payment_declined." No FW-9082. No eu-west. No transaction references. No acquirer codes. The `FlutterwaveError` was translated to a `PaymentError` at the persistence boundary. Switch providers tomorrow — your API contract doesn't change.

Same stack. Same endpoints. Same database. Only the error discipline changed.

**Transition → Slide 19:** "And I want to be precise about what that means."

---

## Slide 19 — Important Clarification

**Header strip:** `I M P O R T A N T  C L A R I F I C A T I O N`

**Slide text:**
```
This is not about perfect errors

  Preserve   meaning, as errors cross boundaries
  Reduce     coupling between layers
  Enforce    architectural clarity
```

**Speaker notes · 1 minute:**

I want to be clear about what I'm not saying. I'm not saying your error messages need to be beautiful. I'm not saying you need an error-handling library, or a framework, or a lint rule for every case.

Three things. Preserve meaning as errors cross boundaries — "not found" stays "not found." Reduce coupling between layers — the service layer should not need to know what database you're running. Enforce architectural clarity — every layer's `errors.go` is a contract. It says: these are the error conditions this layer can produce. Everything else is an implementation detail.

When you treat errors as architecture from the start, you get three things in return: a system that's easier to reason about, a system that's easier to debug at 3am in a log aggregator, and a system that's easier to evolve — because your layers are genuinely decoupled.

**Transition → Slide 20:** "One final thought."

---

## Slide 20 — Final Thought

**Header strip:** `F I N A L  T H O U G H T`

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

The repo is open source — clone it, run the bad version on `v0.1`, run the good version on `v0.2`, and see the difference yourself. Both branches have the same endpoints, the same database, the same three dependencies. Only the error discipline changed.

Thank you for your time. Questions?

---

## Time Budget Summary

| Section | Slides | Mode | Time | Cumulative |
|---|---|---|---|---|
| Title | 1 | Talk | 0:30 | 0:00–0:30 |
| The Illusion → The Map | 2–5 | Talk | 3:30 | 0:30–4:00 |
| Demo setup + Anti-pattern 01 card | 6 + setup | Demo | 1:00 | 4:00–5:00 |
| AP 1 — Evidence + Lesson | 7–8 | Demo + Talk | 2:30 | 5:00–7:30 |
| Anti-pattern 02 card | 9 | Talk | 0:20 | 7:30–7:50 |
| AP 2 — Evidence + Lesson | 10–11 | Demo + Talk | 2:30 | 7:50–10:20 |
| Anti-pattern 03 card | 12 | Talk | 0:20 | 10:20–10:40 |
| AP 3 — Evidence + Lesson | 13–14 | Demo + Talk | 2:30 | 10:40–13:10 |
| Anti-pattern 04 card | 15 | Talk | 0:20 | 13:10–13:30 |
| AP 4 — Evidence + Lesson | 16–17 | Demo + Talk | 3:00 | 13:30–16:30 |
| v0.2 switch + What Changed? | 18 | Demo | 3:00 | 16:30–19:30 |
| Important Clarification | 19 | Talk | 1:00 | 19:30–20:30 |
| Final Thought | 20 | Talk | 0:30 | 20:30–21:00 |
| Buffer for overrun + Q&A | — | — | 9:00 | 21:00–30:00 |
| **Total** | | | **~30 min** | |

---

## Demo Quick Reference

### Branch setup
```bash
# Bad version
git checkout v0.1
make run

# Good version (after principle slides)
git checkout v0.2
make run
```

### Reset + seed (run before each demo block)
```bash
curl -s -X POST localhost:8080/demo/reset | jq .
curl -s -X POST localhost:8080/demo/seed  | jq .
# copy a product id from the seed output — needed for scenarios 3 and 4
```

### v0.1 scenarios

```bash
# Scenario 1 — Excessive wrapping
curl localhost:8080/orders/o_ghost | jq .

# Scenario 2 — Abstraction leakage
curl localhost:8080/products/p_ghost | jq .

# Scenario 3 — Meaning reinterpretation (503 instead of 404)
curl -i -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"p_ghost","quantity":1}'

# Scenario 4 — Provider leakage
curl -s -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"<id>","quantity":1}' | jq .
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"card_declined"}' | jq .
curl -s -X POST localhost:8080/checkout | jq .
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"ok"}' | jq .
```

### v0.2 scenarios (run during Slide 18)

```bash
# Scenario 1 fix — "order not found" → 404
curl localhost:8080/orders/o_ghost | jq .

# Scenario 3 fix — 404, not 503
curl -i -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"p_ghost","quantity":1}'

# Scenario 4 fix — "payment failed: payment_declined" → 402
curl -s -X POST localhost:8080/demo/reset | jq .
curl -s -X POST localhost:8080/demo/seed  | jq .
curl -s -X POST localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id":"<id>","quantity":1}' | jq .
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"card_declined"}' | jq .
curl -s -X POST localhost:8080/checkout | jq .
curl -s -X POST localhost:8080/demo/payment-mode \
  -H "Content-Type: application/json" \
  -d '{"mode":"ok"}' | jq .
```

---

## Transition Cheat Sheet

| From → To | Transition line |
|---|---|
| Slide 1 → 2 | "And it starts with something Go makes look deceptively simple." |
| Slide 2 → 3 | "So what actually happens to an error as it travels up through the layers?" |
| Slide 3 → 4 | "And here's what I've found. Every boundary can corrupt meaning." |
| Slide 4 → 5 | "Here are the four specific patterns I see this become. Over and over." |
| Slide 5 → 6 | [switch to terminal, run setup, return to slide] "Let's start with the first one." |
| Slide 6 → 7 | [terminal] "Watch what happens when I request an order that doesn't exist." |
| Slide 7 → 8 | "Let me show you why this is noise, not signal." |
| Slide 8 → 9 | "That's anti-pattern 1. Anti-pattern 2 is quieter — and in some ways more dangerous." |
| Slide 9 → 10 | [terminal + editor] "Let me show you." |
| Slide 10 → 11 | "What happens when you swap the store?" |
| Slide 11 → 12 | "Anti-pattern 3 builds on this. Same root cause — but now the meaning changes." |
| Slide 12 → 13 | [terminal] "Same product ID. Same root cause. Different path through the code." |
| Slide 13 → 14 | "Because a status code is a promise." |
| Slide 14 → 15 | "And now the last one — this one should make the security folks in the room sit up." |
| Slide 15 → 16 | [terminal] "Let me set the payment provider to a failure mode and checkout." |
| Slide 16 → 17 | "The fix is always the same: translate at the boundary." |
| Slide 17 → 18 | [switch to v0.2 branch] "Let me now show you what the fix looks like." |
| Slide 18 → 19 | "And I want to be precise about what that means." |
| Slide 19 → 20 | "One final thought." |
