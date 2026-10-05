---
worth: no
added: 2026-10-05
---
# no linter for a table case named like case 1

`{name: "case 1", ...}` or `{name: "ok", ...}`.

The prototype (`test-case-name-vague`) asked whether a case name describes its input or expectation.
Rejected: style, and the question needed the case's literals, which are source.
`table-case-vs-expectation` covers the case name that contradicts its `want`, which is the part that can
be wrong.
