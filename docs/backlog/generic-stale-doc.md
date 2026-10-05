---
worth: no
added: 2026-10-05
---
# no generic linter for a doc the body contradicts

The prototype (`doc-stale`) sent a doc with facts about the body and asked whether any sentence is
contradicted. Its one finding on an internal service was real (a doc says the function returns a status
and an error, and it returns nothing), and `doc-vs-signature` now covers that shape.

Rejected in favour of narrow promise linters such as `doc-vs-signature` and `read-only-promise`. Retire
the generic check so two linters do not report one promise. A whole-function question also undoes the
split into narrow questions that made the shipped linters work.