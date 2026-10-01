package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ssgreg/lintuition/builtin"
	"github.com/ssgreg/lintuition/cli"
)

// sample copies examples/sample to a temp dir, so tests can change its config freely.
func sample(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs("../examples/sample")
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	err = filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o700)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dst)
	return dst
}

func run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := cli.Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunSample(t *testing.T) {
	sample(t)
	code, out, errs := run("run", "./...")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s%s", code, out, errs)
	}
	want := `metrics.go:10:30: counter Help describes a current value, not a running total: "Disk I/O utilization." (metric-type-vs-help)`
	if !strings.HasPrefix(out, want+"\n") {
		t.Fatalf("text output:\n%s\nwant first line:\n%s", out, want)
	}
	if !strings.Contains(errs, "1 issue(s). 1 packages, 4 candidates: 4 asked") {
		t.Fatalf("summary: %s", errs)
	}
}

func TestRunExitCodes(t *testing.T) {
	dir := sample(t)
	if code, _, _ := run("run", "--issues-exit-code", "0", "./..."); code != 0 {
		t.Fatalf("--issues-exit-code 0: exit %d", code)
	}
	if code, _, errs := run("run", "--max-requests", "2", "./..."); code != 2 || !strings.Contains(errs, "INCOMPLETE") {
		t.Fatalf("budget cap: exit %d, %s", code, errs)
	}
	if code, _, errs := run("run", "-E", "no-such-linter", "./..."); code != 2 || !strings.Contains(errs, `unknown linter "no-such-linter"`) {
		t.Fatalf("unknown linter: exit %d, %s", code, errs)
	}
	if code, _, errs := run("run", "--payload", "facts", "./..."); code != 0 || !strings.Contains(errs, "4 skipped") {
		t.Fatalf("facts policy must skip the prose-only linter: exit %d, %s", code, errs)
	}
	// A backend failure makes the run incomplete, not clean.
	cfg := filepath.Join(dir, ".lintuition.yml")
	b, _ := os.ReadFile(cfg)
	os.WriteFile(cfg, []byte(strings.Replace(string(b), "      rules:", "      unmatched: error\n      rules: []\n      old:", 1)), 0o600)
	if code, _, errs := run("run", "./..."); code != 2 || !strings.Contains(errs, "field old not found") {
		t.Fatalf("strict classifier settings: exit %d, %s", code, errs)
	}
	os.WriteFile(cfg, []byte("version: \"2\"\nsemantic:\n  classifier: fake\n  classifiers:\n    fake:\n      unmatched: error\n"), 0o600)
	if code, _, errs := run("run", "./..."); code != 2 || !strings.Contains(errs, "no rule answers") || !strings.Contains(errs, "4 failed") {
		t.Fatalf("backend failure: exit %d, %s", code, errs)
	}
	// A package that does not type-check is a problem, never clean.
	os.WriteFile(cfg, b, 0o600)
	os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package sample\n\nvar x int = \"s\"\n"), 0o600)
	if code, _, errs := run("run", "./..."); code != 2 || !strings.Contains(errs, "not analysed") {
		t.Fatalf("broken package: exit %d, %s", code, errs)
	}
}

func TestRunJSONAndDryRun(t *testing.T) {
	sample(t)
	code, out, _ := run("run", "--output.json.path", "stdout", "./...")
	var rep struct {
		Issues []struct {
			FromLinter string
			Pos        struct {
				Filename string
				Line     int
			}
		}
		Report struct{ Lintuition struct{ Incomplete bool } }
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil || code != 1 {
		t.Fatalf("exit %d, %v\n%s", code, err, out)
	}
	if len(rep.Issues) != 1 || rep.Issues[0].Pos.Filename != "metrics.go" || rep.Issues[0].Pos.Line != 10 || rep.Report.Lintuition.Incomplete {
		t.Fatalf("json: %+v", rep)
	}
	code, out, errs := run("run", "--dry-run", "./...")
	if code != 0 || out != "" || !strings.Contains(errs, "4 to ask (4 requests at 1 vote(s) each)") {
		t.Fatalf("dry run: exit %d, out %q, %s", code, out, errs)
	}
}

func TestConfigVerify(t *testing.T) {
	dir := sample(t)
	if code, out, errs := run("config", "verify"); code != 0 || !strings.Contains(out, ": ok") {
		t.Fatalf("verify: exit %d %s %s", code, out, errs)
	}
	os.WriteFile(filepath.Join(dir, ".lintuition.yml"), []byte("version: \"2\"\nlinters:\n  settings:\n    metric-type-vs-help:\n      threshold: 1.5\n"), 0o600)
	if code, _, errs := run("config", "verify"); code != 2 || !strings.Contains(errs, "threshold 1.5") {
		t.Fatalf("verify bad settings: exit %d %s", code, errs)
	}
	if code, out, _ := run("config", "path"); code != 0 || !strings.HasSuffix(strings.TrimSpace(out), ".lintuition.yml") {
		t.Fatalf("path: %d %s", code, out)
	}
}

func TestReferenceConfigVerifies(t *testing.T) {
	ref, err := filepath.Abs("../.lintuition.reference.yml")
	if err != nil {
		t.Fatal(err)
	}
	if code, out, errs := run("config", "verify", "-c", ref); code != 0 {
		t.Fatalf("reference config: exit %d %s %s", code, out, errs)
	}
}
