---
worth: later
rank: 80
added: 2026-10-05
---
# no linter for a converter that assigns a field to one with another role

`out.Remaining = in.AttemptsUsed`: both are ints, the roles are opposite.

Never run. It would share the role question with `log-key-value-role`, applied to assignments between
struct fields. A candidate scan of two internal services found no pair where both fields have docs that
state their roles; few fields had docs.

Unknown that settles it: whether field names alone are enough for the role question, since docs are
rare, and the precision that gives.
