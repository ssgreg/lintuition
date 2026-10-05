package docvstable

import "testing"

func TestDue(t *testing.T) {
	tests := []struct {
		name     string
		daysLeft int
		want     bool
	}{
		// Defect: the row was copied from the one below and its want not flipped.
		{name: "three days late", daysLeft: -3, want: false}, // want `doc of Due implies true for case "three days late", the test expects false`
		// Fixed twin.
		{name: "three days late, fixed", daysLeft: -3, want: true},
		// Defect: the opposite way.
		{name: "a week before the date", daysLeft: 7, want: true}, // want `doc of Due implies false for case "a week before the date", the test expects true`
		// Fixed twin.
		{name: "a week before the date, fixed", daysLeft: 7, want: false},
		// Negative: the doc does not say anything about currencies.
		{name: "invoice in another currency", daysLeft: 1, want: false},
	}
	for _, tt := range tests {
		if got := Due(tt.daysLeft); got != tt.want {
			t.Errorf("%s: got %v", tt.name, got)
		}
	}
}

func TestPrintable(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		// Defect: by the doc a tab is not printable; the row says it is.
		{name: "a string with a tab character", in: "a\tb", want: true}, // want `doc of Printable implies false for case "a string with a tab character", the test expects true`
		// Fixed twin.
		{name: "a string with a tab character, fixed", in: "a\tb", want: false},
		// Negative: the name only labels the input. A guess from "upper end" reads as past the
		// range; the input is the tilde, which is printable.
		{name: "upper end", in: "~", want: true},
		// Negative: as above, at the bottom of the range.
		{name: "lower end", in: " ", want: true},
	}
	for _, tt := range tests {
		if got := Printable(tt.in); got != tt.want {
			t.Errorf("%s: got %v", tt.name, got)
		}
	}
}
