// Package severeunderstated finds an unintended loss logged at debug or info level, where nobody
// looks:
//
//	slog.Info("queue full, dropping events") // the events are gone for good
//
// Code finds log calls at debug or info level, resolved through go/types, with a constant message of
// at least two words. The classifier reads the message, its level, and the conditions the code
// checked on the way to the log, read from the enclosing branches through go/types: an error it
// tested (errors.Is(err, fs.ErrNotExist), os.IsNotExist, err == context.Canceled) while that error
// is still the one at hand, a receive from a context's Done channel or from a channel of
// os.Signal. The message alone often hides this: "notify failed" at debug reads as a loss until you
// see that it sits in the branch where the context was cancelled. The facts say what the code
// checked, not why; whether the missing file was optional or the cancel was asked for is left to
// the classifier.
//
// It is asked two things: what consequence the message states, with an answer for each kind of
// expected event (an anticipated absence, a requested stop, the program's own recovery), and
// whether what went away was meant to go. An unintended loss that was not meant is a finding.
package severeunderstated

import (
	"fmt"
	"go/ast"
	"regexp"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/internal/classify"
	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "severe-event-understated"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of "unintended_loss" for a finding. The default 0.85 is
	// the prototype's; it is not yet validated on a labelled set.
	Threshold *float64 `yaml:"threshold"`
}

// onPurposeMax is the most probability of "on purpose" a finding allows, and onPurposeMin the
// least that makes the loss intended (clean); in between the rule abstains. 0.3 is the prototype's.
const (
	onPurposeMax = 0.3
	onPurposeMin = 0.7
)

// Analyzer extracts debug and info log calls with a constant message of two words or more.
var Analyzer = &analysis.Analyzer{
	Name:       "severeunderstated",
	Doc:        "extract debug and info log calls with a constant message",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "an unintended loss of data or work, or an outage, logged at debug or info level",
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
		if c := candidate(pass, n.(*ast.CallExpr), stack); c != nil {
			c.Subject = facts.EnclosingFunc(stack) + "/" + c.Subject
			out = append(out, c)
		}
		return true
	})
	return out, nil
}

var wordRE = regexp.MustCompile(`[A-Za-z]+`)

func candidate(pass *analysis.Pass, call *ast.CallExpr, stack []ast.Node) *sdk.Candidate {
	lc, ok := facts.AsLogCall(pass.TypesInfo, call)
	if !ok || (lc.Level != facts.LevelDebug && lc.Level != facts.LevelInfo) {
		return nil
	}
	c := &sdk.Candidate{Pos: pass.Fset.Position(call.Pos()), Local: map[string]string{"level": lc.Level}}
	if !lc.MessageKnown {
		c.Subject = "message"
		c.Unsupported = "the log message is not a constant string"
		return c
	}
	if len(wordRE.FindAllString(lc.Message, -1)) < 2 {
		return nil // "dropped" alone says too little to judge
	}
	c.Subject = lc.Message
	c.Local["message"] = lc.Message
	c.Payload.AddProse("message", lc.Message)
	c.Payload.Fact("level", lc.Level)
	if b := branchFacts(pass.Pkg, pass.TypesInfo, stack, call); len(b) > 0 {
		c.Payload.Fact("branch", b)
	}
	return c
}

type rule struct{ threshold float64 }

// cleanOptions are the answers to "consequence" that are not an unintended loss.
var cleanOptions = []string{"routine", "expected_absence", "requested_stop", "recovery", "inconvenience"}

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{
		{
			ID:   "consequence",
			Kind: sdk.Choice,
			Text: "What consequence does the log message `message` state? `level` is its log level; `branch`, when present, lists conditions the code checked on the way to logging `message`, nearest first: an error test that held (such as \"error is fs.ErrNotExist\", \"error is context.Canceled\" or \"os.IsNotExist\"), \"context done\", \"receive from a channel of os.Signal\", or \"callback passed along with a signal value\". They say what was checked, not why.",
			Options: []sdk.Option{
				{Key: "routine", Description: "Routine progress or a state change the program handles as designed, including a deletion or cleanup that was requested, and an entry that expired or timed out and is removed."},
				{Key: "expected_absence", Description: "Something optional or anticipated is missing or not found, such as no saved state on the first start, and the program carries on without it."},
				{Key: "requested_stop", Description: "A stop, shutdown or cancellation that someone asked for (an operator, a signal, a caller that cancelled), and the work that stop is meant to end. Giving up after a failure is not a requested stop."},
				{Key: "recovery", Description: "The program deals with a problem itself: it retries, resumes, falls back, or sets the work aside and reports it."},
				{Key: "inconvenience", Description: "A temporary inconvenience."},
				{Key: "unintended_loss", Description: "Data or work was lost and will not come back, or an operation is now unavailable, and nobody asked for it."},
			},
		},
		{
			ID:   "on_purpose",
			Kind: sdk.Noul,
			Text: "Does `message` say that what went away was meant to go: deleted, dropped or skipped as requested or configured, for example by a retention or sampling setting? A drop forced by a full queue, a failure or a limit nobody chose is not meant.",
		},
	}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["consequence"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the message does not say what the consequence is")
	}
	if a.Choice != "unintended_loss" {
		// Every answer but unintended_loss is one outcome for this rule, so their probabilities add
		// up: a message the classifier splits between "routine" and "recovery" is still confidently
		// not a loss. A missing probability counts as 0. The shared answer check has already
		// rejected a distribution whose mass rounding cannot explain and normalized the rest, so
		// the sum is at most 1; it is taken in a fixed order with compensation, so a cached answer
		// always decides the same way, and a sum within MassNoise of the threshold is at it.
		clean := classify.OptionMass(a.Probabilities, cleanOptions)
		if clean+classify.MassNoise < r.threshold {
			return sdk.Abstain(fmt.Sprintf("the answers other than a loss together at %.2f are below the threshold %.2f", clean, r.threshold))
		}
		return sdk.Clean()
	}
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	y := answers["on_purpose"].Yes
	switch {
	case y == nil:
		return sdk.Abstain("the classifier gave no probability for on purpose")
	case *y >= onPurposeMin:
		return sdk.Clean()
	case *y > onPurposeMax:
		return sdk.Abstain(fmt.Sprintf("on purpose at %.2f is neither ruled out nor established", *y))
	}
	return sdk.Report("unintended loss logged at %s level: %q", c.Local["level"], c.Local["message"])
}
