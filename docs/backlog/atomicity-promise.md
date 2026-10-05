---
worth: later
rank: 480
added: 2026-10-05
---
# no linter for an unchanged-on-error promise a test breaks

`// On error, the store is left unchanged.` with a snapshot after an injected failure that differs from
the one before.

Never run. A candidate scan of two internal services found no snapshot pair; the rollback tests check
cleanup calls instead. The facts would come from a person-written test table or trace, which lintuition
trusts more than inferred facts.

Unknown that settles it: tests that compare before and after snapshots, which this shape needs and the
scanned code did not have.
