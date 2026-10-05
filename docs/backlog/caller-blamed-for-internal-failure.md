---
worth: later
rank: 570
added: 2026-10-05
---
# no linter for an error that blames the caller for an internal failure

```go
if err := loadServerConfig(); err != nil {
	return errors.New("correct your request and try again")
}
```

Never run. The classifier would ask whether the message blames the caller; Go code would need to know
that the failing call reads server-side state, not the request.

Unknown that settles it: failure-source facts that separate request validation from internal state.
