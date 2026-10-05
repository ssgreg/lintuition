---
worth: no
added: 2026-10-05
---
# no linter that ranks hunks or functions by review risk

A score for how likely a hunk or a function is to break callers, used to order review attention.

It covers a per-hunk risk score, a per-function risk profile, a count of unrelated jobs per function,
and comment inconsistency as a bug predictor. Never run. Rejected as a linter: it produces a ranking,
not a finding with a location and a contradiction. A candidate scan showed the population is every
function, with no labels.