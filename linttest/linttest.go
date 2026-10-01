// Package linttest runs lintuition over a directory of defect / fixed twins and checks the findings
// against `// want "regexp"` (or backquoted) comments, like analysistest does for diagnostics.
//
// A line with a want comment must get a finding whose text matches; any other finding is
// unexpected. A twin without a want comment is the fixed version: it must stay quiet. The directory
// holds its own .lintuition.yml, which names the classifier: the scripted fake for offline tests, a
// real backend for an evaluation run.
package linttest

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ssgreg/lintuition/internal/config"
	"github.com/ssgreg/lintuition/internal/engine"
	"github.com/ssgreg/lintuition/internal/report"
)

// Result is what a run produced, for checks beyond the want comments.
type Result struct {
	Issues []report.Issue
	Run    report.Run
}

var wantRE = regexp.MustCompile("// want (`[^`]*`|\"(?:[^\"\\\\]|\\\\.)*\")")

// Run lints the patterns in dir and checks the findings against the want comments. The run must be
// complete: an incomplete run fails the test, whatever it found.
func Run(t testing.TB, dir string, patterns ...string) *Result {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfgPath, err := config.Find(abs)
	if err != nil || cfgPath == "" {
		t.Fatalf("linttest: no .lintuition.yml in or above %s: %v", abs, err)
	}
	c, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	c.Run.RelativePathMode = "wd"
	res, err := engine.Run(context.Background(), engine.Options{Config: c, Dir: abs, Patterns: patterns})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.Incomplete {
		t.Fatalf("linttest: incomplete run:\n  %s", strings.Join(res.Run.Problems, "\n  "))
	}
	wants, err := readWants(abs)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]report.Issue{}
	for _, is := range res.Issues {
		k := fmt.Sprintf("%s:%d", is.Pos.Filename, is.Pos.Line)
		got[k] = append(got[k], is)
	}
	keys := make([]string, 0, len(wants)+len(got))
	seen := map[string]bool{}
	for k := range wants {
		keys, seen[k] = append(keys, k), true
	}
	for k := range got {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		w, issues := wants[k], got[k]
		switch {
		case w == nil:
			for _, is := range issues {
				t.Errorf("%s: unexpected finding: %s (%s)", k, is.Text, is.FromLinter)
			}
		case len(issues) == 0:
			t.Errorf("%s: no finding, want one matching %q", k, w)
		default:
			for _, is := range issues {
				if !w.MatchString(is.Text) {
					t.Errorf("%s: finding %q does not match %q", k, is.Text, w)
				}
			}
		}
	}
	return &Result{Issues: res.Issues, Run: res.Run}
}

// readWants collects the want comments of every .go file under dir, keyed by relative file:line.
func readWants(dir string) (map[string]*regexp.Regexp, error) {
	out := map[string]*regexp.Regexp{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		rel, _ := filepath.Rel(dir, p)
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			m := wantRE.FindStringSubmatch(sc.Text())
			if m == nil {
				continue
			}
			s, err := strconv.Unquote(m[1])
			if err != nil {
				return fmt.Errorf("%s:%d: bad want: %v", rel, n, err)
			}
			re, err := regexp.Compile(s)
			if err != nil {
				return fmt.Errorf("%s:%d: bad want: %v", rel, n, err)
			}
			out[fmt.Sprintf("%s:%d", rel, n)] = re
		}
		return sc.Err()
	})
	return out, err
}
