package d

type Token struct{ Expired bool }

// IsExpired reports whether the token's deadline has passed. IsExpired never looks at the clock.
func IsExpired(t Token) bool { return t.Expired }

// New returns a token.
func New(expired bool) Token { return Token{Expired: expired} }

func undocumented(v bool) bool { return v }

// Valid reports whether the token can still be used.
func (t *Token) Valid() bool { return !t.Expired }

// Generic reports whether v is the zero value.
func Generic[T comparable](v T) bool {
	var zero T
	return v == zero
}
