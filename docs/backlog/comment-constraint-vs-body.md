---
worth: later
rank: 550
added: 2026-10-05
---
# no linter for a design constraint in a comment that the code breaks

`// Called once, at startup.` on a function called from a loop, or `// n is in 1..100` with a caller
passing 0.

The prototype (`comment-constraint`) classified the kind of constraint (nullness, range, once, caller
restriction, concurrency) and asked whether the body facts contradict it. No recorded run produced a
finding. Treat it as constraint discovery until one kind has a fact adapter that can test it.

Unknown that settles it: which constraint kind is common enough to earn its own fact adapter. Each kind
would become its own linter, the way `read-only-promise` covers mutation.
