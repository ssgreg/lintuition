// Package effects reads what a function body does to state its caller can see. It is the shared
// effects model of the promise linters: a doc that says a function does not modify something is
// checked against the writes found here.
//
// The model is direct and conservative. It counts a write only when the written place is reached
// from a receiver, a parameter or a package-level variable by a path the caller shares: a pointer
// dereference (explicit, or implicit in a field selection on a pointer), a map index or a slice
// index. Assigning a field of a struct received by value writes the function's own copy and is not
// a write here; neither is assigning the parameter itself, nor a place saved to a local and later
// assigned back from it (old := s.result; ...; s.result = old). Writes through a local alias (p := r;
// p.n = 1), through a callee, or by calling a mutating method are not followed. Writes inside a
// function literal are reported apart, because when the literal runs is not known.
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

// Write is one write the caller can see.
type Write struct {
	Pos  token.Pos
	Kind RootKind
	// Root is the name of the receiver, parameter or package-level variable.
	Root string
	// Path is the written place as identifiers and selectors (q.head, m[...], *p), for messages.
	Path string
	// InClosure is set for a write inside a function literal.
	InClosure bool
}

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
	w := &walker{info: info, roots: roots, saved: map[types.Object]string{}, restored: map[string]bool{}}
	w.walk(fd.Body, false)
	var out []Write
	for _, x := range w.out {
		if !w.restored[x.Path] {
			out = append(out, x)
		}
	}
	return out
}

type walker struct {
	info  *types.Info
	roots map[types.Object]RootKind
	out   []Write
	// saved maps a local variable to the place it was set from (old := s.result); restored holds
	// the places later assigned back from such a local (s.result = old). A place saved and restored
	// is a temporary change the caller does not see, so its writes are dropped.
	saved    map[types.Object]string
	restored map[string]bool
}

func (w *walker) walk(n ast.Node, inClosure bool) {
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			w.walk(n.Body, true)
			return false
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE || n.Tok == token.ASSIGN {
				w.saveRestore(n)
			}
			if n.Tok != token.DEFINE {
				for _, l := range n.Lhs {
					w.lvalue(l, inClosure)
				}
			}
		case *ast.IncDecStmt:
			w.lvalue(n.X, inClosure)
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				for _, l := range []ast.Expr{n.Key, n.Value} {
					if l != nil {
						w.lvalue(l, inClosure)
					}
				}
			}
		case *ast.CallExpr:
			w.call(n, inClosure)
		}
		return true
	})
}

// saveRestore records old := P (a local set from a place) and P = old (the place set back from it),
// pairwise for tuple assignments.
func (w *walker) saveRestore(n *ast.AssignStmt) {
	if len(n.Lhs) != len(n.Rhs) {
		return
	}
	for i, l := range n.Lhs {
		r := n.Rhs[i]
		if id, ok := l.(*ast.Ident); ok {
			if o := w.info.Defs[id]; o != nil {
				w.saved[o] = types.ExprString(r)
				continue
			}
			if o := w.info.Uses[id]; o != nil && n.Tok == token.ASSIGN {
				if _, isRoot := w.roots[o]; !isRoot {
					w.saved[o] = types.ExprString(r)
				}
			}
		}
		if id, ok := r.(*ast.Ident); ok && n.Tok == token.ASSIGN {
			if from, ok := w.saved[w.info.Uses[id]]; ok && from == types.ExprString(l) {
				w.restored[from] = true
			}
		}
	}
}

// call records the writes of builtins and sync/atomic: delete and clear of a shared map or slice,
// copy into one, an atomic store or add through an address, and the Store, Add, Swap and
// CompareAndSwap methods of the sync/atomic types.
func (w *walker) call(c *ast.CallExpr, inClosure bool) {
	switch fun := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		b, ok := w.info.Uses[fun].(*types.Builtin)
		if !ok || len(c.Args) == 0 {
			return
		}
		switch b.Name() {
		case "delete", "clear", "copy":
			// The argument itself is shared: a map or a slice the caller holds too.
			w.place(c.Args[0], inClosure, true)
		}
	case *ast.SelectorExpr:
		fn, ok := w.info.Uses[fun.Sel].(*types.Func)
		if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "sync/atomic" || !atomicWrite(fn.Name()) {
			return
		}
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
			// r.n.Add(1): the atomic value is the receiver expression, addressed implicitly.
			w.place(fun.X, inClosure, true)
			return
		}
		if len(c.Args) > 0 {
			if u, ok := ast.Unparen(c.Args[0]).(*ast.UnaryExpr); ok && u.Op == token.AND {
				w.place(u.X, inClosure, true)
			}
		}
	}
}

func atomicWrite(name string) bool {
	for _, p := range []string{"Store", "Add", "Swap", "CompareAndSwap", "And", "Or"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func (w *walker) lvalue(e ast.Expr, inClosure bool) { w.place(e, inClosure, false) }

// place records a write of the place e when it is reached from a root by a shared path. addressed
// says the place is written through its address (an atomic, a builtin on a map or a slice), which
// shares it even when no step of the path does.
func (w *walker) place(e ast.Expr, inClosure, addressed bool) {
	shared := addressed
	steps := 0
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
			if !shared {
				return // the function's own copy
			}
			if kind != Global && steps == 0 && !addressed {
				return // the parameter variable itself
			}
			w.out = append(w.out, Write{Pos: e.Pos(), Kind: kind, Root: x.Name, Path: types.ExprString(e), InClosure: inClosure})
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
			switch u := w.info.TypeOf(x.X).Underlying().(type) {
			case *types.Map, *types.Slice:
				shared = true
			case *types.Pointer:
				if _, ok := u.Elem().Underlying().(*types.Array); ok {
					shared = true
				}
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

func isPointer(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Pointer)
	return ok
}
