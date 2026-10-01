package showcase

import "testing"

// table-case-vs-expectation: the row was copied and its want not flipped. doc-vs-table sees it too:
// Expired's doc says the same thing as the case name.
func TestExpired(t *testing.T) {
	tests := []struct {
		name   string
		passed bool
		want   bool
	}{
		{name: "expired when the deadline has passed", passed: true, want: true},
		{name: "not expired before the deadline", passed: false, want: true}, // want `reads as want false, but the table sets true` `doc of Expired implies false`
	}
	for _, tt := range tests {
		if got := Expired(tt.passed); got != tt.want {
			t.Errorf("%s: got %v", tt.name, got)
		}
	}
}

// doc-vs-table: Due's doc says late invoices are due; this row says otherwise.
func TestDue(t *testing.T) {
	tests := []struct {
		name     string
		daysLate int
		want     bool
	}{
		{name: "three days late", daysLate: 3, want: false}, // want `doc of Due implies true for case "three days late", the test expects false`
	}
	for _, tt := range tests {
		if got := Due(tt.daysLate); got != tt.want {
			t.Errorf("%s: got %v", tt.name, got)
		}
	}
}

// test-name-vs-assertion: the name promises a rejection, the check requires success.
func TestUnquoteRejectsUnbalancedQuote(t *testing.T) { // want `test name expects an error from Unquote, but the test fails when Unquote returns one`
	_, err := Unquote(`"abc`)
	if err != nil {
		t.Fatal(err)
	}
}
