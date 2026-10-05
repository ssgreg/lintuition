---
worth: later
rank: 150
where: linters/logsensitive
added: 2026-10-05
---
# no finding for a logged struct or URL that carries a secret inside

`log.Info("calling upstream", "req", req)` where `req` has a `Password` field, or a URL with user info,
logged under a neutral key.

An extension of `log-sensitive-field`, not a second secret linter. For a first version: known types, a
direct local path to the field, and formatting that knows about redaction. An empty or redacted instance
is a required negative, and the presence of a field in the type is not a leak by itself. The payload
stays without values. On the 25-repo precision run `log-sensitive-field` had 1004 candidates and no
findings, so the top-level key alone finds little.

Unknown that settles it: type-to-field path facts, and whether the logger prints those fields (a
`String` or `LogValue` method may hide them).
