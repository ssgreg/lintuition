---
worth: later
rank: 390
added: 2026-10-05
---
# no linter for a deep-copy promise the tests show is shallow

`// Clone returns a deep copy.` with a test that mutates a nested slice of the clone and sees the
original change.

Never run. A candidate scan of two internal services found 6 deep-copy promises with no test that
mutates the copy. The facts would come from a person-written test table or trace, which lintuition
trusts more than inferred facts.

Unknown that settles it: candidate density; promises without tests give the classifier nothing to
compare.
