// Package readonlypromise finds a doc comment that promises a function changes nothing while its
// body writes state the caller can see:
//
//	// Peek returns the next item without altering the queue.
//	func (q *Queue) Peek() Item { q.head++; return q.items[q.head-1] }
//
// Code finds the writes through internal/effects: a write the caller sees, reached from the
// receiver, a parameter or a package-level variable. For each written root the classifier reads the
// doc, with the function's own name masked, and a description of the root built from identifiers
// ("the receiver q, a Queue"), and says whether the doc promises to leave it, as a whole, unchanged.
// One yes/no question per root, rather than one choice among all of them: offered every root at
// once, the classifier spread its answer over them and no option reached the threshold. A promise
// about part of a root (its flags, its size) does not count: which fields that part covers is not
// known, so matching it to a write would be a guess.
//
// Every documented function whose body has a direct write is asked, without a filter on the doc's
// words: a filter of no-change phrases missed "is left as it was" and "does not reorder", and on 14
// public repositories asking every such function found the same two findings while the classifier
// answered a clear no for docs that promise nothing. A root whose only writes are inside a function
// literal, or are uncertain (after the root was reassigned), is unsupported for that root; a write a
// proven save and restore undoes is left out.
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
	// between the rule abstains. The default 0.7 is lower than the other promise linters' 0.85: on
	// this repository's twins, two held-out sets and 14 public repositories, most docs that plainly
	// promised no change scored 0.72 to 0.93 and docs that did not scored at most 0.52. It is tuned
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
			t, err := sdk.Threshold(s.(*Settings).Threshold, 0.7)
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
			// asks one question, whether the doc promises to leave that root unchanged. A root whose
			// writes are all uncertain or inside a closure is unsupported; a write a proven save and
			// restore undoes is not a change at return and is left out.
			type rootState struct {
				root, path, reason string
				direct             bool
			}
			var order []string
			states := map[string]*rootState{}
			for _, w := range ws {
				if w.Restored {
					continue
				}
				target := describe(pass.TypesInfo, fd, w)
				st, ok := states[target]
				if !ok {
					st = &rootState{root: w.Root}
					states[target] = st
					order = append(order, target)
				}
				switch {
				case w.Direct():
					if !st.direct {
						st.direct, st.path = true, w.Path
					}
				case st.reason == "" && w.InClosure:
					st.reason = "the only writes to " + w.Root + " are inside a function literal; when it runs is not known"
				case st.reason == "":
					st.reason = "the only writes to " + w.Root + " are uncertain: " + w.Uncertain
				}
			}
			for _, target := range order {
				st := states[target]
				c := &sdk.Candidate{
					Pos:     pass.Fset.Position(fd.Name.Pos()),
					Subject: fd.Name.Name + " " + target,
				}
				if !st.direct {
					c.Unsupported = st.reason
					out = append(out, c)
					continue
				}
				c.Local = map[string]string{"name": fd.Name.Name, "root": st.root, "path": st.path}
				c.Payload.AddProse("doc", sdk.Mask(text, fd.Name.Name, self))
				c.Payload.Fact("target", target)
				out = append(out, c)
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
		Text: "Does `doc` promise that the documented function leaves `target` unchanged? A promise that the function changes nothing, is read-only, is pure or has no side effects covers every target. These do not count: a promise that names only some part of it, such as its flags, its size or its order; a promise about something else; one that holds only in some cases (such as on failure); a promise about what later changes to a returned copy do; a description of what the function changes.",
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
	return sdk.Report("doc of %s promises to leave %s unchanged, but %s writes %s", name, c.Local["root"], name, c.Local["path"])
}
