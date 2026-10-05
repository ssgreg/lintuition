---
worth: later
rank: 270
added: 2026-10-05
---
# no linter for a locking comment the code breaks

`// Safe for concurrent use.` on a type whose methods write fields with no lock, or `// Caller must hold
s.mu.` on a function that locks `s.mu` itself.

The prototype (`comment-lock-claim`) first reported a contradiction when a lock claim had no `Lock` call
nearby. The inference is wrong: "the caller holds the lock" means there is no local `Lock` call. The
rule was cut down to an inventory of claims: 6 on logf, none checked against the code.

Unknown that settles it: lock facts by object, which mutex guards which field and which functions take
it. Without them the classifier can label a claim but nothing can test it.
