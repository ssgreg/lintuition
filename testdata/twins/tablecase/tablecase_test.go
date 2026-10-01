package tablecase

import "testing"

func TestExpired(t *testing.T) {
	tests := []struct {
		name   string
		passed bool
		want   bool
	}{
		{name: "expired when the deadline has passed", passed: true, want: true},
		// Defect: the row was copied and its want not flipped.
		{name: "not expired before the deadline", passed: false, want: true}, // want `reads as want false, but the table sets true` `doc of Expired implies false for case "not expired before the deadline", the test expects true`
		// Fixed twin.
		{name: "not expired before the deadline (fixed)", passed: false, want: false},
		// Negative: a name that only describes the input says nothing about the result.
		{name: "passed=false", passed: false, want: false},
	}
	for _, tt := range tests {
		if got := Expired(tt.passed); got != tt.want {
			t.Errorf("%s: got %v", tt.name, got)
		}
	}
}

// Two expectations in one row are two questions; the defect is found whichever field comes first.
func TestExpiredAndValid(t *testing.T) {
	tests := []struct {
		name        string
		wantExpired bool
		wantValid   bool
	}{
		{name: "fresh token: not expired and valid", wantExpired: false, wantValid: false},                  // want `reads as wantValid true, but the table sets false`
		{name: "fresh token: not expired and valid (fields swapped)", wantValid: false, wantExpired: false}, // want `reads as wantValid true, but the table sets false`
	}
	for _, tt := range tests {
		if Expired(false) != tt.wantExpired || Valid(false) != tt.wantValid {
			t.Fail()
		}
	}
}
