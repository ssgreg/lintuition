// Package readonlypromise finds a doc comment that promises a function changes nothing while its
// body writes state the caller can see:
//
//	// Peek returns the next item without altering the queue.
//	func (q *Queue) Peek() Item { q.head++; return q.items[q.head-1] }
//
// Code finds the writes through internal/effects: a write the caller sees, reached from the
// receiver, a parameter or a package-level variable. For each written root the classifier reads the
// doc, with the function's own name masked, and a description of the root built from identifiers
// ("the receiver q, a Queue"), and says whether the doc promises to leave it unchanged. One yes/no
// question per root, rather than one choice among all of them: offered every root at once, the
// classifier spread its answer over them and no option reached the threshold.
//
// Every documented function whose body has a direct write is asked, without a filter on the doc's
// words: a filter of no-change phrases missed "is left as it was" and "does not reorder", and on 14
// public repositories asking every such function found the same two findings while the classifier
// answered a clear no for docs that promise nothing. A body whose only writes are inside a function
// literal is unsupported: when the literal runs is not known.
package readonlypromise

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ssgreg/lintuition/internal/effects"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "read-only-promise"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of yes for a finding, and of no for a clean answer; in
	// between the rule abstains. The default 0.75 is lower than the other promise linters' 0.85:
	// on this repository's twins, a held-out set and 14 public repositories, docs that plainly
	// promised no change scored 0.77 to 0.95 and docs that did not scored at most 0.56. It is tuned
	// on those sets, not validated on a labelled one.
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts documented functions whose doc promises no change and whose body writes state.
var Analyzer = &analysis.Analyzer{
	Name:       "readonlypromise",
	Doc:        "extract documented functions that promise no change and write caller-visible state",
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a doc comment that promises a function changes nothing while its body writes the receiver, a parameter or a package-level variable",
		Standard:    true,
		Version:     "1",
		Analyzer:    Analyzer,
		NewSettings: func() any { return &Settings{} },
		New: func(s any) (sdk.Rule, error) {
			t, err := sdk.Threshold(s.(*Settings).Threshold, 0.75)
			if err != nil {
				return nil, err
			}
			return &rule{threshold: t}, nil
		},
	})
}

const self = "the documented function"

func run(pass *analysis.Pass) (any, error) {
	var out []*sdk.Candidate
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Doc == nil || fd.Body == nil {
				continue
			}
			text := strings.TrimSpace(fd.Doc.Text())
			if text == "" {
				continue // only directives
			}
			ws := effects.Writes(pass.TypesInfo, fd)
			if len(ws) == 0 {
				continue
			}
			// One candidate per written root: the receiver, each parameter, package-level state. Each
			// asks one question, whether the doc promises to leave that root unchanged.
			seen := map[string]bool{}
			direct := false
			for _, w := range ws {
				if w.InClosure {
					continue
				}
				direct = true
				target := describe(pass.TypesInfo, fd, w)
				if seen[target] {
					continue
				}
				seen[target] = true
				c := &sdk.Candidate{
					Pos:     pass.Fset.Position(fd.Name.Pos()),
					Subject: fd.Name.Name + " " + w.Root,
					Local:   map[string]string{"name": fd.Name.Name, "root": w.Root, "path": w.Path},
				}
				if w.Kind == effects.Global {
					c.Local["global"] = "true"
				}
				c.Payload.AddProse("doc", sdk.Mask(text, fd.Name.Name, self))
				c.Payload.Fact("target", target)
				out = append(out, c)
			}
			if !direct {
				out = append(out, &sdk.Candidate{
					Pos:         pass.Fset.Position(fd.Name.Pos()),
					Subject:     fd.Name.Name,
					Unsupported: "the only writes are inside a function literal; when it runs is not known",
				})
			}
		}
	}
	return out, nil
}

// describe names a written root for the classifier: "the receiver q, a Queue", "the parameter xs",
// "the package-level variable calls". All package-level writes share one description.
func describe(info *types.Info, fd *ast.FuncDecl, w effects.Write) string {
	switch w.Kind {
	case effects.Receiver:
		return "the receiver " + w.Root + typed(typeName(info, fd, w.Root))
	case effects.Param:
		return "the parameter " + w.Root + typed(typeName(info, fd, w.Root))
	}
	return "package-level state, the variable " + w.Root
}

func typed(t string) string {
	if t == "" {
		return ""
	}
	return ", a " + t
}

// typeName returns the name of the named type of a receiver or parameter, through one pointer
// (Cache for c *Cache), or the kind of an unnamed one (slice, map), so the description can say "the
// receiver c, a Cache" or "the parameter names, a slice" and a doc that says "the cache" or "the
// input slice" is matched to it; empty otherwise.
func typeName(info *types.Info, fd *ast.FuncDecl, name string) string {
	if name == "" {
		return ""
	}
	var lists []*ast.FieldList
	if fd.Recv != nil {
		lists = append(lists, fd.Recv)
	}
	lists = append(lists, fd.Type.Params)
	for _, l := range lists {
		for _, f := range l.List {
			for _, n := range f.Names {
				if n.Name != name {
					continue
				}
				t := info.TypeOf(n)
				if p, ok := t.(*types.Pointer); ok {
					t = p.Elem()
				}
				if nt, ok := types.Unalias(t).(*types.Named); ok {
					return nt.Obj().Name()
				}
				switch t.Underlying().(type) {
				case *types.Slice:
					return "slice"
				case *types.Map:
					return "map"
				case *types.Array:
					return "array"
				case *types.Struct:
					return "struct"
				}
				return ""
			}
		}
	}
	return ""
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "promises_unchanged",
		Kind: sdk.Noul,
		Text: "Does `doc` promise that the documented function leaves `target` unchanged? A promise that the function changes nothing, is read-only, is pure or has no side effects covers every target. These do not count: a promise about something else, one that holds only in some cases (such as on failure), a promise about what later changes to a returned copy do, or a description of what the function changes.",
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	y := answers["promises_unchanged"].Yes
	switch {
	case y == nil:
		return sdk.Abstain("the classifier gave no probability of yes")
	case *y >= r.threshold:
	case *y <= 1-r.threshold:
		return sdk.Clean()
	default:
		return sdk.Abstain(fmt.Sprintf("yes at %.2f is neither ruled out nor established (threshold %.2f)", *y, r.threshold))
	}
	name := c.Local["name"]
	if c.Local["global"] != "" {
		return sdk.Report("doc of %s promises it changes nothing, but %s writes %s", name, name, c.Local["path"])
	}
	return sdk.Report("doc of %s promises to leave %s unchanged, but %s writes %s", name, c.Local["root"], name, c.Local["path"])
}
