---
worth: no
added: 2026-10-05
---
# no linter for a unit in a struct tag that contradicts the field doc

``Timeout int `json:"timeout_ms"` // in seconds``.

Never run. A candidate scan of two internal services found 54 JSON fields with a unit suffix in the tag,
and none had a doc comment. Rejected: the classifier part had no candidates, and comparing the tag with
the field name is a token check that needs no classifier.
