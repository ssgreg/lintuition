---
worth: no
added: 2026-10-05
---
# no linter for a log line that does not say what it is about

`log.Info("done")`.

The prototype (`log-no-subject`) asked whether a reader can tell the operation and object from the
message and fields. On logf it fired 9 to 13 times per run, almost all in example and benchmark
programs; it made up a good share of a first 45-finding logf run in which 2 or 3 findings were useful.
Rejected: it judges how informative a message is, not whether it is true, and structured fields often
carry the subject.
