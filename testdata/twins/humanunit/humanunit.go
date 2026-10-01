// Package humanunit holds twins for human-unit-contradiction.
package humanunit

import (
	"fmt"
	"time"
)

// Defect: the text says milliseconds, the value is in seconds.
func reportDefect(elapsed time.Duration) string {
	return fmt.Sprintf("compaction took %.0f ms", elapsed.Seconds()) // want `text says milliseconds, the value is in seconds`
}

// Fixed twin: the unit method matches the text.
func reportFixed(elapsed time.Duration) string {
	return fmt.Sprintf("compaction took %d ms", elapsed.Milliseconds())
}

// Negative: the text names no unit.
func backoff(wait time.Duration) string {
	return fmt.Sprintf("compaction retry, backoff factor %.1f", wait.Seconds())
}

// Negative: a Duration printed as is carries its own unit; it is not asked.
func reportDuration(elapsed time.Duration) string {
	return fmt.Sprintf("compaction took %v", elapsed)
}

// Negative: weak support abstains.
func reportVague(elapsed time.Duration) string {
	return fmt.Sprintf("compaction window %.0f", elapsed.Minutes())
}
