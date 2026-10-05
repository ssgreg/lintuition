---
worth: later
rank: 540
added: 2026-10-05
---
# no linter for a comment that contradicts the operation under it

`// Keep the oldest items first.` over `sort.Sort(sort.Reverse(byCreated(items)))`.

The prototype (`obsolete-operation-comment`) sent the next six lines as code and asked whether the
comment still fits. Facts-only runs turned it off, and no recorded run produced a finding. The next six
lines are not the operation the comment is attached to, and a callee's name is not its effect.

Unknown that settles it: an attached-operation fact for one known family, sort direction for example,
that says what the code does without sending it.
