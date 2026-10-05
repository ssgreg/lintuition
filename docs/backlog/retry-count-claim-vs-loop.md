---
worth: no
added: 2026-10-05
---
# no linter for a promised attempt count that differs from the loop

`// Tries three times before giving up.` over `for i := 0; i < 5; i++`.

Never run. A candidate scan of two internal services found no pair of a numeric attempt claim and a
literal loop bound: retry contracts there talk about time budgets and backoff, not attempt counts.
Rejected for now: no population. Worth a fresh look only if the public corpus shows the shape.
