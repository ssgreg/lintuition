// Package linttest runs lintuition over a directory of defect / fixed twins and checks the findings
// against `// want "regexp"` (or backquoted) comments, like analysistest does for diagnostics.
//
// A want comment holds one or more patterns, `// want "a" "b"`; each must be matched by its own
// finding on that line, and every finding must be matched by a pattern. Any other finding is
// unexpected. A twin without a want comment is the fixed version: it must stay quiet. The directory
// holds its own .lintuition.yml, which names the classifier: the scripted fake for offline tests, a
// real backend for an evaluation run.
package linttest

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ssgreg/lintuition/internal/report"
	"github.com/ssgreg/lintuition/internal/twins"
)

// Result is what a run produced, for checks beyond the want comments.
type Result struct {
	Issues []report.Issue
	Run    report.Run
}

// Run lints the patterns in dir and checks the findings against the want comments. The run must be
// complete: an incomplete run fails the test, whatever it found.
func Run(t testing.TB, dir string, patterns ...string) *Result {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := twins.Load(abs, "")
	if err != nil {
		t.Fatalf("linttest: %v", err)
	}
	out, err := twins.Run(context.Background(), c, abs, patterns)
	if err != nil {
		t.Fatal(err)
	}
	if out.Result.Run.Incomplete {
		t.Fatalf("linttest: incomplete run:\n  %s", strings.Join(out.Result.Run.Problems, "\n  "))
	}
	for i, w := range out.Wants {
		if !out.Caught[i] {
			t.Errorf("%s: no finding matching %q", w.Key, w.Pattern)
		}
	}
	for _, is := range out.Unexpected {
		t.Errorf("%s:%d: unexpected finding: %s (%s)", is.Pos.Filename, is.Pos.Line, is.Text, is.FromLinter)
	}
	return &Result{Issues: out.Result.Issues, Run: out.Result.Run}
}
