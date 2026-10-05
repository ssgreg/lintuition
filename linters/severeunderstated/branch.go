package severeunderstated

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/ssgreg/lintuition/internal/facts"
)

// branchFacts lists, nearest first, conditions the code checked on the way to the log call, read
// from the enclosing statements of the same function through go/types:
//
//   - "error is fs.ErrNotExist": an if or case condition errors.Is(x, fs.ErrNotExist) or
//     x == io.EOF, with a package-level error variable as the target, holds in its body;
//   - "os.IsNotExist": the same for the os.IsNotExist, os.IsExist, os.IsPermission and
//     os.IsTimeout predicates;
//   - the same conditions when an earlier statement of the block left it under their negation
//     with return, panic, break or continue: if !os.IsNotExist(err) { return err } leaves the rest
//     of the block on os.IsNotExist;
//   - "context done": a select case receiving from the Done channel of a context.Context;
//   - "receive from a channel of os.Signal": a select case that receives a single value from such
//     a channel;
//   - "callback passed along with a signal value": the log is in a function literal passed to a
//     call that also takes a signal value. This describes the call's shape only; nothing says the
//     call registers a handler or that a signal arrived.
//
// An error condition is kept only while it still describes the value it checked. The checked
// place is a local variable, a field path below one (p.first) or ctx.Err() on a context; an index
// or another method's result has no place and checks nothing nameable. The variable must not be
// written between the check and the log, nor anywhere in a loop around the log that does not run
// the check again (the write reaches the next pass), nor in a function literal or through its
// address; it must not be shadowed at the log, the function must have no goto, and the log call
// must log no value whose type implements error but that same place. Only conditions that hold positively count: the else
// branch of errors.Is(err, X), or a condition under !, names nothing. The walk stops at a
// function literal, which may run later with other values. A case that the previous case falls
// through into (fallthrough as its last non-empty statement), and code after a label, establish
// nothing.
//
// The facts say what the code checked, not why: a not-exist check does not prove that the file was
// optional, nor a cancellation check that the stop was requested. The classifier weighs that.
func branchFacts(pkg *types.Package, info *types.Info, stack []ast.Node, call *ast.CallExpr) []string {
	var out []string
	seen := map[string]bool{}
	body := enclosingBody(stack)
	add := func(cs []cond) {
		for _, c := range cs {
			if c.operand != nil && !stillChecked(pkg, info, body, stack, c, call) {
				continue
			}
			if c.text != "" && !seen[c.text] && facts.FactSafe(c.text) {
				seen[c.text] = true
				out = append(out, c.text)
			}
		}
	}
	for i := len(stack) - 1; i > 0; i-- {
		child, parent := stack[i], stack[i-1]
		if _, ok := child.(*ast.FuncLit); ok {
			if p, ok := parent.(*ast.CallExpr); ok && takesSignal(info, p) {
				add([]cond{{text: "callback passed along with a signal value"}})
			}
			break
		}
		switch p := parent.(type) {
		case *ast.BlockStmt:
			add(guardsBefore(info, p.List, child))
		case *ast.CaseClause:
			add(guardsBefore(info, p.Body, child))
			if inList(p.Body, child) {
				add(caseConditions(info, p, stack[:i-1]))
			}
		case *ast.CommClause:
			add(guardsBefore(info, p.Body, child))
			if inList(p.Body, child) {
				if s := commCondition(info, p.Comm); s != "" {
					add([]cond{{text: s}})
				}
			}
		case *ast.IfStmt:
			switch child {
			case p.Body:
				add(conditions(info, p.Cond, false))
			case p.Else:
				add(conditions(info, p.Cond, true))
			}
		}
	}
	return out
}

// cond is one nameable condition, the local variable whose value it checked (nil for the select
// and callback facts, which check no value) and where the check ends.
type cond struct {
	text    string
	operand *types.Var
	// path is the checked place below the variable: "" for err itself, ".First" for p.First,
	// ".Err()" for ctx.Err().
	path string
	at   token.Pos
}

// enclosingBody is the body of the innermost function on the stack.
func enclosingBody(stack []ast.Node) *ast.BlockStmt {
	for i := len(stack) - 1; i >= 0; i-- {
		switch f := stack[i].(type) {
		case *ast.FuncLit:
			return f.Body
		case *ast.FuncDecl:
			return f.Body
		}
	}
	return nil
}

// stillChecked reports whether the variable a condition checked still holds the checked value at
// the log call, as far as the code shows, and is the only error value the call logs.
func stillChecked(pkg *types.Package, info *types.Info, body *ast.BlockStmt, stack []ast.Node, c cond, call *ast.CallExpr) bool {
	v := c.operand
	if body == nil || v.Pkg() == nil || v.Parent() == nil || v.Parent() == v.Pkg().Scope() {
		return false
	}
	if pkg != nil {
		if s := pkg.Scope().Innermost(call.Pos()); s != nil {
			if _, o := s.LookupParent(v.Name(), call.Pos()); o != v {
				return false // shadowed: the name means another variable at the log
			}
		}
	}
	ok := true
	// A loop around the log that does not also run the check again carries any write in it, before
	// or after the log, into the next pass: the log then runs with the written value.
	for _, n := range stack {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			if n.Pos() > c.at && n.Pos() < call.Pos() {
				ast.Inspect(n, func(m ast.Node) bool {
					if writes(info, m, v) {
						ok = false
					}
					return ok
				})
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		if !ok {
			return false
		}
		// A goto can make a loop the walk does not see, so its function keeps no error test.
		if b, isB := n.(*ast.BranchStmt); isB && b.Tok == token.GOTO {
			ok = false
		}
		if lit, isLit := n.(*ast.FuncLit); isLit {
			ast.Inspect(lit.Body, func(m ast.Node) bool {
				if writes(info, m, v) {
					ok = false
				}
				return ok
			})
			return false
		}
		if u, isU := n.(*ast.UnaryExpr); isU && u.Op == token.AND && rootVar(info, u.X) == v {
			ok = false // address-taken: written through a pointer anywhere
		}
		if writes(info, n, v) && n.Pos() > c.at && n.Pos() < call.Pos() {
			ok = false
		}
		return ok
	})
	if !ok {
		return false
	}
	for _, a := range call.Args {
		ast.Inspect(a, func(n ast.Node) bool {
			e, isExpr := n.(ast.Expr)
			if !isExpr || !ok {
				return ok
			}
			if tv, isTV := info.Types[e]; isTV && tv.IsValue() && implementsError(tv.Type) {
				if pv, path, known := place(info, e); !known || pv != v || path != c.path {
					ok = false // the line logs another error value, or one it cannot tell apart
				}
				return false
			}
			return true
		})
	}
	return ok
}

// implementsError reports whether a value of type t is an error value: t implements error, or *t
// does (a method with a pointer receiver), so customError, *customError and a named interface with
// Error() string count, not only the error type itself. This is for telling which values a log
// line logs; the signature checks elsewhere stay exact.
func implementsError(t types.Type) bool {
	if t == nil || t == types.Typ[types.Invalid] {
		return false
	}
	if types.Implements(t, errorIface) {
		return true
	}
	if _, isPtr := t.Underlying().(*types.Pointer); isPtr || types.IsInterface(t) {
		return false
	}
	return types.Implements(types.NewPointer(t), errorIface)
}

// writes reports whether a node assigns to v or to something rooted in it.
func writes(info *types.Info, n ast.Node, v *types.Var) bool {
	switch n := n.(type) {
	case *ast.AssignStmt:
		for _, l := range n.Lhs {
			if rootVar(info, l) == v {
				return true
			}
		}
	case *ast.IncDecStmt:
		return rootVar(info, n.X) == v
	case *ast.RangeStmt:
		return (n.Key != nil && rootVar(info, n.Key) == v) || (n.Value != nil && rootVar(info, n.Value) == v)
	case *ast.UnaryExpr:
		return n.Op == token.AND && rootVar(info, n.X) == v
	}
	return false
}

// rootVar is the variable an expression is rooted in: err, ctx in ctx.Err(), r in r.err. Nil
// when the root is anything else (a function call, a literal, a package-qualified name).
func rootVar(info *types.Info, e ast.Expr) *types.Var {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		v, _ := info.ObjectOf(e).(*types.Var)
		return v
	case *ast.SelectorExpr:
		if s, ok := info.Selections[e]; ok && s.Kind() == types.FieldVal {
			return rootVar(info, e.X)
		}
	case *ast.CallExpr:
		if sel, ok := ast.Unparen(e.Fun).(*ast.SelectorExpr); ok {
			if s, ok := info.Selections[sel]; ok && s.Kind() == types.MethodVal {
				return rootVar(info, sel.X)
			}
		}
	case *ast.StarExpr:
		return rootVar(info, e.X)
	case *ast.IndexExpr:
		return rootVar(info, e.X)
	}
	return nil
}

// place names the checked place an error expression reads, by the variable it starts from and
// the path below it: err, p.First (field selections only) and ctx.Err() on a context. Anything
// else, an index, another method's result, a function's, has no place: two of them on one
// variable can be different errors.
func place(info *types.Info, e ast.Expr) (*types.Var, string, bool) {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		v, ok := info.ObjectOf(e).(*types.Var)
		return v, "", ok
	case *ast.SelectorExpr:
		if s, ok := info.Selections[e]; ok && s.Kind() == types.FieldVal {
			if v, p, ok := place(info, e.X); ok {
				return v, p + "." + e.Sel.Name, true
			}
		}
	case *ast.CallExpr:
		sel, ok := ast.Unparen(e.Fun).(*ast.SelectorExpr)
		if ok && len(e.Args) == 0 && sel.Sel.Name == "Err" && isContext(info.TypeOf(sel.X)) {
			if v, p, ok := place(info, sel.X); ok {
				return v, p + ".Err()", true
			}
		}
	}
	return nil, "", false
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
// without else whose body always leaves (return, panic, break, continue) passes only under the
// negation of its condition. A label resets it, since a goto can reach the code after a label
// from elsewhere, and a goto in a guard is not taken to leave.
func guardsBefore(info *types.Info, list []ast.Stmt, child ast.Node) []cond {
	var out []cond
	for _, s := range list {
		if _, ok := s.(*ast.LabeledStmt); ok {
			out = nil
		}
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

// conditions returns the nameable conditions that hold when c is true, or, with negated, when c
// is false. True splits on &&; false splits on ||; ! flips.
func conditions(info *types.Info, c ast.Expr, negated bool) []cond {
	c = ast.Unparen(c)
	switch e := c.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return conditions(info, e.X, !negated)
		}
	case *ast.BinaryExpr:
		if (e.Op == token.LAND && !negated) || (e.Op == token.LOR && negated) {
			return append(conditions(info, e.X, negated), conditions(info, e.Y, negated)...)
		}
		if (e.Op == token.EQL && !negated) || (e.Op == token.NEQ && negated) {
			if c, ok := equalsSentinel(info, e.X, e.Y); ok {
				return []cond{c}
			}
		}
		return nil
	}
	if negated {
		return nil
	}
	if c, ok := predicate(info, c); ok {
		return []cond{c}
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

// predicate names a call that tests an error rooted in a variable: errors.Is(err, pkg.Sentinel)
// or os.IsNotExist(err).
func predicate(info *types.Info, e ast.Expr) (cond, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return cond{}, false
	}
	fn := facts.Callee(info, call)
	if fn == nil || fn.Pkg() == nil {
		return cond{}, false
	}
	full := fn.Pkg().Path() + "." + fn.Name()
	v, path, ok := place(info, call.Args[0])
	if !ok {
		return cond{}, false
	}
	if errorPredicates[full] {
		return cond{text: full, operand: v, path: path, at: call.End()}, true
	}
	if isFuncs[full] && len(call.Args) == 2 {
		if s := sentinel(info, call.Args[1]); s != "" {
			return cond{text: "error is " + s, operand: v, path: path, at: call.End()}, true
		}
	}
	return cond{}, false
}

// equalsSentinel names x == pkg.Sentinel, with an error-typed other side rooted in a variable.
func equalsSentinel(info *types.Info, x, y ast.Expr) (cond, bool) {
	for _, p := range [][2]ast.Expr{{x, y}, {y, x}} {
		if s := sentinel(info, p[1]); s != "" && facts.IsError(info.TypeOf(p[0])) {
			if v, path, ok := place(info, p[0]); ok {
				return cond{text: "error is " + s, operand: v, path: path, at: p[0].End()}, true
			}
		}
	}
	return cond{}, false
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
// expressions holds any one of them, and a case the previous one falls through into holds
// nothing, so neither names anything.
func caseConditions(info *types.Info, cc *ast.CaseClause, outer []ast.Node) []cond {
	if len(cc.List) != 1 || len(outer) < 2 {
		return nil
	}
	sw, ok := outer[len(outer)-2].(*ast.SwitchStmt)
	if !ok {
		return nil
	}
	for i, s := range sw.Body.List {
		if s != cc || i == 0 {
			continue
		}
		if prev, ok := sw.Body.List[i-1].(*ast.CaseClause); ok {
			// fallthrough is the last statement of the case, but an empty statement may follow it
			// in source gofmt has not touched: `fallthrough; ;`.
			for j := len(prev.Body) - 1; j >= 0; j-- {
				if _, empty := prev.Body[j].(*ast.EmptyStmt); empty {
					continue
				}
				if b, ok := prev.Body[j].(*ast.BranchStmt); ok && b.Tok == token.FALLTHROUGH {
					return nil
				}
				break
			}
		}
	}
	if sw.Tag == nil {
		return conditions(info, cc.List[0], false)
	}
	if c, ok := equalsSentinel(info, sw.Tag, cc.List[0]); ok {
		return []cond{c}
	}
	return nil
}

// commCondition names what a select case receives: the Done channel of a context.Context, or a
// single value from a channel of os.Signal. A two-value receive (v, ok := <-ch) may report a
// closed channel, so it names nothing.
func commCondition(info *types.Info, comm ast.Stmt) string {
	var x ast.Expr
	switch s := comm.(type) {
	case *ast.ExprStmt:
		x = s.X
	case *ast.AssignStmt:
		if len(s.Rhs) == 1 && len(s.Lhs) == 1 {
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
		return "receive from a channel of os.Signal"
	}
	return ""
}

// method returns the signature of the named method of t, or of *t when t is neither a pointer
// nor an interface (an addressable value calls pointer methods too); nil when there is none.
func method(t types.Type, name string) *types.Signature {
	sets := []*types.MethodSet{types.NewMethodSet(t)}
	if _, isPtr := t.Underlying().(*types.Pointer); !isPtr && !types.IsInterface(t) {
		sets = append(sets, types.NewMethodSet(types.NewPointer(t)))
	}
	for _, ms := range sets {
		for i := 0; i < ms.Len(); i++ {
			if ms.At(i).Obj().Name() == name {
				s, _ := ms.At(i).Obj().Type().(*types.Signature)
				return s
			}
		}
	}
	return nil
}

// isContext reports whether t implements context.Context, checked by the full signatures of its
// methods with exact types: Deadline() (time.Time, bool), Done() <-chan struct{}, Err() error,
// Value(any) any.
// context.Context itself, an alias of it and a type embedding it count; methods that only share
// the names do not.
func isContext(t types.Type) bool {
	if t == nil {
		return false
	}
	d, done, e, v := method(t, "Deadline"), method(t, "Done"), method(t, "Err"), method(t, "Value")
	if d == nil || done == nil || e == nil || v == nil {
		return false
	}
	if d.Params().Len() != 0 || d.Results().Len() != 2 || !isNamed(d.Results().At(0).Type(), "time", "Time") || !isBasic(d.Results().At(1).Type(), types.Bool) {
		return false
	}
	if done.Params().Len() != 0 || done.Results().Len() != 1 {
		return false
	}
	ch, ok := types.Unalias(done.Results().At(0).Type()).(*types.Chan)
	if !ok || ch.Dir() != types.RecvOnly {
		return false
	}
	if st, ok := types.Unalias(ch.Elem()).(*types.Struct); !ok || st.NumFields() != 0 {
		return false
	}
	if e.Params().Len() != 0 || e.Results().Len() != 1 || !facts.IsError(e.Results().At(0).Type()) {
		return false
	}
	return v.Params().Len() == 1 && v.Results().Len() == 1 && !v.Variadic() &&
		isAny(v.Params().At(0).Type()) && isAny(v.Results().At(0).Type())
}

func isNamed(t types.Type, pkg, name string) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == pkg && n.Obj().Name() == name
}

func isBasic(t types.Type, k types.BasicKind) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Kind() == k
}

// isAny reports whether t is any, interface{}, or an alias of them. A defined type, even one
// whose underlying type is interface{}, is another type, and a method using it has another
// signature: it does not implement context.Context.
func isAny(t types.Type) bool {
	return types.Identical(types.Unalias(t), types.NewInterfaceType(nil, nil))
}

// isSignal reports whether t is os.Signal or implements it: Signal() and String() string. The
// method set decides, not the name, so syscall.Signal counts and a type named Signal does not.
func isSignal(t types.Type) bool {
	if t == nil {
		return false
	}
	ms := types.NewMethodSet(t)
	var s1, s2 *types.Signature
	for i := 0; i < ms.Len(); i++ {
		switch ms.At(i).Obj().Name() {
		case "Signal":
			s1, _ = ms.At(i).Obj().Type().(*types.Signature)
		case "String":
			s2, _ = ms.At(i).Obj().Type().(*types.Signature)
		}
	}
	if s1 == nil || s2 == nil {
		return false
	}
	if s1.Params().Len() != 0 || s1.Results().Len() != 0 || s2.Params().Len() != 0 || s2.Results().Len() != 1 {
		return false
	}
	return isBasic(s2.Results().At(0).Type(), types.String)
}

// takesSignal reports whether a call takes a signal value among its arguments.
func takesSignal(info *types.Info, call *ast.CallExpr) bool {
	for _, a := range call.Args {
		if isSignal(info.TypeOf(a)) {
			return true
		}
	}
	return false
}
