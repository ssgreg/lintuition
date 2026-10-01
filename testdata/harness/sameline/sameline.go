// Package sameline has two findings on each line: one want must not cover both, two wants must.
package sameline

import "github.com/prometheus/client_golang/prometheus"

var _, _ = prometheus.CounterOpts{Help: "Disk I/O utilization."}, prometheus.CounterOpts{Help: "Disk I/O utilization."} // want `current value`

var _, _ = prometheus.CounterOpts{Help: "Disk I/O utilization."}, prometheus.CounterOpts{Help: "Disk I/O utilization."} // want `current value` `current value`
