// Package normalaterror finds an expected, routine event logged at error level, where it pages
// someone or hides the real errors among the noise:
//
//	if errors.Is(err, context.Canceled) {
//		log.Error("client went away") // the client closed the connection; nothing failed here
//	}
//
// Code finds log calls at error or fatal level, resolved through go/types, with a constant message.
// A message that names a failure itself (failed, error, cannot, unable, ...) is not asked about: it
// says an operation failed. The classifier reads the message and, from go/types, whether the call
// logs a value of type error and what the code checked about that error where it logs (not nil,
// errors.Is(err, context.Canceled), err == io.EOF, os.IsNotExist). In a structured log the message
// often only names the operation and the error says that it failed:
//
//	if err := ln.Close(); err != nil {
//		logger.Error("closing the listener", zap.Error(err)) // a failure, though the words are routine
//	}
//
// The classifier never sees the surrounding code, and is asked what kind of event the call reports.
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
	errs := attachedErrors(pass.TypesInfo, lc)
	if len(errs) == 0 {
		return c
	}
	if len(errs) == 1 {
		if name := sentinel(pass.TypesInfo, errs[0]); name != "" && facts.FactSafe(name) {
			// slog.Error("...", "err", context.Canceled): the logged value is that variable's, read
			// at the call.
			c.Payload.Fact("error", name)
			c.Local["error"] = errorIdentified
			return c
		}
	}
	c.Payload.Fact("error", "attached")
	c.Local["error"] = errorUnproven
	if v := checkedVar(pass.TypesInfo, errs, outermostBody(stack)); v != nil {
		cs := errorChecks(pass.TypesInfo, v, stack)
		if len(cs) > maxChecks {
			cs = cs[:maxChecks]
		}
		if len(cs) > 0 {
			c.Payload.Fact("error_check", cs)
		}
		c.Local["error"] = errorState(cs)
	}
	return c
}

// maxChecks bounds the checks sent, nearest first: a long chain of !errors.Is exclusions says no
// more after a few.
const maxChecks = 6

// What the code knows about the error a log call carries, in Local["error"]:
//   - identified: it checked which error it is (errors.Is(err, context.Canceled), err == io.EOF,
//     os.IsNotExist(err)), or it logs a package-level error by name;
//   - nil: it checked that the error is nil;
//   - failed: it checked that the error is not nil, and not which one it is;
//   - unproven: neither, including a check that only says what the error is not.
const (
	errorIdentified = "identified"
	errorNil        = "nil"
	errorFailed     = "failed"
	errorUnproven   = "unproven"
)

// errorState reads the state from the checks.
func errorState(checks []string) string {
	state := errorUnproven
	for _, c := range checks {
		switch {
		case c == "nil":
			return errorNil
		case (strings.HasPrefix(c, "is ") && !strings.HasPrefix(c, "is not ")) || strings.HasPrefix(c, "os.Is"):
			state = errorIdentified
		case c == "not nil" && state == errorUnproven:
			state = errorFailed
		}
	}
	return state
}

// outermostBody returns the body of the outermost function on the stack, nil at package level.
func outermostBody(stack []ast.Node) ast.Node {
	for _, n := range stack {
		switch n := n.(type) {
		case *ast.FuncDecl:
			return n.Body
		case *ast.FuncLit:
			return n.Body
		}
	}
	return nil
}

var wordRE = regexp.MustCompile(`[a-z]+(?:'[a-z]+)?`)

var failureWords = map[string]bool{
	"fail": true, "fails": true, "failed": true, "failing": true, "failure": true, "failures": true,
	"error": true, "errors": true, "err": true, "errored": true, "cannot": true, "can't": true, "cant": true,
	"couldn't": true, "unable": true, "invalid": true, "panic": true, "panicked": true, "crash": true,
	"crashed": true, "refused": true, "denied": true, "timeout": true, "timed": true, "abort": true,
	"aborted": true, "aborting": true, "unexpected": true, "broken": true, "corrupt": true, "corrupted": true,
	"fatal": true, "exception": true, "rejected": true, "lost": true, "won't": true, "wouldn't": true,
}

// namesFailure reports whether a message says, in its own words, that something failed: failed,
// error, cannot, unable, could not, timed out. Such a message is an error by its own account. Words
// are matched whole: a failover or a failback is an operation, not a failure. A negated failure
// ("finished without errors") is still read as one, a known limit: such a message is not asked.
func namesFailure(message string) bool {
	ws := wordRE.FindAllString(strings.ToLower(message), -1)
	for i, w := range ws {
		if failureWords[w] {
			return true
		}
		if (w == "could" || w == "can" || w == "did") && i+1 < len(ws) && ws[i+1] == "not" {
			return true
		}
	}
	return false
}

type rule struct{ threshold float64 }

// operationOnly is the probability of "action" on the operation question from which a message is
// read as only naming an operation. At or above it, an error the code checked is set and did not
// single out is taken as the news of the log line, and the candidate is clean whatever the event
// question says; with an error not shown to be set it abstains. Clean is not free: a routine event
// worded as an action is missed, and the bar is set below the report threshold knowing that.
const operationOnly = 0.7

var eventQuestion = sdk.Question{
	ID:   "event",
	Kind: sdk.Choice,
	Text: "What does the event logged by `message` mean? `error`, when present, says that the log call carries a value of type error along with the message: \"attached\", or the name of the package-level error it logs, such as \"context.Canceled\"; `error_check`, when present, lists, nearest first, the checks the code made on that error on the way to the log call: that it is nil or not nil, or that it is or is not a particular error (such as \"is context.Canceled\", \"is not net.ErrClosed\" or \"os.IsNotExist\").",
	Options: []sdk.Option{
		{Key: "routine", Description: "An expected routine event the program handles as designed: a cache miss, a retry scheduled, a client that went away, or an operation that stopped for an expected reason the code checked for, such as a cancellation, a closed connection or the end of input (context.Canceled, net.ErrClosed, io.EOF)."},
		{Key: "degradation", Description: "A recoverable degradation."},
		{Key: "failure", Description: "An operation failed. A message that names an operation (\"closing the listener\", \"applying the config\") and carries an error that is not one the code checked for as expected reports that this operation failed."},
	},
}

var operationQuestion = sdk.Question{
	ID:   "operation",
	Kind: sdk.Choice,
	Text: "What does the log message `message` say, read on its own?",
	Options: []sdk.Option{
		{Key: "action", Description: "Only the name of an action or step the program was performing, such as \"closing the listener\", \"config reload\" or \"renewing certificates\", with nothing about how it went."},
		{Key: "outcome", Description: "What happened or what state things are in, such as \"cache miss\", \"retry scheduled\", \"client went away\" or \"listener closed for shutdown\"."},
	},
}

// Questions asks what the event means; for a call that carries an error the code did not identify
// and did not find nil,
// it also asks whether the message only names an action or says what happened. The second
// question reads the message alone.
func (r *rule) Questions(c *sdk.Candidate) []sdk.Question {
	if e := c.Local["error"]; e == errorFailed || e == errorUnproven {
		return []sdk.Question{eventQuestion, operationQuestion}
	}
	return []sdk.Question{eventQuestion}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	if p, ok := answers["operation"].Probability("action"); ok && p >= operationOnly {
		switch c.Local["error"] {
		case errorFailed:
			// "closing the listener" with an error the code checked is not nil and did not single
			// out: the message says what was being done and the error that it did not work. This
			// is how a structured log usually reports a failure, so it is taken as one, whatever
			// the message alone reads like. It is a trade-off: a routine event worded as an action
			// with an unexpected error attached is missed.
			return sdk.Clean()
		case errorUnproven:
			// The same, but nothing shows the error is not nil: the line may report no failure at
			// all, and the message alone is not enough to call it routine.
			return sdk.Abstain("the message names an action and carries an error the code did not show to be set")
		}
	}
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
