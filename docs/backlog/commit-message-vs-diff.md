---
worth: no
added: 2026-10-05
---
# no linter for a commit message that does not describe its diff

A commit titled "fix typo" that changes a retry limit.

Never run. Rejected for lintuition: the input is a commit message and a diff, not Go code in packages,
so it belongs in a commit hook or a review tool.