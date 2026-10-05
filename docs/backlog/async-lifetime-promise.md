---
worth: later
rank: 260
added: 2026-10-05
---
# no linter for a doc whose lifetime promise the goroutine breaks

`// Stop returns after the worker has exited.` on a function that signals the worker and returns without
waiting. Callback timing and cancellation completion are the same family.

Never run. A scan of two internal services found one family with both a promise and tests that exercise
it.

Unknown that settles it: a "starts a goroutine and does not wait" fact by control flow, or a test trace
adapter, and whether either finds enough candidates.
