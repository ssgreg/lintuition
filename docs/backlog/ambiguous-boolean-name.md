---
worth: no
added: 2026-10-05
---
# no linter for a boolean whose name does not say what true means

`if !notDisabled { ... }`.

The prototype (`bool-name-reads-wrong`) first asked whether a name is negated or ambiguous, then whether
true has an identifiable meaning. It fired twice on one internal service, once on a test type. Rejected:
style; `disabled`, `missing` and `readOnly` are precise names.