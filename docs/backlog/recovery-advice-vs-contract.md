---
worth: later
rank: 560
added: 2026-10-05
---
# no linter for recovery advice the operation's contract does not allow

`return errors.New("payment timed out; submit it again")` where submitting is not idempotent and a retry
can charge twice.

Never run. The classifier would read what the advice tells the user to do; Go code would need a fact
about the operation's contract.

Unknown that settles it: where contract facts such as idempotency come from. Nothing extracts them
today, and they may need a person-written annotation.
