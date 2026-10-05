---
worth: no
added: 2026-10-05
---
# no linter for a method that does not fit its type's role

`func (s *Store) Render(w io.Writer)` on a type documented as a key-value store.

Never run. Rejected: style, with no ground truth for which methods belong; a candidate scan only gave a
population of documented types.