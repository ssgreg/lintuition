---
worth: later
rank: 30
added: 2026-10-05
---
# no linter for the odd pair in a run of parallel lines

```go
p.Lat = q.Lat
p.Lng = q.Lat
```

Go code would find runs of parallel assignments or comparisons and send the name pairs. The classifier
says which names belong together, and the finding is the pair that breaks the run's pattern. Literal
duplicates are left to existing tools; the value is pairs that differ in spelling but are swapped in
meaning (`lat` and `lng`, `min` and `max`). Never run.

Unknown that settles it: how many candidates real code has, and how often the classifier pairs names
confidently enough to report.
