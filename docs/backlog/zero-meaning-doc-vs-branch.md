---
worth: later
rank: 130
added: 2026-10-05
---
# no linter for a doc whose meaning of zero contradicts the code

`// MaxRetries: 0 disables retries.` while the code does `if n == 0 { n = defaultRetries }`.

The precise successor of the unit-and-zero field check. Never run. The classifier would read what the
doc says zero means (disabled, unlimited, default, ordinary value, unclear); Go code would find the
branch on zero and what it selects; report only opposing meanings.

Unknown that settles it: whether the zero branch can be read as facts often enough, since many configs
hand the value to a helper, and how many candidates real code has.
