---
worth: later
rank: 25
where: linters/humanunit/humanunit.go
added: 2026-10-05
---
# no linter for a unit in a variable name that the value contradicts

```go
timeoutMs := 5 * time.Second // a Duration, the name says milliseconds
sizeMB := n / 1024           // one division by 1024 is kilobytes
```

The classifier reads the unit the name states (`Ms`, `Sec`, `KB`, `MB`, `Bytes`, `Pct`). Go code proves
the other side from the type (`time.Duration`) or the conversion factor applied to a value of a known
unit. `human-unit-contradiction` already does this for message text, so the conversion facts exist; this
reads the unit from an identifier instead of prose.

Unknown that settles it: how often names carry a unit next to a value whose unit the code can prove, and
whether suffixes like `Ms` read reliably once split from the name.
