package showcase

import (
	"errors"
	"fmt"
	"time"
)

// sentinel-name-vs-text: the name and the message describe different conditions.
var ErrQuotaExceeded = errors.New("tenant record not found") // want `sentinel error name and message describe different conditions`

// error-needs-type (policy): a branchable condition as a plain string.
func findInvoice(id string) error {
	return errors.New("invoice not found") // want `branchable condition returned as a plain string error`
}

// human-unit-contradiction: seconds printed as milliseconds.
func report(d time.Duration) string {
	return fmt.Sprintf("compaction took %.0f ms", d.Seconds()) // want `text says milliseconds, the value is in seconds`
}

// Phase is the state of a copy job.
type Phase int

// enum-comment-shift: the second comment describes the third constant.
const (
	// waiting for a free worker
	PhaseQueued Phase = iota
	// every block has been copied and verified
	PhaseCopying // want `comment describes PhaseVerified, not PhaseCopying`
	PhaseVerified
)
