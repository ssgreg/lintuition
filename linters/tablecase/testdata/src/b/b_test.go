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
		{name: "computed", token: New(true), want: New(true).Expired},    // 4 none: not a constant
		{name: "error flag only", token: New(true), wantErr: true},       // 5 none: wantErr is not the result
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
	_ = tests
}

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
		_ = IsExpired(New(tt.want))
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
