---
worth: later
where: linters/readonlypromise
added: 2026-10-05
---
# read-only-promise does not know which object a promise is about

"Does not modify the configuration" next to a write to a lazy cache field is not a contradiction by
itself. The promise has a subject (user data, the configuration, the whole receiver, one field), and the
write has a root and a path; the two need an explicit binding. Matching the doc against every write was
already rejected.

Unknown that settles it: whether precise promises are common enough to pay for it. Tests would use the
same writes with different promise subjects.
