// Package docvstable holds twins for doc-vs-table.
package docvstable

// Due reports whether the invoice must be paid today or is already late.
func Due(daysLeft int) bool { return daysLeft <= 0 }

// Printable reports whether every byte of s is a printable ASCII character: a space through a tilde.
func Printable(s string) bool {
	for i := range len(s) {
		if s[i] < ' ' || s[i] > '~' {
			return false
		}
	}
	return true
}
