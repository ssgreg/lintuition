# Contributing

Bug reports with a small reproduction are the most useful thing you can send.

For code: `go test -race ./...`, `go vet ./...` and `golangci-lint run` must pass. A linter is one
package under `linters/` with an `analysis.Analyzer` that extracts candidates, a rule with fixed
questions and a decision in Go, analysistest probes, decision tests and defect / fixed twins in
`testdata/twins`. Read [docs/linters.md](docs/linters.md) first; the rules there (facts from
go/types, no source sent, unsupported rather than guessed) are what review checks.

Tests never call a paid classifier. `lintuition eval` with a real backend is for measuring, not for
CI.
