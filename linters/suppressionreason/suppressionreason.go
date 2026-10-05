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
// staticcheck SA, S, ST or QF number as a word of its own, a revive rule list in revive's disable
// syntax or before a colon) is compared with that rule's own title from the linter's documentation
// instead. A reason that names a rule the table does not describe, or several rules, is
// unsupported.
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
// and how a reason cites rules. A cited rule missing from the description table is unsupported,
// never replaced by a rule whose name it starts with.
var ruleTables = map[string]struct {
	rules map[string]string
	cite  func(reason string) []string
}{
	"gosec":       {gosecRules, citeIDs(`G[0-9]{3}`)},
	"staticcheck": {staticcheckRules, citeIDs(`(?:SA|ST|QF|S)[0-9]{4}`)},
	"revive":      {reviveRules, citeRevive},
}

// citeIDs returns the IDs a reason cites as words of their own: a whitespace-separated word that,
// without surrounding brackets, quotes and sentence punctuation, is an ID or a list of IDs joined
// by "," or "/" ("G304:", "(G204)", "G304/G703"). An ID inside a larger word is not a citation:
// "G304.json" is a file, "BUG-G304" a ticket, "éG304" a word.
func citeIDs(id string) func(string) []string {
	// The whole word must be the list: a path made of an ID and slashes ("/G304/", "SA1019/") is
	// not one.
	list := regexp.MustCompile(`^(?:` + id + `)(?:[,/](?:` + id + `))*$`)
	return func(reason string) []string {
		var out []string
		for _, w := range strings.Fields(reason) {
			w = strings.TrimRight(strings.TrimLeft(w, `(["'`), `)]"':,.;!?`)
			if !list.MatchString(w) {
				continue
			}
			out = append(out, strings.FieldsFunc(w, func(r rune) bool { return r == ',' || r == '/' })...)
		}
		return out
	}
}

var (
	// revive's disable syntax, as revive reads it: the whole non-blank field after the colon is a
	// comma-separated rule list.
	reviveDisableRE = regexp.MustCompile(`^(?:revive:disable(?:-next-line|-line)?|disable(?:-next-line|-line)):(\S*)`)
	// A heading: a rule name, or a comma-separated list of them, before a colon and a blank, as
	// revive prints its findings ("var-naming: ...").
	reviveHeadingRE = regexp.MustCompile(`^([a-z]+(?:-[a-z0-9]+)*(?:,[a-z]+(?:-[a-z0-9]+)*)*):(?:\s|$)`)
	reviveNameRE    = regexp.MustCompile(`^[a-z]+(?:-[a-z0-9]+)*$`)
)

// citeRevive returns the revive rules a reason cites, at the start of the reason or of a clause
// after "//". The disable syntax always cites: a malformed field is returned whole, so it is
// unsupported rather than cut down to a known prefix. A heading cites when one of its names is a
// revive rule (described or not) or has a hyphen as revive's rule names do; a single unknown word
// ("note:", "todo:") is ordinary prose.
func citeRevive(reason string) []string {
	var out []string
	for _, clause := range strings.Split(reason, "//") {
		clause = strings.TrimSpace(clause)
		if m := reviveDisableRE.FindStringSubmatch(clause); m != nil {
			names := strings.Split(m[1], ",")
			for _, n := range names {
				if !reviveNameRE.MatchString(n) {
					return []string{m[1]} // malformed: cited, and described by nothing
				}
			}
			out = append(out, names...)
			continue
		}
		m := reviveHeadingRE.FindStringSubmatch(clause)
		if m == nil {
			continue
		}
		names := strings.Split(m[1], ",")
		for _, n := range names {
			if _, known := reviveRules[n]; known || reviveUndescribed[n] || strings.Contains(n, "-") {
				out = append(out, names...)
				break
			}
		}
	}
	return out
}

// citedRule returns the one rule a reason cites, and false when it cites several distinct ones. An
// empty rule means the reason cites none, or the linter has no rule table.
func citedRule(linter, reason string) (string, bool) {
	t, ok := ruleTables[linter]
	if !ok {
		return "", true
	}
	var rule string
	for _, id := range t.cite(reason) {
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
		cand.Local["rule"] = rule
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
	if r := c.Local["rule"]; r != "" {
		// The rule is the one the reason names; nothing here knows which rule actually fired.
		return sdk.Report("nolint rationale is about something other than %s %s, the rule it names: %q", c.Local["linter"], r, c.Local["rationale"])
	}
	return sdk.Report("nolint rationale is about something other than what %s reports: %q", c.Local["linter"], c.Local["rationale"])
}
