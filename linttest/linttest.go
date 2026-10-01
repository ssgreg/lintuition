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

var (
	wantRE    = regexp.MustCompile("// want ((?:\\s*(?:`[^`]*`|\"(?:[^\"\\\\]|\\\\.)*\"))+)")
	patternRE = regexp.MustCompile("`[^`]*`|\"(?:[^\"\\\\]|\\\\.)*\"")
)

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
	// Presentation limits would hide the unexpected findings the twins exist to catch.
	zero, no := 0, false
	c.Issues.MaxIssuesPerLinter, c.Issues.MaxSameIssues, c.Issues.UniqByLine = &zero, &zero, &no
	res, err := engine.Run(context.Background(), engine.Options{Config: c, Dir: abs, Patterns: patterns})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.Incomplete {
		t.Fatalf("linttest: incomplete run:\n  %s", strings.Join(res.Run.Problems, "\n  "))
	}
	wants, err := readWants(abs, res.Files)
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
		pats, issues := wants[k], got[k]
		byPat := match(pats, issues)
		used := map[int]bool{}
		for p, i := range byPat {
			if i < 0 {
				t.Errorf("%s: no finding matching %q", k, pats[p])
			} else {
				used[i] = true
			}
		}
		for i, is := range issues {
			if !used[i] {
				t.Errorf("%s: unexpected finding: %s (%s)", k, is.Text, is.FromLinter)
			}
		}
	}
	return &Result{Issues: res.Issues, Run: res.Run}
}

// readWants collects the want comments of the analysed files, the same set the patterns, build tags
// and test selection gave the engine, keyed by file:line relative to dir.
func readWants(dir string, files []string) (map[string][]*regexp.Regexp, error) {
	out := map[string][]*regexp.Regexp{}
	for _, p := range files {
		if err := readFileWants(dir, p, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func readFileWants(dir, p string, out map[string][]*regexp.Regexp) error {
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
		for _, q := range patternRE.FindAllString(m[1], -1) {
			s, err := strconv.Unquote(q)
			if err != nil {
				return fmt.Errorf("%s:%d: bad want: %v", rel, n, err)
			}
			re, err := regexp.Compile(s)
			if err != nil {
				return fmt.Errorf("%s:%d: bad want: %v", rel, n, err)
			}
			k := fmt.Sprintf("%s:%d", rel, n)
			out[k] = append(out[k], re)
		}
	}
	return sc.Err()
}

// match pairs patterns with findings one to one, as many as possible (a maximum bipartite
// matching), so the order of the patterns on a line does not matter. It returns, for each pattern,
// the index of its finding or -1.
func match(pats []*regexp.Regexp, issues []report.Issue) []int {
	ofIssue := make([]int, len(issues))
	for i := range ofIssue {
		ofIssue[i] = -1
	}
	var try func(p int, seen []bool) bool
	try = func(p int, seen []bool) bool {
		for i, is := range issues {
			if seen[i] || !pats[p].MatchString(is.Text) {
				continue
			}
			seen[i] = true
			if ofIssue[i] < 0 || try(ofIssue[i], seen) {
				ofIssue[i] = p
				return true
			}
		}
		return false
	}
	for p := range pats {
		try(p, make([]bool, len(issues)))
	}
	out := make([]int, len(pats))
	for p := range out {
		out[p] = -1
	}
	for i, p := range ofIssue {
		if p >= 0 {
			out[p] = i
		}
	}
	return out
}
