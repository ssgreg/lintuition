// Package severeunderstated finds an unintended loss logged at debug or info level, where nobody
// looks:
//
//	slog.Info("queue full, dropping events") // the events are gone for good
//
// Code finds log calls at debug or info level, resolved through go/types, with a constant message of
// at least two words. The classifier reads only the message and is asked two things: what
// consequence it states, and whether it describes something the program did on purpose (a
// requested deletion is routine). An unintended loss that is not on purpose is a finding.
package severeunderstated

import (
	"fmt"
	"go/ast"
	"regexp"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

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

var wordRE = regexp.MustCompile(`[A-Za-z]+`)

func candidate(pass *analysis.Pass, call *ast.CallExpr) *sdk.Candidate {
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
	return c
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{
		{
			ID:   "consequence",
			Kind: sdk.Choice,
			Text: "What consequence does the message `message` state?",
			Options: []sdk.Option{
				{Key: "routine", Description: "Routine progress, including a deletion or cleanup that was requested."},
				{Key: "inconvenience", Description: "A temporary inconvenience."},
				{Key: "unintended_loss", Description: "Data or work was lost unintentionally, or an operation is now unavailable."},
			},
		},
		{
			ID:   "on_purpose",
			Kind: sdk.Noul,
			Text: "Does `message` describe something the program did on purpose (removed, dropped, deleted or skipped as intended)?",
		},
	}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["consequence"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the message does not say what the consequence is")
	}
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice != "unintended_loss" {
		return sdk.Clean()
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
