package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ssgreg/lintuition/internal/config"
)

// Run is the run-level part of a report: what ran, and whether the run is complete.
type Run struct {
	Linters    []LinterStatus `json:"Linters"`
	Incomplete bool           `json:"Incomplete"`
	// Problems say why a run is incomplete: packages that failed to load, failed requests, a hit budget.
	Problems []string `json:"Problems,omitempty"`
	// Abstentions are the candidates asked but not decided, with the reason.
	Abstentions []Abstention `json:"Abstentions,omitempty"`
	Stats       Stats        `json:"Stats"`
}

// LinterStatus is a linter's line in the run report.
type LinterStatus struct {
	Name       string `json:"Name"`
	Enabled    bool   `json:"Enabled"`
	Candidates int    `json:"Candidates"`
	Asked      int    `json:"Asked"`
	Abstained  int    `json:"Abstained"`
	Skipped    int    `json:"Skipped"`
	// SkippedBy breaks Skipped down by reason: excluded, generated, nolint, payload-policy, no-questions.
	SkippedBy map[string]int `json:"SkippedBy,omitempty"`
	// Unsupported counts shapes the analyzer saw but could not extract facts for.
	Unsupported int `json:"Unsupported"`
	// Planned counts the requests a dry run would have sent.
	Planned int `json:"Planned"`
	Failed  int `json:"Failed"`
	// NotAsked counts candidates left out because the budget ran out.
	NotAsked int `json:"NotAsked"`
}

// Stats are the run totals.
type Stats struct {
	Packages int `json:"Packages"`
	Requests int `json:"Requests"`
	// CacheHits are candidates answered from the cache: replays, not new samples.
	CacheHits int `json:"CacheHits"`
	// Votes is the number of samples per candidate.
	Votes       int     `json:"Votes"`
	InputTokens int     `json:"InputTokens"`
	CostUSD     float64 `json:"CostUSD"`
}

// Write renders the issues to every configured format.
func Write(c *config.Config, issues []Issue, run Run, stdout, stderr io.Writer) error {
	f := c.Output.Formats
	for _, d := range f.Dests() {
		var fn func(io.Writer) error
		switch d.Format {
		case "text":
			fn = func(w io.Writer) error { return writeText(w, f.Text, issues) }
		case "json":
			fn = func(w io.Writer) error { return writeJSON(w, issues, run) }
		case "sarif":
			fn = func(w io.Writer) error { return writeSARIF(w, issues, run) }
		case "checkstyle":
			fn = func(w io.Writer) error { return writeCheckstyle(w, issues) }
		case "code-climate":
			fn = func(w io.Writer) error { return writeCodeClimate(w, issues) }
		case "junit-xml":
			fn = func(w io.Writer) error { return writeJUnit(w, issues) }
		case "github-actions":
			fn = func(w io.Writer) error { return writeGitHubActions(w, issues) }
		}
		if err := to(d.Path, stdout, stderr, fn); err != nil {
			return err
		}
	}
	return nil
}

func to(path string, stdout, stderr io.Writer, fn func(io.Writer) error) error {
	switch path {
	case "", "stdout":
		return fn(stdout)
	case "stderr":
		return fn(stderr)
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := fn(file); err != nil {
		_ = file.Close() // the write's error is the one to report
		return err
	}
	return file.Close()
}

func writeText(w io.Writer, f *config.TextFormat, issues []Issue) error {
	linterName := f.PrintLinterName == nil || *f.PrintLinterName
	issuedLines := f.PrintIssuedLines == nil || *f.PrintIssuedLines
	var b strings.Builder
	for _, is := range issues {
		fmt.Fprintf(&b, "%s:%d:%d: %s", is.Pos.Filename, is.Pos.Line, is.Pos.Column, is.Text)
		if linterName {
			fmt.Fprintf(&b, " (%s)", is.FromLinter)
		}
		b.WriteByte('\n')
		if issuedLines && len(is.SourceLines) > 0 {
			b.WriteString(is.SourceLines[0])
			b.WriteByte('\n')
			if is.Pos.Column > 0 {
				b.WriteString(caret(is.SourceLines[0], is.Pos.Column))
				b.WriteByte('\n')
			}
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// caret puts a ^ under the column, keeping the line's tabs so it lines up.
func caret(line string, col int) string {
	var b strings.Builder
	for i := 0; i < col-1 && i < len(line); i++ {
		if line[i] == '\t' {
			b.WriteByte('\t')
		} else {
			b.WriteByte(' ')
		}
	}
	b.WriteByte('^')
	return b.String()
}

type jsonIssue struct {
	FromLinter  string    `json:"FromLinter"`
	Fingerprint string    `json:"Fingerprint,omitempty"`
	Text        string    `json:"Text"`
	Severity    string    `json:"Severity"`
	SourceLines []string  `json:"SourceLines"`
	Pos         jsonPos   `json:"Pos"`
	Evidence    *Evidence `json:"Lintuition,omitempty"`
}

type jsonPos struct {
	Filename string `json:"Filename"`
	Offset   int    `json:"Offset"`
	Line     int    `json:"Line"`
	Column   int    `json:"Column"`
}

// writeJSON follows golangci-lint's {"Issues": [...], "Report": {...}} shape; lintuition's own
// fields are under "Lintuition" in an issue and in Report.
func writeJSON(w io.Writer, issues []Issue, run Run) error {
	out := struct {
		Issues []jsonIssue `json:"Issues"`
		Report struct {
			Linters    []LinterStatus `json:"Linters"`
			Lintuition Run            `json:"Lintuition"`
		} `json:"Report"`
	}{Issues: []jsonIssue{}}
	for _, is := range issues {
		out.Issues = append(out.Issues, jsonIssue{
			FromLinter: is.FromLinter, Fingerprint: is.Fingerprint, Text: is.Text, Severity: is.Severity, SourceLines: is.SourceLines,
			Pos:      jsonPos{Filename: is.Pos.Filename, Offset: is.Pos.Offset, Line: is.Pos.Line, Column: is.Pos.Column},
			Evidence: is.Evidence,
		})
	}
	out.Report.Linters = run.Linters
	out.Report.Lintuition = run
	enc := json.NewEncoder(w)
	return enc.Encode(out)
}

// Skip counts a skipped candidate under its reason.
func (s *LinterStatus) Skip(reason string) {
	s.Skipped++
	if s.SkippedBy == nil {
		s.SkippedBy = map[string]int{}
	}
	s.SkippedBy[reason]++
}

// Abstention is a candidate the rule did not decide.
type Abstention struct {
	Linter string `json:"Linter"`
	File   string `json:"File"`
	Line   int    `json:"Line"`
	Reason string `json:"Reason"`
}
