---
worth: later
rank: 530
added: 2026-10-05
---
# no linter for a package doc that promises what the package no longer does

`// Package manifest parses and validates manifests.` after the validation moved elsewhere, or a package
whose doc says it never touches storage importing the storage layer.

Never run. Two variants: capability sentences against the exported API, and the stated purpose against
the package's imports. A scan of two internal services found 20 package docs and no written rules about
which layer may import which.

Unknown that settles it: precision of "does an exported function do this?" per capability sentence, and
where an import rule would come from.
