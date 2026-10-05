package builtin_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/ssgreg/lintuition/builtin"
	"github.com/ssgreg/lintuition/internal/report"
	"github.com/ssgreg/lintuition/sdk"
)

// TestDocsHeadings keeps the findings' links working: every built-in linter is in
// report.BuiltinLinters and has a `### <name>` heading in docs/linters.md, whose anchor is the name.
func TestDocsHeadings(t *testing.T) {
	var names []string
	for _, l := range sdk.Linters() {
		names = append(names, l.Name)
	}
	if !slices.Equal(names, report.BuiltinLinters) {
		t.Errorf("report.BuiltinLinters = %v,\nregistered %v", report.BuiltinLinters, names)
	}
	b, err := os.ReadFile("../docs/linters.md")
	if err != nil {
		t.Fatal(err)
	}
	headings := map[string]bool{}
	for line := range strings.Lines(string(b)) {
		if h, ok := strings.CutPrefix(strings.TrimRight(line, "\n"), "### "); ok {
			headings[h] = true
		}
	}
	for _, name := range names {
		if !headings[name] {
			t.Errorf("docs/linters.md has no heading `### %s`, so the link to it from a finding is broken", name)
		}
	}
}
