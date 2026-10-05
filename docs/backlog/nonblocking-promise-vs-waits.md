---
worth: later
rank: 200
added: 2026-10-05
---
# no linter for a doc that promises never to wait on a function that blocks

`// Enqueue never waits.` on a function that takes a mutex or sends on an unbuffered channel.

The prototype (`nonblocking-promise`) asked whether the doc promises no waiting and checked lock and
channel facts. It fired once per run on logf, a `Flush` that takes a mutex; not labelled. A send in a
`select` with `default`, a `TryLock` and work inside a started goroutine are all non-blocking, and "has
a Lock call" cannot tell them apart.

Unknown that settles it: blocking-effect facts typed by object, with those negatives handled.
