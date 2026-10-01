package showcase

import (
	"errors"
	"os"
	"time"
)

// Expired reports whether a deadline has passed.
func Expired(passed bool) bool { return passed }

// Due reports whether the invoice is past its due date.
func Due(daysLate int) bool { return daysLate > 0 }

// ErrUnbalanced is returned for a quote without its pair.
var ErrUnbalanced = errors.New("unbalanced quote")

// Unquote removes the quotes around s.
func Unquote(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", ErrUnbalanced
	}
	return s[1 : len(s)-1], nil
}

// readSmall reads a small file. suppression-rationale: the reason talks about size, errcheck about
// the unchecked error.
func readSmall(p string) []byte {
	data, _ := os.ReadFile(p) //nolint:errcheck // the file is small, so reading it whole is fine // want `nolint rationale is about something other than what errcheck reports`
	return data
}

var _ = time.Second
