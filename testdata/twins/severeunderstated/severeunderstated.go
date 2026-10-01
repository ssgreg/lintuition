// Package severeunderstated holds twins for severe-event-understated.
package severeunderstated

import "log/slog"

type Event struct{ ID string }

// Defect: events are lost for good, and the line says so at info level.
func enqueueDefect(q chan Event, e Event) {
	select {
	case q <- e:
	default:
		slog.Info("queue full, event dropped and not retried") // want `unintended loss logged at info level`
	}
}

// Fixed twin: the same loss at error level is not this linter's business.
func enqueueFixed(q chan Event, e Event) {
	select {
	case q <- e:
	default:
		slog.Error("queue full, event dropped and not retried")
	}
}

// Negative: a requested deletion is routine progress.
func purge(ids []string) {
	slog.Info("deleted expired sessions as requested")
}

// Negative: a loss the program chose on purpose (sampling) is not understated.
func sample(e Event) {
	slog.Debug("debug event dropped by the sampler as configured")
}
