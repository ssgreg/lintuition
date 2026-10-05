---
worth: later
rank: 610
added: 2026-10-05
---
# no linter for a wrap that repeats the context of the error it wraps

`open config: open config: no such file`, built when both the callee and the caller add the same words.

Never run. The classifier would compare the outer message with the inner one; Go code would resolve the
wrap chain across calls. This is a separate defect from a wrap that adds no context.

Unknown that settles it: wrap-chain facts across function calls, and how often the inner message is a
constant the linter can read.
