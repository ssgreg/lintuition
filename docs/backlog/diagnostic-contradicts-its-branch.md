---
worth: later
rank: 50
where: docs/linters.md:724
added: 2026-10-05
---
# no linter for a diagnostic that says the opposite of its branch

`if user != nil { return errors.New("user not found") }`: the message states the opposite of the
condition that leads to it.

The prototype (`diagnostic-polarity-mismatch`) asked whether the message is consistent with the branch
condition, contradicts it, or is unrelated, and reported "contradicts" at 0.85. Seeded pair: caught 3 of
3, no false alarm. It sent the condition as source, so it never ran on real code. Its extractor also
kept only the innermost condition, so a nested `if` lost the outer test.

The "error names an unsupported cause" idea is folded into this one: report only when the stated cause
conflicts with an explicit known one.

Unknown that settles it: a branch condition, nesting included, described as facts without its source,
and the precision on real code once it is.
