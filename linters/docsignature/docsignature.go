// Package docsignature finds a doc comment that promises a result the function's signature does not
// have, as when a function loses its results and its doc is not updated:
//
//	// Sync writes the buffer to disk and returns the number of bytes written.
//	func (b *Buffer) Sync() { ... }
//
//	// Validate returns an error if the name is empty.
//	func Validate(name string) bool { ... }
//
// Code reads the signature through go/types: whether the function has results at all, and whether
// any of them is an error. The classifier reads only the doc, with the function's own name masked,
// and says whether it promises a returned result (for a function with none) or a returned error (for
// a function with results but no error among them). A function that already has an error result
// makes neither claim checkable and is not a candidate.
//
// To keep the classifier to docs that can make the claim, a doc is asked about only when it uses a
// return word (return, returns, reports whether, yields) or an error word (error, errors, err,
// ErrX). A result that carries an error inside it (chan error, func() error, a struct with an error
// field, a type parameter) is unsupported for the error claim: the doc may mean that error.
package docsignature

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "doc-vs-signature"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of yes for a finding, and of no for a clean answer; in
	// between the rule abstains. The default 0.85 matches the other promise linters; it is not yet
	// validated on a labelled set.
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts documented functions and interface methods whose doc can promise a result their
// signature lacks.
var Analyzer = &analysis.Analyzer{
	Name:       "docsignature",
	Doc:        "extract documented functions whose doc may promise a result the signature lacks",
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a doc comment that promises a returned result or error the function's signature does not have",
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

// Claims a candidate is asked about; Local["claim"] holds one of them.
const (
	claimResult = "result"
	claimError  = "error"
)

// The documented function's own name is masked in the doc, so the classifier reads what the doc
// says and not a name like GetSize.
const self = "the documented function"

var (
	returnWords = regexp.MustCompile(`(?i)\b(return|returns|returned|returning|yields?)\b|\breports whether\b`)
	// ErrX and errX are case-sensitive: errgroup is not an error word.
	errorWords = regexp.MustCompile(`(?i:\berr(?:or|ors)?\b)|\bErr[A-Z0-9_]\w*|\berr[A-Z]\w*`)
	errorIface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
)

func run(pass *analysis.Pass) (any, error) {
	var out []*sdk.Candidate
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if isTestingFunc(pass, d) {
					continue
				}
				if obj, ok := pass.TypesInfo.Defs[d.Name].(*types.Func); ok {
					sig := obj.Type().(*types.Signature)
					if onErrorType(sig) {
						continue
					}
					out = appendCandidate(out, pass, d.Doc, d.Name, sig)
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					ts, ok := s.(*ast.TypeSpec)
					if !ok {
						continue
					}
					it, ok := ts.Type.(*ast.InterfaceType)
					if !ok {
						continue
					}
					for _, m := range it.Methods.List {
						if len(m.Names) != 1 {
							continue // an embedded interface or constraint term
						}
						if obj, ok := pass.TypesInfo.Defs[m.Names[0]].(*types.Func); ok {
							out = appendCandidate(out, pass, m.Doc, m.Names[0], obj.Type().(*types.Signature))
						}
					}
				}
			}
		}
	}
	return out, nil
}

// onErrorType reports a method of a type that implements error, Error() string among them: its doc
// says "error" about the receiver ("Error implements error", "GetFlag returns the flag for which
// the error occurred"), not about a result.
func onErrorType(sig *types.Signature) bool {
	recv := sig.Recv()
	if recv == nil {
		return false
	}
	t := recv.Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	return implementsError(t) || implementsError(types.NewPointer(t))
}

// isTestingFunc reports a Test, Benchmark, Fuzz or Example function in a _test.go file: its doc
// describes what it checks, and the results it mentions are another function's.
func isTestingFunc(pass *analysis.Pass, fd *ast.FuncDecl) bool {
	if fd.Recv != nil || !strings.HasSuffix(pass.Fset.Position(fd.Pos()).Filename, "_test.go") {
		return false
	}
	for _, p := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(fd.Name.Name, p) {
			return true
		}
	}
	return false
}

func appendCandidate(out []*sdk.Candidate, pass *analysis.Pass, doc *ast.CommentGroup, id *ast.Ident, sig *types.Signature) []*sdk.Candidate {
	if doc == nil {
		return out
	}
	text := strings.TrimSpace(doc.Text())
	if text == "" {
		return out // only directives
	}
	res := sig.Results()
	var claim string
	switch {
	case res.Len() == 0:
		if !returnWords.MatchString(text) {
			return out
		}
		claim = claimResult
	case hasErrorResult(res):
		return out
	default:
		if !errorWords.MatchString(text) {
			return out
		}
		claim = claimError
	}
	c := &sdk.Candidate{
		Pos:     pass.Fset.Position(id.Pos()),
		Subject: id.Name,
		Local:   map[string]string{"name": id.Name, "claim": claim},
	}
	if claim == claimError {
		for i := 0; i < res.Len(); i++ {
			if carriesError(res.At(i).Type(), map[types.Type]bool{}, 0) {
				c.Unsupported = "a result carries an error inside it; the doc may mean that error"
				return append(out, c)
			}
		}
	}
	c.Payload.AddProse("doc", sdk.Mask(text, id.Name, self))
	return append(out, c)
}

// hasErrorResult reports a result whose type is or implements error.
func hasErrorResult(res *types.Tuple) bool {
	for i := 0; i < res.Len(); i++ {
		if implementsError(res.At(i).Type()) {
			return true
		}
	}
	return false
}

func implementsError(t types.Type) bool {
	if facts.IsError(t) {
		return true
	}
	if _, ok := t.(*types.TypeParam); ok {
		return false // decided by carriesError
	}
	return types.Implements(t, errorIface)
}

// carriesError reports a type that holds an error somewhere a doc could call "an error": an element,
// a field, a function result, or a type parameter that may be one.
func carriesError(t types.Type, seen map[types.Type]bool, depth int) bool {
	if depth > 4 || seen[t] {
		return false
	}
	seen[t] = true
	if depth > 0 && implementsError(t) {
		return true
	}
	switch u := t.(type) {
	case *types.TypeParam:
		return true
	case *types.Alias:
		return carriesError(types.Unalias(u), seen, depth)
	case *types.Named:
		return carriesError(u.Underlying(), seen, depth)
	case *types.Pointer:
		return carriesError(u.Elem(), seen, depth+1)
	case *types.Slice:
		return carriesError(u.Elem(), seen, depth+1)
	case *types.Array:
		return carriesError(u.Elem(), seen, depth+1)
	case *types.Chan:
		return carriesError(u.Elem(), seen, depth+1)
	case *types.Map:
		return carriesError(u.Key(), seen, depth+1) || carriesError(u.Elem(), seen, depth+1)
	case *types.Signature:
		for i := 0; i < u.Results().Len(); i++ {
			if carriesError(u.Results().At(i).Type(), seen, depth+1) {
				return true
			}
		}
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if carriesError(u.Field(i).Type(), seen, depth+1) {
				return true
			}
		}
	}
	return false
}

type rule struct{ threshold float64 }

func (r *rule) Questions(c *sdk.Candidate) []sdk.Question {
	if c.Local["claim"] == claimResult {
		return []sdk.Question{{
			ID:   "returns_result",
			Kind: sdk.Noul,
			Text: "Does any sentence of `doc` state that the documented function gives something back to its caller, as in \"returns the number of bytes written\", \"returns any errors\" or \"reports whether the name is valid\"? One such sentence is enough. Sentences that only say when or how it returns, that it returns something into a pool or to an owner, what it does, or what another function or a request returns do not count.",
		}}
	}
	return []sdk.Question{{
		ID:   "returns_error",
		Kind: sdk.Noul,
		Text: "Does any sentence of `doc` state that one of the documented function's own results is an error value, as in \"returns an error if the file is missing\"? One such sentence is enough. These do not count: a returned value that carries, wraps, describes or formats an error; an error the function logs, records, panics with or passes on; an error another function returns; errors mentioned in general.",
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	id, what := "returns_result", "a result"
	if c.Local["claim"] == claimError {
		id, what = "returns_error", "an error"
	}
	y := answers[id].Yes
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
	if c.Local["claim"] == claimError {
		return sdk.Report("doc of %s says it returns %s, but %s has no error result", name, what, name)
	}
	return sdk.Report("doc of %s says it returns %s, but %s returns nothing", name, what, name)
}
