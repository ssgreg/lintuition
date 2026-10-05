---
worth: later
where: sdk/linter.go:36
added: 2026-10-05
---
# prose in the payload can carry a literal secret

A log message or error string can embed a token as a literal, and prose is sent as written. The facts
payload mode does not help, because it skips the prose linters instead.

The idea: keep each payload field traceable to its source position, and drop or skip prose that holds an
obviously secret-shaped substring, counting it as lost coverage. It would not promise full secret
detection, and it must never send a value to ask whether it is a secret.

Unknown that settles it: cutting part of a text changes its meaning, so this needs a privacy decision
(drop the candidate, or send a redacted text) before any code.
