---
worth: later
where: internal/effects/writes.go:53
added: 2026-10-05
---
# a restored write is treated the same as no write

`internal/effects` marks a write that a proven save and restore undoes as `Restored`. The final value is
the same, so a read-only promise holds, but a promise of no observable change on the way (or of thread
safety, or atomicity) does not. The write and the flag must stay; the read-only filter must not become
proof of anything else.

Unknown that settles it: a written list of promise classes that says which ones a restore satisfies. No
race claim without a model of synchronization.
