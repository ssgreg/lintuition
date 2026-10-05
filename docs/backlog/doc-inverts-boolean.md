---
worth: later
rank: 170
where: linters/docvstable/docvstable.go
added: 2026-10-05
---
# no linter for a doc that inverts the boolean its name states

`// Missing reports whether the key is present.`

Never built as its own linter. The prototype found the obstacle while running `doc-vs-table`: when the
doc contradicts the function's name, the classifier believes the name, so the doc-vs-table question
never sees the inversion. `doc-vs-table` now masks the function name in the doc.

Unknown that settles it: whether a masked doc checked against the truth table catches this shape. No
twin covers it yet.
