---
worth: later
rank: 380
added: 2026-10-05
---
# no linter for a closed-state promise a lifecycle test breaks

`// After Close, Write returns ErrClosed.` with a test row that expects `nil` from `Write` after
`Close`.

Never run. A candidate scan of two internal services found no complete pair: one real
request-after-close test, but no doc that states the contract. The facts would come from a
person-written test table or trace, which lintuition trusts more than inferred facts.

Unknown that settles it: candidate density, and a lifecycle-row adapter.
