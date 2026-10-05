---
worth: later
rank: 210
added: 2026-10-05
---
# no linter for a result that carries an error code and a success reason together

```go
return Result{Value: def, ErrorCode: TypeMismatch, Reason: ReasonCached}
```

One returned result says both "this failed" and "this succeeded because it was cached". Go code would
start with struct literals and a fully traced local result on one path; the classifier reads what each
enum constant or its doc means (success, failure, neutral). Legitimate partial success and the
difference between a transport status and a business status are the negatives. Never run.

Unknown that settles it: candidate density in real code, and how many result types document their reason
constants well enough to classify.
