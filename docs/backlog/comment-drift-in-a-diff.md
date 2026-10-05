---
worth: later
rank: 160
added: 2026-10-05
---
# no linter for a comment a change made untrue

A change flips a function from "returns nil when not found" to returning `ErrNotFound`, and the comment
above it stays.

Never run. The classifier would ask whether the unchanged comment still holds, given the facts that
changed.

Instead of sending changed source, compare old and new normalized facts of one object: the kind of
result, a returned constant, a write target, a close or cancel contract. A changed contract is a reason
to ask about the doc again; an untouched doc is not stale by default. Renamed or moved symbols and
dependency changes need mapping.

Unknown that settles it: diff mode (load the change, restrict findings to it), which lintuition does not
have, and whether a fact delta without the changed source is enough to judge.
