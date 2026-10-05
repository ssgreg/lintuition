---
worth: later
rank: 330
added: 2026-10-05
---
# no linter for a regression test that does not check the regression

`// Regression: results came back in the wrong order.` on a test that checks only `len(got)`.

Never run. A candidate scan of two internal services found 3 families with explicit regression wording
in the test. The facts would come from a person-written test table or trace, which lintuition trusts
more than inferred facts.

Unknown that settles it: whether the classifier can read what the regression was well enough for Go code
to check the assertions cover it.
