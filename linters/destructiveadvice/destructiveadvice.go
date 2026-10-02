// Package destructiveadvice finds an error or log message that tells its reader to delete, wipe,
// reset or reinstall something that holds data, without saying what is lost or how to keep it:
//
//	return errors.New("index is corrupt; delete the data directory and restart")
//
// Code finds error constructors (errors.New, fmt.Errorf, pkg/errors) and log calls at warn level
// and above or without a level, resolved through go/types, with a constant text of at least one
// word, and asks about every one. Debug and info logs are not read, to cut the false positives of a
// program narrating its own steps ("purge temp files" right before purging them); a destructive
// instruction at those levels is a coverage limit. There is no keyword prefilter: "format the
// data volume", "overwrite the database" and "run mkfs" advise destruction in words no list
// anticipates, and a missed text looks checked. The cost is a request per constant message;
// semantic.budget bounds it.
//
// The classifier reads the text, the log level, and the function the statement after a log call
// calls, when code can tell it, so that a warning followed by the deletion
// it names reads as the program's own step. It is asked whether the text advises the reader or
// names the program's own action, and separately whether it says what would be lost or how to
// preserve it first.
package destructiveadvice

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "destructive-remediation"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of "destructive_advice" for a finding. The default 0.85 is
	// the prototype's; it is not yet validated on a labelled set.
	Threshold *float64 `yaml:"threshold"`
}

// statesLossMax is the most probability of "states what is lost" a finding allows, and
// statesLossMin the least that makes the advice informed (clean); in between the rule abstains.
// 0.3 is the prototype's.
const (
	statesLossMax = 0.3
	statesLossMin = 0.7
)

// Analyzer extracts constant error and log texts.
var Analyzer = &analysis.Analyzer{
	Name:       "destructiveadvice",
	Doc:        "extract constant error and log texts",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "an error or log message that advises deleting, wiping, resetting or reinstalling without saying what is lost",
		Standard:    true,
		Version:     "3",
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
		if c := candidate(pass, n.(*ast.CallExpr), stack); c != nil {
			c.Subject = facts.EnclosingFunc(stack) + "/" + c.Subject
			out = append(out, c)
		}
		return true
	})
	return out, nil
}

func candidate(pass *analysis.Pass, call *ast.CallExpr, stack []ast.Node) *sdk.Candidate {
	var kind, text, level string
	var known bool
	if ec, ok := facts.AsErrorCall(pass.TypesInfo, call); ok {
		kind, text, known = "error message", ec.Message, ec.MessageKnown
	} else if lc, ok := facts.AsLogCall(pass.TypesInfo, call); ok {
		if lc.Level == facts.LevelDebug || lc.Level == facts.LevelInfo {
			return nil
		}
		kind, text, known, level = "log message", lc.Message, lc.MessageKnown, lc.Level
	} else {
		return nil
	}
	c := &sdk.Candidate{Pos: pass.Fset.Position(call.Pos()), Local: map[string]string{"kind": kind}}
	if !known {
		c.Subject = "text"
		c.Unsupported = "the " + kind + " is not a constant string"
		return c
	}
	if !wordRE.MatchString(text) {
		return nil // "%s: %v" advises nothing
	}
	c.Subject = text
	c.Local["text"] = text
	c.Payload.AddProse("text", text)
	c.Payload.Fact("kind", kind)
	if level != "" {
		c.Payload.Fact("level", level)
		if next, ok := nextCall(pass, level, stack); ok {
			c.Payload.Fact("next_call", next)
		}
	}
	return c
}

// nextCall names the function the statement after the log call's statement calls, as written in
// the code: "Store.Drop" for a method, "os.RemoveAll" for a function. It is a call site, not a
// proof that the call runs: an operand that panics or blocks first (paths[i], *p, a field through a
// nil pointer, <-ch) is a bug or a wait, not a decision against the call, and is not modelled. The
// fact is left out where the code itself decides that the call may not come next, or where another
// call comes first:
//   - the log call terminates (fatal, panic), is not a statement of its own (defer, go), or is the
//     last statement of its block;
//   - the next statement is not a call, an assignment or return of exactly one call, or an if
//     statement whose init is one (a call in a branch, a stored func literal, a defer or go, a
//     short-circuited operand runs maybe, later, or never);
//   - the call's receiver or arguments make calls of their own, which Go evaluates first
//     (DeletePath(MustConfirm()), MustStore().Drop());
//   - the callee does not resolve statically, or is a log call.
func nextCall(pass *analysis.Pass, level string, stack []ast.Node) (string, bool) {
	if level == facts.LevelFatal || level == facts.LevelPanic || len(stack) < 3 {
		return "", false
	}
	es, ok := stack[len(stack)-2].(*ast.ExprStmt)
	if !ok {
		return "", false
	}
	var list []ast.Stmt
	switch b := stack[len(stack)-3].(type) {
	case *ast.BlockStmt:
		list = b.List
	case *ast.CaseClause:
		list = b.Body
	case *ast.CommClause:
		list = b.Body
	default:
		return "", false
	}
	var next ast.Stmt
	for i, s := range list {
		if s == es && i+1 < len(list) {
			next = list[i+1]
		}
	}
	if is, ok := next.(*ast.IfStmt); ok {
		next = is.Init
	}
	call, ok := soleCall(next)
	if !ok || !plainCallee(call.Fun) {
		return "", false
	}
	for _, a := range call.Args {
		if makesCall(a) {
			return "", false
		}
	}
	if _, isLog := facts.AsLogCall(pass.TypesInfo, call); isLog {
		return "", false
	}
	fn := facts.Callee(pass.TypesInfo, call)
	if fn == nil {
		return "", false
	}
	name := funcName(fn)
	return name, facts.FactSafe(name)
}

// soleCall returns the call a statement is made of: f(), x = f(), x, err := f(), return f().
func soleCall(s ast.Stmt) (*ast.CallExpr, bool) {
	var e ast.Expr
	switch s := s.(type) {
	case *ast.ExprStmt:
		e = s.X
	case *ast.AssignStmt:
		if len(s.Rhs) != 1 {
			return nil, false
		}
		for _, l := range s.Lhs {
			if _, ok := ast.Unparen(l).(*ast.Ident); !ok {
				return nil, false // x.f = g() and m[k] = g() evaluate operands first
			}
		}
		e = s.Rhs[0]
	case *ast.ReturnStmt:
		if len(s.Results) != 1 {
			return nil, false
		}
		e = s.Results[0]
	default:
		return nil, false
	}
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	return call, ok
}

// plainCallee reports whether a call's function is a name or a selector whose operand makes no
// call: os.RemoveAll, s.store.Drop, c.pool[k].Evict, but not MustStore().Drop.
func plainCallee(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return !makesCall(x.X)
	}
	return false
}

// makesCall reports whether evaluating e calls a function: any call or conversion in it, a func
// literal included.
func makesCall(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.CallExpr, *ast.FuncLit:
			found = true
		}
		return !found
	})
	return found
}

// funcName is Type.Method for a method of a named type, pkg.Func for a package function, and the
// bare name otherwise.
func funcName(fn *types.Func) string {
	if recv := fn.Type().(*types.Signature).Recv(); recv != nil {
		t := recv.Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		if n, ok := t.(*types.Named); ok {
			return n.Obj().Name() + "." + fn.Name()
		}
		return fn.Name()
	}
	if fn.Pkg() != nil {
		return fn.Pkg().Name() + "." + fn.Name()
	}
	return fn.Name()
}

var wordRE = regexp.MustCompile(`[A-Za-z]{2,}`)

type rule struct{ threshold float64 }

// cleanOptions are the answers to "advice" that are not destructive advice.
var cleanOptions = []string{"safe_advice", "own_action", "no_advice"}

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{
		{
			ID:   "advice",
			Kind: sdk.Choice,
			Text: "Does `text` advise its reader (an operator or a user) to do something? `kind` and `level` say where the text appears; `next_call`, when present, is the function called by the statement that follows the log call in the code.",
			Options: []sdk.Option{
				{Key: "destructive_advice", Description: "It tells the reader to delete, wipe, reset or reinstall something that holds data or state."},
				{Key: "safe_advice", Description: "It tells the reader to do something non-destructive."},
				{Key: "own_action", Description: "It names an operation the program itself is doing, is about to do, has decided it must do, or failed to do, such as \"drop cache\", \"wiping the scratch disk\" or \"the stale volume has to be removed\"; it gives the reader no instruction."},
				{Key: "no_advice", Description: "It only reports or describes what happened; it gives the reader no instruction."},
			},
		},
		{
			ID:   "states_loss",
			Kind: sdk.Noul,
			Text: "Does `text` state what would be lost, or how to preserve it first?",
		},
	}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["advice"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the text does not let the classifier tell whether it advises anything")
	}
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if a.Choice != "destructive_advice" {
		// safe_advice, own_action and no_advice are one outcome for this rule, so their probabilities
		// add up: a text the classifier splits between "no advice" and "the program's own action" is
		// still confidently not destructive advice. A missing probability counts as 0.
		var clean float64
		for _, k := range cleanOptions {
			q, _ := a.Probability(k)
			clean += q
		}
		if clean < r.threshold {
			return sdk.Abstain(fmt.Sprintf("the non-destructive answers together at %.2f are below the threshold %.2f", clean, r.threshold))
		}
		return sdk.Clean()
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	y := answers["states_loss"].Yes
	switch {
	case y == nil:
		return sdk.Abstain("the classifier gave no probability for whether the loss is stated")
	case *y >= statesLossMin:
		return sdk.Clean()
	case *y > statesLossMax:
		return sdk.Abstain(fmt.Sprintf("states the loss at %.2f is neither ruled out nor established", *y))
	}
	return sdk.Report("advises a destructive step without saying what is lost: %q", c.Local["text"])
}
