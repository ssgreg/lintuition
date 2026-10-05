---
worth: later
rank: 600
added: 2026-10-05
---
# no linter for an error that is logged and also returned

`log.Error("open config", "err", err); return fmt.Errorf("open config: %w", err)`: the caller logs it
again and the failure shows up twice.

Never run. Go code can find the pair; the classifier would judge whether the two texts report the same
failure.

Unknown that settles it: whether the classifier adds anything over a structural check. If the pair alone
is enough, this belongs in an ordinary linter.
