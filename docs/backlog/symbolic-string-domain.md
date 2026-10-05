---
worth: later
rank: 640
added: 2026-10-05
---
# no policy linter for one state spelled several ways as bare strings

`status == "canceled"` in one place and `status = "cancelled"` in another, both meaning one state of one
field.

Go code would group comparisons and assignments of one field or type; the classifier tells a symbolic
value from user-facing text. URLs, display text and third-party protocol values are not candidates. A
general "move every string to a const" is a taste rule and stays out of the standard set; this would be
opt-in policy. Never run.

Unknown that settles it: how often such groups exist, and whether the classifier separates symbols from
prose well on short strings.
