---
worth: later
rank: 70
added: 2026-10-05
---
# no linter for a doc whose error list differs from what the function returns

`// Returns ErrNotFound if the key is missing.` on a function that returns `ErrConflict` and never
`ErrNotFound`.

Never run. Go code collects the sentinels a function returns; the classifier asks whether the doc
mentions each condition. A scan of two internal services found 6 small local families. Functions that
return `errors.Is(err, ErrX)` are booleans and must be excluded; that alone removed six misleading first
hits.

Unknown that settles it: resolving imported error sets, and whether a doc that names no errors is a
finding.
