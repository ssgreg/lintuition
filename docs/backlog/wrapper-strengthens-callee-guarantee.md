---
worth: later
rank: 410
added: 2026-10-05
---
# no linter for a wrapper that promises more than its callee gives

`// Get never returns nil, nil.` on a wrapper that forwards a callee whose test table has a `nil, nil`
row.

Never run. A candidate scan of two internal services found no complete candidate; the one near miss had
success tests that return real objects. The facts would come from a person-written test table or trace,
which lintuition trusts more than inferred facts.

Unknown that settles it: a table adapter that reads the callee's rows, and a real case to start from.
