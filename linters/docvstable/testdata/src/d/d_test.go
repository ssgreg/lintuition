package d

import "testing"

func TestIsExpired(t *testing.T) {
	tests := []struct {
		name  string
		token Token
		want  bool
	}{
		{name: "deadline passed an hour ago", token: New(true), want: true}, // 1 candidate: masked doc
		{name: "deadline tomorrow", token: New(false)},                      // 2 candidate: want omitted, so false
		{name: "computed", token: New(true), want: New(true).Expired},       // 3 unsupported: not a constant
		{name: "", token: New(true), want: true},                            // 4 none: no name
	}
	for _, tt := range tests {
		if got := IsExpired(tt.token); got != tt.want {
			t.Errorf("got %v", got)
		}
	}
}

func TestNamedWant(t *testing.T) {
	tests := []struct {
		name        string
		wantExpired bool
	}{
		{name: "deadline passed", wantExpired: true}, // 5 none: a want named after what it is about
	}
	for _, tt := range tests {
		if IsExpired(New(true)) != tt.wantExpired {
			t.Fail()
		}
	}
}

func TestUndocumented(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "set", want: true}, // 6 none: the function has no doc
	}
	for _, tt := range tests {
		if undocumented(true) != tt.want {
			t.Fail()
		}
	}
}

func TestBindings(t *testing.T) {
	negated := []struct {
		name string
		want bool
	}{
		{name: "deadline passed", want: true}, // 7 unsupported: compared negated
	}
	for _, tt := range negated {
		if !IsExpired(New(true)) != tt.want {
			t.Fail()
		}
	}
	reassigned := []struct {
		name string
		want bool
	}{
		{name: "deadline passed", want: true}, // 8 unsupported: got written again
	}
	for _, tt := range reassigned {
		got := IsExpired(New(true))
		got = !got
		if got != tt.want {
			t.Fail()
		}
	}
	two := []struct {
		name string
		want bool
	}{
		{name: "deadline passed", want: true}, // 9 unsupported: compared with two calls
	}
	for _, tt := range two {
		tok := New(true)
		if IsExpired(tok) != tt.want || tok.Valid() != tt.want {
			t.Fail()
		}
	}
	method := []struct {
		name string
		want bool
	}{
		{name: "fresh token", want: true}, // 10 candidate: a method's doc, through got
	}
	for _, tt := range method {
		tok := New(false)
		got := tok.Valid()
		if got != tt.want {
			t.Fail()
		}
	}
	generic := map[string]struct {
		want bool
	}{
		"empty string": {want: true}, // 11 candidate: a generic function's doc, keyed by the map
	}
	for _, tt := range generic {
		if Generic("") != tt.want {
			t.Fail()
		}
	}
}

type row struct {
	name        string
	input, want bool
}

func TestIsReadyRowMutated(t *testing.T) {
	tests := []row{{name: "worker is ready", input: true, want: false}} // 13 unsupported: tt.want is set in the loop
	for _, tt := range tests {
		tt.want = true
		if IsReady(tt.input) != tt.want {
			t.Fail()
		}
	}
}

func TestIsReadyTableReassigned(t *testing.T) {
	tests := []row{{name: "worker is ready", input: true, want: false}} // 14 unsupported: the table is replaced
	tests = []row{{name: "worker is ready", input: true, want: true}}   // 14 unsupported: the same table variable
	for _, tt := range tests {
		if IsReady(tt.input) != tt.want {
			t.Fail()
		}
	}
}

func TestIsReadyIndexed(t *testing.T) {
	tests := []row{{name: "worker is ready", input: true, want: false}} // 15 unsupported: a row written through the table
	tests[0].want = true
	for _, tt := range tests {
		if IsReady(tt.input) != tt.want {
			t.Fail()
		}
	}
}
