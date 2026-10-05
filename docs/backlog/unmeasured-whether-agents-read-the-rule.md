---
worth: later
added: 2026-10-05
---
# unmeasured whether an agent reads the rule before fixing

The docs links and the planned `explain` command assume a coding agent looks up the rule before it edits
code or suppresses a finding. Nothing measures that, so the README must not promise it.

Unknown that settles it: a small run with the same ambiguous findings and the same task, in three
variants (no hint, a URL, the `explain` hint), recording which tool calls the agent makes and whether the
fix is right.
