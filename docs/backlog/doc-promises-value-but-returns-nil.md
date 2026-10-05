---
worth: later
rank: 40
where: internal/facts/facts.go:30
added: 2026-10-05
---
# no linter for a doc that promises a value on a path that returns nil

`// Lookup always returns a usable record when err is nil.` with a `return nil, nil` path.

Two prototype linters: `doc-nil-contract` (the doc is silent about a nil result) and `nil-guarantee`
(the doc promises a value and a path returns nil), the second meant to replace the first. On the seeded
pair `nil-guarantee` caught 3 of 3, no false alarm. On logf the early `doc-nil-contract` hits came from
a fact error: the nil in the error slot of `MarshalText` read as a nil result. One internal hit was true
but minor, a lookup whose doc does not say what happens when nothing is found. `nil-guarantee` fired
once or twice per run on logf, logrus and one internal service, not labelled.

Report only an explicit `return nil, nil` with the result slot known. `facts.ErrorSlots` now numbers
result slots by type.

The result slots and the paired error decide it: `return value, nil` is not a counterexample, and
`return nil, err` may be allowed by the promise. Find real complete pairs in public Go repositories
before building.

Unknown that settles it: precision on the 25-repo corpus with slot-aware facts. The silent-doc half
alone is probably not worth a finding.
