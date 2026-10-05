---
worth: later
rank: 290
added: 2026-10-05
---
# no linter for an internal error shown to an end user

`http.Error(w, err.Error(), http.StatusInternalServerError)` where `err` carries a file path or a SQL
fragment.

Never run. The classifier would ask whether a message exposes internals (paths, queries, host names,
stack traces); Go code would establish that the message reaches a response or CLI output.

Unknown that settles it: sink facts. Messages that leak are usually built at run time, so it is also
open whether enough constant text exists for the classifier to read.
