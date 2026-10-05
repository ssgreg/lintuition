---
worth: later
rank: 90
where: docs/linters.md:724
added: 2026-10-05
---
# no linter for an HTTP status that contradicts the error it reports

```go
case errors.Is(err, ErrNotFound):
	w.WriteHeader(http.StatusBadRequest)
```

The prototype (`error-to-http-status`, a policy linter) asked what kind of failure the error's message
reads as (invalid input, missing, permission, conflict, temporary, internal) and compared that with a
fixed table of status families. On three internal services it gave 0, 0 and 1 findings; the one was not
labelled. A candidate scan of two internal services found 13 error-to-status bindings in 4 mapping
functions, so the population is small.

Keep it policy, and first compare with a plain word dictionary. If the dictionary already sorts the
messages, the classifier adds little.

Unknown that settles it: binding an error to the status chosen in its branch without sending the branch,
and whether the classifier beats the dictionary.
