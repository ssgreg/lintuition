---
worth: later
rank: 120
added: 2026-10-05
---
# no linter for a documented default that differs from the code

`// Timeout defaults to 30s.` with `c.timeout = 10 * time.Second`.

Never run. Includes the variant of a default promised as shared by several entry points that differ. A
candidate scan of two internal services found 2 clean local pairs, plus one claim whose default lives in
a dependency; no shared-default promise.

Unknown that settles it: constant arithmetic and unit binding in Go code (the classifier reads the
stated default; Go computes the real one), and candidate density on the public corpus.
