// Package twins runs lintuition over defect / fixed twins marked with `// want` comments and
// compares the findings with the marks. linttest uses it in Go tests with a scripted classifier;
// lintuition eval uses it with a real one, several times over.
package twins

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/ssgreg/lintuition/internal/config"
	"github.com/ssgreg/lintuition/internal/engine"
	"github.com/ssgreg/lintuition/internal/report"
)

// Outcome is one run compared with the marks. Keys are file:line relative to the directory.
type Outcome struct {
	Result *engine.Result
	// Wants are the expected findings, one entry per pattern.
	Wants []Want
	// Caught[i] says whether Wants[i] got its finding.
	Caught []bool
	// Unexpected are findings no pattern accounts for.
	Unexpected []report.Issue
}

// Want is one expected finding.
type Want struct {
	Key     string
	Pattern *regexp.Regexp
}

// Load finds and loads the configuration for dir, with the presentation limits off: they would hide
// the unexpected findings twins exist to catch.
func Load(dir, cfgPath string) (*config.Config, error) {
	if cfgPath == "" {
		var err error
		if cfgPath, err = config.Find(dir); err != nil {
			return nil, err
		}
		if cfgPath == "" {
			return nil, fmt.Errorf("no .lintuition.yml in or above %s", dir)
		}
	}
	c, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	c.Run.RelativePathMode = "wd"
	zero, no := 0, false
	c.Issues.MaxIssuesPerLinter, c.Issues.MaxSameIssues, c.Issues.UniqByLine = &zero, &zero, &no
	return c, nil
}

// Run lints the patterns in dir once and compares the findings with the marks of the analysed files.
func Run(ctx context.Context, c *config.Config, dir string, patterns []string) (*Outcome, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	res, err := engine.Run(ctx, engine.Options{Config: c, Dir: abs, Patterns: patterns})
	if err != nil {
		return nil, err
	}
	marks, err := ReadWants(abs, res.Files)
	if err != nil {
		return nil, err
	}
	got := map[string][]report.Issue{}
	for _, is := range res.Issues {
		k := fmt.Sprintf("%s:%d", is.Pos.Filename, is.Pos.Line)
		got[k] = append(got[k], is)
	}
	keys := make([]string, 0, len(marks)+len(got))
	seen := map[string]bool{}
	for k := range marks {
		keys, seen[k] = append(keys, k), true
	}
	for k := range got {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := &Outcome{Result: res}
	for _, k := range keys {
		pats, issues := marks[k], got[k]
		byPat := Match(pats, issues)
		used := map[int]bool{}
		for p, i := range byPat {
			out.Wants = append(out.Wants, Want{Key: k, Pattern: pats[p]})
			out.Caught = append(out.Caught, i >= 0)
			if i >= 0 {
				used[i] = true
			}
		}
		for i, is := range issues {
			if !used[i] {
				out.Unexpected = append(out.Unexpected, is)
			}
		}
	}
	return out, nil
}
