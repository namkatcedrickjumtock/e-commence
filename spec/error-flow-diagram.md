# Error Origination & Layer Ownership

## The Diagram

The architecture has four layers stacked vertically, with an **Incoming Request** flowing down from the top and an **Outgoing Response** flowing back up.

```
Incoming Request
      ↓
┌─────────────────────────┐
│   PRESENTATION LAYER    │ ○ ← error travels up through here
└─────────────────────────┘
      ↓
┌─────────────────────────┐
│    BUSINESS LAYER       │ ○ ← and through here
└─────────────────────────┘  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌ (connected to code panel)
      ↓
┌─────────────────────────┐
│   PERSISTENCE LAYER     │ ○ ← and through here
└─────────────────────────┘
      ↓
┌─────────────────────────┐
│       DATABASE          │ ● ← IF ERROR ORIGINATES HERE
└─────────────────────────┘
         ↑
  IF ERROR ORIGINATES HERE
  This layer owns the meaning.
```

A red dashed line traces the error's path upward through each layer. The filled circle at the **DATABASE** layer marks where the error is born — every hollow circle above it is just a stop on the way up.

---

## The Code (Business Layer)

The dashed line connects to this `createOrder` function, which lives in the **Business Layer**:

```go
// creating an example order in the Business logic
func createOrder(ctx context.Context, req order) error {

    user, err := users.Get(ctx, req.UserID)
    if err != nil {
        return err
    }

    err = inventory.Reserve(ctx, req.ItemID, req.Qty)
    if err != nil {
        return err
    }

    err = payments.Charge(ctx, user.PaymentID, req.Total)
    if err != nil {
        return err
    }

    return saveOrder(ctx, user.ID, req.ItemID)
}
```

---

## What This Illustrates

Every `return err` in `createOrder` is a silent pass-through. The Business Layer receives errors from three dependencies — `users.Get`, `inventory.Reserve`, and `payments.Charge` — and returns all of them unchanged to the Presentation Layer above.

**The problem:** none of these callers are the layer where the error was born. A database-level error (e.g. `sql: no rows in result set`) originates at the DATABASE layer, travels through PERSISTENCE, arrives at BUSINESS, and exits as-is. By the time the PRESENTATION layer sees it, the error's original meaning is still intact — but nothing translated it into a domain concept the caller can act on.

**The principle the diagram encodes:** the layer where an error originates *owns its meaning*. The DATABASE layer is responsible for translating `sql.ErrNoRows` into `ErrUserNotFound` before it crosses into PERSISTENCE. PERSISTENCE is responsible for translating infrastructure errors into domain errors before they cross into BUSINESS. Each layer should only ever see — and return — errors from its own vocabulary.

---
