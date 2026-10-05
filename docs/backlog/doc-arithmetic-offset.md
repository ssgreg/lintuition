---
worth: later
rank: 590
added: 2026-10-05
---
# no linter for a doc that promises a value off by one from the return

`// Len returns the highest index plus one.` on a function that returns the highest index.

Go code would prove a simple local return expression (a field, a field plus a constant); the classifier
reads only the stated contract (index, count, index plus one, one-based). A helper's result, overflow or
an unclear subject is unsupported. Never run.

Unknown that settles it: whether real Go code has enough candidates. It may turn out to be a special
case of the collection-cardinality item.
