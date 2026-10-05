---
worth: no
added: 2026-10-05
---
# no linter for history and argument in comments

`// This used to be a string before we switched to IDs.`

The prototype (`comment-narrative`) sorted comments into meaning, invariant, history, argument,
comparison and instruction, and reported the narrative kinds as "belongs in the commit message". On logf
it fired 4 to 6 times per run: a separator banner, a two-word test comment, and examples that compare
two approaches on purpose. None was worth changing.

Rejected: an editorial preference, not text that contradicts code. History and comparison comments often
explain a deliberate design, and the message claims more than the classification shows.