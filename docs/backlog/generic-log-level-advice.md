---
worth: no
added: 2026-10-05
---
# no linter that picks the right level for any log line

The prototype (`log-level-mismatch`) asked which level fits an event and reported a gap of two steps or
more. On zerodt it fired 13 times, info lines about restarts and signal handling read as errors. It was
removed after that run and replaced by two directional linters that ship: `normal-event-at-error` and
`severe-event-understated`.

Rejected: the right level depends on the project's convention, and an open-ended level question gives no
fact for Go code to check.
