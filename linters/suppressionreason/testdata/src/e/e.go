package e

import "os"

func Read(path string) []byte {
	f, _ := os.Open(path)
	defer f.Close() //nolint:errcheck // a close error on a read-only file loses nothing // 1 candidate
	//nolint:gosec // the path comes from the operator's own config // 2 candidate, own line
	g, _ := os.Open(path)
	_ = g
	// 3 none: the directive below gives no reason.
	_ = f.Close() //nolint:errcheck
	_ = f.Close() //nolint // a bare directive suppresses nothing // 4 none
	_ = f.Close() //nolint:all // nothing here matters // 5 none: all
	_ = f.Close() //nolint:errcheck,gosec // both are fine here // 6 unsupported: two linters
	_ = f.Close() //nolint:mylinter // it is fine // 7 unsupported: unknown linter
	_ = f.Close() //nolint:suppression-rationale // checked by hand // 8 candidate: a lintuition linter's Doc
	_ = f.Close() //nolint:quoted-doc // fine // 9 unsupported: the Doc is not a plain fact
	_ = f.Close() //nolint:lll // a URL cannot be wrapped // 10 candidate, the reason stops at the next comment
	_ = f.Close() // see nolint:errcheck // 11 none: not a directive
	_ = f.Close() /*nolint:errcheck // reason*/ // 12 none: a block comment
	_ = f.Close() // nolint:errcheck // spaced directive // 13 candidate: read as lintuition reads it
	return nil
}
