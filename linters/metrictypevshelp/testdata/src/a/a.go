package a

import (
	p "github.com/prometheus/client_golang/prometheus"
	"other"
)

const help = "Disk " + "I/O utilization."

type alias = p.GaugeOpts

func dynamic() string { return "built at run time" }

var (
	_ = p.CounterOpts{Name: "io_seconds_total", Help: help}            // constant through a const and a concatenation
	_ = alias{Name: "in_flight", Help: "Requests being served now."}   // an alias of the options type
	_ = p.HistogramOpts{Name: "latency_seconds", Help: "Latency."}     // histogram
	_ = &p.SummaryOpts{Name: "size_bytes", Help: "Sizes."}             // summary through a pointer
	_ = p.CounterOpts{Name: "dynamic_total", Help: dynamic()}          // not constant: no candidate
	_ = p.CounterOpts{Name: "empty_total"}                             // no Help: no candidate
	_ = other.CounterOpts{Name: "unrelated_total", Help: "Unrelated."} // same name, other package: no candidate
)
