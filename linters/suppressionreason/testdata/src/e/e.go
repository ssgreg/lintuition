package e

import "os"

func Read(path string) []byte {
	f, _ := os.Open(path)
	defer f.Close() //nolint:errcheck // a close error on a read-only file loses nothing
	//nolint:gosec // the path comes from the operator's own config
	g, _ := os.Open(path)
	_ = g
	_ = f.Close() //nolint:errcheck
	_ = f.Close() //nolint // a bare directive suppresses nothing
	_ = f.Close() //nolint:all // nothing here matters
	_ = f.Close() //nolint:errcheck,gosec // both are fine here
	_ = f.Close() //nolint:mylinter // it is fine
	_ = f.Close() //nolint:suppression-rationale // checked by hand
	_ = f.Close() //nolint:quoted-doc // fine
	_ = f.Close() //nolint:lll // a URL cannot be wrapped // see the style guide
	_ = f.Close() // see nolint:errcheck // not a directive
	_ = f.Close() /*nolint:errcheck // a block comment*/
	_ = f.Close() // nolint:errcheck // spaced directive
	_ = f.Close() //nolint:errcheck // the buffer is small // errors from this best-effort cleanup are deliberately ignored
	return nil
}
