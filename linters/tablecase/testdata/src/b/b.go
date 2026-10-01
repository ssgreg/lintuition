package b

type Token struct{ Expired bool }

// IsExpired reports whether the token has expired.
func IsExpired(t Token) bool { return t.Expired }

// New returns a token.
func New(expired bool) Token { return Token{Expired: expired} }

// Parse returns the token and an error.
func Parse(s string) (Token, error) { return Token{}, nil }

// table is not a test: its rows are not candidates.
var table = []struct {
	name string
	want bool
}{{name: "expired", want: true}}

// Both returns two booleans.
func Both(v bool) (bool, bool) { return v, !v }
