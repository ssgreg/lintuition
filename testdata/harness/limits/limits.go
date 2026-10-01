// Package limits has four identical findings; the fourth has no want comment and must fail the
// harness even though max-same-issues would hide it from a normal report.
package limits

import "github.com/prometheus/client_golang/prometheus"

var (
	_ = prometheus.CounterOpts{Help: "Disk I/O utilization."} // want `current value`
	_ = prometheus.CounterOpts{Help: "Disk I/O utilization."} // want `current value`
	_ = prometheus.CounterOpts{Help: "Disk I/O utilization."} // want `current value`
	_ = prometheus.CounterOpts{Help: "Disk I/O utilization."}
)
