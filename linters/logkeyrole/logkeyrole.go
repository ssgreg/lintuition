// Package logkeyrole finds a structured log field whose key names a different quantity or role than
// the variable logged under it, as when a line is copied and the value is changed but not the key:
//
//	slog.Info("retrying", "remaining_attempts", elapsedAttempts)
//
// Only fields whose value is a variable, by name or through field selections, are checked: the
// variable's name is the evidence a person wrote. A literal, a call result, a constant, a function
// value and an error are not. A key whose words are the value's words (user_id and userID) is the
// same by construction and is not asked about. The classifier is told the keys and the variable
// names, never the values, and asked whether any key names something else. Arguments that cannot
// be read as fields are counted as a separate unsupported candidate beside the asked one.
package logkeyrole

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "log-key-value-role"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of "conflict" for a finding. The default 0.85 is the
	// prototype's; it is not yet validated on a labelled set.
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts log calls with fields whose value is a named variable.
var Analyzer = &analysis.Analyzer{
	Name:       "logkeyrole",
	Doc:        "extract log calls with structured fields whose value is a named variable",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a structured log key that names a different quantity or role than the variable logged under it",
		Standard:    true,
		Version:     "2",
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
	ins.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		for _, c := range candidates(pass, n.(*ast.CallExpr)) {
			c.Subject = facts.EnclosingFunc(stack) + "/" + c.Subject
			out = append(out, c)
		}
		return true
	})
	return out, nil
}

// candidates returns the candidate of a log call and, when some of its arguments could not be read
// as fields while others were asked about, an unsupported one for the unread part: a clean answer
// about the readable fields says nothing about the rest.
func candidates(pass *analysis.Pass, call *ast.CallExpr) []*sdk.Candidate {
	lc, ok := facts.AsLogCall(pass.TypesInfo, call)
	if !ok {
		return nil
	}
	fields, partial := facts.Fields(pass.TypesInfo, lc)
	c := candidate(pass, call, lc, fields, partial)
	if c == nil {
		return nil
	}
	out := []*sdk.Candidate{c}
	if partial && c.Unsupported == "" {
		out = append(out, &sdk.Candidate{
			Pos:         c.Pos,
			Subject:     "unread fields",
			Unsupported: "some log fields are not readable; only the readable ones are asked about",
		})
	}
	return out
}

func candidate(pass *analysis.Pass, call *ast.CallExpr, lc facts.LogCall, fields []facts.LogField, partial bool) *sdk.Candidate {
	c := &sdk.Candidate{Pos: pass.Fset.Position(call.Pos()), Local: map[string]string{}}
	var pairs, descs []string
	for _, f := range fields {
		name, ok := variable(pass.TypesInfo, f.Value)
		if !ok || slices.Equal(facts.Words(f.Key), facts.Words(name)) {
			continue
		}
		d, ok := facts.FieldFact(f)
		if !ok {
			c.Subject = "fields"
			c.Unsupported = "a field key cannot be sent as a fact"
			return c
		}
		pairs = append(pairs, f.Key+" ("+f.ValueDesc+")")
		descs = append(descs, d)
	}
	switch {
	case len(pairs) == 0 && partial:
		c.Subject = "fields"
		c.Unsupported = "the log fields are not readable"
		return c
	case len(pairs) == 0:
		return nil
	}
	c.Subject = strings.Join(pairs, ",")
	c.Local["pairs"] = strings.Join(pairs, ", ")
	c.Payload.Fact("fields", descs)
	return c
}

// variable returns the name of the variable an expression reads, by identifier or field selection
// (x, s.elapsed), resolved through go/types: a constant, a function, a package name or an error
// value is not one.
func variable(info *types.Info, e ast.Expr) (string, bool) {
	var id *ast.Ident
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
		for x := ast.Unparen(e.X); ; {
			if s, ok := x.(*ast.SelectorExpr); ok {
				x = ast.Unparen(s.X)
				continue
			}
			if _, ok := x.(*ast.Ident); !ok {
				return "", false // a call or an index on the way: not a plain variable
			}
			break
		}
	default:
		return "", false
	}
	v, ok := info.Uses[id].(*types.Var)
	if !ok || facts.IsError(v.Type()) {
		return "", false
	}
	return id.Name, true
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "role",
		Kind: sdk.Choice,
		Text: "In `fields`, does any key name a different quantity or role than the variable passed as its value (key remaining_attempts with value elapsedAttempts)?",
		Options: []sdk.Option{
			{Key: "same", Description: "Every key names the same quantity or role as its value."},
			{Key: "conflict", Description: "A key names a different quantity or role than its value."},
			{Key: "insufficient", Description: "Values are not named well enough to tell."},
		},
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["role"]
	if a.Choice == sdk.Unclear || a.Choice == "insufficient" {
		return sdk.Abstain("the keys and values are not named well enough to tell")
	}
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice != "conflict" {
		return sdk.Clean()
	}
	if pairs := c.Local["pairs"]; strings.Contains(pairs, ", ") {
		return sdk.Report("a log key names a different quantity than its value: one of %s", pairs)
	}
	return sdk.Report("log key names a different quantity than its value: %s", c.Local["pairs"])
}
