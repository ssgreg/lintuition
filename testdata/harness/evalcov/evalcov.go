// Package evalcov feeds the eval command's own tests: two different unmarked findings on one line,
// and a candidate the analyzer cannot read.
package evalcov

import "github.com/prometheus/client_golang/prometheus"

func help() string { return "built at run time" }

var _, _ = prometheus.CounterOpts{Help: "Disk I/O utilization."}, prometheus.CounterOpts{Help: "Disk I/O utilization now."}

var _ = prometheus.CounterOpts{Help: help()}
