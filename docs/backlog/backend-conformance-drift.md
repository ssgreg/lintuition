---
worth: later
rank: 490
added: 2026-10-05
---
# no linter for backends whose tests disagree on one scenario

An in-memory and a SQL backend of one interface, where the same scenario expects `ErrNotFound` from one
and `nil` from the other, and the docs excuse neither.

Never run. A candidate scan of two internal services found no scenario matrix shared by two backends;
the fixtures differ in shape. The facts would come from a person-written test table or trace, which
lintuition trusts more than inferred facts.

Unknown that settles it: a way to match scenarios across backends other than by similar names, which
would pair the wrong cases.