# Error Tracing: Lessons Learned From the Trenches
GopherCon Europe 2026 · ~30 min (≈15 slides-driven, ≈15 demo) · Audience: proficient Go devs

## Thesis
Errors are an architectural concern. Each layer boundary is a chance to **corrupt meaning**.
Translate at the boundary → preserve meaning, reduce coupling, enforce clarity.
"Simplicity does not scale automatically."

## Design system
- Go-brand: deep navy bg (#0a1822 / #0d1b26), Gopher cyan #00ADD8 primary.
- Bad = coral oklch(0.64 0.16 28); Good = green oklch(0.72 0.14 158). Same chroma/lightness band.
- Neutrals: text #e8edf0, muted slate #7d909c.
- Type: Space Grotesk (display) + JetBrains Mono (code/error strings/labels).
- Error strings ARE the hero visual. Terminal cards. Layer-stack diagrams.

## Title sequence (chapters — short topic noun-phrases / declarative)
01  Error Tracing: Lessons Learned From the Trenches      [title]
02  The System: A Layered E-Commerce Backend              [architecture diagram]
03  What Happens When an Error Travels Up?                [hook / error-flow]
04  The Problem: Every Boundary Can Corrupt Meaning       [concept]
05  Four Ways Errors Go Wrong                             [section index of the 4]
06  Anti-Pattern 1 — Excessive Wrapping                   [divider]
07  One Missing Row, Five Clauses                         [before error string]
08  Signal vs. Noise                                      [why it hurts]
09  Anti-Pattern 2 — Abstraction Leakage                  [divider]
10  database/sql in the Business Layer                    [code + leak path]
11  Swap the Store, Break the Caller                      [why it hurts]
12  Anti-Pattern 3 — Meaning Gets Reinterpreted           [divider]
13  Not Found Becomes 503                                 [status mismatch]
14  Anti-Pattern 4 — Provider Internals Leak              [divider]
15  Flutterwave's Topology, In Your API                   [before string + risks]
16  The Common Thread: Routing by String Match            [synthesis]
17  Demo Part 1 — The "Bad" Version                       [interstitial]
18  The Fix: Translate at the Boundary                    [principle]
19  v0.1 → v0.2: The Four Fixes                            [before/after table]
20  Demo Part 2 — The Structured Version                  [interstitial]
21  What Changed? Not the Errors                          [architecture changed]
22  This Is Not About Perfect Errors                      [clarification]
23  Simplicity Does Not Scale Automatically               [final + thank you]

## Hero error strings
BAD  (v0.1, scenario 1):
  handler: GET /orders/{id} failed: service layer: failed to retrieve order
  details: order lookup failed: repository: GetOrder query failed: order
  record not found in database: sql: no rows in result set
GOOD (v0.2):
  getting order o_ghost: order not found

BAD payment (s4):
  ...flutterwave error FW-9082 region=eu-west retryable=false
  tx_ref=FW-TXN-84712947: CARD_DECLINED: insufficient_funds,
  issuer_code=05, network=VISA_EU
GOOD:
  checkout: payment failed: payment_declined
