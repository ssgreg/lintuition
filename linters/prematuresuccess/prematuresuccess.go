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
// When the statement before the log is a fallible call the message also matches, the log may
// report that one; such a candidate is unsupported.
package prematuresuccess

import (
	"fmt"
	"go/ast"
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
	ins.Preorder([]ast.Node{(*ast.BlockStmt)(nil), (*ast.CaseClause)(nil), (*ast.CommClause)(nil)}, func(n ast.Node) {
		var list []ast.Stmt
		switch n := n.(type) {
		case *ast.BlockStmt:
			list = n.List
		case *ast.CaseClause:
			list = n.Body
		case *ast.CommClause:
			list = n.Body
		}
		for i := 0; i+1 < len(list); i++ {
			var prev ast.Stmt
			if i > 0 {
				prev = list[i-1]
			}
			if c := candidate(pass, prev, list[i], list[i+1]); c != nil {
				out = append(out, c)
			}
		}
	})
	return out, nil
}

func candidate(pass *analysis.Pass, prev, s, next ast.Stmt) *sdk.Candidate {
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
	// A log line right after a fallible call it also matches may report that call, done, before
	// moving on (reset a, log, reset b). Which one it means is not in the code; do not guess.
	if prev != nil {
		if pf, ok := facts.FallibleStmt(pass.TypesInfo, prev); ok && sharesWord(lc.Message, pf.Callee.Name()) {
			c.Unsupported = "the message may report the call before it"
			return c
		}
	}
	c.Local["message"] = lc.Message
	c.Payload.AddProse("message", lc.Message)
	c.Payload.Fact("next_call", fb.Callee.Name())
	return c
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
