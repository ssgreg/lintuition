// Package testname holds twins for test-name-vs-assertion.
package testname

import "errors"

// ErrBadQuote is returned for an unbalanced quote.
var ErrBadQuote = errors.New("unbalanced quote")

// Unquote removes the quotes around s.
func Unquote(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", ErrBadQuote
	}
	return s[1 : len(s)-1], nil
}
