# Mission — Gophercon Europe 2026

**Talk:** Error Handling as Architecture in Go — Designing Predictable Error Flows in Layered Systems
**Speaker:** Breshnev

## Purpose

This demo app is the live coding / walkthrough vehicle for a Gophercon Europe 2026 talk. It illustrates how error handling in Go shifts from simple to complex as a system grows, and why **error ownership** is the missing architectural discipline in most layered Go services.

## Narrative Arc

The talk follows four acts, mirrored in this codebase and its alternate version:

### Act 1 — The Illusion (Slides 2–3)

Go error handling looks trivial: `if err != nil { return err }`. Small programs make it feel easy. Production systems do not. Errors travel through layers — HTTP Handler → Service → Domain → Repository → Payment Provider — and every layer wants to wrap, reinterpret, add context, or translate. Meaning erodes.

### Act 2 — The Problem (Slides 4–6)

In layered architectures, error handling becomes inconsistent, leaky, and tightly coupled. Not because Go errors are bad, but because architectures rarely define **who owns errors**. Infrastructure errors leak upward. Business meaning gets buried. HTTP layers depend on database semantics.

### Act 3 — The Principle (Slides 7–13)

Errors are contracts, boundaries, and architectural signals. The layer where an error originates should own its meaning. Define errors at the point of origin, then propagate upward unchanged. Each layer defines only what it knows:
- **Presentation layer:** invalid request, malformed payload, bad query params
- **Domain layer:** business rule failures, domain invariants
- **Infrastructure layer:** database failures, third-party failures, network failures

### Act 4 — The Demo (Slides 17–19)

A before-and-after walkthrough of the same e-commerce backend:
- **Slide 18 — The "Bad" Version:** excessive wrapping, infrastructure leakage, unreadable error chains
- **Slide 19 — The Structured Version:** layer-owned errors, clean propagation, centralized HTTP mapping via `errors.Is`

## Target Audience

Go developers at Gophercon Europe 2026 who:

- Work on or design multi-layer backend services
- Struggle with noisy / unhelpful error messages in production
- Want a concrete, minimal example of sentinel-error ownership they can adopt tomorrow

## Takeaway

Simplicity does not scale automatically. Go gives us simple primitives. **Architecture** determines whether they remain simple at scale.
