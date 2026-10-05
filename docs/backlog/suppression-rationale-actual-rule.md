---
worth: yes
where: linters/suppressionreason/rules.go
added: 2026-10-05
---
# suppression-rationale reads a gosec reason without a rule ID against all of gosec

```go
n := rand.Intn(255) //nolint:gosec // it is just a test
```

When the reason names a rule (`G404: ...`), the rule's own title is sent. When it names none, the
classifier gets gosec's general description (credentials, file paths, commands, crypto, slice bounds),
and against that "it is just a test" answers no particular risk, so it is reported as being about
something else. Here gosec reports a weak random number generator, and "a test" is a fine answer to
that. On one internal service all 4 findings were this shape. The `G304.golden` false positive listed
in docs/linters.md is the same gap.

The fix: let Go name the rule when the suppressed line has an unambiguous shape, and send that rule's
title. A call into `math/rand` is G404; `os.WriteFile` or `os.OpenFile` with a mode is G306 or G302;
`os.Open` or `os.ReadFile` with a non-constant path is G304; `exec.Command` with a non-constant argument
is G204. This is a fact proven from the code, not a guess from the reason's wording. A line with no such
shape keeps today's behaviour.

Effort: a small table of call shapes in the linter, twins for each rule, and a check that the two
existing false positives stop firing without losing the held-out findings.
