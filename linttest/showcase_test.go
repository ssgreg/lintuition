package linttest_test

import (
	"testing"

	"github.com/ssgreg/lintuition/linttest"
)

// TestShowcase runs every linter over examples/showcase, the examples of docs/linters.md as real
// code, with scripted answers: each documented example must be a finding, and nothing else.
func TestShowcase(t *testing.T) {
	res := linttest.Run(t, "../examples/showcase", "./...")
	for _, l := range res.Run.Linters {
		if l.Enabled && l.Asked == 0 {
			t.Errorf("%s: no candidate in the showcase", l.Name)
		}
	}
}
