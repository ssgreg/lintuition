---
worth: later
rank: 20
where: docs/linters.md:724
added: 2026-10-05
---
# no linter for a diagnostic that names a different operation than the failing call

```go
if err := os.Remove(tmp); err != nil {
	return fmt.Errorf("create snapshot: %w", err)
}
```

The prototype (`diagnostic-subject-mismatch`) asked whether the message refers to the same operation as
the failing call (same / different / too generic) and reported "different" at 0.85. Seeded pair: caught
3 of 3, no false alarm. Like its siblings it sent the condition as source, so it never ran on real code.
Keep it narrow, a direct `if err := op(...); err != nil` with the operation and its object supplied as
recognised facts.

Start with one `if` with an init statement, the error bound exactly, and callee and receiver facts;
wrapper contracts and causes outside the function stay unknown. Real logging defects from public
projects would make better fixtures than seeded ones.

Unknown that settles it: whether the failing call described by callee identity alone ("the call
os.Remove") is enough for the question, and the precision on real code. `facts.FallibleStmt` already
finds the call.
