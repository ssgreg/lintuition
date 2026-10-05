---
worth: later
rank: 55
added: 2026-10-05
---
# no linter for a boolean variable whose name means the opposite of its expression

```go
ok := err != nil
isEmpty := len(items) > 0
```

The classifier reads what true means by the name ("no error", "nothing in it"). Go code supplies the
expression as facts, the same normalized condition `diagnostic-contradicts-its-branch` needs, and decides
whether the two agree. A name that reads either way (`flag`, `done`) abstains.

Unknown that settles it: depends on `normalized-branch-facts`; then the candidate density, since most
such assignments are right and the finding needs a confident inversion.
