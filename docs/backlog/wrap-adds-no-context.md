---
worth: later
rank: 620
where: internal/facts/errors.go:30
added: 2026-10-05
---
# no linter for an error wrap that adds no context

`return fmt.Errorf("failed: %w", err)`.

The prototype (`errorf-empty-context`) asked whether the wrap says which operation or object failed,
beyond "failed" or "error". Seeded pair: caught 3 of 3, no false alarm. On three internal services it
fired 9, 6 and 6 times, several inside tests; none was labelled.

Unknown that settles it: precision on labelled real findings, and whether it belongs in lintuition. A
wrap with no context is missing information, not text that contradicts the code.
