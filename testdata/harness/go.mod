// Fixtures for linttest's own tests: cases that must make the harness fail.
module harness

go 1.26

require github.com/prometheus/client_golang v1.24.1

replace github.com/prometheus/client_golang => ../../examples/sample/promstub
