---
worth: no
added: 2026-10-05
---
# no linter for a comment that only restates the code

`// increment i` over `i++`.

The prototype (`comment-restates-code`) sent the comment with the one to five statements under it and
asked whether it adds any meaning, unit, reason or invariant. Rejected: the question needs those
statements, which are source, and the built-in linters never send source. It is also editorial. A
comment that restates the code makes no claim the code contradicts. A plugin could still do it under
`semantic.payload: source`.
