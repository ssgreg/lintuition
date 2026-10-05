---
worth: later
rank: 140
added: 2026-10-05
---
# no linter for a doc that promises a copy and returns shared storage

`// Items returns a copy that callers may modify.` on `func (s *Store) Items() []Item { return s.items
}`.

The prototype (`detached-result`) asked whether the doc promises a copy and checked a "returns a
receiver field" fact. It fired once per run on logf, not labelled. A receiver field can be a scalar, a
value struct or an array, which are copied anyway, and a read-only view is not a promise of separate
storage.

Unknown that settles it: aliasing facts by type (does the returned value share backing storage with the
receiver?) and which part of the doc the promise covers.
