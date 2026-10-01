// Package sentinelname finds a sentinel error whose name and message describe different failure
// conditions, as when a declaration is copied and only one half is edited:
//
//	var ErrNotFound = errors.New("permission denied")
//
// Code finds the shape: a package-level variable of type error, named Err... or err..., initialised
// with a constant-message error constructor (errors.New, fmt.Errorf without %w, and the pkg/errors,
// xerrors and cockroachdb equivalents). The classifier reads the name, also split into words, and
// the message, and says whether they describe the same condition. A message built at run time or one
// that wraps another error is unsupported: its text is not the whole condition.
package sentinelname

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"

	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "sentinel-name-vs-text"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of the classifier's answer for a decision either way
	// (default 0.55, from the prototype). It is low because on a short name and a short message the
	// classifier splits its mass between "same" and "different" even when a reader would not
	// hesitate; Margin then keeps a near tie from becoming a finding. Not yet validated on a labelled
	// set.
	Threshold *float64 `yaml:"threshold"`
	// Margin is how much more probable "different" must be than "same" for a finding (default 0.2,
	// from the prototype).
	Margin *float64 `yaml:"margin"`
}

// Analyzer extracts package-level sentinel errors with a constant message.
var Analyzer = &analysis.Analyzer{
	Name:       "sentinelname",
	Doc:        "extract package-level sentinel errors with a constant message",
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a sentinel error whose name and message describe different conditions",
		Standard:    true,
		Version:     "2",
		Analyzer:    Analyzer,
		NewSettings: func() any { return &Settings{} },
		New: func(s any) (sdk.Rule, error) {
			st := s.(*Settings)
			t, err := sdk.Threshold(st.Threshold, 0.55)
			if err != nil {
				return nil, err
			}
			m, err := sdk.Threshold(st.Margin, 0.2)
			if err != nil {
				return nil, fmt.Errorf("margin: %w", err)
			}
			return &rule{threshold: t, margin: m}, nil
		},
	})
}

func run(pass *analysis.Pass) (any, error) {
	var out []*sdk.Candidate
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, s := range gd.Specs {
				vs := s.(*ast.ValueSpec)
				if len(vs.Values) != len(vs.Names) {
					continue // var a, b = f(): not an error constructor per name
				}
				for i, id := range vs.Names {
					if c := candidate(pass, id, vs.Values[i]); c != nil {
						out = append(out, c)
					}
				}
			}
		}
	}
	return out, nil
}

func candidate(pass *analysis.Pass, id *ast.Ident, value ast.Expr) *sdk.Candidate {
	obj := pass.TypesInfo.Defs[id]
	if obj == nil || !facts.IsError(obj.Type()) {
		return nil
	}
	words, ok := nameWords(id.Name)
	if !ok {
		return nil
	}
	call, ok := ast.Unparen(value).(*ast.CallExpr)
	if !ok {
		return nil
	}
	ec, ok := facts.AsErrorCall(pass.TypesInfo, call)
	if !ok {
		return nil
	}
	c := &sdk.Candidate{
		Pos:     pass.Fset.Position(id.Pos()),
		Subject: id.Name,
		Local:   map[string]string{"name": id.Name},
	}
	switch {
	case ec.Wraps:
		c.Unsupported = "the sentinel wraps another error; its message is not the whole condition"
		return c
	case !ec.MessageKnown:
		c.Unsupported = "the message is not a constant string"
		return c
	case strings.TrimSpace(ec.Message) == "":
		c.Unsupported = "the message is empty"
		return c
	}
	c.Local["text"] = ec.Message
	c.Payload.Fact("name", id.Name)
	c.Payload.Fact("name_words", words)
	c.Payload.AddProse("text", ec.Message)
	return c
}

// nameWords returns the words of a sentinel's name after its Err prefix (ErrNotFound -> "not
// found"), and false when the name does not follow the Err/err convention or says nothing more.
func nameWords(name string) (string, bool) {
	rest, ok := strings.CutPrefix(name, "Err")
	if !ok {
		rest, ok = strings.CutPrefix(name, "err")
	}
	if !ok || rest == "" {
		return "", false
	}
	if r := []rune(rest)[0]; !unicode.IsUpper(r) && !unicode.IsDigit(r) && r != '_' {
		return "", false // Errata, errand
	}
	ws := facts.Words(strings.TrimLeft(rest, "_"))
	if len(ws) == 0 {
		return "", false
	}
	return strings.Join(ws, " "), true
}

type rule struct{ threshold, margin float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "condition",
		Kind: sdk.Choice,
		Text: "Do the error variable name `name` (its words are in `name_words`) and its message `text` describe the same failure condition?",
		Options: []sdk.Option{
			{Key: "same", Description: "They describe the same failure condition, possibly in other words."},
			{Key: "different", Description: "They describe different failure conditions."},
			{Key: "insufficient", Description: "Too vague to tell."},
		},
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["condition"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the classifier cannot tell what the name or the message describes")
	}
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	// Weak support abstains whichever way the answer goes.
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice != "different" {
		return sdk.Clean()
	}
	// The margin needs the competing probability: a backend that omits it has not said it is zero.
	same, ok := a.Probability("same")
	if !ok {
		return sdk.Abstain("the classifier gave no probability for same, so the margin cannot be checked")
	}
	if p-same < r.margin {
		return sdk.Abstain(fmt.Sprintf("different at %.2f is within %.2f of same at %.2f", p, r.margin, same))
	}
	return sdk.Report("sentinel error name and message describe different conditions: %s %q", c.Local["name"], clip(c.Local["text"], 80))
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
