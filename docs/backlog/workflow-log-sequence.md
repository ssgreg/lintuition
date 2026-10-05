---
worth: later
rank: 500
added: 2026-10-05
---
# no linter for start and finish log lines that do not pair up

"starting upload" with no success or failure line on the error path, or "upload done" and "upload
failed" both reachable on one path.

Never run. The classifier tags each message as start, progress, success or failure; Go code checks the
paths. A scan of two internal services found 263 functions with two or more constant log messages, which
are seeds, not candidates.

Unknown that settles it: path facts that pair messages of one operation, and how to tell one operation
from the next inside a function.
