---
worth: later
rank: 280
added: 2026-10-05
---
# no linter for a secret field on a type the API exposes

``Password string `json:"password"` `` on a response type an HTTP handler encodes.

Never run. The classifier would judge the field's role as `log-sensitive-field` does for log fields; Go
code would establish that the type reaches a public response. A candidate scan of two internal services
found no secret field bound to a public surface; request types, credential ids and status flags must not
count.

Unknown that settles it: public-surface facts, a type that reaches a response encoder.
