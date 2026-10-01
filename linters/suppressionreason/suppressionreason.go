// Package suppressionreason checks the reason given in a nolint directive against what the
// suppressed linter reports:
//
//	defer f.Close() //nolint:errcheck // the file is small, so reading it whole is fine
//
// errcheck reports an unchecked error; the reason talks about the file's size. The reason is
// person-written; what the linter reports is a fixed description: the Doc of a lintuition linter,
// or a short built-in description of a common golangci-lint linter. The classifier is asked only
// whether the reason is about that reported risk or about something else; Go code decides.
//
// A directive is read as lintuition and golangci-lint read it: `//nolint:<linters> // <reason>`.
// The reason is the whole rest of the comment, a second `//` included: a line comment runs to the
// end of the line, and internal/report keeps it all. Only a trailing want mark in the syntax of
// lintuition's twin harness (`// want "regexp"` or backquoted) is cut off, so a twin's expected
// message is never sent as part of its reason. A directive without a
// reason, a bare //nolint and //nolint:all are not candidates. A directive naming several linters
// is unsupported (which risk the reason addresses is not established), as is a linter with no
// known description.
package suppressionreason

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "suppression-rationale"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of "elsewhere" for a finding. The default 0.8 is above the
	// prototype's 0.7: the descriptions of umbrella linters (gosec, staticcheck, govet, gocritic,
	// revive) are coarse, so a reason that addresses the one check that fired can read as
	// "something else" to a classifier that sees only the umbrella. Not yet validated on a
	// labelled set.
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts one candidate per nolint directive with a reason.
var Analyzer = &analysis.Analyzer{
	Name:       "suppressionreason",
	Doc:        "extract nolint directives with their reason and what the suppressed linter reports",
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a nolint reason that explains something other than what the suppressed linter reports",
		Standard:    true,
		Version:     "2",
		Analyzer:    Analyzer,
		NewSettings: func() any { return &Settings{} },
		New: func(s any) (sdk.Rule, error) {
			t, err := sdk.Threshold(s.(*Settings).Threshold, 0.8)
			if err != nil {
				return nil, err
			}
			return &rule{threshold: t}, nil
		},
	})
}

// golangci describes what common golangci-lint linters report, in the words a reason would answer.
var golangci = map[string]string{
	"errcheck":    "an error returned by a call is not checked",
	"gosec":       "a security weakness, such as hard-coded credentials, injection, weak crypto or unsafe file permissions",
	"unused":      "a constant, variable, function or type that is never used",
	"ineffassign": "an assignment to a variable whose value is never used",
	"staticcheck": "a likely bug, a deprecated API or code that can be simplified",
	"govet":       "a suspicious construct, such as wrong printf arguments, a copied lock or an unreachable statement",
	"gocritic":    "a bug-prone, slow or unidiomatic code pattern",
	"revive":      "a style, naming or documentation convention is not followed",
	"lll":         "a line is longer than the configured limit",
	"funlen":      "a function has more lines or statements than the configured limit",
	"gocyclo":     "the cyclomatic complexity of a function is above the configured limit",
	"dupl":        "a block of code duplicates another block",
	"goconst":     "a repeated string literal that could be a named constant",
	"nestif":      "if statements are nested too deeply",
}

var (
	// As internal/report reads directives, so a candidate is a directive that would suppress.
	directiveRE = regexp.MustCompile(`^//\s*nolint:([a-z0-9][a-z0-9,-]*)\s*(?://\s*(.*))?$`)
	// A trailing twin-harness mark, as internal/twins reads it: // want "regexp", or backquoted.
	wantMarkRE = regexp.MustCompile("\\s*// want (?:\\s*(?:`[^`]*`|\"(?:[^\"\\\\]|\\\\.)*\"))+\\s*$")
	// The engine's rule for a fact string; a description that breaks it cannot be sent as one.
	factRE = regexp.MustCompile(`^[A-Za-z0-9_ .,/()-]{0,120}$`)
)

// describe returns what a linter reports: a lintuition linter's Doc, or the built-in description.
func describe(name string) (string, bool) {
	for _, l := range sdk.Linters() {
		if l.Name == name {
			return l.Doc, true
		}
	}
	d, ok := golangci[name]
	return d, ok
}

func run(pass *analysis.Pass) (any, error) {
	var out []*sdk.Candidate
	for _, f := range pass.Files {
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if cand := candidate(pass, c); cand != nil {
					out = append(out, cand)
				}
			}
		}
	}
	return out, nil
}

func candidate(pass *analysis.Pass, c *ast.Comment) *sdk.Candidate {
	m := directiveRE.FindStringSubmatch(c.Text)
	if m == nil {
		return nil
	}
	reason := reasonOf(m[2])
	if reason == "" {
		return nil // no reason to check; lintuition does not honour such a directive anyway
	}
	var names []string
	for _, n := range strings.Split(m[1], ",") {
		if n == "all" {
			return nil // suppresses every risk: there is no one risk to compare with
		}
		if n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return nil
	}
	cand := &sdk.Candidate{
		Pos:     pass.Fset.Position(c.Pos()),
		Subject: strings.Join(names, ",") + " // " + reason,
		Local:   map[string]string{"linter": strings.Join(names, ","), "rationale": reason},
	}
	if len(names) > 1 {
		cand.Unsupported = "the directive names several linters; which one the reason addresses is not established"
		return cand
	}
	desc, ok := describe(names[0])
	switch {
	case !ok:
		cand.Unsupported = "no description of what " + names[0] + " reports"
		return cand
	case !factRE.MatchString(desc):
		cand.Unsupported = "the description of " + names[0] + " is not a plain fact"
		return cand
	}
	cand.Payload.Fact("linter", names[0])
	cand.Payload.Fact("suppressed", desc)
	cand.Payload.AddProse("rationale", reason)
	return cand
}

// reasonOf returns the reason of a directive: the rest of the comment, without a trailing want
// mark of the twin harness.
func reasonOf(rest string) string {
	return strings.TrimSpace(wantMarkRE.ReplaceAllString(rest, ""))
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "about",
		Kind: sdk.Choice,
		// Asked what the reason is about, not whether it justifies the suppression: any reason
		// "justifies" it to a classifier. On eight labelled reasons with jev-latest, the old
		// question called "this function is short and easy to read" an errcheck justification at
		// 0.9; this one called every off-topic reason off-topic at 0.98 or more, and no on-topic
		// reason off-topic.
		Text: "Does the reason `rationale` mention or address what `suppressed` describes?",
		Options: []sdk.Option{
			{Key: "addresses", Description: "Yes: it is about that very thing."},
			{Key: "elsewhere", Description: "No: it is about a different subject."},
		},
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["about"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the reason does not let it tell")
	}
	// Weak support abstains whichever way the answer goes.
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice != "elsewhere" {
		return sdk.Clean()
	}
	return sdk.Report("nolint rationale is about something other than what %s reports: %q", c.Local["linter"], c.Local["rationale"])
}
