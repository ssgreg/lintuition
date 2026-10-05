---
worth: later
rank: 110
added: 2026-10-05
---
# no linter for a doc that promises cancellation when ctx is unused

`// Fetch stops as soon as ctx is cancelled.` on a function that never reads `ctx`.

The prototype (`doc-context-promise`) asked whether the doc promises cancellation, and Go code checked
that `ctx` is never used. Seeded pair: caught 3 of 3, no false alarm. No recorded real run produced a
finding.

Unknown that settles it: whether real code has this. The unused-parameter half is an ordinary check
other linters already make; the classifier only adds the promise, which is worth a linter only if the
shape shows up in the corpus.
