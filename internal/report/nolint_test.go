package report

import (
	"os"
	"path/filepath"
	"testing"
)

const src = `package p

func f() error { return nil }

func a() {
	_ = f() //nolint:premature-success // f is a pure lookup and cannot fail
	_ = f() //nolint:premature-success
	_ = f() //nolint
	_ = f() //nolint:all // everything
	_ = f() //nolint:other-tool,premature-success // shared with another linter
	_ = f()
}

//nolint:table-case-vs-expectation // a whole function, explained
func b() {
	_ = f()
	_ = f()
}

func c() {
	//nolint:metric-type-vs-help // the next statement only
	_ = []int{
		1,
	}
	_ = f()
}
`

func TestNolint(t *testing.T) {
	p := filepath.Join(t.TempDir(), "p.go")
	os.WriteFile(p, []byte(src), 0o600)
	n := NewNolint()
	for _, tc := range []struct {
		linter string
		line   int
		want   bool
	}{
		{"premature-success", 6, true},
		{"premature-success", 7, false},  // no explanation
		{"premature-success", 8, false},  // bare
		{"premature-success", 9, false},  // all
		{"premature-success", 10, true},  // a list with a foreign name
		{"premature-success", 11, false}, // no directive
		{"metric-type-vs-help", 6, false},
		{"table-case-vs-expectation", 15, true}, // function scope
		{"table-case-vs-expectation", 17, true},
		{"table-case-vs-expectation", 21, false},
		{"metric-type-vs-help", 22, true}, // statement scope, first line
		{"metric-type-vs-help", 24, true}, // statement scope, last line
		{"metric-type-vs-help", 25, false},
	} {
		if got := n.Covers(p, tc.linter, tc.line); got != tc.want {
			t.Errorf("%s line %d: got %v, want %v", tc.linter, tc.line, got, tc.want)
		}
	}
}

func TestNolintFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "p.go")
	os.WriteFile(p, []byte("//nolint:premature-success // generated-like fixture\npackage p\n\nvar x = 1\n"), 0o600)
	if !NewNolint().Covers(p, "premature-success", 4) {
		t.Fatal("a directive above the package clause covers the file")
	}
}
