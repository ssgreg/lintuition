---
worth: no
added: 2026-10-05
---
# no linter for a vague TODO

`// TODO fix this`.

The prototype (`todo-vague`) classified TODOs as actionable, vague, obsolete or a workaround marker. On
one internal service it fired 11 times, 5 of them copies of one boilerplate line. Rejected: style, and a
TODO makes no claim about the code to check.
