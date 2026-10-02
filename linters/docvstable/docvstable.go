// Package docvstable checks a table test's boolean expectation against the doc of the function it
// tests: a row whose situation, by the doc's own condition, gives the opposite of its want.
//
//	// IsExpired reports whether the deadline has passed.
//	{name: "deadline passed an hour ago", want: false}, // the doc says true here
//
// The doc and the case name are person-written; the want is a constant in the table. The row is
// bound to the function the way table-case-vs-expectation binds it (internal/tables): the loop
// over the table compares a generic want (want, expected, ok, ...), as is, with the single bool
// result of one call. The classifier reads the doc, with the function's own name masked, and the
// case name, and says whether the doc's condition for true is met in that situation and whether
// the case name states the facts that condition depends on; Go code compares the first answer with
// the want, and a contradiction found in a name that only labels its input abstains.
//
// The doc is read from the package's own syntax: a function tested from an external _test
// package, whose doc this pass cannot see, is unsupported. A function without a doc says nothing
// and is not a candidate.
package docvstable

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
const Name = "doc-vs-table"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of the doc's answer for a finding (default 0.8, from the
	// prototype; not yet validated on a labelled set).
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts one candidate per generic boolean want of a named table row bound to a
// documented function.
var Analyzer = &analysis.Analyzer{
	Name:       "docvstable",
	Doc:        "extract table-test rows whose want is a documented function's bool result",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a table test case whose boolean want contradicts the tested function's doc",
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

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	docs := map[*types.Func]*ast.FuncDecl{}
	ins.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fd := n.(*ast.FuncDecl)
		if fn, ok := pass.TypesInfo.Defs[fd.Name].(*types.Func); ok {
			docs[fn] = fd
		}
	})
	var out []*sdk.Candidate
	ins.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fd := n.(*ast.FuncDecl)
		if fd.Body == nil || !facts.IsTest(pass.Fset, fd) {
			return
		}
		for _, row := range tables.Rows(pass, fd.Body) {
			for _, f := range row.Fields {
				if !tables.IsGeneric(f.Var.Name()) {
					continue // a want named after what it is about is not the function's result
				}
				if c := candidate(pass, docs, row, f); c != nil {
					c.Subject = fd.Name.Name + "/" + c.Subject
					out = append(out, c)
				}
			}
		}
	})
	return out, nil
}

func candidate(pass *analysis.Pass, docs map[*types.Func]*ast.FuncDecl, row tables.Row, f tables.Field) *sdk.Candidate {
	c := &sdk.Candidate{
		Pos:     pass.Fset.Position(f.Pos),
		Subject: row.CaseName + "#" + f.Var.Name(),
		Local:   map[string]string{"case_name": row.CaseName, "field": f.Var.Name()},
	}
	if f.Unsupported != "" {
		c.Unsupported = f.Unsupported
		return c
	}
	fn := f.Callee.Origin()
	c.Local["function"] = fn.Name()
	decl, ok := docs[fn]
	if !ok {
		c.Unsupported = "the doc of " + fn.Name() + " is not in this package"
		return c
	}
	doc := strings.TrimSpace(decl.Doc.Text())
	if doc == "" {
		return nil // nothing to compare with
	}
	c.Local["value"] = fmt.Sprint(f.Value)
	// The name would be trusted over the doc (IsExpired reads as "true when expired").
	c.Payload.AddProse("doc", sdk.Mask(doc, fn.Name(), "this function"))
	c.Payload.AddProse("situation", row.CaseName)
	return c
}

// statedMin is the probability that the case name states the facts the doc's condition depends on
// at or below which a contradicting answer abstains: the classifier is then at least 0.7 sure the
// name leaves them out. A short label ("ASCII high", "Errata") names the input without saying what
// it is, and the condition answer is a guess about the input; the probability of that guess is no
// lower than for a name that states the facts.
const statedMin = 0.3

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "condition",
		Kind: sdk.Choice,
		Text: "`doc` says when the function returns true. Is that condition met in the situation `situation`? Judge only by the facts the situation describes; ignore any word in it that repeats the function's name.",
		Options: []sdk.Option{
			{Key: "met", Description: "The situation meets the doc's condition for true."},
			{Key: "not_met", Description: "The situation does not meet the doc's condition for true."},
			{Key: "not_covered", Description: "The doc does not say enough to decide."},
		},
	}, {
		ID:   "stated",
		Kind: sdk.Noul,
		Text: "Does `situation` describe the test case in enough detail to tell whether the condition in `doc` holds, without guessing details it leaves out?",
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["condition"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the doc and the situation do not let it tell")
	}
	// Weak support abstains whichever way the answer goes.
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	var implied string
	switch a.Choice {
	case "met":
		implied = "true"
	case "not_met":
		implied = "false"
	default:
		return sdk.Clean()
	}
	if implied == c.Local["value"] {
		return sdk.Clean()
	}
	stated := answers["stated"].Yes
	if stated == nil {
		return sdk.Abstain("the classifier did not say whether the case name states the facts")
	}
	if *stated <= statedMin {
		return sdk.Abstain(fmt.Sprintf("the case name may not state the facts the doc's condition depends on (%.2f)", *stated))
	}
	return sdk.Report("doc of %s implies %s for case %q, the test expects %s", c.Local["function"], implied, c.Local["case_name"], c.Local["value"])
}
