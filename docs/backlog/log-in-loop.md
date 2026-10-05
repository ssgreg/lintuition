---
worth: no
added: 2026-10-05
---
# no linter for a per-iteration log line in a loop

A log call inside a `for` loop, reported as one that should be aggregated or rate limited.

The prototype (`log-in-loop`) asked whether the message is a per-iteration event. Rejected. Syntax shows
the call is nested in a loop, not how often it runs. A loop can wait an hour per pass, run twice, or log
only on a rare branch. Without rate or cardinality facts this is a frequency guess presented as a fact.