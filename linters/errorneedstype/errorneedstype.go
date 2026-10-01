// Package errorneedstype is a policy linter: it finds a condition a caller would want to branch on,
// returned as a plain string error that the caller can only match by its text.
//
//	if u == nil {
//		return errors.New("user not found") // a caller cannot errors.Is this
//	}
//
// Code finds the shape: a return statement that returns, as one of its results, an error built on
// the spot from a constant message (errors.New, fmt.Errorf, and the pkg/errors, xerrors and
// cockroachdb equivalents) that does not wrap another error. A wrapping error is not this linter's
// business: the caller can branch on the wrapped one. A message built at run time is unsupported.
// The classifier reads only the message and says whether it names a branchable condition.
package errorneedstype

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "error-needs-type"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of yes for a finding, and of no for a clean answer
	// (default 0.8, from the prototype; not yet validated on a labelled set).
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts errors returned on the spot from a constant message.
var Analyzer = &analysis.Analyzer{
	Name:       "errorneedstype",
	Doc:        "extract plain string errors built from a constant message and returned",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "policy: a branchable condition returned as a plain string error instead of a sentinel or type",
		Standard:    false,
		Version:     "1",
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
	var out []*sdk.Candidate
	ins.WithStack([]ast.Node{(*ast.ReturnStmt)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		fn := enclosing(stack)
		for i, res := range n.(*ast.ReturnStmt).Results {
			if c := candidate(pass, res); c != nil {
				c.Subject = fmt.Sprintf("%s/%d", fn, i)
				out = append(out, c)
			}
		}
		return true
	})
	return out, nil
}

// enclosing names the outermost function declaration on the stack ((T).M or F), with "/func" for
// each closure inside it.
func enclosing(stack []ast.Node) string {
	name := ""
	for _, n := range stack {
		switch n := n.(type) {
		case *ast.FuncDecl:
			name = n.Name.Name
			if n.Recv != nil && len(n.Recv.List) > 0 {
				name = "(" + types.ExprString(n.Recv.List[0].Type) + ")." + name
			}
		case *ast.FuncLit:
			name += "/func"
		}
	}
	return name
}

func candidate(pass *analysis.Pass, res ast.Expr) *sdk.Candidate {
	call, ok := ast.Unparen(res).(*ast.CallExpr)
	if !ok {
		return nil
	}
	ec, ok := facts.AsErrorCall(pass.TypesInfo, call)
	if !ok || ec.Wraps {
		return nil
	}
	c := &sdk.Candidate{Pos: pass.Fset.Position(call.Pos()), Local: map[string]string{}}
	switch {
	case !ec.MessageKnown:
		c.Unsupported = "the message is not a constant string"
		return c
	case strings.TrimSpace(ec.Message) == "":
		c.Unsupported = "the message is empty"
		return c
	}
	c.Local["message"] = ec.Message
	c.Payload.AddProse("message", ec.Message)
	return c
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "branchable",
		Kind: sdk.Noul,
		Text: "Does `message` describe a condition a caller would plausibly want to branch on (not found, already exists, permission denied, timeout, closed)?",
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["branchable"]
	if a.Yes == nil {
		return sdk.Abstain("the classifier gave no probability of yes")
	}
	yes := *a.Yes
	switch {
	case yes >= r.threshold:
		return sdk.Report("branchable condition returned as a plain string error; consider a sentinel or type: %q", clip(c.Local["message"], 80))
	case 1-yes >= r.threshold:
		return sdk.Clean()
	}
	// Weak support abstains whichever way the answer goes.
	return sdk.Abstain(fmt.Sprintf("yes at %.2f is not decisive at the threshold %.2f", yes, r.threshold))
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
