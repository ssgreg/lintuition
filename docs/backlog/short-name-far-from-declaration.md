---
worth: no
added: 2026-10-05
---
# no linter for a short name used far from its declaration

`c` declared at the top of a 200-line function and used at the bottom.

Never run. Rejected: distance and use count are measurable without a classifier, and existing linters
already do that. Whether a short name is clear is style.