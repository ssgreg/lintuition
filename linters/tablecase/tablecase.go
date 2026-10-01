// Package tablecase checks the rows of table-driven tests: a case whose name says one boolean outcome
// while its expectation field says the other, as when a row is copied and the want is not flipped.
//
//	{name: "rejects an expired token", token: expired, want: true},   // name says false
//
// A person wrote both the name and the want, so the table is evidence nobody had to derive. The
// classifier reads only the case name and what the boolean is about; Go code compares its answer
// with the constant in the table.
//
// A row is checked only when the loop over its table compares the expectation, as is, with a
// result: `got := F(tt.in); got != tt.want`, `F(tt.in) != tt.want`, or an Equal assertion. A
// generic want must be compared with a call's bool result, which names what it is about. A
// negated or otherwise transformed comparison, or comparisons with different calls, make the row
// unsupported rather than guessed.
package tablecase

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/internal/tables"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "table-case-vs-expectation"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of the name's answer for a finding (default 0.85, from
	// the prototype; not yet validated on a labelled set).
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts one candidate per boolean expectation of a named table row.
var Analyzer = &analysis.Analyzer{
	Name:       "tablecase",
	Doc:        "extract named table-test rows with a constant boolean expectation",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a table test case whose name says the opposite of its boolean expectation",
		Standard:    true,
		Version:     "1",
		Analyzer:    Analyzer,
		NewSettings: func() any { return &Settings{} },
		New: func(s any) (sdk.Rule, error) {
			t, err := sdk.Threshold(s.(*Settings).Threshold, 0.85)
			if err != nil {
				return nil, err
			}
			return &rule{threshold: t}, nil
		},
	})
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	var out []*sdk.Candidate
	ins.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fd := n.(*ast.FuncDecl)
		if fd.Body == nil || !facts.IsTest(pass.Fset, fd) {
			return
		}
		for _, row := range tables.Rows(pass, fd.Body) {
			for _, c := range rowCandidates(pass, row) {
				c.Subject = fd.Name.Name + "/" + c.Subject
				out = append(out, c)
			}
		}
	})
	return out, nil
}

func rowCandidates(pass *analysis.Pass, row tables.Row) []*sdk.Candidate {
	var out []*sdk.Candidate
	for _, f := range row.Fields {
		name := f.Var.Name()
		c := &sdk.Candidate{
			Pos: pass.Fset.Position(f.Pos),
			// The field is part of the identity: two expectations of one row are two questions.
			Subject:     row.CaseName + "#" + name,
			Local:       map[string]string{"case_name": row.CaseName, "field": name},
			Unsupported: f.Unsupported,
		}
		if c.Unsupported != "" {
			out = append(out, c)
			continue
		}
		about := "the result of " + calleeName(f.Callee)
		if !tables.IsGeneric(name) {
			about = aboutName(name)
		}
		c.Local["value"] = fmt.Sprint(f.Value)
		c.Payload.AddProse("case_name", row.CaseName)
		c.Payload.Fact("about", about)
		out = append(out, c)
	}
	return out
}

func calleeName(fn *types.Func) string {
	if fn == nil {
		return ""
	}
	return fn.Name()
}

// aboutName says what a named expectation is about: wantInUse -> "whether in use".
func aboutName(name string) string {
	words := facts.Words(name)
	for len(words) > 0 && (words[0] == "want" || words[0] == "expect" || words[0] == "expected") {
		words = words[1:]
	}
	return "whether " + strings.Join(words, " ")
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "outcome",
		Kind: sdk.Choice,
		Text: "The boolean in this test case is about `about`. Reading only the case name `case_name`, does the name say that boolean is true or false?",
		Options: []sdk.Option{
			{Key: "true", Description: "The name states it outright (conflicts, expired, keeps it in use, is missing)."},
			{Key: "false", Description: "The name states the opposite outright (does not conflict, not expired, deletes it)."},
			{Key: "unstated", Description: "The name only describes the input or the setup and says nothing about this boolean."},
		},
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["outcome"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the case name does not say")
	}
	// Weak support abstains whichever way the answer goes.
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice == "unstated" || a.Choice == c.Local["value"] {
		return sdk.Clean()
	}
	return sdk.Report("case %q reads as %s %s, but the table sets %s", c.Local["case_name"], c.Local["field"], a.Choice, c.Local["value"])
}
