---
worth: no
added: 2026-10-05
---
# no linter for a numeric field whose doc gives no unit

`Timeout int // timeout`.

The prototype (`field-doc-unit`) asked whether the doc of a numeric field gives a unit or the meaning of
zero. It fired 9 or 10 times per run on logf (benchmark `ID` fields, statistics counters) and more than
50 times on one internal service, mostly size fields. Rejected as noise. Two independent questions
joined by "or", and an integer is not always a physical quantity.

The precise part lives on as "zero meaning in a doc contradicts the zero branch".
