---
worth: later
rank: 520
where: internal/effects/writes.go
added: 2026-10-05
---
# no linter for an exported function whose doc is silent about a strong effect

An exported `Load` that starts a goroutine, writes a file or closes its argument, with a doc that
mentions none of it.

Never run. One question per effect from a fixed list: "does the doc mention that the function
<effect>?". A scan of two internal services found 9 structural hits (a direct `go` statement in a
documented function), with no typed effect list behind them.

Unknown that settles it: whether silence about an effect is a defect most reviewers agree on, and a
typed effect list beyond writes.
