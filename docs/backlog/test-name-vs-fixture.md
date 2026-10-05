---
worth: later
rank: 230
added: 2026-10-05
---
# no linter for a test whose name describes a different input than the one it uses

`t.Run("rejects an empty identifier", func(t *testing.T) { require.Error(t, validateID("alice")) })`.

The prototype (`test-scenario-fixture`) sent the test name with the source next to it and asked whether
the fixture matches the scenario. Facts-only runs turned it off. `test-name-vs-assertion` covers the
assertion side of the same test.

Unknown that settles it: describing a fixture without its literal value (empty, non-empty, nil, a
constant's name) and binding it to its subtest, then measuring whether real tests drift this way.
