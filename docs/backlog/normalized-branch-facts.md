---
worth: yes
where: internal/facts/facts.go
added: 2026-10-05
---
# branch conditions are not available as facts

Four linters wait on this: `error-message-vs-condition`, `diagnostic-subject-mismatch`,
`diagnostic-polarity-mismatch` and `error-to-http-status`. Each needs to know what the branch tested,
and the built-in linters never send the condition as source.

The work: normalize nil and not nil, equality with a sentinel, a numeric bound, the sign of the branch
and the identity of the tested value into facts, keeping where each fact came from and when it still
holds. A check done before an arbitrary call says nothing about state after it. Before the first linter
uses it, a shared probe set: negation, shadowing, reassignment, loops, closures and switches.
