---
worth: later
rank: 220
added: 2026-10-05
---
# no linter for flag help that says the opposite of what the flag does

`flag.BoolVar(&offline, "offline", false, "Permit connections to remote services")`, or `"no-file-log"`
with help "Disable all logging".

The prototype (`flag-help-polarity`) compared the help text with the flag's name. Name against help is a
consistency hint, not evidence about behaviour; split polarity from scope, since they need different
facts. No recorded run produced a finding.

Unknown that settles it: facts about what the flag's variable does (which branch reads it and what that
branch does), described without source.
