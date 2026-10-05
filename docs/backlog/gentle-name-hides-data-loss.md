---
worth: later
rank: 320
where: internal/effects/writes.go
added: 2026-10-05
---
# no linter for a gentle name on a function that discards data

`func RefreshQueue() { pending = nil }`: the name says refresh, the body throws away pending work.

The prototype (`gentle-name-hides-loss`) asked whether the name promises to keep data and checked a
"sets to nil or empty" fact. No recorded run produced a finding. Setting a slice to nil can follow a
successful delivery or clear a derived cache, so the fact does not show loss. This is a stronger claim
than a query name that writes state, which has its own item.

Unknown that settles it: a loss fact (pending or undelivered data dropped), which `internal/effects`
does not model.
