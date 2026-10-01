// Package logkeyrole holds twins for log-key-value-role.
package logkeyrole

import "log/slog"

// Defect: the key says remaining, the value counts the attempts made.
func retryDefect(elapsedAttempts int) {
	slog.Warn("retrying upload", "remaining_attempts", elapsedAttempts) // want `log key names a different quantity than its value: remaining_attempts \(elapsedAttempts\)`
}

// Fixed twin: the key names the same quantity in other words.
func retryFixed(elapsedAttempts int) {
	slog.Warn("retrying upload", "attempts_made", elapsedAttempts)
}

// Negative: the value's name says too little to compare.
func retryVague(v int) {
	slog.Warn("retrying upload", "remaining_attempts", v)
}

// Negative: the key spells the variable's name; it is not even asked about.
func retrySame(elapsedAttempts int) {
	slog.Warn("retrying upload", "elapsed_attempts", elapsedAttempts)
}
