---
worth: no
added: 2026-10-05
---
# no linter for a package doc that does not say what the package is for

`// Package util contains utilities.`

The prototype (`package-doc-purpose`) asked whether the package doc says what the package is for. It
fired on example `main` packages in logf and on a mocks file in an internal service. Rejected: style,
and the hits were noise.
