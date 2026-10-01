// Package prometheus is a minimal stand-in for the real client's option types.
package prometheus

type Opts struct {
	Namespace, Subsystem, Name, Help string
}

type CounterOpts Opts

type GaugeOpts Opts

type HistogramOpts struct {
	Namespace, Subsystem, Name, Help string
	Buckets                          []float64
}

type SummaryOpts struct {
	Namespace, Subsystem, Name, Help string
}

type Counter struct{}

func NewCounter(CounterOpts) *Counter { return &Counter{} }

type Gauge struct{}

func NewGauge(GaugeOpts) *Gauge { return &Gauge{} }

type Histogram struct{}

func NewHistogram(HistogramOpts) *Histogram { return &Histogram{} }
