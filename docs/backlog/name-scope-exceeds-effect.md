---
worth: later
rank: 580
added: 2026-10-05
---
# no linter for a name that claims a wider effect than the body has

`func FlushEveryLogger() { defaultLogger.Flush() }`.

Never run. The classifier would read the scope the name claims (one, all, every); Go code would count
what the effect reaches.

Unknown that settles it: facts about how many objects an effect reaches, which `internal/effects` does
not model, and whether real code has the shape.
