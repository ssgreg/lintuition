package severeunderstated

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/ssgreg/lintuition/internal/facts"
)

// branchFacts names, nearest first, the conditions the code has established where the log call
// runs, read from the enclosing statements through go/types:
//
//   - "error is fs.ErrNotExist": an if or case condition errors.Is(x, fs.ErrNotExist) or
//     x == io.EOF, with a package-level error variable as the target, holds in its body;
//   - "os.IsNotExist": the same for the os.IsNotExist, os.IsExist, os.IsPermission and
//     os.IsTimeout predicates;
//   - the same conditions when an earlier statement of the block left it under their negation:
//     if !os.IsNotExist(err) { return err } leaves the rest of the block on os.IsNotExist;
//   - "context done": a select case receiving from the Done channel of a context.Context;
//   - "signal received": a select case receiving from a channel of os.Signal, or a function
//     literal passed to a call that also takes a signal (a signal handler).
//
// Only conditions that hold positively count: the else branch of errors.Is(err, X), or a
// condition under !, establishes nothing that can be named.
func branchFacts(info *types.Info, stack []ast.Node) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if s != "" && !seen[s] && facts.FactSafe(s) {
			seen[s] = true
			out = append(out, s)
		}
	}
	for i := len(stack) - 1; i > 0; i-- {
		child, parent := stack[i], stack[i-1]
		switch p := parent.(type) {
		case *ast.BlockStmt:
			for _, s := range guardsBefore(info, p.List, child) {
				add(s)
			}
		case *ast.CaseClause:
			for _, s := range guardsBefore(info, p.Body, child) {
				add(s)
			}
			if inList(p.Body, child) {
				for _, s := range caseConditions(info, p, stack[:i-1]) {
					add(s)
				}
			}
		case *ast.CommClause:
			for _, s := range guardsBefore(info, p.Body, child) {
				add(s)
			}
			if inList(p.Body, child) {
				add(commCondition(info, p.Comm))
			}
		case *ast.IfStmt:
			switch child {
			case p.Body:
				for _, s := range conditions(info, p.Cond, false) {
					add(s)
				}
			case p.Else:
				for _, s := range conditions(info, p.Cond, true) {
					add(s)
				}
			}
		case *ast.CallExpr:
			if _, ok := child.(*ast.FuncLit); ok && takesSignal(info, p) {
				add("signal received")
			}
		}
	}
	return out
}

func inList(list []ast.Stmt, n ast.Node) bool {
	for _, s := range list {
		if s == n {
			return true
		}
	}
	return false
}

// guardsBefore returns what the statements of a block before child leave established: an if
// without else whose body always leaves (return, panic, continue, break, goto) passes only under
// the negation of its condition.
func guardsBefore(info *types.Info, list []ast.Stmt, child ast.Node) []string {
	var out []string
	for _, s := range list {
		if s == child {
			return out
		}
		ifs, ok := s.(*ast.IfStmt)
		if !ok || ifs.Else != nil || !leaves(info, ifs.Body) {
			continue
		}
		out = append(out, conditions(info, ifs.Cond, true)...)
	}
	return nil // child is not a statement of this list
}

// leaves reports whether a block ends in a statement that leaves it.
func leaves(info *types.Info, b *ast.BlockStmt) bool {
	if len(b.List) == 0 {
		return false
	}
	switch s := b.List[len(b.List)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return s.Tok != token.FALLTHROUGH
	case *ast.ExprStmt:
		call, ok := ast.Unparen(s.X).(*ast.CallExpr)
		if !ok {
			return false
		}
		id, ok := ast.Unparen(call.Fun).(*ast.Ident)
		if !ok {
			return false
		}
		b, ok := info.Uses[id].(*types.Builtin)
		return ok && b.Name() == "panic"
	}
	return false
}

// conditions returns the nameable conditions that hold when cond is true, or, with negated, when
// cond is false. True splits on &&; false splits on ||; ! flips.
func conditions(info *types.Info, cond ast.Expr, negated bool) []string {
	cond = ast.Unparen(cond)
	switch e := cond.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return conditions(info, e.X, !negated)
		}
	case *ast.BinaryExpr:
		if (e.Op == token.LAND && !negated) || (e.Op == token.LOR && negated) {
			return append(conditions(info, e.X, negated), conditions(info, e.Y, negated)...)
		}
		if (e.Op == token.EQL && !negated) || (e.Op == token.NEQ && negated) {
			if s := equalsSentinel(info, e.X, e.Y); s != "" {
				return []string{s}
			}
		}
		return nil
	}
	if negated {
		return nil
	}
	if s := predicate(info, cond); s != "" {
		return []string{s}
	}
	return nil
}

// errorPredicates are the error predicates whose name says the condition.
var errorPredicates = map[string]bool{
	"os.IsNotExist": true, "os.IsExist": true, "os.IsPermission": true, "os.IsTimeout": true,
}

// isFuncs are the errors.Is functions: the standard one and its drop-in replacements.
var isFuncs = map[string]bool{
	"errors.Is": true, "github.com/pkg/errors.Is": true, "golang.org/x/xerrors.Is": true,
	"github.com/cockroachdb/errors.Is": true,
}

// predicate names a call that tests an error: errors.Is(x, pkg.Sentinel) or os.IsNotExist(x).
func predicate(info *types.Info, e ast.Expr) string {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return ""
	}
	fn := facts.Callee(info, call)
	if fn == nil || fn.Pkg() == nil {
		return ""
	}
	full := fn.Pkg().Path() + "." + fn.Name()
	if errorPredicates[full] {
		return full
	}
	if isFuncs[full] && len(call.Args) == 2 {
		if s := sentinel(info, call.Args[1]); s != "" {
			return "error is " + s
		}
	}
	return ""
}

// equalsSentinel names x == pkg.Sentinel, with an error-typed other side.
func equalsSentinel(info *types.Info, x, y ast.Expr) string {
	for _, p := range [][2]ast.Expr{{x, y}, {y, x}} {
		if s := sentinel(info, p[1]); s != "" && facts.IsError(info.TypeOf(p[0])) {
			return "error is " + s
		}
	}
	return ""
}

// sentinel names a package-level variable whose type implements error: fs.ErrNotExist,
// context.Canceled, io.EOF. Anything else (a local, a call, a field) is "".
func sentinel(info *types.Info, e ast.Expr) string {
	var id *ast.Ident
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
	default:
		return ""
	}
	v, ok := info.Uses[id].(*types.Var)
	if !ok || v.Pkg() == nil || v.Parent() != v.Pkg().Scope() {
		return ""
	}
	if !types.Implements(v.Type(), errorIface) && !types.Implements(types.NewPointer(v.Type()), errorIface) {
		return ""
	}
	return v.Pkg().Name() + "." + v.Name()
}

var errorIface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

// caseConditions names the conditions a switch case establishes: in a switch without a tag a case
// is a condition; in a switch on an error value a case is a comparison with it. A case of several
// expressions holds any one of them, so it names nothing unless it has one.
func caseConditions(info *types.Info, cc *ast.CaseClause, outer []ast.Node) []string {
	if len(cc.List) != 1 || len(outer) < 2 {
		return nil
	}
	sw, ok := outer[len(outer)-2].(*ast.SwitchStmt)
	if !ok {
		return nil
	}
	if sw.Tag == nil {
		return conditions(info, cc.List[0], false)
	}
	if s := equalsSentinel(info, sw.Tag, cc.List[0]); s != "" {
		return []string{s}
	}
	return nil
}

// commCondition names what a select case receives: the Done channel of a context.Context, or a
// signal from a channel of os.Signal.
func commCondition(info *types.Info, comm ast.Stmt) string {
	var x ast.Expr
	switch s := comm.(type) {
	case *ast.ExprStmt:
		x = s.X
	case *ast.AssignStmt:
		if len(s.Rhs) == 1 {
			x = s.Rhs[0]
		}
	}
	u, ok := ast.Unparen(x).(*ast.UnaryExpr)
	if !ok || u.Op != token.ARROW {
		return ""
	}
	ch := ast.Unparen(u.X)
	if call, ok := ch.(*ast.CallExpr); ok {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok && sel.Sel.Name == "Done" {
			if isContext(info.TypeOf(sel.X)) {
				return "context done"
			}
		}
	}
	if c, ok := types.Unalias(info.TypeOf(ch)).Underlying().(*types.Chan); ok && isSignal(c.Elem()) {
		return "signal received"
	}
	return ""
}

// isContext reports whether t has the methods of context.Context; the method set decides, so a
// type that embeds or implements it counts.
func isContext(t types.Type) bool {
	if t == nil {
		return false
	}
	ms := types.NewMethodSet(t)
	for _, name := range []string{"Deadline", "Done", "Err", "Value"} {
		if lookupAny(ms, name) == nil {
			return false
		}
	}
	return true
}

func lookupAny(ms *types.MethodSet, name string) *types.Selection {
	for i := 0; i < ms.Len(); i++ {
		if ms.At(i).Obj().Name() == name {
			return ms.At(i)
		}
	}
	return nil
}

// isSignal reports whether t is os.Signal or implements it: Signal() and String() string. The
// method set decides, not the name, so syscall.Signal counts and a type named Signal does not.
func isSignal(t types.Type) bool {
	if t == nil {
		return false
	}
	ms := types.NewMethodSet(t)
	sig, str := lookupAny(ms, "Signal"), lookupAny(ms, "String")
	if sig == nil || str == nil {
		return false
	}
	s1 := sig.Obj().Type().(*types.Signature)
	s2 := str.Obj().Type().(*types.Signature)
	if s1.Params().Len() != 0 || s1.Results().Len() != 0 || s2.Params().Len() != 0 || s2.Results().Len() != 1 {
		return false
	}
	b, ok := s2.Results().At(0).Type().(*types.Basic)
	return ok && b.Kind() == types.String
}

// takesSignal reports whether a call takes a signal among its arguments: trap.Bind(syscall.SIGINT,
// func() {...}). A function literal passed to it is a signal handler.
func takesSignal(info *types.Info, call *ast.CallExpr) bool {
	for _, a := range call.Args {
		if isSignal(info.TypeOf(a)) {
			return true
		}
	}
	return false
}

// joinFacts renders the branch facts as one fact.
func joinFacts(fs []string) string { return strings.Join(fs, "; ") }
