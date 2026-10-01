// Package report holds the canonical issue model, the issue processors (exclusions, severity, limits,
// sorting) and the output writers.
package report

import (
	"bufio"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/ssgreg/lintuition/internal/config"
)

// Issue is one finding.
type Issue struct {
	FromLinter  string
	Text        string
	Severity    string
	Pos         token.Position
	SourceLines []string
	// Evidence is lintuition's extension: what the decision rested on.
	Evidence *Evidence
	// Fingerprint identifies the finding across runs and line moves: linter, file, subject, text.
	Fingerprint string
}

// Evidence records the classifier answers behind an issue.
type Evidence struct {
	Classifier string             `json:"classifier"`
	Answers    map[string]string  `json:"answers"`
	Scores     map[string]float64 `json:"scores,omitempty"`
	// Model is the backend's model; LinterVersion the version of the rule that decided.
	Model         string `json:"model,omitempty"`
	LinterVersion string `json:"linter_version,omitempty"`
	// Samples is how many answers were voted on; Replayed says they came from the cache.
	Samples  int  `json:"samples"`
	Replayed bool `json:"replayed,omitempty"`
	// Agreement is, per question, how the samples split ("current 2, total 1"). Agreement of
	// correlated samples is not a probability of being right.
	Agreement map[string]string `json:"agreement,omitempty"`
}

// Processor applies exclusions, severity and limits from the config.
type Processor struct {
	// Base is the directory issue paths are relative to.
	Base       string
	exclusions []rule
	paths      []*regexp.Regexp
	severity   []sevRule
	defaultSev string
	maxPer     int
	maxSame    int
	uniqByLine bool
}

type rule struct {
	path, pathExcept, text, source *regexp.Regexp
	linters                        map[string]bool
}

type sevRule struct {
	rule
	severity string
}

// NewProcessor compiles the config's rules.
func NewProcessor(c *config.Config) (*Processor, error) {
	p := &Processor{defaultSev: c.Severity.Default, maxPer: 50, maxSame: 3, uniqByLine: true}
	if c.Issues.MaxIssuesPerLinter != nil {
		p.maxPer = *c.Issues.MaxIssuesPerLinter
	}
	if c.Issues.MaxSameIssues != nil {
		p.maxSame = *c.Issues.MaxSameIssues
	}
	if c.Issues.UniqByLine != nil {
		p.uniqByLine = *c.Issues.UniqByLine
	}
	for i, s := range c.Linters.Exclusions.Paths {
		re, err := regexp.Compile(s)
		if err != nil {
			return nil, fmt.Errorf("linters.exclusions.paths[%d]: %w", i, err)
		}
		p.paths = append(p.paths, re)
	}
	for i, r := range c.Linters.Exclusions.Rules {
		cr, err := compile(fmt.Sprintf("linters.exclusions.rules[%d]", i), r.Path, r.PathExcept, r.Text, r.Source, r.Linters)
		if err != nil {
			return nil, err
		}
		p.exclusions = append(p.exclusions, cr)
	}
	for i, r := range c.Severity.Rules {
		if r.Severity == "" {
			return nil, fmt.Errorf("severity.rules[%d]: severity is required", i)
		}
		cr, err := compile(fmt.Sprintf("severity.rules[%d]", i), r.Path, "", r.Text, "", r.Linters)
		if err != nil {
			return nil, err
		}
		p.severity = append(p.severity, sevRule{rule: cr, severity: r.Severity})
	}
	return p, nil
}

func compile(where, path, pathExcept, text, source string, linters []string) (rule, error) {
	var r rule
	var err error
	re := func(field, s string) *regexp.Regexp {
		if s == "" || err != nil {
			return nil
		}
		var x *regexp.Regexp
		x, err = regexp.Compile(s)
		if err != nil {
			err = fmt.Errorf("%s.%s: %w", where, field, err)
		}
		return x
	}
	r.path, r.pathExcept, r.text, r.source = re("path", path), re("path-except", pathExcept), re("text", text), re("source", source)
	if err != nil {
		return r, err
	}
	if len(linters) > 0 {
		r.linters = map[string]bool{}
		for _, l := range linters {
			r.linters[l] = true
		}
	}
	if r.path == nil && r.pathExcept == nil && r.text == nil && r.source == nil && r.linters == nil {
		return r, fmt.Errorf("%s: a rule needs at least one of path, path-except, text, source, linters", where)
	}
	return r, nil
}

func (r rule) match(is *Issue) bool {
	if r.linters != nil && !r.linters[is.FromLinter] {
		return false
	}
	if r.path != nil && !r.path.MatchString(is.Pos.Filename) {
		return false
	}
	if r.pathExcept != nil && r.pathExcept.MatchString(is.Pos.Filename) {
		return false
	}
	if r.text != nil && !r.text.MatchString(is.Text) {
		return false
	}
	if r.source != nil && (len(is.SourceLines) == 0 || !r.source.MatchString(is.SourceLines[0])) {
		return false
	}
	return true
}

// Covers reports whether a candidate of the linter at the path is excluded whatever the issue text,
// so the classifier need not be asked about it.
func (p *Processor) Covers(linter, path string) bool {
	probe := &Issue{FromLinter: linter, Pos: token.Position{Filename: path}}
	for _, re := range p.paths {
		if re.MatchString(path) {
			return true
		}
	}
	for _, r := range p.exclusions {
		if r.text == nil && r.source == nil && r.match(probe) {
			return true
		}
	}
	return false
}

// Process fills source lines, drops excluded issues, sets severity, applies limits and sorts.
func (p *Processor) Process(issues []Issue) []Issue {
	var kept []Issue
	for _, is := range issues {
		if len(is.SourceLines) == 0 {
			pos := is.Pos
			if !filepath.IsAbs(pos.Filename) {
				pos.Filename = filepath.Join(p.Base, pos.Filename)
			}
			is.SourceLines = sourceLine(pos)
		}
		if p.excluded(&is) {
			continue
		}
		is.Severity = p.defaultSev
		for _, r := range p.severity {
			if r.match(&is) {
				is.Severity = r.severity
				break
			}
		}
		kept = append(kept, is)
	}
	Sort(kept)
	var out []Issue
	perLinter := map[string]int{}
	sameText := map[string]int{}
	lines := map[string]bool{}
	for _, is := range kept {
		lk := fmt.Sprintf("%s:%d", is.Pos.Filename, is.Pos.Line)
		if p.uniqByLine && lines[lk] {
			continue
		}
		if p.maxPer > 0 && perLinter[is.FromLinter] >= p.maxPer {
			continue
		}
		if p.maxSame > 0 && sameText[is.Text] >= p.maxSame {
			continue
		}
		lines[lk] = true
		perLinter[is.FromLinter]++
		sameText[is.Text]++
		out = append(out, is)
	}
	return out
}

func (p *Processor) excluded(is *Issue) bool {
	for _, re := range p.paths {
		if re.MatchString(is.Pos.Filename) {
			return true
		}
	}
	for _, r := range p.exclusions {
		if r.match(is) {
			return true
		}
	}
	return false
}

// Sort orders issues by file, line, column, linter, text: the same input gives the same report
// whatever order the workers finished in.
func Sort(is []Issue) {
	sort.SliceStable(is, func(i, j int) bool {
		a, b := is[i], is[j]
		if a.Pos.Filename != b.Pos.Filename {
			return a.Pos.Filename < b.Pos.Filename
		}
		if a.Pos.Line != b.Pos.Line {
			return a.Pos.Line < b.Pos.Line
		}
		if a.Pos.Column != b.Pos.Column {
			return a.Pos.Column < b.Pos.Column
		}
		if a.FromLinter != b.FromLinter {
			return a.FromLinter < b.FromLinter
		}
		return a.Text < b.Text
	})
}

func sourceLine(pos token.Position) []string {
	f, err := os.Open(pos.Filename)
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		if n == pos.Line {
			return []string{sc.Text()}
		}
	}
	return nil
}
