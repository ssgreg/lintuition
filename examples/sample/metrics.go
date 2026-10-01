// Package sample is a small project to try lintuition on, offline, with the fake classifier.
package sample

import prom "github.com/prometheus/client_golang/prometheus"

const ioHelp = "Disk I/O utilization."

var (
	// A counter of seconds whose Help describes the derived utilization: the finding.
	ioSeconds = prom.NewCounter(prom.CounterOpts{Name: "io_seconds_total", Help: ioHelp})

	// The fixed twin: the Help says what the counter adds up.
	ioSecondsFixed = prom.NewCounter(prom.CounterOpts{Name: "io_seconds_fixed_total", Help: "Total seconds spent in disk I/O."})

	inFlight = prom.NewGauge(prom.GaugeOpts{Name: "requests_in_flight", Help: "Number of requests being served now."})

	latency = prom.NewHistogram(prom.HistogramOpts{Name: "request_duration_seconds", Help: "Request latency."})
)
