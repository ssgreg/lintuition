package effects

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

const src = `package p

import "sync/atomic"

type Q struct {
	head  int
	items []int
	m     map[string]int
	arr   [2]int
	inner *Q
	n     atomic.Int64
	c     int64
}

type V struct {
	n   int
	m   map[string]int
	s   []int
	arr [2]int
	p   *Q
}

type E struct{ *Q }

var count int
var table = map[string]int{}

func (q *Q) PtrField()           { q.head++ }
func (q *Q) PtrAppend(x int)     { q.items = append(q.items, x) }
func (q *Q) PtrMap(k string)     { q.m[k] = 1 }
func (q *Q) PtrArray()           { q.arr[0] = 1 }
func (q *Q) Deref()              { *q = Q{} }
func (q *Q) Nested()             { q.inner.head = 2 }
func (q *Q) Read() int           { return q.head }
func (q *Q) Reassign()           { q = nil; _ = q }
func (v V) ValueField()          { v.n = 1 }
func (v V) ValueArray()          { v.arr[0] = 1 }
func (v V) ValueMap()            { v.m["k"] = 1 }
func (v V) ValueSlice()          { v.s[0] = 1 }
func (v V) ValueThroughPtr()     { v.p.head = 1 }
func (e E) Promoted()            { e.head = 3 }
func SliceParam(xs []int)        { xs[0] = 1 }
func SliceReassign(xs []int)     { xs = append(xs, 1); _ = xs }
func MapParam(m map[string]int)  { m["a"] = 1 }
func PtrParam(p *int)            { *p = 1 }
func StructParam(v V)            { v.n = 2 }
func Global()                    { count++ }
func GlobalMap()                 { table["a"] = 1 }
func Local()                     { x := Q{}; x.head = 1; y := &x; y.head = 2 }
func AliasNotFollowed(q *Q)      { p := q; p.head = 1 }
func Closure(q *Q) func()        { return func() { q.head = 1 } }
func Deferred(q *Q)              { defer func() { q.head = 1 }() }
func DeleteMap(m map[string]int) { delete(m, "a") }
func ClearLocal()                { m := map[string]int{}; clear(m) }
func CopyInto(dst, src []int)    { copy(dst, src) }
func (q *Q) AtomicMethod()       { q.n.Add(1) }
func (q *Q) AtomicLoad() int64   { return q.n.Load() }
func (q *Q) AtomicFunc()         { atomic.AddInt64(&q.c, 1) }
func (q *Q) AtomicFuncLoad() int64 { return atomic.LoadInt64(&q.c) }
func (q *Q) RangeAssign()        { for q.head = range q.items {} }
func (q *Q) Multi()              { q.head, count = 1, 2 }
func Named() (n int)             { n = 1; return }
func (q *Q) SaveRestore()        { old := q.head; q.head = 0; q.head = old }
func (q *Q) SaveRestoreTuple()   { a, b := q.head, q.items; q.head, q.items = 0, nil; q.head, q.items = a, b }
func (q *Q) SaveOnly()           { old := q.head; q.head = 0; _ = old }
func (q *Q) RestoreOther()       { old := q.head; q.items = nil; q.head = old }
`

func TestWrites(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Types:      map[ast.Expr]types.TypeAndValue{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("p", fset, []*ast.File{f}, info); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"PtrField":         "receiver q.head",
		"PtrAppend":        "receiver q.items",
		"PtrMap":           "receiver q.m[k]",
		"PtrArray":         "receiver q.arr[0]",
		"Deref":            "receiver *q",
		"Nested":           "receiver q.inner.head",
		"Read":             "",
		"Reassign":         "",
		"ValueField":       "",
		"ValueArray":       "",
		"ValueMap":         `receiver v.m["k"]`,
		"ValueSlice":       "receiver v.s[0]",
		"ValueThroughPtr":  "receiver v.p.head",
		"Promoted":         "receiver e.head",
		"SliceParam":       "param xs[0]",
		"SliceReassign":    "",
		"MapParam":         `param m["a"]`,
		"PtrParam":         "param *p",
		"StructParam":      "",
		"Global":           "global count",
		"GlobalMap":        `global table["a"]`,
		"Local":            "",
		"AliasNotFollowed": "",
		"Closure":          "param q.head (closure)",
		"Deferred":         "param q.head (closure)",
		"DeleteMap":        "param m",
		"ClearLocal":       "",
		"CopyInto":         "param dst",
		"AtomicMethod":     "receiver q.n",
		"AtomicLoad":       "",
		"AtomicFunc":       "receiver q.c",
		"AtomicFuncLoad":   "",
		"RangeAssign":      "receiver q.head",
		"Multi":            "receiver q.head; global count",
		"Named":            "",
		"SaveRestore":      "",
		"SaveRestoreTuple": "",
		"SaveOnly":         "receiver q.head",
		"RestoreOther":     "receiver q.items",
	}
	kinds := map[RootKind]string{Receiver: "receiver", Param: "param", Global: "global"}
	seen := 0
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		var got []string
		for _, w := range Writes(info, fd) {
			s := kinds[w.Kind] + " " + w.Path
			if w.InClosure {
				s += " (closure)"
			}
			got = append(got, s)
		}
		exp, ok := want[fd.Name.Name]
		if !ok {
			t.Errorf("%s: no expectation", fd.Name.Name)
			continue
		}
		seen++
		if g := strings.Join(got, "; "); g != exp {
			t.Errorf("%s: got %q, want %q", fd.Name.Name, g, exp)
		}
	}
	if seen != len(want) {
		t.Errorf("checked %d functions, want %d", seen, len(want))
	}
}
