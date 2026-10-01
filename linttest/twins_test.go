package linttest_test

import (
	"testing"

	_ "github.com/ssgreg/lintuition/builtin"
	"github.com/ssgreg/lintuition/linttest"
)

// TestTwins runs every built-in linter over its defect / fixed twins with scripted answers. It checks
// extraction, policy, decision and reporting together; the live evaluation reuses the same twins
// with a real classifier.
func TestTwins(t *testing.T) {
	res := linttest.Run(t, "../testdata/twins", "./...")
	for _, l := range res.Run.Linters {
		if l.Enabled && l.Asked == 0 {
			t.Errorf("%s: no candidate was asked; its twins test nothing", l.Name)
		}
	}
}
