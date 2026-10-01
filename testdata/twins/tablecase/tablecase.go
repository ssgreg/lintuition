// Package tablecase holds twins for table-case-vs-expectation.
package tablecase

// Expired reports whether a deadline has passed.
func Expired(passed bool) bool { return passed }

// Valid reports whether a token is valid.
func Valid(expired bool) bool { return !expired }
