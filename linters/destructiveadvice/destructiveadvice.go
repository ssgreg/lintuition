// Package destructiveadvice finds an error or log message that tells its reader to delete, wipe,
// reset or reinstall something that holds data, without saying what is lost or how to keep it:
//
//	return errors.New("index is corrupt; delete the data directory and restart")
//
// Code finds error constructors (errors.New, fmt.Errorf, pkg/errors) and log calls, resolved
// through go/types, with a constant text of at least one word, and asks about every one. There is
// no keyword prefilter: "format the data volume", "overwrite the database" and "run mkfs" advise
// destruction in words no list anticipates, and a missed text looks checked. The cost is a request
// per constant message; semantic.budget bounds it. The classifier reads only the text and is asked
// whether it advises the reader, and separately whether it says what would be lost or how to
// preserve it first.
package destructiveadvice

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
		if c := candidate(pass, n.(*ast.CallExpr)); c != nil {
			c.Subject = facts.EnclosingFunc(stack) + "/" + c.Subject
			out = append(out, c)
		}
		return true
	})
	return out, nil
}

func candidate(pass *analysis.Pass, call *ast.CallExpr) *sdk.Candidate {
	var kind, text string
	var known bool
	if ec, ok := facts.AsErrorCall(pass.TypesInfo, call); ok {
		kind, text, known = "error message", ec.Message, ec.MessageKnown
	} else if lc, ok := facts.AsLogCall(pass.TypesInfo, call); ok {
		kind, text, known = "log message", lc.Message, lc.MessageKnown
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
	return c
}

var wordRE = regexp.MustCompile(`[A-Za-z]{2,}`)

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{
		{
			ID:   "advice",
			Kind: sdk.Choice,
			Text: "Does `text` advise its reader (an operator or a user) to do something?",
			Options: []sdk.Option{
				{Key: "destructive_advice", Description: "It tells the reader to delete, wipe, reset or reinstall something that holds data or state."},
				{Key: "safe_advice", Description: "It tells the reader to do something non-destructive."},
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
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice != "destructive_advice" {
		return sdk.Clean()
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
