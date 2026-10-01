package b

import "testing"

const yes = true

func TestIsExpired(t *testing.T) {
	tests := []struct {
		name    string
		token   Token
		want    bool
		wantErr bool
	}{
		{name: "rejects an expired token", token: New(true), want: true}, // 1 candidate: the result of IsExpired
		{name: "fresh token", token: New(false), want: false},            // 2 candidate
		{name: "named constant", token: New(true), want: yes},            // 3 candidate: value through a constant
		{name: "computed", token: New(true), want: New(true).Expired},    // 4 unsupported: not a constant
		{name: "error flag only", token: New(true), wantErr: true},       // 5 candidate: want omitted, so false (wantErr is not the result)
		{name: "", token: New(true), want: true},                         // 6 none: no name
	}
	for _, tt := range tests {
		if got := IsExpired(tt.token); got != tt.want {
			t.Errorf("got %v", got)
		}
	}
}

func TestPorts(t *testing.T) {
	tests := map[string]*struct {
		attached  bool
		wantInUse bool
	}{
		"keeps the port in use while attached": {attached: true, wantInUse: false}, // 7 candidate: whether in use
	}
	for _, tt := range tests {
		if inUse(tt.attached) != tt.wantInUse {
			t.Fail()
		}
	}
}

func inUse(attached bool) bool { return attached }

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "parses a token", want: true}, // 8 unsupported: Parse returns no bool
	}
	for range tests {
		_, _ = Parse("x")
	}
}

func TestIsExpiredNotRanged(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "table nobody ranges over", want: true}, // 10 unsupported: no binding to IsExpired
	}
	_ = tests
	_ = IsExpired(New(true))
}

func TestIsExpiredSubtests(t *testing.T) {
	for _, tt := range []struct {
		name string
		want bool
	}{
		{name: "ranged literal", want: true}, // 11 candidate: the loop ranges over the literal itself
	} {
		if got := IsExpired(New(tt.want)); got != tt.want {
			t.Fail()
		}
	}
	cases := []struct {
		name string
		want bool
	}{
		{name: "through t.Run", want: false}, // 12 candidate: the call is inside the subtest closure
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if IsExpired(New(tc.want)) != tc.want {
				t.Fail()
			}
		})
	}
}

// Allowed reports whether a value is allowed.
func Allowed(v bool) bool { return v }

func TestBindingNegatives(t *testing.T) {
	negated := []struct {
		name string
		want bool
	}{
		{name: "not expired", want: true}, // 13 unsupported: compared negated
	}
	for _, tt := range negated {
		if IsExpired(New(!tt.want)) != !tt.want {
			t.Fail()
		}
	}
	other := []struct {
		name  string
		input bool
		want  bool
	}{
		{name: "allowed input", input: true, want: true}, // 14 candidate: bound to Allowed, not to the unrelated IsExpired call
	}
	for _, tt := range other {
		_ = IsExpired(New(false))
		if Allowed(tt.input) != tt.want {
			t.Fail()
		}
	}
	two := []struct {
		name        string
		wantExpired bool
		wantValid   bool
	}{
		{name: "not expired and valid", wantExpired: false, wantValid: false}, // 15 two candidates, one per field
	}
	for _, tt := range two {
		if IsExpired(New(false)) != tt.wantExpired || Allowed(true) != tt.wantValid {
			t.Fail()
		}
	}
	mixed := []struct {
		name string
		want bool
	}{
		{name: "compared with two calls", want: true}, // 16 unsupported: two different calls
	}
	for _, tt := range mixed {
		if IsExpired(New(true)) != tt.want || Allowed(true) != tt.want {
			t.Fail()
		}
	}
}

func TestIsExpiredReassigned(t *testing.T) {
	reassigned := []struct {
		name string
		want bool
	}{
		{name: "not expired", want: true}, // 17 unsupported: got is negated after the call
	}
	for _, tt := range reassigned {
		got := IsExpired(New(false))
		got = !got
		if got != tt.want {
			t.Fail()
		}
	}
	later := []struct {
		name string
		want bool
	}{
		{name: "later call", want: true}, // 18 unsupported: got is written again after the comparison
	}
	for _, tt := range later {
		got := Allowed(true)
		if got != tt.want {
			t.Fail()
		}
		got = IsExpired(New(false))
		_ = got
	}
	slots := []struct {
		name string
		want bool
	}{
		{name: "second of two booleans", want: true}, // 19 unsupported: Both has two bool results
	}
	for _, tt := range slots {
		_, valid := Both(true)
		if valid != tt.want {
			t.Fail()
		}
	}
	ok := []struct {
		name string
		want bool
	}{
		{name: "through got", want: true}, // 20 candidate: got := Allowed(...), unmodified, compared
	}
	for _, tt := range ok {
		got := Allowed(true)
		if got != tt.want {
			t.Fail()
		}
	}
}

type readyRow struct {
	name        string
	input, want bool
}

func TestIsReadyWrites(t *testing.T) {
	mutated := []readyRow{
		{name: "row mutated", input: true, want: false}, // 21 unsupported: tt.want is set in the loop
	}
	for _, tt := range mutated {
		tt.want = true
		if Allowed(tt.input) != tt.want {
			t.Fail()
		}
	}
	reassigned := []readyRow{
		{name: "replaced table", input: true, want: false}, // 22 unsupported: the table is replaced
	}
	reassigned = []readyRow{
		{name: "replacing table", input: true, want: true}, // 22 unsupported: the same table variable
	}
	for _, tt := range reassigned {
		if Allowed(tt.input) != tt.want {
			t.Fail()
		}
	}
	indexed := []readyRow{
		{name: "row written through the table", input: true, want: false}, // 23 unsupported: tests[0].want is set
	}
	indexed[0].want = true
	for _, tt := range indexed {
		if Allowed(tt.input) != tt.want {
			t.Fail()
		}
	}
	input := []readyRow{
		{name: "input normalised", input: true, want: true}, // 24 candidate: only the input is written
	}
	for _, tt := range input {
		tt.input = !tt.input
		if Allowed(tt.input) != tt.want {
			t.Fail()
		}
	}
	escaped := []readyRow{
		{name: "row passed on", input: true, want: true}, // 25 unsupported: the row is passed by address
	}
	for _, tt := range escaped {
		fix(&tt)
		if Allowed(tt.input) != tt.want {
			t.Fail()
		}
	}
	lenOnly := []readyRow{
		{name: "len of the table", input: true, want: true}, // 26 candidate: len does not write the table
	}
	if len(lenOnly) == 0 {
		t.Fatal()
	}
	for _, tt := range lenOnly {
		if Allowed(tt.input) != tt.want {
			t.Fail()
		}
	}
}

func fix(r *readyRow) { r.want = !r.want }

func TestIsReadyEarlierLoop(t *testing.T) {
	pointers := []*readyRow{
		{name: "pointer row set earlier", input: true, want: false}, // 27 unsupported: an earlier loop sets tt.want
	}
	for _, tt := range pointers {
		tt.want = true
	}
	for _, tt := range pointers {
		if Allowed(tt.input) != tt.want {
			t.Fail()
		}
	}
	passed := []*readyRow{
		{name: "pointer row passed on earlier", input: true, want: false}, // 28 unsupported: an earlier loop passes the row on
	}
	for _, tt := range passed {
		fix(tt)
	}
	for _, tt := range passed {
		if Allowed(tt.input) != tt.want {
			t.Fail()
		}
	}
	values := []readyRow{
		{name: "value row set earlier", input: true, want: true}, // 29 unsupported: a write in any loop over the table
	}
	for _, tt := range values {
		tt.want = false
	}
	for _, tt := range values {
		if Allowed(tt.input) != tt.want {
			t.Fail()
		}
	}
}
