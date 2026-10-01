// Package metric holds twins for metric-type-vs-help.
package metric

import "github.com/prometheus/client_golang/prometheus"

var (
	// Defect: a counter of seconds whose Help describes the derived utilization.
	_ = prometheus.CounterOpts{Name: "io_seconds_total", Help: "Disk I/O utilization."} // want `counter Help describes a current value`
	// Fixed twin.
	_ = prometheus.CounterOpts{Name: "io_seconds_total", Help: "Total seconds spent in disk I/O."}

	// Defect: a gauge whose Help counts events since start.
	_ = prometheus.GaugeOpts{Name: "restarts", Help: "Number of restarts since the process started."} // want `gauge Help describes a running total`
	// Fixed twin.
	_ = prometheus.GaugeOpts{Name: "workers", Help: "Number of workers running now."}

	// Negative: a Help that does not say which kind of value it is must abstain, not report.
	_ = prometheus.HistogramOpts{Name: "x", Help: "See the runbook."}
)
