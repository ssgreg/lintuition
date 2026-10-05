---
worth: later
added: 2026-10-05
---
# no project profile for logging conventions

`normal-event-at-error` and `severe-event-understated` use one general idea of what each level is for. A
team may decide that a routine cancellation is debug and an exhausted retry is error. A profile with
explicit meanings and exceptions, enabled on purpose and versioned together with the questions and the
cache, could feed those linters.

Unknown that settles it: whether teams write such a profile. Deriving the norm from how most existing
logs are written is out: it would lock in the existing mistakes.
