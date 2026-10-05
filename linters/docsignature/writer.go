package docsignature

import (
	"go/types"
	"regexp"
	"strconv"
	"strings"
)

// An HTTP handler's doc often says "returns X" about what it writes to the client, on a function
// with no results. To let the classifier tell that from a value given back to the Go caller, the
// result claim carries a fact naming an http.ResponseWriter the function can reach, when the
// signature has one: a parameter or the receiver whose type implements it, or a value reached
// from one through exported (or same-package) fields and methods without arguments, such as a
// framework context's Writer field or Response() method. The fact states a capability only: the
// types do not show that the function writes a response, or that the response is what its doc
// talks about.
//
// A plain io.Writer gives no fact. It is reachable from much that is not an output the doc could
// mean (a test's t.Output(), an analysis pass's flag set, a trace or debug writer), and told only
// that such a writer is there, the classifier discounted ordinary stale promises ("returns the
// stored value") about as often as handler docs. Nor is a writer the function captures in a
// closure, reads from a package variable or gets behind an interface such as any seen. In all
// these cases the candidate is asked as it always was.

// maxWriterHops bounds the path from a parameter to its writer: c.Writer is one hop,
// c.Ctx.Writer two. maxWriterNodes bounds the whole walk from one parameter. A walk that runs out
// finds nothing, which leaves the candidate as it was without the fact.
const (
	maxWriterHops  = 3
	maxWriterNodes = 5000
)

var ioWriter = func() *types.Interface {
	p := types.NewVar(0, nil, "p", types.NewSlice(types.Typ[types.Byte]))
	n := types.NewVar(0, nil, "n", types.Typ[types.Int])
	err := types.NewVar(0, nil, "err", types.Universe.Lookup("error").Type())
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(p), types.NewTuple(n, err), false)
	return types.NewInterfaceType([]*types.Func{types.NewFunc(0, nil, "Write", sig)}, nil).Complete()
}()

// writerKind returns "http.ResponseWriter" when t implements it, and "" otherwise. When the value
// is addressable (a parameter, a receiver, a field reached through a pointer or of an addressable
// value), a pointer that implements it counts too, since its pointer methods can be called; a
// method's result is not addressable.
func writerKind(t types.Type, addressable bool) string {
	t = types.Unalias(t)
	if _, ok := t.(*types.TypeParam); ok {
		return "" // its constraint is not the value: a writer constraint is rare, and missing it is safe
	}
	cands := []types.Type{t}
	if addressable {
		cands = append(cands, pointerTo(t))
	}
	for _, c := range cands {
		if c != nil && types.Implements(c, ioWriter) && implementsResponseWriter(c) {
			return "http.ResponseWriter"
		}
	}
	return ""
}

// pointerTo returns *t for a type whose pointer may have more methods, or nil.
func pointerTo(t types.Type) types.Type {
	if _, ok := t.(*types.Pointer); ok || types.IsInterface(t) {
		return nil
	}
	return types.NewPointer(t)
}

// implementsResponseWriter reports whether t implements net/http.ResponseWriter. The interface is
// built from the Header type t's own Header method returns, so the check needs no import of
// net/http: a type without such a method cannot implement it.
func implementsResponseWriter(t types.Type) bool {
	obj, _, _ := types.LookupFieldOrMethod(t, false, nil, "Header")
	m, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	sig := m.Type().(*types.Signature)
	if sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}
	h, ok := types.Unalias(sig.Results().At(0).Type()).(*types.Named)
	if !ok || h.Obj().Pkg() == nil || h.Obj().Pkg().Path() != "net/http" || h.Obj().Name() != "Header" {
		return false
	}
	header := types.NewSignatureType(nil, nil, nil, nil, types.NewTuple(types.NewVar(0, nil, "", h)), false)
	code := types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewVar(0, nil, "code", types.Typ[types.Int])), nil, false)
	rw := types.NewInterfaceType([]*types.Func{
		types.NewFunc(0, nil, "Header", header),
		ioWriter.Method(0),
		types.NewFunc(0, nil, "WriteHeader", code),
	}, nil).Complete()
	return types.Implements(t, rw)
}

// writerFact returns the fact naming the first response writer the signature reaches: the receiver first,
// then the parameters in order, each by the shortest path. It returns "" when there is none. A root
// whose fact cannot be sent (a non-ASCII name, a long path) is passed over for the next one. In a
// declared function a blank or unnamed parameter is skipped, since the body cannot use it; an
// interface method's parameters are often unnamed and are named by position.
func writerFact(pkg *types.Package, sig *types.Signature, declared bool) string {
	type root struct {
		label, name string
		t           types.Type
	}
	var roots []root
	if r := sig.Recv(); r != nil && usable(r.Name(), declared) {
		if r.Name() == "" || r.Name() == "_" {
			// An interface method's receiver is the interface value.
			roots = append(roots, root{"the receiver", "", r.Type()})
		} else {
			roots = append(roots, root{"receiver " + r.Name(), r.Name(), r.Type()})
		}
	}
	for i := 0; i < sig.Params().Len(); i++ {
		p := sig.Params().At(i)
		if sig.Variadic() && i == sig.Params().Len()-1 {
			continue // a slice of values, not one writer
		}
		if !usable(p.Name(), declared) {
			continue
		}
		if name := p.Name(); name != "" && name != "_" {
			roots = append(roots, root{"parameter " + name, name, p.Type()})
		} else {
			roots = append(roots, root{"parameter " + strconv.Itoa(i+1), "", p.Type()})
		}
	}
	for _, r := range roots {
		kind, path := findWriter(pkg, r.t)
		var fact string
		switch {
		case kind == "":
			continue
		case path == "":
			fact = r.label + " is an " + kind + " it could write a response to"
		case r.name == "":
			fact = r.label + " gives access to an " + kind + " at " + path
		default:
			fact = r.label + " gives access to an " + kind + " at " + r.name + "." + path
		}
		if plainFact.MatchString(fact) {
			return fact
		}
	}
	return ""
}

func usable(name string, declared bool) bool {
	return !declared || name != "" && name != "_"
}

// plainFact is what a fact string may look like under the payload policy: short, identifiers and
// punctuation only.
var plainFact = regexp.MustCompile(`^[A-Za-z0-9_ .,()]{1,120}$`)

// findWriter walks from t breadth first, through fields of structs (behind pointers too) and the
// results of methods without arguments, and returns the writer kind and the path of the first
// value that is a writer: "" for t itself, "Writer" or "Response()" or "Ctx.Writer" below it.
// Breadth first, a type is first met on its shortest path, so each named type is expanded once
// and the answer does not depend on which field comes first; that also ends every type cycle.
func findWriter(pkg *types.Package, t types.Type) (kind, path string) {
	type node struct {
		t    types.Type
		path []string
		addr bool
	}
	type key struct {
		t    types.Type
		addr bool
	}
	queue := []node{{t: t, addr: true}}
	seen := map[key]bool{}
	nodes := 0
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if nodes++; nodes > maxWriterNodes {
			return "", ""
		}
		if k := writerKind(n.t, n.addr); k != "" {
			return k, strings.Join(n.path, ".")
		}
		if len(n.path) == maxWriterHops {
			continue
		}
		base, addr := types.Unalias(n.t), n.addr
		if p, ok := base.(*types.Pointer); ok {
			base, addr = types.Unalias(p.Elem()), true
		}
		if _, ok := base.(*types.TypeParam); ok {
			continue
		}
		if named, ok := base.(*types.Named); ok {
			if seen[key{named, addr}] {
				continue
			}
			seen[key{named, addr}] = true
		}
		next := func(t types.Type, step string, addr bool) {
			queue = append(queue, node{t: t, path: append(append([]string(nil), n.path...), step), addr: addr})
		}
		if s, ok := base.Underlying().(*types.Struct); ok {
			for i := 0; i < s.NumFields(); i++ {
				// A blank field cannot be named, so it is no way to the writer.
				if f := s.Field(i); f.Name() != "_" && visible(pkg, f) {
					next(f.Type(), f.Name(), addr)
				}
			}
		}
		for _, m := range methods(base, addr) {
			if !visible(pkg, m) {
				continue
			}
			sig := m.Type().(*types.Signature)
			if sig.Params().Len() == 0 && sig.Results().Len() == 1 {
				next(sig.Results().At(0).Type(), m.Name()+"()", false)
			}
		}
	}
	return "", ""
}

// methods returns the methods callable on a value of type t, in name order: with the pointer
// methods when the value is addressable.
func methods(t types.Type, addressable bool) []*types.Func {
	var ms *types.MethodSet
	if types.IsInterface(t) || !addressable {
		ms = types.NewMethodSet(t)
	} else {
		ms = types.NewMethodSet(types.NewPointer(t))
	}
	out := make([]*types.Func, 0, ms.Len())
	for i := 0; i < ms.Len(); i++ {
		if f, ok := ms.At(i).Obj().(*types.Func); ok {
			out = append(out, f)
		}
	}
	return out
}

// visible reports a field or method the analysed package can use: exported, or its own.
func visible(pkg *types.Package, obj types.Object) bool {
	return obj.Exported() || obj.Pkg() == pkg
}
