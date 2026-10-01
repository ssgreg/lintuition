// The examples from docs/linters.md as real code: every one is a finding.
module example.com/showcase

go 1.26

require github.com/prometheus/client_golang v1.24.1

replace github.com/prometheus/client_golang => ../sample/promstub
