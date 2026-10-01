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
		{name: "not expired before the deadline", passed: false, want: true}, // want `reads as want false, but the table sets true`
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
