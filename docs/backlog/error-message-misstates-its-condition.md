---
worth: later
rank: 60
where: docs/linters.md:724
added: 2026-10-05
---
# no linter for an error message that does not describe its condition

`if n > max { return errors.New("value is empty") }`: the message names a condition the branch does not
test.

The prototype (`error-message-vs-condition`) sent the message with the branch condition and asked
whether the message describes it. On the seeded pair it caught the defect in 2 of 3 runs, with no false
alarm on the fixed twin; the answer sat near the threshold. It never ran on real code, because the
condition went out as source text and facts-only runs turned it off.

Not shipped: a branch condition is source, and the built-in linters never send source.

Unknown that settles it: whether a condition described as facts (operands by identifier, the operator as
a word, outer conditions kept) still lets the classifier judge the match, and what precision that gives
on real code.
