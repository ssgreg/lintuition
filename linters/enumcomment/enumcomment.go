// Package enumcomment finds a comment in a block of constants that describes a neighbour rather
// than the constant it is attached to, as when a constant is inserted or removed and the comments
// shift by one:
//
//	const (
//		StateIdle State = iota
//		// StateRunning means the job finished and its result is stored.
//		StateRunning
//		StateDone
//	)
//
// Code finds the shape: a comment (above the constant, or at the end of its line) on a single
// constant of a parenthesised const block of at least two constants. The classifier reads the
// comment, with the constant's own name masked so the name cannot outvote the text, and picks which
// constant of the block it describes; the block's constant names are its options, with "none" for a
// section heading or a note. Go code reports when the pick is another constant.
package enumcomment

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "enum-comment-shift"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of the picked constant for a decision either way
	// (default 0.8). The prototype used 0.7. Neighbours in an enum often describe related states,
	// so a pick at 0.7 is often a near tie between two of them; 0.8 gives up a few catches for fewer
	// false findings. Not yet validated on a labelled set.
	Threshold *float64 `yaml:"threshold"`
}

// maxConstants bounds the options of one question: some backends label options with letters.
const maxConstants = 20

// none is the option for a comment that describes no single constant.
const none = "none"

// Analyzer extracts comments on single constants of parenthesised const blocks.
var Analyzer = &analysis.Analyzer{
	Name:       "enumcomment",
	Doc:        "extract comments on constants of parenthesised const blocks",
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a comment in a const block describes a neighbouring constant, not its own",
		Standard:    true,
		Version:     "1",
		Analyzer:    Analyzer,
		NewSettings: func() any { return &Settings{} },
		New: func(s any) (sdk.Rule, error) {
			t, err := sdk.Threshold(s.(*Settings).Threshold, 0.8)
			if err != nil {
				return nil, err
			}
			return &rule{threshold: t}, nil
		},
	})
}

func run(pass *analysis.Pass) (any, error) {
	var out []*sdk.Candidate
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			gd, ok := n.(*ast.GenDecl)
			if !ok {
				return true
			}
			if gd.Tok == token.CONST && gd.Lparen.IsValid() {
				out = append(out, block(pass, gd)...)
			}
			return false
		})
	}
	return out, nil
}

func block(pass *analysis.Pass, gd *ast.GenDecl) []*sdk.Candidate {
	var names []string
	for _, s := range gd.Specs {
		for _, id := range s.(*ast.ValueSpec).Names {
			if id.Name != "_" {
				names = append(names, id.Name)
			}
		}
	}
	if len(names) < 2 {
		return nil
	}
	var out []*sdk.Candidate
	for _, s := range gd.Specs {
		vs := s.(*ast.ValueSpec)
		for _, g := range []struct {
			cg   *ast.CommentGroup
			kind string
		}{{vs.Doc, "doc"}, {vs.Comment, "line"}} {
			if g.cg == nil {
				continue
			}
			text := strings.TrimSpace(g.cg.Text()) // directives such as //nolint are dropped
			if text == "" {
				continue
			}
			// The finding is on the constant the comment is attached to.
			c := &sdk.Candidate{Pos: pass.Fset.Position(vs.Names[0].Pos())}
			if len(vs.Names) != 1 {
				c.Subject = vs.Names[0].Name + "/" + g.kind
				c.Unsupported = "the comment is on a line of several constants"
				out = append(out, c)
				continue
			}
			own := vs.Names[0].Name
			c.Subject = own + "/" + g.kind
			switch {
			case own == "_":
				continue
			case len(names) > maxConstants:
				c.Unsupported = fmt.Sprintf("the block has more than %d constants to offer as options", maxConstants)
			case contains(names, none) || contains(names, sdk.Unclear):
				c.Unsupported = "a constant is named like an answer option"
			}
			if c.Unsupported != "" {
				out = append(out, c)
				continue
			}
			c.Local = map[string]string{"name": own, "comment": text, "constants": strings.Join(names, " ")}
			c.Payload.AddProse("comment", sdk.Mask(text, own, "this constant"))
			out = append(out, c)
		}
	}
	return out
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

type rule struct{ threshold float64 }

func (r *rule) Questions(c *sdk.Candidate) []sdk.Question {
	var opts []sdk.Option
	for _, n := range strings.Fields(c.Local["constants"]) {
		opts = append(opts, sdk.Option{Key: n, Description: "The constant " + n + "."})
	}
	opts = append(opts, sdk.Option{Key: none, Description: "A section heading or a note that describes no single constant."})
	return []sdk.Question{{
		ID:      "describes",
		Kind:    sdk.Choice,
		Text:    "Which constant of the block does the comment `comment` describe? The block's constants are listed as options.",
		Options: opts,
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["describes"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the comment does not let the classifier tell which constant it describes")
	}
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	// Weak support abstains whichever way the answer goes.
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice == none || a.Choice == c.Local["name"] {
		return sdk.Clean()
	}
	return sdk.Report("comment describes %s, not %s: %q", a.Choice, c.Local["name"], clip(c.Local["comment"], 80))
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
