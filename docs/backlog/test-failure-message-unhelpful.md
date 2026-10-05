---
worth: no
added: 2026-10-05
---
# no linter for a test failure message that does not explain the failure

`t.Errorf("wrong result")`.

The prototype (`test-failure-message`) asked whether the message says what was tested and what went
wrong. On logf it fired 4 or 5 times per run. Rejected: style. A failure message need not print both got
and want, especially for an unexpected error.