// Package suppressionreason checks the reason given in a nolint directive against what the
// suppressed linter reports:
//
//	defer f.Close() //nolint:errcheck // the file is small, so reading it whole is fine
//
// errcheck reports an unchecked error; the reason talks about the file's size. The reason is
// person-written; what the linter reports is a fixed description: the Doc of a lintuition linter,
// or a short built-in description of a common golangci-lint linter. The classifier is asked only
// whether the reason argues about that reported thing or only about some other property of the
// code; Go code decides.
//
// gosec, staticcheck and revive report many unrelated things under one name, and a reason usually
// answers the one rule that fired: "G304: a constant file name under the build directory". Their
// one-line description cannot carry that, so a reason that names a rule (a gosec G-number, a
// staticcheck SA, S, ST or QF number, a revive rule name before a colon) is compared with that
// rule's own title from the linter's documentation instead. A reason that names a rule the table
// does not describe, or several rules, is unsupported.
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
	// Threshold is the minimum probability of the answer for a decision either way (default 0.9).
	// On 147 labelled reasons with jev-latest, 0.85 reported three reasons that do answer their
	// rule, two of them a reason the classifier puts at 0.83 to 0.88 from one sample to the next;
	// 0.9 reported one and kept 15 of 20 reasons about something else.
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
		Version:     "3",
		Analyzer:    Analyzer,
		NewSettings: func() any { return &Settings{} },
		New: func(s any) (sdk.Rule, error) {
			t, err := sdk.Threshold(s.(*Settings).Threshold, 0.9)
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

// ruleTables are the linters whose reasons name the rule they answer, with each rule's description
// and the pattern of a rule ID in a reason.
var ruleTables = map[string]struct {
	rules map[string]string
	id    func(reason string) []string
}{
	"gosec":       {gosecRules, matchAll(regexp.MustCompile(`\bG[0-9]{3}\b`))},
	"staticcheck": {staticcheckRules, matchAll(regexp.MustCompile(`\b(?:SA|ST|QF|S)[0-9]{4}\b`))},
	"revive":      {reviveRules, reviveRule},
}

func matchAll(re *regexp.Regexp) func(string) []string {
	return func(reason string) []string { return re.FindAllString(reason, -1) }
}

// A revive rule name in a reason: first, before a colon, as revive prints its findings
// ("var-naming: ..."), or after revive's own disable syntax ("disable-line:unused-receiver"). A rule
// name anywhere else is an ordinary word ("range", "defer") and is not read as one.
var reviveRuleRE = regexp.MustCompile(`^(?:(?:revive:)?disable(?:-next)?-line:([a-z]+(?:-[a-z]+)*)\b|([a-z]+(?:-[a-z]+)*):)`)

func reviveRule(reason string) []string {
	m := reviveRuleRE.FindStringSubmatch(reason)
	switch {
	case m == nil:
		return nil
	case m[1] != "":
		return []string{m[1]}
	case reviveRules[m[2]] != "":
		return []string{m[2]}
	}
	return nil // a leading word with a colon that names no revive rule ("note: ...")
}

// citedRule returns the one rule a reason names, and false when it names several distinct ones. An
// empty rule means the reason names none, or the linter has no rule table.
func citedRule(linter, reason string) (string, bool) {
	t, ok := ruleTables[linter]
	if !ok {
		return "", true
	}
	var rule string
	for _, id := range t.id(reason) {
		if rule != "" && id != rule {
			return "", false
		}
		rule = id
	}
	return rule, true
}

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
	linter := names[0]
	rule, one := citedRule(linter, reason)
	if !one {
		cand.Unsupported = "the reason names several " + linter + " rules; which one it addresses is not established"
		return cand
	}
	desc, ok := describe(linter)
	if rule != "" {
		desc, ok = ruleTables[linter].rules[rule]
	}
	switch {
	case !ok && rule != "":
		cand.Unsupported = "no description of what " + linter + " rule " + rule + " reports"
		return cand
	case !ok:
		cand.Unsupported = "no description of what " + linter + " reports"
		return cand
	case !factRE.MatchString(desc):
		cand.Unsupported = "the description of " + linter + " is not a plain fact"
		return cand
	}
	cand.Payload.Fact("linter", linter)
	if rule != "" {
		cand.Payload.Fact("rule", rule)
		cand.Local["reported"] = linter + " " + rule
	} else {
		cand.Local["reported"] = linter
	}
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
		// Asked what the reason argues about, not whether it justifies the suppression: any reason
		// "justifies" it to a classifier, which called "this function is short and easy to read" an
		// errcheck justification at 0.9. The question before this one asked whether the reason
		// mentions what `suppressed` describes, and read the subject literally: "G703: a file name
		// this program generated" was not about "path traversal" to it, because the reason argues
		// where the path comes from instead of naming the risk. The options now say what an argument about
		// the report looks like, and what talking about something else looks like.
		Text: "`suppressed` is what a linter reported on this line, and `rationale` is the reason a person gave for silencing it. Does the reason argue about the reported thing itself, or does it only talk about some other property of the code?",
		Options: []sdk.Option{
			{Key: "addresses", Description: "It argues about the reported thing, or restates it: why that risk does not apply here, where the input or value comes from and why it is safe, or why the construct is needed, intended or unavoidable."},
			{Key: "elsewhere", Description: "It talks only about something the report is not about, such as the code's size, speed, age, callers or readability, and says nothing about the reported thing."},
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
	return sdk.Report("nolint rationale is about something other than what %s reports: %q", c.Local["reported"], c.Local["rationale"])
}
