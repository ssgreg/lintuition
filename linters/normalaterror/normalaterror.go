// Package normalaterror finds an expected, routine event logged at error level, where it pages
// someone or hides the real errors among the noise:
//
//	if errors.Is(err, context.Canceled) {
//		log.Error("client went away") // the client closed the connection; nothing failed here
//	}
//
// Code finds log calls at error or fatal level, resolved through go/types, with a constant message.
// A message that names a failure itself (failed, error, cannot, unable, ...) is not asked about: it
// says an operation failed. The classifier reads only the message, never the surrounding code, and
// is asked what kind of event it reports.
package normalaterror

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
const Name = "normal-event-at-error"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of "routine" for a finding. The default 0.9 is the
	// prototype's: a recoverable degradation is a fair error, so the bar is high. It is not yet
	// validated on a labelled set.
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts error-level log calls whose message does not name a failure.
var Analyzer = &analysis.Analyzer{
	Name:       "normalaterror",
	Doc:        "extract error and fatal log calls whose message does not name a failure",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "an expected routine event (cache miss, retry scheduled, client went away) logged at error level",
		Standard:    true,
		Version:     "1",
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

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	var out []*sdk.Candidate
	ins.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		if c := candidate(pass, n.(*ast.CallExpr)); c != nil {
			c.Subject = facts.EnclosingFunc(stack) + "/" + c.Subject
			out = append(out, c)
		}
		return true
	})
	return out, nil
}

func candidate(pass *analysis.Pass, call *ast.CallExpr) *sdk.Candidate {
	lc, ok := facts.AsLogCall(pass.TypesInfo, call)
	if !ok || (lc.Level != facts.LevelError && lc.Level != facts.LevelFatal) {
		return nil
	}
	c := &sdk.Candidate{Pos: pass.Fset.Position(call.Pos()), Local: map[string]string{"level": lc.Level}}
	if !lc.MessageKnown {
		c.Subject = "message"
		c.Unsupported = "the log message is not a constant string"
		return c
	}
	if namesFailure(lc.Message) {
		return nil
	}
	c.Subject = lc.Message
	c.Local["message"] = lc.Message
	c.Payload.AddProse("message", lc.Message)
	return c
}

var wordRE = regexp.MustCompile(`[a-z]+(?:'[a-z]+)?`)

var failureWords = map[string]bool{
	"error": true, "errors": true, "err": true, "errored": true, "cannot": true, "can't": true, "cant": true,
	"couldn't": true, "unable": true, "invalid": true, "panic": true, "panicked": true, "crash": true,
	"crashed": true, "refused": true, "denied": true, "timeout": true, "timed": true, "abort": true,
	"aborted": true, "aborting": true, "unexpected": true, "broken": true, "corrupt": true, "corrupted": true,
	"fatal": true, "exception": true, "rejected": true, "lost": true, "won't": true, "wouldn't": true,
}

// namesFailure reports whether a message says, in its own words, that something failed: failed,
// error, cannot, unable, could not, timed out. Such a message is an error by its own account.
func namesFailure(message string) bool {
	ws := wordRE.FindAllString(strings.ToLower(message), -1)
	for i, w := range ws {
		if failureWords[w] || strings.HasPrefix(w, "fail") {
			return true
		}
		if (w == "could" || w == "can" || w == "did") && i+1 < len(ws) && ws[i+1] == "not" {
			return true
		}
	}
	return false
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "event",
		Kind: sdk.Choice,
		Text: "What does the event logged by `message` mean?",
		Options: []sdk.Option{
			{Key: "routine", Description: "An expected routine event (cache miss, retry scheduled, client went away)."},
			{Key: "degradation", Description: "A recoverable degradation."},
			{Key: "failure", Description: "An operation failed."},
		},
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["event"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the message does not say what kind of event it is")
	}
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice != "routine" {
		return sdk.Clean()
	}
	return sdk.Report("routine event logged at %s level: %q", c.Local["level"], c.Local["message"])
}
