---
worth: later
rank: 510
where: linters/docvstable/docvstable.go
added: 2026-10-05
---
# no linter for a documented behaviour no table row exercises

`// Valid returns false for an expired token.` and the test table for `Valid` has no expired case.

Never run. Unlike `doc-vs-table`, which checks rows against the doc, this checks the doc against the
rows. A scan of two internal services found at least 16 function families with a doc and a nearby table.

Unknown that settles it: whether a clause with no row is a finding people accept. Another test may cover
the clause, so a missing row does not prove missing coverage.