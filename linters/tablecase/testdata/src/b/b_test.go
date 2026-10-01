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
