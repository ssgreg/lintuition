---
worth: later
rank: 250
added: 2026-10-05
---
# no linter for an implementation that breaks its interface's documented contract

The interface says `// Close is idempotent.` and one implementation returns an error on the second call.

Never run. The classifier would read one contract dimension at a time (ownership, idempotency,
blocking); Go code would find the implementations by `go/types` and the matching fact in each. A scan of
two internal services found 2 families an adapter could use and 11 doc pairs worth a look; one service
had no docs on its interface methods.

Unknown that settles it: one dimension with a fact adapter that works across implementations.
