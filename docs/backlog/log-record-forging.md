---
worth: later
rank: 650
added: 2026-10-05
---
# no linter for external text that can forge a log record

`log.Printf("login failed for %s", r.FormValue("user"))` with a plain text logger: a newline in the user
name starts a fake record.

Taint, sink and escaping are all Go code's job; the classifier could at most judge the role of the
message. A structured JSON logger, or a newline kept inside a safe value, is not a defect. Never run.

Unknown that settles it: a source-to-sink path the analyzer can prove. If a deterministic rule is
enough, the classifier adds nothing and this belongs in an ordinary security linter.
