---
worth: later
where: internal/effects/writes.go
added: 2026-10-05
---
# no linter for a query-named function that writes state

A function named like a query (`GetX`, `IsX`, `HasX`, `FindX`) that writes caller-visible state is a
contract the name does not state. `internal/effects` already finds caller-visible writes for
`read-only-promise`, so the code side exists; the classifier would only judge whether the name promises
no side effects.

Unknown that settles it: precision on real code. Lazy caches, memoization and counters are common
writes in getters and read as fine to most reviewers, so the linter may be mostly noise. Measure on the
precision corpus before building it out.
