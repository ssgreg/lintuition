---
worth: later
rank: 10
added: 2026-10-05
---
# no linter for arguments passed in an order their names contradict

`func NewGrid(width, height int)` called as `NewGrid(rows, cols)`: `rows` is a height, so the two
arguments are swapped.

Go code would find calls whose arguments are named identifiers and whose parameters have names, and send
only those names. The classifier is asked which parameter each argument name corresponds to (`rows` to
`height`); Go code reports a call where the mapping crosses. A string-distance rule catches `dst`/`src`
but not `rows`/`height`, `begin`/`from` or `lat`/`lng`, and that is the gap. Never run.

Unknown that settles it: how many candidates real code has, and how often the classifier maps names
confidently enough to report.
