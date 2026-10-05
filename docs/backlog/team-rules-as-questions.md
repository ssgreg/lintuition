---
worth: later
added: 2026-10-05
---
# no way to turn a team's written rule into a check without writing Go

A rules file says "a log line about a payment must name the payment id", and lintuition asks that
question of every log call.

Never run. Asking the rule text about a whole file would mean sending the file, which lintuition never
does. Today a team writes a plugin linter in Go, see `docs/plugins.md`.

A workable shape for Go is a rule pack: a typed extractor, the rule text, near-miss fixtures and listed
exceptions, shipped through the existing custom builder. Starting with two real team rules would show
whether the pack format holds.

Unknown that settles it: how a free-text rule picks its candidates and facts. The shipped design picks
both in Go and fixes the question text per linter, so it is open whether a rule file can do without
code.
