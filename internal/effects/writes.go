// Package effects reads what a function body does to state its caller can see. It is the shared
// effects model of the promise linters: a doc that says a function does not modify something is
// checked against the writes found here.
//
// The model is direct and conservative. It counts a write when the written place is reached from a
// receiver, a parameter or a package-level variable by a path the caller shares: a pointer
// dereference (explicit, or implicit in a field selection on a pointer), a map or slice element,
// the backing store of a map or slice the caller passed (delete, clear, copy into it), or a
// sync/atomic store. Assigning a field of a struct received by value, or taking the address of
// one, touches the function's own copy and is not a write; neither is assigning the parameter
// itself. Writes through a local alias (p := r; p.n = 1), through a callee, or by a method value
// stored in a variable are not followed.
//
// Some writes are reported with a qualifier instead of being dropped or trusted: one inside a
// function literal (when it runs is not known), one after its root was reassigned (the root may now
// hold fresh storage), one through a generic index whose constraint does not settle the shape, and
// one undone by a proven save and restore (old := s.result; ...; s.result = old, in the narrow shape
// restorations describes). A restored write
// still happened, and a callback could have seen it; the caller's state at return is unchanged.
package effects

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// RootKind says what a written path starts from.
type RootKind int

const (
	// Receiver is the method's receiver.
	Receiver RootKind = iota + 1
	// Param is a parameter of the function.
	Param
	// Global is a package-level variable.
	Global
)

// Write is one write to state the caller can see.
type Write struct {
	Pos  token.Pos
	Kind RootKind
	// Root is the name of the receiver, parameter or package-level variable.
	Root string
	// Path is the written place as written in the source (q.head, m[k], *p), for messages.
	Path string
	// InClosure is set for a write inside a function literal.
	InClosure bool
	// Uncertain, when set, says why the write may not reach the caller's state.
	Uncertain string
	// Restored is set for a write a proven save and restore undoes before the function returns.
	Restored bool

	root types.Object
}

// Direct reports a write that is certain, outside a closure, and not undone before return.
func (w Write) Direct() bool { return !w.InClosure && w.Uncertain == "" && !w.Restored }

// Writes returns the caller-visible writes of a function declaration's body, in source order.
func Writes(info *types.Info, fd *ast.FuncDecl) []Write {
	if fd.Body == nil {
		return nil
	}
	roots := map[types.Object]RootKind{}
	if fd.Recv != nil {
		for _, f := range fd.Recv.List {
			for _, n := range f.Names {
				if o := info.Defs[n]; o != nil {
					roots[o] = Receiver
				}
			}
		}
	}
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			if o := info.Defs[n]; o != nil {
				roots[o] = Param
			}
		}
	}
	w := &walker{info: info, roots: roots, rebound: map[types.Object]token.Pos{}}
	w.walk(fd.Body, false)
	for _, r := range restorations(info, fd.Body, w.rebound) {
		for i := range w.out {
			x := &w.out[i]
			if x.root == r.root && x.Path == r.path && x.Pos > r.from && x.Pos <= r.to && !x.InClosure {
				x.Restored = true
			}
		}
	}
	return w.out
}

type walker struct {
	info  *types.Info
	roots map[types.Object]RootKind
	out   []Write
	// rebound holds where a root variable was first assigned in the body: from there on it may hold
	// fresh storage, so later writes through it are uncertain.
	rebound map[types.Object]token.Pos
}

func (w *walker) walk(n ast.Node, inClosure bool) {
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			w.walk(n.Body, true)
			return false
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				// p, ok := new(int), true reuses p: a name without a new object is an assignment.
				for _, l := range n.Lhs {
					if id, ok := l.(*ast.Ident); ok && w.info.Defs[id] == nil {
						w.rebind(id)
					}
				}
			}
			if n.Tok != token.DEFINE {
				for _, l := range n.Lhs {
					w.place(l, inClosure, asLvalue)
				}
				for _, l := range n.Lhs {
					w.rebind(l)
				}
			}
		case *ast.IncDecStmt:
			w.place(n.X, inClosure, asLvalue)
			w.rebind(n.X)
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				for _, l := range []ast.Expr{n.Key, n.Value} {
					if l != nil {
						w.place(l, inClosure, asLvalue)
						w.rebind(l)
					}
				}
			}
		case *ast.CallExpr:
			w.call(n, inClosure)
		}
		return true
	})
}

// rebind records an assignment to a root variable itself (p = new(int), xs = append(...)).
func (w *walker) rebind(e ast.Expr) {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return
	}
	o := w.info.Uses[id]
	if _, isRoot := w.roots[o]; isRoot {
		if _, seen := w.rebound[o]; !seen {
			w.rebound[o] = id.End()
		}
	}
}

// call records the writes of builtins and sync/atomic: delete and clear of a map or slice, copy
// into a slice, and an atomic store, add, swap or compare-and-swap, whether through a package
// function, a bound method or a method expression.
func (w *walker) call(c *ast.CallExpr, inClosure bool) {
	switch fun := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		b, ok := w.info.Uses[fun].(*types.Builtin)
		if !ok || len(c.Args) == 0 {
			return
		}
		switch b.Name() {
		case "delete", "clear", "copy":
			w.place(c.Args[0], inClosure, asBacking)
		}
	case *ast.SelectorExpr:
		fn, ok := w.info.Uses[fun.Sel].(*types.Func)
		if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "sync/atomic" || !atomicWrite(fn.Name()) {
			return
		}
		if sel, ok := w.info.Selections[fun]; ok {
			switch sel.Kind() {
			case types.MethodVal: // p.Add(1), s.n.Add(1)
				w.pointee(fun.X, inClosure)
			case types.MethodExpr: // (*atomic.Int64).Add(p, 1)
				if len(c.Args) > 0 {
					w.pointee(c.Args[0], inClosure)
				}
			}
			return
		}
		if len(c.Args) > 0 { // atomic.AddInt64(&s.n, 1), atomic.AddInt64(p, 1)
			w.pointee(c.Args[0], inClosure)
		}
	}
}

// pointee records a write of what e points to: the place x for &x, *e for a pointer value, and e
// itself for an addressable value whose pointer method is called.
func (w *walker) pointee(e ast.Expr, inClosure bool) {
	if u, ok := ast.Unparen(e).(*ast.UnaryExpr); ok && u.Op == token.AND {
		w.place(u.X, inClosure, asLvalue)
		return
	}
	if isPointer(w.info.TypeOf(e)) {
		w.place(e, inClosure, asPointee)
		return
	}
	w.place(e, inClosure, asLvalue)
}

func atomicWrite(name string) bool {
	for _, p := range []string{"Store", "Add", "Swap", "CompareAndSwap", "And", "Or"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// How a place is written.
type mode int

const (
	// asLvalue: the place itself is assigned.
	asLvalue mode = iota
	// asBacking: the place is a map or slice whose elements are written (delete, clear, copy); its
	// backing store is the caller's when the value came from the caller, unless it is a view of a
	// local array.
	asBacking
	// asPointee: the place is a pointer value and what it points to is written.
	asPointee
)

// place records a write of the place e when it is reached from a root by a shared path.
func (w *walker) place(e ast.Expr, inClosure bool, m mode) {
	shared := m == asPointee
	steps := 0
	if m == asPointee {
		steps = 1
	}
	arrayView := false
	uncertain := ""
	cur := e
	for {
		switch x := ast.Unparen(cur).(type) {
		case *ast.Ident:
			obj, _ := w.info.Uses[x].(*types.Var)
			if obj == nil {
				return
			}
			kind, ok := w.roots[obj]
			if !ok {
				if obj.Pkg() == nil || obj.Parent() != obj.Pkg().Scope() {
					return // a local variable
				}
				kind, shared = Global, true
			}
			if m == asBacking && !arrayView {
				shared = true // the map or slice value refers to the caller's backing store
			}
			if !shared {
				return // the function's own copy
			}
			if kind != Global && steps == 0 && m == asLvalue {
				return // the parameter variable itself
			}
			if at, ok := w.rebound[obj]; ok && e.Pos() >= at && uncertain == "" {
				uncertain = "the root was reassigned before this write and may hold fresh storage"
			}
			path := types.ExprString(e)
			if m == asPointee {
				path = "*" + path
			}
			w.out = append(w.out, Write{Pos: e.Pos(), Kind: kind, Root: x.Name, Path: path, InClosure: inClosure, Uncertain: uncertain, root: obj})
			return
		case *ast.SelectorExpr:
			sel, ok := w.info.Selections[x]
			if !ok || sel.Kind() != types.FieldVal {
				return // a package-qualified name or a method value
			}
			if sel.Indirect() || isPointer(w.info.TypeOf(x.X)) {
				shared = true
			}
			cur = x.X
		case *ast.IndexExpr:
			switch shape(w.info.TypeOf(x.X)) {
			case shapeSlice:
				// A slice element is shared, unless the slice is a view of an array value: then it is
				// that array's element, shared only if the array is reached through something shared.
				if !arrayValueView(w.info, x.X) {
					shared = true
				}
			case shapeMap, shapePtrArray:
				shared = true
			case shapeUnknown:
				shared = true // possibly; reported as uncertain, not dropped
				uncertain = "a generic index whose constraint does not settle whether it is a slice, a map or an array"
			}
			cur = x.X
		case *ast.SliceExpr:
			switch shape(w.info.TypeOf(x.X)) {
			case shapeArray:
				arrayView = true // a view of an array value: shared only if the array is reached so
			case shapePtrArray:
				shared = true
			case shapeUnknown:
				shared = true
				uncertain = "a generic slice expression whose constraint does not settle the shape"
			}
			cur = x.X
		case *ast.StarExpr:
			shared = true
			cur = x.X
		default:
			return // a call result, a composite literal: not a place the caller holds
		}
		steps++
	}
}

// arrayValueView reports a slice expression whose backing store is an array value: a[:], a[1:][:2].
func arrayValueView(info *types.Info, e ast.Expr) bool {
	for {
		x, ok := ast.Unparen(e).(*ast.SliceExpr)
		if !ok {
			return false
		}
		switch shape(info.TypeOf(x.X)) {
		case shapeArray:
			return true
		case shapeSlice:
			e = x.X
		default:
			return false
		}
	}
}

type typeShape int

const (
	shapeOther typeShape = iota
	shapeSlice
	shapeMap
	shapeArray
	shapePtrArray
	shapeUnknown
)

// shape is the shape of an indexed or sliced operand, through a type parameter's constraint: every
// term of the type set must agree, or the shape is unknown.
func shape(t types.Type) typeShape {
	if t == nil {
		return shapeOther
	}
	if tp, ok := types.Unalias(t).(*types.TypeParam); ok {
		iface, _ := tp.Constraint().Underlying().(*types.Interface)
		if iface == nil {
			return shapeUnknown
		}
		var got typeShape = -1
		for i := 0; i < iface.NumEmbeddeds(); i++ {
			terms := []types.Type{iface.EmbeddedType(i)}
			if u, ok := iface.EmbeddedType(i).(*types.Union); ok {
				terms = terms[:0]
				for j := 0; j < u.Len(); j++ {
					terms = append(terms, u.Term(j).Type())
				}
			}
			for _, term := range terms {
				s := shape(term)
				if got == -1 {
					got = s
				} else if got != s {
					return shapeUnknown
				}
			}
		}
		if got == -1 || got == shapeOther {
			return shapeUnknown
		}
		return got
	}
	switch u := t.Underlying().(type) {
	case *types.Slice:
		return shapeSlice
	case *types.Map:
		return shapeMap
	case *types.Array:
		return shapeArray
	case *types.Pointer:
		if _, ok := u.Elem().Underlying().(*types.Array); ok {
			return shapePtrArray
		}
	}
	return shapeOther
}

func isPointer(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Pointer)
	return ok
}

// restoration is a proven save and restore: writes of path through root between from and to are
// undone by the assignment that ends at to.
type restoration struct {
	root     types.Object
	path     string
	from, to token.Pos
}

// restorations finds old := P ... P = old pairs among the top-level statements of a body. Only one
// shape is trusted:
//
//   - P is a place made of identifiers, field selections and dereferences, no index, so its
//     spelling names one location; if the path dereferences anything beyond the root variable
//     itself (*s.next, s.next.n), no call may run in between, since a callee could repoint it;
//   - the root's address is never taken in the body, so no callee can reassign it;
//   - between the two, nothing returns, jumps out or calls panic, and old is left alone, its fields
//     and elements included;
//   - the root is not reassigned in between.
//
// A restore in a branch, after an early return, or of a changed value proves nothing. A panic from
// inside a callee is not considered: the restore is taken to run when the statements in between
// complete.
func restorations(info *types.Info, body *ast.BlockStmt, rebound map[types.Object]token.Pos) []restoration {
	var out []restoration
	stmts := body.List
	for i, s := range stmts {
		save, ok := s.(*ast.AssignStmt)
		if !ok || save.Tok != token.DEFINE || len(save.Lhs) != len(save.Rhs) {
			continue
		}
		for k, l := range save.Lhs {
			id, ok := l.(*ast.Ident)
			if !ok {
				continue
			}
			old := info.Defs[id]
			root, deep, ok := stableRoot(info, save.Rhs[k])
			if old == nil || !ok || addressTaken(info, body, root) {
				continue
			}
			path := types.ExprString(save.Rhs[k])
			for j := i + 1; j < len(stmts); j++ {
				if !restores(info, stmts[j], old, root, path) {
					continue
				}
				if !leavesAlone(info, stmts[i+1:j], old, deep) {
					break
				}
				if at, ok := rebound[root]; ok && at > save.End() && at < stmts[j].Pos() {
					break
				}
				out = append(out, restoration{root: root, path: path, from: save.End(), to: stmts[j].End()})
				break
			}
		}
	}
	return out
}

// stableRoot returns the root object of a place made of identifiers, field selections and
// dereferences only, and whether the path dereferences anything beyond the root variable itself.
func stableRoot(info *types.Info, e ast.Expr) (types.Object, bool, bool) {
	deep := false
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			o, ok := info.Uses[x].(*types.Var)
			return o, deep, ok
		case *ast.SelectorExpr:
			sel, ok := info.Selections[x]
			if !ok || sel.Kind() != types.FieldVal {
				return nil, false, false
			}
			if _, onRoot := ast.Unparen(x.X).(*ast.Ident); !onRoot && (sel.Indirect() || isPointer(info.TypeOf(x.X))) {
				deep = true
			}
			e = x.X
		case *ast.StarExpr:
			if _, onRoot := ast.Unparen(x.X).(*ast.Ident); !onRoot {
				deep = true
			}
			e = x.X
		default:
			return nil, false, false
		}
	}
}

// addressTaken reports &root anywhere in the body, closures included.
func addressTaken(info *types.Info, body *ast.BlockStmt, root types.Object) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.AND {
			if id, ok := ast.Unparen(u.X).(*ast.Ident); ok && info.Uses[id] == root {
				found = true
			}
		}
		return !found
	})
	return found
}

// restores reports a top-level P = old assignment.
func restores(info *types.Info, s ast.Stmt, old, root types.Object, path string) bool {
	a, ok := s.(*ast.AssignStmt)
	if !ok || a.Tok != token.ASSIGN || len(a.Lhs) != len(a.Rhs) {
		return false
	}
	for m, l := range a.Lhs {
		id, ok := ast.Unparen(a.Rhs[m]).(*ast.Ident)
		if !ok || info.Uses[id] != old || types.ExprString(l) != path {
			continue
		}
		if r, _, ok := stableRoot(info, l); ok && r == root {
			return true
		}
	}
	return false
}

// rootedAt reports a place whose path starts at obj: obj, obj.f, obj[i], *obj and their mixes.
func rootedAt(info *types.Info, e ast.Expr, obj types.Object) bool {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			return info.Uses[x] == obj
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.SliceExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return false
		}
	}
}

// leavesAlone reports statements that neither return, jump out nor call panic, that leave old and
// its fields and elements alone, closures included, and, when deep is set, that make no call.
func leavesAlone(info *types.Info, stmts []ast.Stmt, old types.Object, deep bool) bool {
	ok := true
	touchesOld := func(n ast.Node) {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				if rootedAt(info, l, old) {
					ok = false
				}
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND && rootedAt(info, n.X, old) {
				ok = false
			}
		case *ast.IncDecStmt:
			if rootedAt(info, n.X, old) {
				ok = false
			}
		case *ast.RangeStmt:
			for _, l := range []ast.Expr{n.Key, n.Value} {
				if l != nil && n.Tok == token.ASSIGN && rootedAt(info, l, old) {
					ok = false
				}
			}
		}
	}
	for _, s := range stmts {
		ast.Inspect(s, func(n ast.Node) bool {
			if !ok {
				return false
			}
			touchesOld(n)
			switch n := n.(type) {
			case *ast.FuncLit:
				ast.Inspect(n.Body, func(m ast.Node) bool { touchesOld(m); return ok })
				return false
			case *ast.ReturnStmt:
				ok = false
			case *ast.BranchStmt:
				if n.Tok == token.GOTO || n.Label != nil {
					ok = false
				}
			case *ast.CallExpr:
				if id, isID := ast.Unparen(n.Fun).(*ast.Ident); isID {
					if b, isB := info.Uses[id].(*types.Builtin); isB {
						if b.Name() == "panic" {
							ok = false
						}
						return ok
					}
				}
				if tv, isT := info.Types[n.Fun]; isT && tv.IsType() {
					return ok // a conversion
				}
				if deep {
					ok = false
				}
			}
			return ok
		})
		if !ok {
			return false
		}
	}
	return true
}
