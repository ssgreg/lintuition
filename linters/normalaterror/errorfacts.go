package normalaterror

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/ssgreg/lintuition/internal/facts"
)

var errorIface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

// attachedErrors returns the expressions a log call logs whose type implements error: a field value
// (zap.Error(err), slog's "err", err, logf.Error(err)), an argument after a printf format or a
// print message, or an argument of a call of the same logger package the log call is made on
// (logrus WithError(err), zerolog Err(err)). An untyped nil is not one.
func attachedErrors(info *types.Info, lc facts.LogCall) []ast.Expr {
	var out []ast.Expr
	seen := map[ast.Expr]bool{}
	add := func(e ast.Expr) {
		e = ast.Unparen(e)
		if seen[e] || !implementsError(info, e) {
			return
		}
		seen[e] = true
		out = append(out, e)
	}
	fields, _ := facts.Fields(info, lc)
	for _, f := range fields {
		add(f.Value)
	}
	if lc.MessageArg >= 0 {
		for _, a := range lc.Call.Args[lc.MessageArg+1:] {
			add(a)
		}
	}
	for x := lc.Call.Fun; ; {
		sel, ok := ast.Unparen(x).(*ast.SelectorExpr)
		if !ok {
			break
		}
		inner, ok := ast.Unparen(sel.X).(*ast.CallExpr)
		if !ok {
			break
		}
		if fn := facts.Callee(info, inner); fn != nil && fn.Pkg() == lc.Func.Pkg() {
			for _, a := range inner.Args {
				add(a)
			}
		}
		x = inner.Fun
	}
	return out
}

func implementsError(info *types.Info, e ast.Expr) bool {
	tv, ok := info.Types[e]
	if !ok || tv.Type == nil || tv.IsNil() {
		return false
	}
	return types.Implements(tv.Type, errorIface)
}

// checkedVar returns the variable every attached error is, when they are all the same local
// variable of an interface type that the code can only change by assigning it: not a field, not a
// package-level variable, its address never taken and never assigned by a function literal that
// captures it, anywhere in body, the outermost function, and body has no goto. Otherwise nil: what
// was checked about it cannot be tied to the value that is logged.
func checkedVar(info *types.Info, errs []ast.Expr, body ast.Node) *types.Var {
	var v *types.Var
	for _, e := range errs {
		id, ok := e.(*ast.Ident)
		if !ok {
			return nil
		}
		w, ok := info.Uses[id].(*types.Var)
		if !ok || (v != nil && w != v) {
			return nil
		}
		v = w
	}
	if v == nil || v.IsField() || v.Pkg() == nil || v.Parent() == nil || v.Parent() == v.Pkg().Scope() || !types.IsInterface(v.Type()) || body == nil {
		return nil
	}
	untracked := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.UnaryExpr:
			if n.Op == token.AND && isVar(info, n.X, v) {
				untracked = true
			}
		case *ast.FuncLit:
			// A literal that declares v is where it lives; one that captures it and assigns it can
			// change it whenever it is called.
			declares := n.Pos() <= v.Pos() && v.Pos() < n.End()
			if !declares && assigns(info, v, n.Body) {
				untracked = true
			}
		case *ast.BranchStmt:
			// A goto can jump past a guard within a block.
			if n.Tok == token.GOTO {
				untracked = true
			}
		}
		return !untracked
	})
	if untracked {
		return nil
	}
	return v
}

func isVar(info *types.Info, e ast.Expr, v *types.Var) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	return info.Uses[id] == v || info.Defs[id] == v
}

// assigns reports whether v is assigned anywhere in the nodes: =, :=, op=, a range clause that
// assigns it.
func assigns(info *types.Info, v *types.Var, nodes ...ast.Node) bool {
	found := false
	for _, n := range nodes {
		ast.Inspect(n, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				for _, l := range n.Lhs {
					if isVar(info, l, v) {
						found = true
					}
				}
			case *ast.RangeStmt:
				if n.Tok == token.ASSIGN && (isVar(info, n.Key, v) || isVar(info, n.Value, v)) {
					found = true
				}
			}
			return !found
		})
	}
	return found
}

// errorChecks names, nearest first, what the code has checked about v on the way to the log call
// at the end of stack, within the function the call is in:
//
//   - an if condition holds in its body and its negation in its else: err != nil gives "not nil",
//     errors.Is(err, context.Canceled) "is context.Canceled", !errors.Is(...) "is not
//     context.Canceled", err == io.EOF "is io.EOF", os.IsNotExist(err) "os.IsNotExist";
//   - an earlier if of the same block, without else, whose body always leaves (return, panic,
//     break, continue) leaves the rest of the block under its negation;
//   - a case of a switch without a tag holds its single condition, and a case of a switch on v
//     its single value, unless the case before it falls through (empty statements after the
//     fallthrough do not hide it).
//
// && splits a condition that holds, || one that does not. A check counts only if nothing in the
// region it covers assigns v; the first region that does ends the list, since what was checked
// outside it may no longer hold. A check other than nil / not nil also depends on the error's
// state and the compared variable, so it is dropped when the region writes a field, an element,
// a pointer target or a package-level variable (w.cause = x, ErrStopped = io.EOF). A call that
// changes that state is not seen: the checks are what the code tested on the way to the log, and
// the fact is described that way. The walk stops at the enclosing function: a condition around a
// function literal says nothing about when the literal runs.
func errorChecks(info *types.Info, v *types.Var, stack []ast.Node) []string {
	var out []string
	seen := map[string]bool{}
	add := func(cs []string) {
		for _, c := range cs {
			if !seen[c] && facts.FactSafe(c) {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	for i := len(stack) - 1; i > 0; i-- {
		child, parent := stack[i], stack[i-1]
		switch p := parent.(type) {
		case *ast.FuncLit, *ast.FuncDecl:
			return out
		case *ast.BlockStmt:
			cs, ok := guards(info, v, p.List, child)
			add(cs)
			if !ok {
				return out
			}
		case *ast.CommClause:
			cs, ok := guards(info, v, p.Body, child)
			add(cs)
			if !ok {
				return out
			}
		case *ast.CaseClause:
			cs, ok := guards(info, v, p.Body, child)
			add(cs)
			if !ok {
				return out
			}
			if !inList(p.Body, child) {
				continue
			}
			if assigns(info, v, p) {
				return out
			}
			if i >= 3 {
				cs := caseChecks(info, v, p, stack[i-2], stack[i-3])
				if writesShared(info, p) {
					cs = bindingOnly(cs)
				}
				add(cs)
			}
		case *ast.IfStmt:
			var cs []string
			switch child {
			case p.Body:
				cs = conditions(info, v, p.Cond, false)
			case p.Else:
				cs = conditions(info, v, p.Cond, true)
			default:
				continue
			}
			if assigns(info, v, child) {
				return out
			}
			if writesShared(info, child) {
				cs = bindingOnly(cs)
			}
			add(cs)
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

// guards returns what the statements of a list before child leave established about v, nearest
// first: an if without else whose body always leaves passes only under the negation of its
// condition, if nothing from it up to child assigns v. ok is false when an assignment ends the
// walk.
func guards(info *types.Info, v *types.Var, list []ast.Stmt, child ast.Node) (out []string, ok bool) {
	at := -1
	for i, s := range list {
		if s == child {
			at = i
		}
	}
	if at < 0 {
		return nil, true
	}
	for g := at - 1; g >= 0; g-- {
		ifs, isIf := list[g].(*ast.IfStmt)
		if !isIf || ifs.Else != nil || !leaves(info, ifs.Body) {
			continue
		}
		region := make([]ast.Node, 0, at-g)
		for _, s := range list[g+1 : at+1] {
			region = append(region, s)
		}
		if assigns(info, v, region...) {
			return out, false
		}
		cs := conditions(info, v, ifs.Cond, true)
		if writesShared(info, region...) {
			cs = bindingOnly(cs)
		}
		out = append(out, cs...)
	}
	return out, true
}

// leaves reports whether a block ends in a statement that leaves it for good: return, panic,
// break or continue. A goto can jump back into the same block, so it does not count.
func leaves(info *types.Info, b *ast.BlockStmt) bool {
	if len(b.List) == 0 {
		return false
	}
	switch s := b.List[len(b.List)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return s.Tok == token.BREAK || s.Tok == token.CONTINUE
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

// conditions returns the checks of v that hold when cond is true, or, with negated, when it is
// false.
func conditions(info *types.Info, v *types.Var, cond ast.Expr, negated bool) []string {
	cond = ast.Unparen(cond)
	switch e := cond.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return conditions(info, v, e.X, !negated)
		}
		return nil
	case *ast.BinaryExpr:
		if (e.Op == token.LAND && !negated) || (e.Op == token.LOR && negated) {
			return append(conditions(info, v, e.X, negated), conditions(info, v, e.Y, negated)...)
		}
		if e.Op == token.EQL || e.Op == token.NEQ {
			equal := (e.Op == token.EQL) != negated
			if s := comparison(info, v, e.X, e.Y); s != "" {
				if equal {
					return []string{s}
				}
				return []string{negate(s)}
			}
		}
		return nil
	}
	if s := predicate(info, v, cond); s != "" {
		if negated {
			return []string{negate(s)}
		}
		return []string{s}
	}
	return nil
}

// negate turns a check into its negation: "nil" into "not nil", "is io.EOF" into "is not io.EOF",
// "os.IsNotExist" into "not os.IsNotExist".
func negate(s string) string {
	if len(s) > 3 && s[:3] == "is " {
		return "is not " + s[3:]
	}
	return "not " + s
}

// comparison names v == nil as "nil" and v == pkg.Sentinel as "is pkg.Sentinel".
func comparison(info *types.Info, v *types.Var, x, y ast.Expr) string {
	for _, p := range [][2]ast.Expr{{x, y}, {y, x}} {
		if !isVar(info, p[0], v) {
			continue
		}
		if tv, ok := info.Types[ast.Unparen(p[1])]; ok && tv.IsNil() {
			return "nil"
		}
		if s := sentinel(info, p[1]); s != "" {
			return "is " + s
		}
	}
	return ""
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

// predicate names a call that tests v: errors.Is(v, pkg.Sentinel) as "is pkg.Sentinel",
// os.IsNotExist(v) as "os.IsNotExist".
func predicate(info *types.Info, v *types.Var, e ast.Expr) string {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 || !isVar(info, call.Args[0], v) {
		return ""
	}
	fn := facts.Callee(info, call)
	if fn == nil || fn.Pkg() == nil {
		return ""
	}
	full := fn.Pkg().Path() + "." + fn.Name()
	if errorPredicates[full] && len(call.Args) == 1 {
		return full
	}
	if isFuncs[full] && len(call.Args) == 2 {
		if s := sentinel(info, call.Args[1]); s != "" {
			return "is " + s
		}
	}
	return ""
}

// sentinel names a package-level variable whose type implements error: context.Canceled,
// net.ErrClosed, io.EOF. Anything else (a local, a call, a field) is "".
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

// caseChecks names what a switch case establishes about v: in a switch without a tag a case of one
// expression is a condition; in a switch on v a case of one value is a comparison with it. A case
// the one before falls through to establishes nothing.
func caseChecks(info *types.Info, v *types.Var, cc *ast.CaseClause, body, sw ast.Node) []string {
	s, ok := sw.(*ast.SwitchStmt)
	if !ok || body != s.Body || len(cc.List) != 1 {
		return nil
	}
	for i, c := range s.Body.List {
		if c != cc || i == 0 {
			continue
		}
		if b, ok := lastStmt(s.Body.List[i-1].(*ast.CaseClause).Body).(*ast.BranchStmt); ok && b.Tok == token.FALLTHROUGH {
			return nil
		}
	}
	if s.Tag == nil {
		return conditions(info, v, cc.List[0], false)
	}
	if !isVar(info, s.Tag, v) {
		return nil
	}
	if c := comparison(info, v, s.Tag, cc.List[0]); c != "" {
		return []string{c}
	}
	return nil
}

// lastStmt returns the last statement of a list that is not empty: "fallthrough; ;" ends in
// fallthrough.
func lastStmt(list []ast.Stmt) ast.Stmt {
	for i := len(list) - 1; i >= 0; i-- {
		if _, empty := list[i].(*ast.EmptyStmt); !empty {
			return list[i]
		}
	}
	return nil
}

// bindingOnly keeps the checks that depend only on which value v holds: "nil" and "not nil". The
// others (errors.Is, ==, os.IsNotExist) also depend on the state the error reaches and on the
// variable it was compared with, which a write in the region can change.
func bindingOnly(checks []string) []string {
	var out []string
	for _, c := range checks {
		if c == "nil" || c == "not nil" {
			out = append(out, c)
		}
	}
	return out
}

// writesShared reports whether the nodes write anything other than a local variable: a field
// (w.cause = x), an element, a pointer target, a package-level variable (ErrStopped = io.EOF).
// Such a write can change what errors.Is or == would answer for the logged error, so a check of
// that kind does not survive it. A call that writes the same state is not seen; the check is then
// what the code tested on the way to the log, which is what the fact says.
func writesShared(info *types.Info, nodes ...ast.Node) bool {
	found := false
	shared := func(e ast.Expr) bool {
		id, ok := ast.Unparen(e).(*ast.Ident)
		if !ok {
			return true
		}
		if id.Name == "_" {
			return false
		}
		obj := info.Uses[id]
		if obj == nil {
			obj = info.Defs[id]
		}
		v, ok := obj.(*types.Var)
		return !ok || v.Pkg() == nil || v.Parent() == v.Pkg().Scope()
	}
	for _, n := range nodes {
		ast.Inspect(n, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				if n.Tok != token.DEFINE {
					for _, l := range n.Lhs {
						if shared(l) {
							found = true
						}
					}
				}
			case *ast.IncDecStmt:
				if shared(n.X) {
					found = true
				}
			case *ast.RangeStmt:
				if n.Tok == token.ASSIGN && ((n.Key != nil && shared(n.Key)) || (n.Value != nil && shared(n.Value))) {
					found = true
				}
			}
			return !found
		})
	}
	return found
}
