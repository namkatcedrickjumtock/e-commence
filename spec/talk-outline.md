# Talk Outline — Error Tracing: Lessons Learned From the Trenches
**GopherCon Europe 2026 · Namkat Cedrick**
**Format: Talk only — all code is on slides, no terminal switching**

---

## Suggestion for Slide 03 · "What you're about to see"

The current overview tells the audience **what broke** but not **what the resolution looks like**.
They leave that slide knowing three problems exist but with no sense of the payoff.

**Consider adding a 4th item** (or a sub-line beneath the existing three):

> **4 — And the single pattern that fixed all three**

This gives the audience a reason to stay engaged through each anti-pattern —
they know there is a unifying answer coming, not just a list of war stories.
The Takeaways slide (22) delivers it as **Preserve · Reduce · Enforce**, but
the audience isn't told to expect that until they're already there.

---

## Slide-by-Slide Outline

---

### Pre-slide — Verbal opener (no slide)
**Duration:** ~1 min
**Purpose:** Warm the room. Establish personality before anything technical appears.

- German greeting → running joke → name + role
- Sets the tone: this is a story, not a lecture

---

### Slide 01 — Title
**Duration:** ~30 sec
**Key idea:** Frame what this talk is — a field story, not a tutorial.

On screen: Title, subtitle, GopherCon Gopher image
Point to land: "Lessons Learned From the Trenches" means real work, real failures.

**Transition:** "Let me tell you who I am, then let's get into it."

---

### Slide 02 — About Me
**Duration:** ~30 sec
**Key idea:** Credibility without a CV recitation. The audience needs to know you wrote the code you're about to show.

On screen: Name, role at Iknite Inc, West/Central African Gophers, contact links
Point to land: You've been in the situation you're about to describe.

**Transition:** "So here's what happened."

---

### Slide 03 — The Overview · *What you're about to see*
**Duration:** ~45 sec
**Key idea:** Give the audience a map. Three specific failures, one principle.

On screen: Numbered list of 3 problems + footer "The fix wasn't discipline. It was architecture."
Point to land: These are real patterns from real work. The sample app on screen demonstrates them.

**Transition:** "But let me start with the illusion we had going in."

---

### Slide 04 — The Illusion · *`if err != nil { return err }`*
**Duration:** ~45 sec
**Key idea:** This line feels like error handling. In a layered system, it's the beginning of the problem.

On screen: Terminal block with the canonical Go error idiom
Point to land: Returning an error is not the same as deciding what it means.

**Transition:** "Because here's what actually happens to that error."

---

### Slide 05 — The Question · *What happens when an error travels up through the layers?*
**Duration:** ~1 min
**Key idea:** The visual flow from `sql: no rows in result set` through each layer is the question the rest of the talk answers.

On screen: Flow diagram — bad error node → persistence → services → api → browser
Point to land: "By the time it reaches the top — is it still telling the truth?"

**Transition:** "It usually isn't. Here's why."

---

### Slide 06 — The Problem · *Errors don't stay where they start*
**Duration:** ~30 sec
**Key idea:** Three failure modes. Everything after this slide is one concrete example of each.

On screen: Three columns — Amplified · Misinterpreted · Leaked
Point to land: Every layer between origin and client is a point where meaning can change.

**Transition:** "Let me show you the system we were working with."

---

### Slide 07 — The Setup · *Small E-commerce API*
**Duration:** ~45 sec
**Key idea:** Set the stage quickly. This is a sample app built to reproduce the patterns — not the actual Iknite codebase.

On screen: Endpoint list, layer diagram (api → services → persistence → Database)
Point to land: Three dependencies, no framework, no ORM — every error decision is explicit code.

**Transition:** "And here are the three patterns we kept hitting."

---

### Slide 08 — The Map · *3 Recurring Anti-Patterns*
**Duration:** ~30 sec
**Key idea:** Chapter map for the audience. Three named patterns, each with a demo in the slides ahead.

On screen: Three apcard tiles — Excessive Wrapping · Tightly Coupling Between Layers · External Provider Leakage
Point to land: Every one of these came from real production work. The sample app reproduces them on purpose.

**Transition:** "Let's start with anti-pattern one."

---

### Slide 09 — Anti-Pattern 01 Divider · *Excessive Wrapping*
**Duration:** ~10 sec
**Key idea:** Chapter break. One sentence to prime the audience.

On screen: Divider "01 · Excessive Wrapping — Every layer stamps its own signature."

---

### Slide 10 — Scenario 1 · Architecture · *GetOrder code*
**Duration:** ~1 min
**Key idea:** Show the actual code. Two layers, two `fmt.Errorf` calls, both signing their name.

On screen: `services/services.go · GetOrder` and `persistence/postgres.go · GetOrder` — both with long error prefix strings highlighted in red
Point to land: Neither prefix adds information you couldn't derive from the call stack.

---

### Slide 11 — Scenario 1 · Error Chain · *When every layer adds its two cents*
**Duration:** ~45 sec
**Key idea:** The stacked diagram makes the noise visible — the root cause is buried at the bottom of a chain.

On screen: Vertical chain — DATABASE (origin) → PERSISTENCE wraps → SERVICE wraps → API HANDLER wraps
Point to land: The actual cause (`sql: no rows in result set`) is a footnote on a paragraph.

**Transition:** "Here's what the client actually receives."

---

### Slide 12 — Scenario 1 · Evidence · *curl /orders/o_ghost*
**Duration:** ~1 min
**Key idea:** The response is the proof. Four layer signatures highlighted in red on one error string.

On screen: Terminal — `curl localhost:8080/orders/o_ghost` → JSON response with `handler:`, `service layer:`, `order lookup failed:`, `repository:` all underlined in red
Point to land: An on-call engineer reading this at 3am has to parse a paragraph to find the problem.

**Transition:** "So what's the rule?"

---

### Slide 13 — Scenario 1 · Lesson · *Sweet spot — under vs. over wrapped*
**Duration:** ~1 min
**Key idea:** Both extremes are wrong. The goal is root cause + minimal context, not a reconstructed call graph.

On screen: Spectrum bar (bad → sweet spot → bad) with three columns: "Every layer stamps its name" · "Root cause + minimal context" · "Tells you nothing"
Point to land: You already have logs. You already have a call stack. The error message is not the place to rebuild that.

**Transition:** "That was pattern one. The second is more dangerous because it looks structural."

---

### Slide 14 — Anti-Pattern 02 Divider · *Abstraction Leakage*
**Duration:** ~10 sec
**Key idea:** Chapter break.

On screen: Divider "02 · Abstraction Leakage — Errors from one layer bleed into another."

---

### Slide 15 — Scenario 2 · Evidence · *`sql: no rows in result set` in the HTTP response*
**Duration:** ~1 min
**Key idea:** A PostgreSQL driver string in an API response. The client now knows your database technology.

On screen: Terminal — `curl localhost:8080/products/p_ghost` → response with `sql: no rows in result set` highlighted in red
Point to land: That string was never meant to be public. It's an implementation detail that leaked.

**Transition:** "Here's the line of code that caused it."

---

### Slide 16 — Scenario 2 · Code · *`services/` importing `database/sql`*
**Duration:** ~1 min
**Key idea:** The import statement is the problem. Business logic is coupled to an infrastructure package.

On screen: Two panels — imports (`database/sql` and `github.com/lib/pq` highlighted in red) + `GetProduct` function using `errors.Is(err, sql.ErrNoRows)` highlighted in red
Point to land: The SQL driver's string became the API contract by accident.

**Transition:** "And here's what that coupling actually costs you."

---

### Slide 17 — Scenario 2 · Lesson · *Swap the store, break the caller*
**Duration:** ~1 min
**Key idea:** Layered architecture promises independence. This coupling breaks that promise silently.

On screen: Flow — Postgres → MongoDB → service breaks (no more sql.ErrNoRows) + code lines showing the import and `errors.Is` call
Point to land: Swap the database tomorrow and `errors.Is(err, sql.ErrNoRows)` stops matching with no compile error and no test failure.

**Transition:** "Now the last pattern — this one had security implications."

---

### Slide 18 — Anti-Pattern 04 Divider · *External Provider Leakage*
**Duration:** ~10 sec
**Key idea:** Chapter break. Raise the stakes — this one goes beyond coupling.

On screen: Divider "04 · External Provider Leakage — An external system's error contract bleeds into yours."

---

### Slide 19 — Scenario 4 · Evidence · *Stripe internals in the API*
**Duration:** ~1 min
**Key idea:** `request_id`, `charge=ch_3RJX...` in an HTTP response to an end user. That is a security exposure.

On screen: Terminal — `curl -X POST localhost:8080/checkout | jq .` → response with `req_7NdMtHtVhh5i2J`, `card_error`, `card_declined`, `insufficient_funds`, `ch_3RJXsD2eZvKYlo2C0B5X3Y7z` highlighted in red
Point to land: A Stripe request ID and charge ID fingerprint your account. An attacker reading this knows your processor and can correlate payment attempts.

**Transition:** "Here's the code that caused it."

---

### Slide 20 — Scenario 4 · Code · *StripeError returned directly*
**Duration:** ~1 min
**Key idea:** `&StripeError{...}` is returned raw from persistence. Services wraps it with `%w` without translating. The provider's schema became the API schema.

On screen: Two panels — `persistence/stripe.go` returning `&StripeError` with all fields highlighted, + `services/checkout.go` doing `fmt.Errorf("payment processing failed: %w", err)`
Point to land: Nobody translated. Nobody decided what the client should see.

**Transition:** "Here's the fix."

---

### Slide 21 — Scenario 4 · Lesson · *Translate at the boundary*
**Duration:** ~1.5 min
**Key idea:** `StripeError` never leaves the persistence package. It becomes `PaymentError` — a domain type.

On screen: Two terminal panels side by side — "what leaked" (StripeError fields) vs "persistence — the fix" (PaymentError with Code + Retryable)
Footer: "StripeError never leaves the persistence package. It becomes a domain type your clients can depend on."
Point to land: Switch from Stripe to Braintree — you update one translation function. Nothing above persistence changes.

**Transition:** "So what's the principle that ties all three of these together?"

---

### Slide 22 — Takeaways · *This is not about perfect errors*
**Duration:** ~1.5 min
**Key idea:** Three rules. Not a style guide — an architectural commitment.

On screen: Three columns with cyan top-border — **Preserve** (meaning across boundaries) · **Reduce** (coupling between layers) · **Enforce** (architectural clarity)
Header: "This is not about perfect errors"
Point to land: Your error handling is part of your architecture contract, not something bolted on after.

**Transition:** "One last thought."

---

### Slide 23 — Final Thought · *Simplicity does not scale automatically*
**Duration:** ~45 sec
**Key idea:** Go's error primitives are simple. Architecture decides whether they stay simple as systems grow.

On screen: Large heading "Simplicity does not scale automatically" + body "Go gives us simple primitives. Architecture decides whether they stay simple at scale."
Point to land: This is not a Go problem. It's a discipline problem. The primitives are fine — what you do with them at boundaries is the work.

---

### Slide 24 — Thank You
**Duration:** ~30 sec
**Key idea:** Contact info + repo link. Invite questions.

On screen: Name, role, Gophers community, contact links, `github.com/namkatcedrickjumtock/e-commence`

---

### Slide 25 — Empty
Buffer for Q&A. No speaker notes needed.

---

## Time Budget

| Section                            | Slides    | Est. Time   |
|------------------------------------|-----------|-------------|
| Verbal opener (pre-slide)          | —         | ~1 min      |
| Title + About Me                   | 01–02     | ~1 min      |
| Overview + Setup (04–08)           | 03–08     | ~4 min      |
| Anti-Pattern 01 (slides 09–13)     | 09–13     | ~4 min      |
| Anti-Pattern 02 (slides 14–17)     | 14–17     | ~3 min      |
| Anti-Pattern 04 (slides 18–21)     | 18–21     | ~4 min      |
| Takeaways + Close                  | 22–24     | ~3 min      |
| **Total**                          |           | **~20 min** |
