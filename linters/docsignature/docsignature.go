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
// return word (return, returns, yields, reports whether, tells the caller whether, gives back) or an error word (error, errors, err,
// ErrX). A result that carries an error inside it (chan error, func() error, a struct with an error
// field, a type parameter) is unsupported for the error claim: the doc may mean that error.
//
// For the result claim, a function that can reach an http.ResponseWriter (a parameter or the
// receiver, or a value reached from one through fields and methods without arguments, see
// writer.go) is asked questions of their own with a fact naming that writer, so the classifier can
// tell an HTTP handler's "returns the version", which is about the response, from a value or an
// error promised to the Go caller. The fact says the writer is there, not that it is used.
package docsignature

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

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

// Claims a candidate is asked about; Local["claim"] holds one of them.
const (
	claimResult = "result"
	claimError  = "error"
)

// The documented function's own name is masked in the doc, so the classifier reads what the doc
// says and not a name like GetSize.
const self = "the documented function"

var (
	returnWords = regexp.MustCompile(`(?i)\b(return|returns|returned|returning|yields?)\b|\b(reports|tells (the )?(caller|callers)) (whether|if)\b|\b(gives|hands) back\b`)
	// ErrX and errX are case-sensitive: errgroup is not an error word.
	errorWords = regexp.MustCompile(`(?i:\berr(?:or|ors)?\b)|\bErr[A-Z0-9_]\w*|\berr[A-Z]\w*`)
	errorIface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
)

func run(pass *analysis.Pass) (any, error) {
	var out []*sdk.Candidate
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || isTestingFunc(pass, fd) {
				continue
			}
			if obj, ok := pass.TypesInfo.Defs[fd.Name].(*types.Func); ok {
				sig := obj.Type().(*types.Signature)
				if sig.Recv() != nil && isErrorType(sig.Recv().Type()) {
					continue
				}
				out = appendCandidate(out, pass, fd.Doc, fd.Name, sig, true)
			}
		}
		// Interface declarations anywhere in the file, local ones and parenthesised ones included.
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			it, ok := ast.Unparen(ts.Type).(*ast.InterfaceType)
			if !ok {
				return true
			}
			if tn, ok := pass.TypesInfo.Defs[ts.Name].(*types.TypeName); ok && isErrorType(tn.Type()) {
				return true // an error interface: its methods' docs say "error" about the receiver
			}
			for _, m := range it.Methods.List {
				if len(m.Names) != 1 {
					continue // an embedded interface or constraint term
				}
				if obj, ok := pass.TypesInfo.Defs[m.Names[0]].(*types.Func); ok {
					out = appendCandidate(out, pass, m.Doc, m.Names[0], obj.Type().(*types.Signature), false)
				}
			}
			return true
		})
	}
	return out, nil
}

// isErrorType reports a type whose value or pointer implements error. Its methods, Error() string
// among them, say "error" about the receiver ("Error implements error", "GetFlag returns the flag
// for which the error occurred"), not about a result.
func isErrorType(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	t = types.Unalias(t)
	return implementsError(t) || !types.IsInterface(t) && implementsError(types.NewPointer(t))
}

// isTestingFunc reports a Test, Benchmark, Fuzz or Example function in a _test.go file, named by the
// go test rule (the prefix alone, or followed by a rune that is not lower case: TestX, Test_x, not
// Testify): its doc describes what it checks, and the results it mentions are another function's.
func isTestingFunc(pass *analysis.Pass, fd *ast.FuncDecl) bool {
	if fd.Recv != nil || !strings.HasSuffix(pass.Fset.Position(fd.Pos()).Filename, "_test.go") {
		return false
	}
	for _, p := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		rest, ok := strings.CutPrefix(fd.Name.Name, p)
		if !ok {
			continue
		}
		if r, _ := utf8.DecodeRuneInString(rest); rest == "" || !unicode.IsLower(r) {
			return true
		}
	}
	return false
}

// appendCandidate adds the candidate for one documented function; declared is false for an
// interface method, which has no body.
func appendCandidate(out []*sdk.Candidate, pass *analysis.Pass, doc *ast.CommentGroup, id *ast.Ident, sig *types.Signature, declared bool) []*sdk.Candidate {
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
			if carriesError(res.At(i).Type()) {
				c.Unsupported = "a result may hold an error (an interface, a type parameter, an error inside it); the doc may mean that error"
				return append(out, c)
			}
		}
	}
	if claim == claimResult {
		if w := writerFact(pass.Pkg, sig, declared); w != "" {
			c.Payload.Fact("writer", w)
			c.Local["writer"] = w
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

// maxCarrierNodes bounds the walk over a result type; a type too large to walk counts as a carrier,
// so the check fails closed.
const maxCarrierNodes = 10000

// carriesError reports a result type that may hold an error a doc could call "an error": a
// non-error interface that an error could also implement (any, io.Reader, a Coded interface), a type
// parameter, or either of them or an error reachable through an element, a field or a function
// result. Named types are walked once:
// every cycle in a type goes through one, so the walk visits each part of the type and is complete.
func carriesError(t types.Type) bool {
	seen := map[types.Type]bool{}
	nodes := 0
	var walk func(t types.Type, root bool) bool
	walk = func(t types.Type, root bool) bool {
		nodes++
		if nodes > maxCarrierNodes {
			return true
		}
		t = types.Unalias(t)
		if _, ok := t.(*types.TypeParam); ok {
			return true // checked before Underlying, which is the constraint
		}
		if n, ok := t.(*types.Named); ok {
			if seen[n] {
				return false
			}
			seen[n] = true
		}
		if !root && implementsError(t) {
			return true
		}
		switch u := t.Underlying().(type) {
		case *types.Interface:
			// A non-error interface can hold a value that also implements error (any, or a Coded
			// interface holding a coded error), so a doc that says it returns an error may be right.
			// Only an interface whose own Error method has another signature cannot.
			return !excludesError(u)
		case *types.Pointer:
			return walk(u.Elem(), false)
		case *types.Slice:
			return walk(u.Elem(), false)
		case *types.Array:
			return walk(u.Elem(), false)
		case *types.Chan:
			return walk(u.Elem(), false)
		case *types.Map:
			return walk(u.Key(), false) || walk(u.Elem(), false)
		case *types.Signature:
			for i := 0; i < u.Results().Len(); i++ {
				if walk(u.Results().At(i).Type(), false) {
					return true
				}
			}
		case *types.Struct:
			for i := 0; i < u.NumFields(); i++ {
				if walk(u.Field(i).Type(), false) {
					return true
				}
			}
		}
		return false
	}
	return walk(t, true)
}

// excludesError reports an interface no error can implement: it has an Error method whose signature
// is not func() string.
func excludesError(it *types.Interface) bool {
	for i := 0; i < it.NumMethods(); i++ {
		if m := it.Method(i); m.Name() == "Error" {
			return !types.Identical(m.Type(), errorIface.Method(0).Type())
		}
	}
	return false
}

type rule struct{ threshold float64 }

// A function that can reach an http.ResponseWriter, named by the `writer` fact, is asked two
// questions instead of returns_result. An HTTP handler's "returns the version" is about the
// response, which questionWriter lets the classifier say; the fact says only that the writer is
// there, not that the function writes to it. A promised error is never about the response, since an error value never reaches a
// client, and questionWriterError asks about it alone, so a doc that promises one is not lost in
// the reading of the rest. Either answer is enough for a finding.
const (
	questionWriter      = "returns_to_caller"
	questionWriterError = "returns_error_to_caller"
)

func (r *rule) Questions(c *sdk.Candidate) []sdk.Question {
	if c.Local["claim"] == claimResult && c.Local["writer"] != "" {
		return []sdk.Question{{
			ID:   questionWriterError,
			Kind: sdk.Noul,
			Text: "Does any sentence of `doc` state that the documented function returns an error value to its caller, as in \"returns an error if the file is missing\" or \"returns the parsed value and an error\"? One such sentence is enough. These do not count: an error the sentence says is written or sent to a client, in a response or to an output; an error the function logs, records, panics with or passes on; an error another function returns; errors mentioned in general.",
		}, {
			ID:   questionWriter,
			Kind: sdk.Noul,
			Text: "The documented function has no results, and `writer` names an http.ResponseWriter it can reach, so it may be an HTTP handler that writes a response to a client; the code does not show that it does. In the doc of an HTTP handler, \"returns X\" often means that X is sent in the response. Does any sentence of `doc` promise something that only a Go return value could give the function's caller: an error value, or a value the doc says the caller gets back? One such sentence is enough. An error counts, alone or next to other values, as in \"returns an error if the upload fails\": an error value is never written to a client. A count or a flag the caller gets back counts, as in \"returns the number of bytes written\". These do not count: what the function returns, responds with or serves as an HTTP response, as in \"returns the list of users\" or \"returns the current configuration\" on a handler; a value the sentence says is written or sent to a client or an output; when or how it returns; returning something into a pool or to an owner; a value delivered another way, such as sent on a channel or passed to a callback; what it does; what another function or a request returns; a statement that it returns nothing.",
		}}
	}
	if c.Local["claim"] == claimResult {
		return []sdk.Question{{
			ID:   "returns_result",
			Kind: sdk.Noul,
			Text: "Does any sentence of `doc` state that the documented function gives something back to its caller, as in \"returns the number of bytes written\", \"returns any errors\" or \"reports whether the name is valid\"? One such sentence is enough. These do not count: when or how it returns; returning something into a pool or to an owner; a value delivered another way, such as sent on a channel, passed to a callback or written to a writer or to output; what it does; what another function or a request returns; a statement that it returns nothing.",
		}}
	}
	return []sdk.Question{{
		ID:   "returns_error",
		Kind: sdk.Noul,
		Text: "Does any sentence of `doc` state that one of the documented function's own results is an error value, as in \"returns an error if the file is missing\"? One such sentence is enough. These do not count: a returned value that carries, wraps, describes or formats an error; an error the function logs, records, panics with or passes on; an error another function returns; errors mentioned in general.",
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	name := c.Local["name"]
	switch {
	case c.Local["claim"] == claimError:
		return r.decide(answers, []string{"returns_error"}, func(string) sdk.Decision {
			return sdk.Report("doc of %s says it returns an error, but %s has no error result", name, name)
		})
	case c.Local["writer"] != "":
		return r.decide(answers, []string{questionWriterError, questionWriter}, func(id string) sdk.Decision {
			if id == questionWriterError {
				return sdk.Report("doc of %s says it returns an error, but %s returns nothing", name, name)
			}
			return sdk.Report("doc of %s says it returns a result, but %s returns nothing", name, name)
		})
	}
	return r.decide(answers, []string{"returns_result"}, func(string) sdk.Decision {
		return sdk.Report("doc of %s says it returns a result, but %s returns nothing", name, name)
	})
}

// decide reports when any of the questions is answered yes at the threshold, naming the first such
// question; it is clean when every one is answered no at the threshold, and abstains otherwise.
func (r *rule) decide(answers map[string]sdk.Answer, ids []string, report func(id string) sdk.Decision) sdk.Decision {
	allNo := true
	var weak []string
	for _, id := range ids {
		y := answers[id].Yes
		switch {
		case y == nil:
			return sdk.Abstain("the classifier gave no probability of yes")
		case *y >= r.threshold:
			return report(id)
		case *y > 1-r.threshold:
			allNo = false
			weak = append(weak, fmt.Sprintf("%s at %.2f", id, *y))
		}
	}
	if allNo {
		return sdk.Clean()
	}
	return sdk.Abstain(fmt.Sprintf("yes (%s) is neither ruled out nor established (threshold %.2f)", strings.Join(weak, ", "), r.threshold))
}
