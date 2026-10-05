---
worth: later
rank: 100
added: 2026-10-05
---
# no linter for a doc that leaves a parameter open while the function closes it

`// Parse leaves r open for the caller.` on a function that calls `r.Close()`.

The prototype (`ownership-close`) asked who closes the parameter according to the doc and checked a
"closes parameter r" fact. Seeded pair: caught 3 of 3, no false alarm. No real run found one. The narrow
version: resolve the parameter by object, count only a direct close on it, and skip closes inside a
closure that may not run.

Before building, find real complete pairs in public Go repositories.

Unknown that settles it: whether real code has this. Every hit so far came from the seeded pair.
