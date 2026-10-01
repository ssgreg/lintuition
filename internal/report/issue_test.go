package report

import (
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ssgreg/lintuition/internal/config"
)

func load(t *testing.T, body string) *config.Config {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".lintuition.yml")
	os.WriteFile(p, []byte(body), 0o600)
	c, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func issue(linter, file string, line int, text string) Issue {
	return Issue{FromLinter: linter, Text: text, Pos: token.Position{Filename: file, Line: line, Column: 1}}
}

func TestProcess(t *testing.T) {
	c := load(t, `version: "2"
linters:
  exclusions:
    paths: ["^gen/"]
    rules:
      - path: _test\.go$
        linters: [a]
      - text: "ignore me"
severity:
  default: warning
  rules:
    - linters: [b]
      severity: error
issues:
  max-same-issues: 1
`)
	p, err := NewProcessor(c)
	if err != nil {
		t.Fatal(err)
	}
	got := p.Process([]Issue{
		issue("b", "z.go", 1, "same"),
		issue("a", "x.go", 2, "same"),
		issue("a", "gen/x.go", 1, "path excluded"),
		issue("a", "x_test.go", 1, "rule excluded"),
		issue("b", "x_test.go", 1, "kept: other linter"),
		issue("a", "y.go", 1, "please ignore me"),
		issue("a", "x.go", 2, "same line, dropped by uniq-by-line"),
	})
	var lines []string
	for _, is := range got {
		lines = append(lines, is.Pos.Filename+" "+is.FromLinter+" "+is.Severity+" "+is.Text)
	}
	want := "x.go a warning same\nx_test.go b error kept: other linter"
	if strings.Join(lines, "\n") != want {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(lines, "\n"), want)
	}
	if !p.Covers("a", "gen/y.go") || !p.Covers("a", "q_test.go") || p.Covers("b", "q_test.go") || p.Covers("a", "y.go") {
		t.Fatal("Covers must hold only for exclusions that match whatever the text")
	}
}

func TestRuleNeedsAField(t *testing.T) {
	c := load(t, "version: \"2\"\nlinters:\n  exclusions:\n    rules:\n      - {}\n")
	if _, err := NewProcessor(c); err == nil || !strings.Contains(err.Error(), "needs at least one") {
		t.Fatalf("got %v", err)
	}
}
