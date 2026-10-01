// Package docvstable holds twins for doc-vs-table.
package docvstable

// Due reports whether the invoice must be paid today or is already late.
func Due(daysLeft int) bool { return daysLeft <= 0 }
