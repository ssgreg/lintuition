---
worth: no
added: 2026-10-05
---
# no linter for a Deprecated note that names no replacement

`// Deprecated: do not use.`

The prototype (`deprecated-no-replacement`) asked whether the note says what to use instead. Rejected:
it checks for missing guidance, not for text the code contradicts, and no recorded run produced a
finding. Retiring something without a replacement is sometimes the plan, so "stop using it" has to count
as guidance.