// Package showcase holds the examples from docs/linters.md as real code. Every line marked
// `// want` is a finding; run `lintuition run ./...` here to see them (offline, scripted answers) or
// `lintuition run -c jev.yml ./...` against a real classifier.
package showcase

import prom "github.com/prometheus/client_golang/prometheus"

// metric-type-vs-help: a counter of seconds whose Help describes utilization.
var ioSeconds = prom.NewCounter(prom.CounterOpts{Name: "io_seconds_total", Help: "Disk I/O utilization."}) // want `counter Help describes a current value`
