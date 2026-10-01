// Package testnameassert finds a test whose name says one outcome of the call under test while its
// assertion on that call's error checks the other:
//
//	func TestValidateRejectsExpiredToken(t *testing.T) {
//		if err := Validate(expired); err != nil {
//			t.Fatal(err) // the name says Validate must fail; the test fails when it does
//		}
//	}
//
// Code binds the test to the call under test by the test's name (TestValidate... calls Validate,
// TestStore_Save... calls (Store).Save), takes the one call of it whose error result is kept, and
// reads what the test asserts about that error: `if err != nil { t.Fatal }` and NoError expect no
// error, `if err == nil { t.Fatal }` and Error expect one. The classifier reads only the test name,
// as words, and says whether it states that the call should fail. Go code compares the two.
//
// Anything ambiguous is unsupported: two functions the name matches equally, several calls of the
// bound function, an error variable written again, an error checked in a form not read here, or
// asserted both ways.
package testnameassert

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "test-name-vs-assertion"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of the name's answer for a finding (default 0.9, from the
	// prototype; not yet validated on a labelled set).
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts one candidate per test whose name binds a call whose error the test asserts.
var Analyzer = &analysis.Analyzer{
	Name:       "testnameassert",
	Doc:        "extract tests named after a call, with what the test asserts about that call's error",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a test whose name says the call should fail while it asserts no error, or the reverse",
		Standard:    true,
		Version:     "1",
		Analyzer:    Analyzer,
		NewSettings: func() any { return &Settings{} },
		New: func(s any) (sdk.Rule, error) {
			t, err := sdk.Threshold(s.(*Settings).Threshold, 0.9)
			if err != nil {
				return nil, err
			}
			return &rule{threshold: t}, nil
		},
	})
}

// What a test expects of the call's error.
const (
	expectError   = "error"
	expectNoError = "no_error"
)

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	var out []*sdk.Candidate
	ins.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fd := n.(*ast.FuncDecl)
		if fd.Body == nil || !facts.IsTest(pass.Fset, fd) || !takesT(pass.TypesInfo, fd) {
			return
		}
		if c := candidate(pass, fd); c != nil {
			out = append(out, c)
		}
	})
	return out, nil
}

// takesT reports whether the test has the signature func(*testing.T).
func takesT(info *types.Info, fd *ast.FuncDecl) bool {
	obj, ok := info.Defs[fd.Name].(*types.Func)
	if !ok {
		return false
	}
	sig := obj.Type().(*types.Signature)
	if sig.Params().Len() != 1 || sig.Results().Len() != 0 {
		return false
	}
	p, ok := sig.Params().At(0).Type().(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := p.Elem().(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "testing" && named.Obj().Name() == "T"
}

func candidate(pass *analysis.Pass, fd *ast.FuncDecl) *sdk.Candidate {
	info := pass.TypesInfo
	rest := strings.TrimLeft(strings.TrimPrefix(fd.Name.Name, "Test"), "_")
	if rest == "" {
		return nil
	}
	// The functions the name matches, longest match first; their calls that return an error.
	best := 0
	var bound []*types.Func
	calls := map[*types.Func][]*ast.CallExpr{}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn := facts.Callee(info, call)
		if fn == nil || len(facts.ErrorSlots(fn.Type().(*types.Signature))) == 0 {
			return true
		}
		l := matchLen(rest, fn)
		if l == 0 {
			return true
		}
		fn = fn.Origin()
		if _, seen := calls[fn]; !seen {
			switch {
			case l > best:
				best, bound = l, []*types.Func{fn}
			case l == best:
				bound = append(bound, fn)
			}
		}
		calls[fn] = append(calls[fn], call)
		return true
	})
	if len(bound) == 0 {
		return nil
	}
	words := facts.Words(rest)
	if len(facts.Words(rest[:best])) == len(words) {
		return nil // the name is only the function's: it says nothing to compare
	}
	fn := bound[0]
	c := &sdk.Candidate{
		Pos:     pass.Fset.Position(fd.Name.Pos()),
		Subject: fd.Name.Name,
		Local:   map[string]string{"test": fd.Name.Name, "call": fn.Name()},
	}
	switch {
	case len(bound) > 1:
		c.Unsupported = "the test name matches several called functions"
		return c
	case len(calls[fn]) > 1:
		c.Unsupported = "the test calls " + fn.Name() + " more than once"
		return c
	}
	e, why := errorOf(info, fd.Body, calls[fn][0])
	if why != "" {
		c.Unsupported = why
		return c
	}
	expect, why := expectation(info, fd.Body, e)
	if why != "" {
		c.Unsupported = why
		return c
	}
	c.Local["expect"] = expect
	c.Payload.AddProse("test", strings.Join(words, " "))
	c.Payload.Fact("call", fn.Name())
	return c
}

// matchLen returns how much of the test name, after Test, names the function: Validate, or
// Store_Save / StoreSave for a method, followed by a word boundary. 0 when it does not.
func matchLen(rest string, fn *types.Func) int {
	names := []string{fn.Name()}
	if recv := fn.Type().(*types.Signature).Recv(); recv != nil {
		t := recv.Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		if named, ok := t.(*types.Named); ok {
			// A method is named after its type too; the bare method name alone also binds.
			names = append(names, named.Obj().Name()+"_"+fn.Name(), named.Obj().Name()+fn.Name())
		}
	}
	best := 0
	for _, n := range names {
		n = upperFirst(n)
		if !strings.HasPrefix(rest, n) {
			continue
		}
		if len(rest) > len(n) {
			next := rune(rest[len(n)])
			if !unicode.IsUpper(next) && !unicode.IsDigit(next) && next != '_' {
				continue
			}
		}
		best = max(best, len(n))
	}
	return best
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// errExpr is the tested call's error as the test sees it: a variable defined once from the call,
// or the call itself when its only result is the error.
type errExpr struct {
	obj  types.Object
	call *ast.CallExpr
}

func (e errExpr) is(info *types.Info, x ast.Expr) bool {
	x = ast.Unparen(x)
	if e.call != nil && x == e.call {
		return true
	}
	id, ok := x.(*ast.Ident)
	return ok && e.obj != nil && info.ObjectOf(id) == e.obj
}

// errorOf binds the call's error: `err := F()`, `v, err := F()` (by the result's type), or the call
// used directly when it returns only an error. The variable must not be written again.
func errorOf(info *types.Info, body *ast.BlockStmt, call *ast.CallExpr) (errExpr, string) {
	fn := facts.Callee(info, call)
	sig := fn.Type().(*types.Signature)
	slots := facts.ErrorSlots(sig)
	var assign *ast.AssignStmt
	ast.Inspect(body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok && len(as.Rhs) == 1 && ast.Unparen(as.Rhs[0]) == call {
			assign = as
		}
		return assign == nil
	})
	if assign == nil {
		if sig.Results().Len() == 1 && used(body, call) {
			return errExpr{call: call}, ""
		}
		return errExpr{}, "the error of " + fn.Name() + " is not kept"
	}
	if len(slots) != 1 || slots[0] >= len(assign.Lhs) {
		return errExpr{}, "the error of " + fn.Name() + " is not defined once from the call"
	}
	id, ok := assign.Lhs[slots[0]].(*ast.Ident)
	if ok && id.Name == "_" {
		return errExpr{}, "the error of " + fn.Name() + " is not kept"
	}
	if !ok || assign.Tok != token.DEFINE {
		return errExpr{}, "the error of " + fn.Name() + " is not defined once from the call"
	}
	obj := info.Defs[id]
	if obj == nil {
		return errExpr{}, "the error of " + fn.Name() + " is not defined once from the call"
	}
	written := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n == assign {
				return true
			}
			for _, l := range n.Lhs {
				if lid, ok := ast.Unparen(l).(*ast.Ident); ok && info.ObjectOf(lid) == obj {
					written = true
				}
			}
		case *ast.UnaryExpr:
			if lid, ok := ast.Unparen(n.X).(*ast.Ident); ok && n.Op == token.AND && info.ObjectOf(lid) == obj {
				written = true
			}
		}
		return true
	})
	if written {
		return errExpr{}, "the error variable of " + fn.Name() + " is written again"
	}
	return errExpr{obj: obj}, ""
}

// used reports whether the call is part of a larger expression, not a statement of its own.
func used(body *ast.BlockStmt, call *ast.CallExpr) bool {
	stmt := false
	ast.Inspect(body, func(n ast.Node) bool {
		if es, ok := n.(*ast.ExprStmt); ok && ast.Unparen(es.X) == call {
			stmt = true
		}
		return !stmt
	})
	return !stmt
}

var (
	testifyNoError = map[string]bool{"NoError": true, "NoErrorf": true, "Nil": true, "Nilf": true}
	testifyError   = map[string]bool{
		// ErrorIs is left out: ErrorIs(t, err, nil) passes on no error, and a target may be nil at
		// run time, so what it expects depends on a value not read here.
		"Error": true, "Errorf": true, "ErrorAs": true, "ErrorAsf": true,
		"ErrorContains": true, "ErrorContainsf": true, "EqualError": true, "EqualErrorf": true,
		"NotNil": true, "NotNilf": true,
	}
	failures = map[string]bool{"Fatal": true, "Fatalf": true, "Error": true, "Errorf": true, "Fail": true, "FailNow": true}
)

func isTestify(fn *types.Func) bool {
	if fn.Pkg() == nil {
		return false
	}
	p := fn.Pkg().Path()
	return p == "github.com/stretchr/testify/assert" || p == "github.com/stretchr/testify/require"
}

// isFailure reports whether a call fails the test: t.Fatal, t.Errorf, t.Fail ... on a testing type.
func isFailure(info *types.Info, call *ast.CallExpr) bool {
	fn := facts.Callee(info, call)
	return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == "testing" && fn.Type().(*types.Signature).Recv() != nil && failures[fn.Name()]
}

// fails reports whether a block fails the test whenever it runs: a failure statement preceded only
// by statements that are known to fall through: t.Log, t.Logf, t.Helper, and assignments or
// declarations that call nothing. Any other call may end the test or the goroutine first (t.Skip,
// runtime.Goexit, panic, os.Exit, a helper that does one of those), and a branch, return or loop
// may skip the failure, so the failure is not established. The failure and the log calls before it
// must themselves be plain (see plain): skipT(t).Fatal(err) or t.Fatal(skipMessage(t)) evaluate a
// call that may skip the test before the failure runs. some reports a failure call anywhere in
// the block, outside closures, established or not.
func fails(info *types.Info, b *ast.BlockStmt, e errExpr) (always, some bool) {
	ast.Inspect(b, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if isFailure(info, n) {
				some = true
			}
		}
		return !some
	})
	for _, s := range b.List {
		switch s := s.(type) {
		case *ast.ExprStmt:
			call, ok := ast.Unparen(s.X).(*ast.CallExpr)
			switch {
			case ok && isFailure(info, call) && plain(info, call, e):
				return true, some
			case ok && isLog(info, call) && plain(info, call, e):
			default:
				return false, some // may stop the test before the failure: t.Skip, panic, os.Exit
			}
		case *ast.AssignStmt, *ast.DeclStmt:
			if callsAny(info, s) {
				return false, some
			}
		case *ast.EmptyStmt:
		default:
			return false, some // a statement that may branch or leave before the failure
		}
	}
	return false, some
}

// plain reports whether a testing method call evaluates nothing that could stop the test before it
// runs: its receiver is a *testing.T variable itself (t, not skipT(t)) and its arguments make no
// call other than type conversions and err.Error() on the asserted error variable itself.
func plain(info *types.Info, call *ast.CallExpr, e errExpr) bool {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return false
	}
	v, ok := info.ObjectOf(id).(*types.Var)
	if !ok || !isTestingT(v.Type()) {
		return false
	}
	for _, a := range call.Args {
		if callsAny(info, a) && !e.errorText(info, a) {
			return false
		}
	}
	return true
}

// errorText reports whether x is exactly err.Error() on the asserted error variable: an
// identifier bound to the same object, calling the method Error() string with no arguments.
func (e errExpr) errorText(info *types.Info, x ast.Expr) bool {
	call, ok := ast.Unparen(x).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 || e.obj == nil {
		return false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok || info.ObjectOf(id) != e.obj {
		return false
	}
	s := info.Selections[sel]
	if s == nil || s.Kind() != types.MethodVal || s.Obj().Name() != "Error" {
		return false
	}
	sig := s.Obj().Type().(*types.Signature)
	if sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}
	b, ok := sig.Results().At(0).Type().(*types.Basic)
	return ok && b.Kind() == types.String
}

func isTestingT(t types.Type) bool {
	p, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := p.Elem().(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "testing" && named.Obj().Name() == "T"
}

// fallThrough are the testing methods known to return to the caller.
var fallThrough = map[string]bool{"Log": true, "Logf": true, "Helper": true}

func isLog(info *types.Info, call *ast.CallExpr) bool {
	fn := facts.Callee(info, call)
	return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == "testing" && fn.Type().(*types.Signature).Recv() != nil && fallThrough[fn.Name()]
}

// callsAny reports whether any of the nodes makes a call other than a type conversion.
func callsAny(info *types.Info, ns ...ast.Node) bool {
	found := false
	for _, n := range ns {
		ast.Inspect(n, func(x ast.Node) bool {
			if c, ok := x.(*ast.CallExpr); ok {
				if tv, ok := info.Types[c.Fun]; !ok || !tv.IsType() {
					found = true
				}
			}
			return !found
		})
	}
	return found
}

// activeClosures returns the function literals a test body is known to run: called on the spot
// (func(){...}(), defer, go) or passed to t.Run. A closure only stored or passed elsewhere may
// never run, so its assertions are not the test's.
func activeClosures(info *types.Info, body *ast.BlockStmt) map[*ast.FuncLit]bool {
	out := map[*ast.FuncLit]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if fl, ok := ast.Unparen(call.Fun).(*ast.FuncLit); ok {
			out[fl] = true
		}
		fn := facts.Callee(info, call)
		if fn != nil && fn.Name() == "Run" && fn.Pkg() != nil && fn.Pkg().Path() == "testing" && fn.Type().(*types.Signature).Recv() != nil {
			for _, a := range call.Args {
				if fl, ok := ast.Unparen(a).(*ast.FuncLit); ok {
					out[fl] = true
				}
			}
		}
		return true
	})
	return out
}

// expectation reads what the test asserts about the error: `if err != nil { t.Fatal }` expects no
// error, `if err == nil { t.Fatal }` expects one, and testify's NoError / Error say it outright. A
// failing check or a testify assertion that mentions the error in any other form makes it
// unsupported, as do a failure the block may skip, an assertion in a closure the test is not
// known to run, and assertions both ways.
func expectation(info *types.Info, body *ast.BlockStmt, e errExpr) (string, string) {
	mentions := func(n ast.Node) bool {
		found := false
		ast.Inspect(n, func(n ast.Node) bool {
			if x, ok := n.(ast.Expr); ok && e.is(info, x) {
				found = true
			}
			return !found
		})
		return found
	}
	seen := map[string]bool{}
	unread := false
	active := activeClosures(info, body)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			if !active[n] {
				if mentions(n) {
					unread = true // an assertion that may never run
				}
				return false
			}
		case *ast.IfStmt:
			always, some := fails(info, n.Body, e)
			if !some || !mentions(n.Cond) {
				return true
			}
			if !always {
				unread = true // the failure may be skipped: if errors.Is(err, x) { return }
				return true
			}
			be, ok := ast.Unparen(n.Cond).(*ast.BinaryExpr)
			if !ok || (be.Op != token.EQL && be.Op != token.NEQ) {
				unread = true
				return true
			}
			isNil := func(x ast.Expr) bool { return info.Types[x].IsNil() }
			if !(e.is(info, be.X) && isNil(be.Y)) && !(e.is(info, be.Y) && isNil(be.X)) {
				unread = true
				return true
			}
			if be.Op == token.EQL {
				seen[expectError] = true // fails when there is no error
			} else {
				seen[expectNoError] = true
			}
		case *ast.CallExpr:
			fn := facts.Callee(info, n)
			if fn == nil || !isTestify(fn) {
				return true
			}
			arg := 1
			if fn.Type().(*types.Signature).Recv() != nil {
				arg = 0 // assert.New(t).NoError(err)
			}
			if arg >= len(n.Args) || !e.is(info, n.Args[arg]) {
				for _, a := range n.Args {
					if mentions(a) {
						unread = true
					}
				}
				return true
			}
			switch {
			case testifyNoError[fn.Name()]:
				seen[expectNoError] = true
			case testifyError[fn.Name()]:
				seen[expectError] = true
			default:
				unread = true
			}
		}
		return true
	})
	switch {
	case unread:
		return "", "the error is checked in a form not read here"
	case seen[expectError] && seen[expectNoError]:
		return "", "the error is asserted both ways"
	case seen[expectError]:
		return expectError, ""
	case seen[expectNoError]:
		return expectNoError, ""
	}
	return "", "the error is not asserted"
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "name_says",
		Kind: sdk.Choice,
		Text: "Judging only by the test name `test`, should the call under test return an error?",
		Options: []sdk.Option{
			{Key: expectError, Description: "Yes: the name says the call must fail, reject, refuse or return an error."},
			{Key: expectNoError, Description: "No: the name says the call accepts, succeeds or returns a value."},
			{Key: "input_only", Description: "The name only describes the input (bad quoting, empty list), not whether the call should fail."},
		},
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["name_says"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the test name does not say")
	}
	// Weak support abstains whichever way the answer goes.
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice == "input_only" || a.Choice == c.Local["expect"] {
		return sdk.Clean()
	}
	call := c.Local["call"]
	if a.Choice == expectError {
		return sdk.Report("test name expects an error from %s, but the test fails when %s returns one", call, call)
	}
	return sdk.Report("test name expects %s to succeed, but the test fails when %s returns no error", call, call)
}
