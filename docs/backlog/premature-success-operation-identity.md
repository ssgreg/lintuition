---
worth: later
where: linters/prematuresuccess
added: 2026-10-05
---
# premature-success gives up after any earlier call the message names

To avoid false findings, `premature-success` makes a candidate unsupported when any earlier call in the
function matches the message. That throws away cases where the earlier call worked on a different
receiver or resource. On the 25-repo precision run it had 155 candidates, 49 unsupported and 1 finding,
which held up. An earlier shortcut, treating the next fallible call as the subject of the log, produced
false facts.

Unknown that settles it: receiver and resource binding by object and path, with negatives for aliases,
other receivers and later phases as strong as the recall it brings back.
