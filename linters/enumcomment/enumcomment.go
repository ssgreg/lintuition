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
// constant of a parenthesised const block of at least two constants, and reads how it opens. A
// comment that opens with its own constant's name ("StateIdle is ...") is sent with that opening
// replaced by "this constant", so the name cannot outvote the description; any other comment is sent
// as written, so a neighbour's name it opens with, or its own name further on ("If trace is set"),
// stays visible. A comment that opens with several constants' names describes a group and is
// unsupported, and so is one that matches another constant's comment word for word, or but for one
// word that does not single out a constant by name: what tells such comments apart (a number, a type
// name) is matched to a constant by the block's convention, which the classifier does not see. The
// classifier picks which constant of the block the comment describes; the block's constant names are
// its options, with "none" for a section heading or a note. Go code reports when the pick is another
// constant.
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
		Version:     "3",
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

// verdict is what block decided about one comment.
type verdict struct {
	skip        bool
	unsupported string
	prose       string // what is sent
	own         string
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
	vs := judge(comments, names)
	var out []*sdk.Candidate
	for i, cm := range comments {
		v := vs[i]
		if v.skip {
			continue
		}
		// The finding is on the constant the comment is attached to.
		c := &sdk.Candidate{Pos: pass.Fset.Position(cm.spec.Names[0].Pos()), Subject: cm.spec.Names[0].Name + "/" + cm.kind}
		if v.unsupported != "" {
			c.Unsupported = v.unsupported
		} else {
			c.Local = map[string]string{"name": v.own, "comment": cm.text, "constants": strings.Join(names, " ")}
			c.Payload.AddProse("comment", v.prose)
		}
		out = append(out, c)
	}
	return out
}

// judge decides, for every comment of a block, whether it is skipped, unsupported or asked, and what
// is sent. Block-wide reasons come first, so a block that cannot be asked about costs no word work.
func judge(comments []comment, names []string) []verdict {
	out := make([]verdict, len(comments))
	var blockReason string
	switch {
	case len(names) > maxConstants:
		blockReason = fmt.Sprintf("the block has more than %d constants to offer as options", maxConstants)
	case contains(names, none) || contains(names, sdk.Unclear):
		blockReason = "a constant is named like an answer option"
	}
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	// compared holds the comments that take part in the template check, with their words.
	type entry struct {
		i     int
		own   string
		words []string
	}
	var compared []entry
	for i, cm := range comments {
		v := &out[i]
		if len(cm.spec.Names) != 1 {
			v.unsupported = "the comment is on a line of several constants"
			continue
		}
		own := cm.spec.Names[0].Name
		v.own = own
		if own == "_" {
			v.skip = true
			continue
		}
		if blockReason != "" {
			v.unsupported = blockReason
			continue
		}
		toks := tokenize(cm.text)
		subj, group := subject(toks, set)
		switch {
		case group:
			v.unsupported = "the comment opens with several constants' names"
			continue
		case subj != "" && subj != own:
			// It says outright which constant it is about: asked as written, whatever it resembles.
			v.prose = cm.text
			continue
		case subj == own:
			v.prose = "this constant" + cm.text[toks[0].end:]
		default:
			v.prose = cm.text
		}
		w := template(toks, set)
		// A doc and a line comment that say the same on one constant are one description.
		dup := false
		for _, e := range compared {
			if e.own == own && differing(e.words, w) == -1 {
				dup = true
				break
			}
		}
		if dup {
			v.skip = true
			continue
		}
		compared = append(compared, entry{i, own, w})
	}
	if len(compared) < 2 {
		return out
	}
	parts := nameParts(names)
	for a := range compared {
		for b := a + 1; b < len(compared); b++ {
			x, y := compared[a], compared[b]
			if x.own == y.own {
				continue
			}
			d := differing(x.words, y.words)
			switch {
			case d == -2:
				continue
			case d >= 0 && len(x.words) < 3:
				continue
			case d >= 0:
				// "permits reads" against "permits writes" on AccessRead and AccessWrite: the
				// differing words name different constants, so the names carry the distinction.
				cx, cy := singles(x.words[d], parts), singles(y.words[d], parts)
				if cx != "" && cy != "" && cx != cy {
					continue
				}
			}
			for _, e := range []entry{x, y} {
				out[e.i].unsupported = "the comment matches another constant's comment but for one word that names no constant"
				if d == -1 {
					out[e.i].unsupported = "the comment matches another constant's comment word for word"
				}
			}
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
	// Consistency guards, not extra precision: in a distribution that adds up to 1 a pick at the
	// default threshold already leads and leaves every other option at most 1-threshold. An answer
	// whose pick does not lead, or that also gives another option more than 1-threshold, contradicts
	// itself.
	for k, q := range a.Probabilities {
		if k == a.Choice {
			continue
		}
		if q >= p {
			return sdk.Abstain(fmt.Sprintf("%s at %.2f does not lead %s at %.2f", a.Choice, p, k, q))
		}
		if q > 1-r.threshold+1e-9 {
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
