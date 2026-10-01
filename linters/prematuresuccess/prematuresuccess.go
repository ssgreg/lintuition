// Package prematuresuccess finds a log line that reports an operation as done before the call that
// can still fail:
//
//	logger.Info("config saved to disk")
//	if err := store.SaveConfig(ctx, cfg); err != nil {
//		return err // the log above already said it worked
//	}
//
// Code finds the shape: a log call followed, in the same block, by a statement that keeps an error
// result (decided by the result's type, not the variable's name), whose callee shares a word with
// the message. The classifier is asked only whether the message claims that call already completed.
// When the function calls, before the log, a function the message also matches, the log may report
// that one; such a candidate is unsupported.
package prematuresuccess

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "premature-success"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of "completed" for a finding. The default 0.6 is from the
	// prototype: the one real catch scored 0.61-0.72 over six runs with one backend, so 0.8 would lose
	// it. It is not yet validated on a labelled set.
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts log calls directly followed by a fallible call they may describe.
var Analyzer = &analysis.Analyzer{
	Name:       "prematuresuccess",
	Doc:        "extract log calls followed by a call whose error is checked",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a log line reports success before the call that can still fail",
		Standard:    true,
		Version:     "1",
		Analyzer:    Analyzer,
		NewSettings: func() any { return &Settings{} },
		New: func(s any) (sdk.Rule, error) {
			t, err := sdk.Threshold(s.(*Settings).Threshold, 0.6)
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
	// Each function body, a closure's included, is scanned on its own: its calls, then its blocks.
	ins.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.FuncLit)(nil)}, func(n ast.Node) {
		var body *ast.BlockStmt
		switch n := n.(type) {
		case *ast.FuncDecl:
			body = n.Body
		case *ast.FuncLit:
			body = n.Body
		}
		if body == nil {
			return
		}
		calls := ownCalls(body)
		walkOwn(body, func(list []ast.Stmt) {
			for i := 0; i+1 < len(list); i++ {
				if c := candidate(pass, calls, list[i], list[i+1]); c != nil {
					out = append(out, c)
				}
			}
		})
	})
	return out, nil
}

// ownCalls returns the calls a function body makes: its own, and those of closures it invokes on
// the spot (func(){...}(), go func(){...}(), defer func(){...}()), whose work may be what a later
// log reports. Closures only stored or passed on are not looked into.
func ownCalls(body *ast.BlockStmt) []*ast.CallExpr {
	var out []*ast.CallExpr
	invoked := map[*ast.FuncLit]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return invoked[n]
		case *ast.CallExpr:
			if fl, ok := ast.Unparen(n.Fun).(*ast.FuncLit); ok {
				invoked[fl] = true
			}
			out = append(out, n)
		}
		return true
	})
	return out
}

// walkOwn calls fn with every statement list of a function body, not of closures defined in it.
func walkOwn(body *ast.BlockStmt, fn func([]ast.Stmt)) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BlockStmt:
			fn(n.List)
		case *ast.CaseClause:
			fn(n.Body)
		case *ast.CommClause:
			fn(n.Body)
		}
		return true
	})
}

func candidate(pass *analysis.Pass, calls []*ast.CallExpr, s, next ast.Stmt) *sdk.Candidate {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := ast.Unparen(es.X).(*ast.CallExpr)
	if !ok {
		return nil
	}
	lc, ok := facts.AsLogCall(pass.TypesInfo, call)
	if !ok {
		return nil
	}
	switch lc.Level {
	case facts.LevelError, facts.LevelFatal, facts.LevelPanic:
		return nil // not a success report
	}
	fb, ok := facts.FallibleStmt(pass.TypesInfo, next)
	if !ok {
		return nil
	}
	c := &sdk.Candidate{
		Pos:     pass.Fset.Position(call.Pos()),
		Subject: fb.Callee.Name(),
		Local:   map[string]string{"next_call": fb.Callee.Name()},
	}
	if !lc.MessageKnown {
		c.Unsupported = "the log message is not a constant string"
		return c
	}
	if !sharesWord(lc.Message, fb.Callee.Name()) {
		// The message is about something else; asking would only invite a guess.
		return nil
	}
	// A log line after an earlier call it also matches may report that call, done, before moving
	// on (reset a, check, log, reset b). Which one it means is not established; do not guess.
	if earlierMatch(pass.TypesInfo, calls, call, lc.Message) {
		c.Unsupported = "the message may report an earlier call"
		return c
	}
	c.Local["message"] = lc.Message
	c.Payload.AddProse("message", lc.Message)
	c.Payload.Fact("next_call", fb.Callee.Name())
	return c
}

// earlierMatch reports whether the function calls, before the log, a function the message also
// names, in this block or an enclosing one.
func earlierMatch(info *types.Info, calls []*ast.CallExpr, log *ast.CallExpr, message string) bool {
	for _, c := range calls {
		if c.Pos() >= log.Pos() {
			continue
		}
		fn := facts.Callee(info, c)
		if fn == nil || !sharesWord(message, fn.Name()) {
			continue
		}
		if _, isLog := facts.AsLogCall(info, c); !isLog {
			return true
		}
	}
	return false
}

var stop = map[string]bool{"have": true, "been": true, "with": true, "from": true, "into": true, "that": true, "this": true}

var wordRE = regexp.MustCompile(`[A-Za-z]{4,}`)

// sharesWord reports whether the message mentions a word of the callee's name: "config saved to
// disk" and SaveConfig share "config". Words are compared after dropping a plural or verb ending.
func sharesWord(message, callee string) bool {
	cw := map[string]bool{}
	for _, w := range facts.Words(callee) {
		if len(w) >= 4 && !stop[w] {
			cw[stem(w)] = true
		}
	}
	for _, w := range wordRE.FindAllString(message, -1) {
		if cw[stem(strings.ToLower(w))] {
			return true
		}
	}
	return false
}

func stem(w string) string {
	for _, suf := range []string{"ing", "ed", "es", "s"} {
		if strings.HasSuffix(w, suf) && len(w)-len(suf) >= 4 {
			return strings.TrimSuffix(w, suf)
		}
	}
	return w
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "claim",
		Kind: sdk.Choice,
		Text: "Does the log message `message` claim that the operation `next_call` has already completed?",
		Options: []sdk.Option{
			{Key: "completed", Description: "It says the operation completed or succeeded."},
			{Key: "starting", Description: "It says the operation is starting or about to happen."},
			{Key: "progress", Description: "Progress or context only, unrelated to that operation's outcome."},
		},
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["claim"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the message does not say whether the operation is done")
	}
	// Weak support abstains whichever way the answer goes.
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice != "completed" {
		return sdk.Clean()
	}
	return sdk.Report("success logged before %s has returned: %q", c.Local["next_call"], c.Local["message"])
}
