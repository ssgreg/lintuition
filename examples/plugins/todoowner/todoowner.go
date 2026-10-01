// Package todoowner is an example plugin linter: a TODO comment that names nobody, no ticket and no
// date is a wish, not a plan. Code finds the TODO comments; the classifier is asked only whether the
// text says who or when; the decision is code.
package todoowner

import (
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ssgreg/lintuition/sdk"
)

// Analyzer extracts TODO comments.
var Analyzer = &analysis.Analyzer{
	Name:       "todoowner",
	Doc:        "extract TODO comments",
	ResultType: sdk.CandidatesType,
	Run: func(pass *analysis.Pass) (any, error) {
		var out []*sdk.Candidate
		for _, f := range pass.Files {
			for _, cg := range f.Comments {
				for _, c := range cg.List {
					text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/*"))
					if !strings.HasPrefix(text, "TODO") {
						continue
					}
					cand := &sdk.Candidate{Pos: pass.Fset.Position(c.Pos()), Subject: text}
					cand.Payload.AddProse("comment", text)
					out = append(out, cand)
				}
			}
		}
		return out, nil
	},
}

type rule struct{}

func (rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{ID: "owner", Kind: sdk.Noul,
		Text: "Does the comment `comment` say who will do it, give a ticket, or give a date?"}}
}

func (rule) Decide(_ *sdk.Candidate, a map[string]sdk.Answer) sdk.Decision {
	switch y := *a["owner"].Yes; {
	case y < 0.2:
		return sdk.Report("TODO names no owner, ticket or date")
	case y > 0.8:
		return sdk.Clean()
	default:
		return sdk.Abstain("the comment may or may not name an owner")
	}
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:     "todo-owner",
		Doc:      "a TODO comment without an owner, ticket or date (example plugin)",
		Version:  "1",
		Analyzer: Analyzer,
		New:      func(any) (sdk.Rule, error) { return rule{}, nil },
	})
}
