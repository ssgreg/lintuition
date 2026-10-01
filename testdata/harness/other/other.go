// Package other is not selected by a ./limits pattern; its want must not be demanded then.
package other

import "github.com/prometheus/client_golang/prometheus"

var _ = prometheus.CounterOpts{Help: "Total seconds spent."} // want `never reported`
