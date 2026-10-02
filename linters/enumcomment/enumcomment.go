// Package enumcomment finds a comment in a block of constants that describes a neighbour rather
// than the constant it is attached to, as when a constant is inserted or removed and the comments
// shift by one:
//
//	const (
//		StateIdle State = iota
//		// the job finished and its result is stored
//		StateRunning
//		StateDone
//	)
//
// Code finds the shape: a comment (above the constant, or at the end of its line) on a single
// constant of a parenthesised const block of at least two constants. A comment that names its own
// constant is taken at its word and is not a candidate. A comment that differs from another comment
// of the block in one word at most is unsupported: what tells such comments apart (a number, a
// type name) is matched to a constant by the block's convention, which the classifier does not see.
// The classifier reads the remaining comments as written and picks which constant of the block each
// describes; the block's constant names are its options, with "none" for a section heading or a
// note. Go code reports when the pick is another constant.
package enumcomment

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "enum-comment-shift"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of the picked option for a decision either way
	// (default 0.8). Neighbours in an enum often describe related states, so a pick at 0.7 is often
	// a near tie between two of them.
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
		Version:     "2",
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

// comment is one comment group attached to a spec of the block.
type comment struct {
	spec *ast.ValueSpec
	kind string // doc or line
	text string
}

func block(pass *analysis.Pass, gd *ast.GenDecl) []*sdk.Candidate {
	var names []string
	var comments []comment
	for _, s := range gd.Specs {
		vs := s.(*ast.ValueSpec)
		for _, id := range vs.Names {
			if id.Name != "_" {
				names = append(names, id.Name)
			}
		}
		for _, g := range []struct {
			cg   *ast.CommentGroup
			kind string
		}{{vs.Doc, "doc"}, {vs.Comment, "line"}} {
			if g.cg == nil {
				continue
			}
			// Directives such as //nolint are dropped by Text.
			if text := strings.TrimSpace(g.cg.Text()); text != "" {
				comments = append(comments, comment{vs, g.kind, text})
			}
		}
	}
	if len(names) < 2 {
		return nil
	}
	words := make([][]string, len(comments))
	for i, cm := range comments {
		words[i] = template(cm.text, names)
	}
	var out []*sdk.Candidate
	for i, cm := range comments {
		vs := cm.spec
		// The finding is on the constant the comment is attached to.
		c := &sdk.Candidate{Pos: pass.Fset.Position(vs.Names[0].Pos()), Subject: vs.Names[0].Name + "/" + cm.kind}
		if len(vs.Names) != 1 {
			c.Unsupported = "the comment is on a line of several constants"
			out = append(out, c)
			continue
		}
		own := vs.Names[0].Name
		if own == "_" || namesWord(cm.text, own) {
			continue
		}
		switch {
		case len(names) > maxConstants:
			c.Unsupported = fmt.Sprintf("the block has more than %d constants to offer as options", maxConstants)
		case contains(names, none) || contains(names, sdk.Unclear):
			c.Unsupported = "a constant is named like an answer option"
		case nearDuplicate(words, i):
			c.Unsupported = "the comment differs from another comment of the block in one word at most"
		}
		if c.Unsupported != "" {
			out = append(out, c)
			continue
		}
		c.Local = map[string]string{"name": own, "comment": cm.text, "constants": strings.Join(names, " ")}
		c.Payload.AddProse("comment", cm.text)
		out = append(out, c)
	}
	return out
}

// namesWord reports whether text has name as a whole word. A one-letter name is never matched: it
// would match the article "A".
func namesWord(text, name string) bool {
	if len(name) < 2 {
		return false
	}
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(text)
}

// constWord stands for any constant name of the block in a template.
const constWord = "\x00"

// template returns the comment's words, lower-cased, without surrounding punctuation, and with
// every constant name of the block replaced by one placeholder.
func template(text string, names []string) []string {
	m := map[string]string{}
	for _, n := range names {
		m[n] = constWord
	}
	var out []string
	for _, w := range strings.Fields(sdk.MaskAll(text, m)) {
		if w = strings.Trim(strings.ToLower(w), ".,;:!?()[]{}\"'`"); w != "" {
			out = append(out, w)
		}
	}
	return out
}

// nearDuplicate reports whether comment i has the same words as another comment of the block, or,
// in comments of three words or more, all but one at the same positions.
func nearDuplicate(words [][]string, i int) bool {
	a := words[i]
	for j, b := range words {
		if j == i || len(a) != len(b) || len(a) == 0 {
			continue
		}
		diff := 0
		for k := range a {
			if a[k] != b[k] {
				diff++
			}
		}
		if diff == 0 || (diff == 1 && len(a) >= 3) {
			return true
		}
	}
	return false
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
	// The pick must stand clear of every other option. In a distribution that adds up to 1 this
	// follows from the threshold; an answer that also gives another option more than 1-threshold
	// contradicts itself.
	for k, q := range a.Probabilities {
		if k != a.Choice && q > 1-r.threshold+1e-9 {
			return sdk.Abstain(fmt.Sprintf("%s at %.2f also gives %s %.2f", a.Choice, p, k, q))
		}
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
