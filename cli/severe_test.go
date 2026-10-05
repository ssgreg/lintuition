package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSevereNestedBranch runs severe-event-understated end to end over a log nested in two checked
// branches: the two facts must reach the classifier as a list the payload policy accepts, not
// fail the candidate and leave the run incomplete.
func TestSevereNestedBranch(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	files := map[string]string{
		"go.mod": "module nested\n\ngo 1.22\n",
		"nested.go": `package nested

import (
	"context"
	"errors"
	"log/slog"
)

func Sync(ctx context.Context, jobs chan int, err error) {
	select {
	case <-ctx.Done():
		if errors.Is(err, context.Canceled) {
			slog.Info("the open batch was thrown away")
		}
	case <-jobs:
	}
}
`,
		".lintuition.yml": `version: "2"
linters:
  default: none
  enable: [severe-event-understated]
semantic:
  classifier: fake
  classifiers:
    fake:
      unmatched: error
      rules:
        - {linter: severe-event-understated, question: consequence, choice: unintended_loss, probability: 0.95}
        - {linter: severe-event-understated, question: on_purpose, yes: 0.1}
  cache:
    dir: ` + filepath.Join(dir, "cache") + `
`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, out, errs := run("run", "--dry-run", "--preview", "requests.jsonl", "./...")
	if code != 0 {
		t.Fatalf("dry run: exit %d\n%s%s", code, out, errs)
	}
	b, err := os.ReadFile("requests.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"branch":["error is context.Canceled","context done"]`) {
		t.Errorf("the request lacks the branch list:\n%s", b)
	}
	code, out, errs = run("run", "./...")
	if code != 1 || strings.Contains(errs, "INCOMPLETE") || !strings.Contains(errs, "1 candidates: 1 asked, 0 abstained, 0 skipped, 0 unsupported, 0 failed") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if !strings.Contains(out, `unintended loss logged at info level: "the open batch was thrown away"`) {
		t.Errorf("output:\n%s", out)
	}
}
