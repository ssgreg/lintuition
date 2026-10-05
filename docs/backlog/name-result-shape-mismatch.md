---
worth: later
rank: 180
added: 2026-10-05
---
# no linter for a name that promises a different kind of result than its type

`func CountActive() bool`, `func IsReady() int`, `func Users() *User`.

The same holds for a variable: `user := ListUsers()` names one item and holds a slice, `count := ids`
holds the ids themselves.

The classifier reads what the name promises (a count, a yes-or-no predicate, a collection, one item); Go
code compares that with the result type, or with the variable's type. Only the declared role against the signature, not a claim about
the whole behaviour. `Get`, negations and short names are not defects by themselves. Changes to an API
make this more likely, so diff mode would help. Never run.

Unknown that settles it: precision with domain negatives and named wrapper types, and whether the
classifier beats a word list.
