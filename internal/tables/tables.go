// Package tables reads the rows of table-driven tests and binds each boolean expectation field to
// what the loop over the table compares it with. It is shared by the linters that check table rows,
// so the binding rules hold the same for all of them.
//
// A row's expectation is bound only when the loop over its table compares it, as is, with a result:
// `got := F(tt.in); got != tt.want`, `F(tt.in) != tt.want`, or an Equal assertion. A generic want
// (want, expected, ok, ...) must be compared with a call's single bool result. A negated or
// otherwise transformed comparison, comparisons with different calls, or a result variable written
// again make the field unsupported rather than guessed. So does anything that may change the want
// between the literal and the comparison: a write to the row's field or to the whole row in the
// loop, the row escaping (&tt, f(tt), a method call), or the table variable being reassigned,
// written through (tests[0].want = x) or passed on.
package tables

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ssgreg/lintuition/internal/facts"
)

var (
	nameFields = map[string]bool{"name": true, "desc": true, "description": true, "title": true, "case": true, "scenario": true}
	wantRE     = regexp.MustCompile(`^(want|expected|expect|exp|ok|result)$|^(want|expect|expected)[A-Z_]\w*$`)
	// generic expectation fields hold the tested function's own result.
	generic = map[string]bool{"want": true, "expected": true, "expect": true, "exp": true, "ok": true, "result": true}
)

// IsGeneric reports whether an expectation field name holds the tested function's own result (want,
// expected, ok) rather than naming what it is about (wantInUse).
func IsGeneric(name string) bool { return generic[name] }

// Row is a named row of a table in a test function.
type Row struct {
	// CaseName is the row's constant name: its map key or its name-like field.
	CaseName string
	// Lit is the row's literal.
	Lit *ast.CompositeLit
	// Fields are the row's boolean expectations, in the struct's field order.
	Fields []Field
}

// Field is one boolean expectation of a row.
type Field struct {
	Var *types.Var
	// Pos is the field's value in the row, or the row when the field is omitted.
	Pos token.Pos
	// Callee is the call whose single bool result the field is compared with; nil when it is
	// compared with something else (allowed only for a field that is not generic).
	Callee *types.Func
	// Value is the row's constant; valid when Unsupported is empty.
	Value bool
	// Unsupported says why the field could not be bound or read.
	Unsupported string
}

// Rows returns the named rows of every table in a test function's body, in source order. A row
// without a constant non-blank name is not returned. An expectation the row omits and the loop
// never compares is not returned either: it says nothing.
func Rows(pass *analysis.Pass, body *ast.BlockStmt) []Row {
	var out []Row
	vars := tableVars(pass.TypesInfo, body)
	ast.Inspect(body, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		st, isMap := rowType(pass.TypesInfo.TypeOf(lit))
		if st == nil {
			return true
		}
		b := bindings(pass.TypesInfo, body, lit, vars[lit])
		for _, el := range lit.Elts {
			var key ast.Expr
			row := el
			if kv, ok := el.(*ast.KeyValueExpr); ok && isMap {
				key, row = kv.Key, kv.Value
			}
			if u, ok := row.(*ast.UnaryExpr); ok {
				row = u.X
			}
			if rl, ok := row.(*ast.CompositeLit); ok {
				if r, ok := readRow(pass.TypesInfo, st, key, rl, b); ok {
					out = append(out, r)
				}
			}
		}
		return true
	})
	return out
}

// rowType returns the struct type of a table's rows: []T, [N]T, []*T or map[string]T with T a struct.
func rowType(t types.Type) (*types.Struct, bool) {
	if t == nil {
		return nil, false
	}
	var elem types.Type
	isMap := false
	switch u := t.Underlying().(type) {
	case *types.Slice:
		elem = u.Elem()
	case *types.Array:
		elem = u.Elem()
	case *types.Map:
		if b, ok := u.Key().Underlying().(*types.Basic); !ok || b.Kind() != types.String {
			return nil, false
		}
		elem, isMap = u.Elem(), true
	default:
		return nil, false
	}
	if p, ok := elem.Underlying().(*types.Pointer); ok {
		elem = p.Elem()
	}
	s, _ := elem.Underlying().(*types.Struct)
	return s, isMap
}

func isExpectation(f *types.Var) bool {
	// wantErr says whether an error is expected, not what the result is.
	return isBool(f.Type()) && wantRE.MatchString(f.Name()) && !strings.Contains(f.Name(), "Err")
}

// binding is how the loop over a table uses one expectation field: compared, as is, with the
// result of a call (got := F(tt.in); got != tt.want) or, for a field named after what it is about
// (wantInUse), with anything.
type binding struct {
	callee *types.Func // the call whose result the field is compared with; nil if not a call
	// conflict is set when the field is compared with different things, or in a transformed form.
	conflict string
}

// bindings finds, in the loops over the table, every direct comparison of an expectation field of
// the loop variable: `x == tt.f`, `x != tt.f`, or an Equal-style assertion with both.
func bindings(info *types.Info, body *ast.BlockStmt, lit *ast.CompositeLit, table types.Object) map[string]*binding {
	out := map[string]*binding{}
	for _, rs := range ranges(info, body, lit, table) {
		row, ok := rs.Value.(*ast.Ident)
		if !ok {
			continue
		}
		rowObj := info.ObjectOf(row)
		if rowObj == nil {
			continue
		}
		defs := callDefs(info, rs.Body)
		bind := func(fieldSide, other ast.Expr) {
			pos := fieldSide.Pos()
			f, ok := fieldOf(info, fieldSide, rowObj)
			if !ok {
				return
			}
			b := out[f.Name()]
			if b == nil {
				b = &binding{}
				out[f.Name()] = b
			}
			callee, transformed := resultOf(info, other, pos, defs)
			switch {
			case transformed:
				b.conflict = "the expectation is compared in a transformed form"
			case b.callee != nil && callee != nil && b.callee != callee:
				b.conflict = "the expectation is compared with results of different calls"
			case callee != nil:
				b.callee = callee
			case generic[f.Name()]:
				// Compared with something whose call is not established: it may be anything.
				b.conflict = "the expectation is compared with a value not established as one call's result"
			}
		}
		ast.Inspect(rs.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BinaryExpr:
				if n.Op == token.EQL || n.Op == token.NEQ {
					bind(n.X, n.Y)
					bind(n.Y, n.X)
				}
			case *ast.CallExpr:
				if fn := facts.Callee(info, n); fn != nil && (fn.Name() == "Equal" || fn.Name() == "EqualValues") && len(n.Args) >= 3 {
					bind(n.Args[1], n.Args[2])
					bind(n.Args[2], n.Args[1])
				}
			}
			return true
		})
		// The row's literal want is the value compared only if nothing in the loop writes it.
		all, fields := rowWrites(info, rs.Body, rowObj)
		for name, b := range out {
			if all || fields[name] {
				b.conflict = "the row's expectation is written in the loop over the table"
			}
		}
	}
	if table != nil && tableWritten(info, body, table) {
		for _, b := range out {
			b.conflict = "the table is written after it is built"
		}
	}
	return out
}

// root returns the variable an lvalue-like expression is rooted at: tt in tt.want, tt.a.b,
// tests[i].want, *p.
func root(info *types.Info, e ast.Expr) types.Object {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			return info.ObjectOf(x)
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// rowWrites finds what a loop body may change of the row variable. A write to one field
// (tt.want = x, tt.want++, &tt.want) changes that field; a write to the row itself, its address,
// a method call on it or any use other than reading a field (passing tt on, tt := tt) may change
// any field, so all is set.
func rowWrites(info *types.Info, body *ast.BlockStmt, row types.Object) (all bool, fields map[string]bool) {
	fields = map[string]bool{}
	// write records an lvalue: tt.f marks f, anything else rooted at the row marks all.
	write := func(e ast.Expr) {
		if root(info, e) != row {
			return
		}
		if sel, ok := ast.Unparen(e).(*ast.SelectorExpr); ok {
			if id, ok := ast.Unparen(sel.X).(*ast.Ident); ok && info.ObjectOf(id) == row {
				if v, ok := info.ObjectOf(sel.Sel).(*types.Var); ok && v.IsField() {
					fields[v.Name()] = true
					return
				}
			}
		}
		all = true
	}
	// fieldReads are the row idents that are only the operand of a field selector.
	fieldReads := map[*ast.Ident]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				write(l)
			}
		case *ast.IncDecStmt:
			write(n.X)
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				write(n.X)
			}
		case *ast.SelectorExpr:
			id, ok := ast.Unparen(n.X).(*ast.Ident)
			if !ok || info.ObjectOf(id) != row {
				return true
			}
			if sel := info.Selections[n]; sel != nil && sel.Kind() == types.FieldVal {
				fieldReads[id] = true
			}
		}
		return true
	})
	ast.Inspect(body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && info.Uses[id] == row && !fieldReads[id] {
			all = true // the row escapes or is used whole: tt.M(), f(tt), tt := tt
		}
		return true
	})
	return all, fields
}

// tableWritten reports whether the table variable is used other than by its one definition, a
// range over it, or len / cap: reassigned (tests = ...), its rows written (tests[0].want = ...), its
// address taken, or passed on, any of which may change what the loop reads.
func tableWritten(info *types.Info, body *ast.BlockStmt, table types.Object) bool {
	allowed := map[*ast.Ident]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.RangeStmt:
			if id, ok := ast.Unparen(n.X).(*ast.Ident); ok {
				allowed[id] = true
			}
		case *ast.CallExpr:
			if b, ok := info.Uses[identOf(n.Fun)].(*types.Builtin); ok && (b.Name() == "len" || b.Name() == "cap") && len(n.Args) == 1 {
				if id, ok := ast.Unparen(n.Args[0]).(*ast.Ident); ok {
					allowed[id] = true
				}
			}
		}
		return true
	})
	written := false
	ast.Inspect(body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && info.Uses[id] == table && !allowed[id] {
			written = true
		}
		return !written
	})
	return written
}

func identOf(e ast.Expr) *ast.Ident {
	id, _ := ast.Unparen(e).(*ast.Ident)
	return id
}

// ranges returns the loops over the table: over its variable or over the literal itself.
func ranges(info *types.Info, body *ast.BlockStmt, lit *ast.CompositeLit, table types.Object) []*ast.RangeStmt {
	var out []*ast.RangeStmt
	ast.Inspect(body, func(n ast.Node) bool {
		rs, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		if ast.Unparen(rs.X) == lit {
			out = append(out, rs)
		} else if id, ok := ast.Unparen(rs.X).(*ast.Ident); ok && table != nil && info.ObjectOf(id) == table {
			out = append(out, rs)
		}
		return true
	})
	return out
}

// fieldOf returns the expectation field e reads from the loop variable: exactly `tt.want`.
func fieldOf(info *types.Info, e ast.Expr, row types.Object) (*types.Var, bool) {
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	x, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok || info.ObjectOf(x) != row {
		return nil, false
	}
	f, ok := info.ObjectOf(sel.Sel).(*types.Var)
	if !ok || !f.IsField() || !isExpectation(f) {
		return nil, false
	}
	return f, true
}

// def is how a loop variable got its value: from the single bool result of one call.
type def struct {
	fn  *types.Func
	pos token.Pos
}

// callDefs maps each variable defined in the loop body as `got := F(...)`, where F has exactly one
// bool result, to that call. A variable written anywhere else in the body (got = !got, got =
// G(), &got, ++), defined from a call with several bool results, or defined twice, maps to nil: its
// value at the comparison is not established, so the row is unsupported rather than guessed.
func callDefs(info *types.Info, body *ast.BlockStmt) map[types.Object]*def {
	out := map[types.Object]*def{}
	taint := func(e ast.Expr) {
		if id, ok := ast.Unparen(e).(*ast.Ident); ok {
			if obj := info.ObjectOf(id); obj != nil {
				out[obj] = nil
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			call, isCall := ast.Unparen(n.Rhs[0]).(*ast.CallExpr)
			if n.Tok != token.DEFINE || len(n.Rhs) != 1 || !isCall {
				for _, l := range n.Lhs {
					taint(l)
				}
				return true
			}
			fn := facts.Callee(info, call)
			var slots []int
			if fn != nil {
				res := fn.Type().(*types.Signature).Results()
				for i := 0; i < res.Len(); i++ {
					if isBool(res.At(i).Type()) {
						slots = append(slots, i)
					}
				}
			}
			for i, l := range n.Lhs {
				id, ok := l.(*ast.Ident)
				if !ok || id.Name == "_" {
					continue
				}
				obj := info.Defs[id]
				if obj == nil {
					taint(l) // := that reuses an existing variable is a write to it
					continue
				}
				if _, seen := out[obj]; seen || fn == nil || len(slots) != 1 || slots[0] != i {
					out[obj] = nil
					continue
				}
				out[obj] = &def{fn: fn, pos: id.Pos()}
			}
		case *ast.IncDecStmt:
			taint(n.X)
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				taint(n.X)
			}
		case *ast.RangeStmt:
			taint(n.Key)
			if n.Value != nil {
				taint(n.Value)
			}
		}
		return true
	})
	return out
}

// resultOf says which call's single bool result an expression is, directly or through a variable
// defined from it before the comparison at pos; transformed is true for a negation, which flips
// what the expectation means.
func resultOf(info *types.Info, e ast.Expr, pos token.Pos, defs map[types.Object]*def) (*types.Func, bool) {
	e = ast.Unparen(e)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.NOT {
		return nil, true
	}
	switch e := e.(type) {
	case *ast.CallExpr:
		fn := facts.Callee(info, e)
		if fn == nil {
			return nil, false
		}
		res := fn.Type().(*types.Signature).Results()
		if res.Len() == 1 && isBool(res.At(0).Type()) {
			return fn, false
		}
	case *ast.Ident:
		if d := defs[info.ObjectOf(e)]; d != nil && d.pos < pos {
			return d.fn, false
		}
	}
	return nil, false
}

func isBool(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.Bool
}

func readRow(info *types.Info, st *types.Struct, key ast.Expr, rl *ast.CompositeLit, binds map[string]*binding) (Row, bool) {
	caseName := ""
	if key != nil {
		caseName, _ = facts.ConstString(info, key)
	}
	// values holds each field's expression in this row, keyed or positional.
	values := map[string]ast.Expr{}
	keyed := false
	for i, e := range rl.Elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			keyed = true
			if id, ok := kv.Key.(*ast.Ident); ok {
				values[id.Name] = kv.Value
			}
		} else if i < st.NumFields() {
			values[st.Field(i).Name()] = e
		}
	}
	if caseName == "" {
		for i := 0; i < st.NumFields(); i++ {
			if f := st.Field(i); nameFields[strings.ToLower(f.Name())] && values[f.Name()] != nil {
				caseName, _ = facts.ConstString(info, values[f.Name()])
				break
			}
		}
	}
	if strings.TrimSpace(caseName) == "" {
		return Row{}, false
	}
	row := Row{CaseName: caseName, Lit: rl}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !isExpectation(f) {
			continue
		}
		expr := values[f.Name()]
		fld := Field{Var: f, Pos: rl.Pos()}
		if expr != nil {
			fld.Pos = expr.Pos()
		}
		b := binds[f.Name()]
		switch {
		case b == nil:
			if expr == nil {
				continue // an unused, unset field says nothing
			}
			fld.Unsupported = "the expectation is not compared, as is, in a loop over the table"
		case b.conflict != "":
			fld.Unsupported = b.conflict
		case generic[f.Name()] && b.callee == nil:
			fld.Unsupported = "the expectation is not compared with the result of a call"
		case expr == nil && keyed:
			fld.Value = false // omitted in a keyed literal: Go's zero value
		case expr == nil:
			fld.Unsupported = "the row does not set the expectation"
		default:
			v, ok := facts.ConstBool(info, expr)
			if !ok {
				fld.Unsupported = "the expectation is not a constant"
			}
			fld.Value = v
		}
		if b != nil {
			fld.Callee = b.callee
		}
		row.Fields = append(row.Fields, fld)
	}
	return row, true
}

// tableVars maps each composite literal assigned to a local variable (tests := []struct{...}{...})
// to that variable.
func tableVars(info *types.Info, body *ast.BlockStmt) map[*ast.CompositeLit]types.Object {
	out := map[*ast.CompositeLit]types.Object{}
	bind := func(lhs []ast.Expr, rhs []ast.Expr) {
		if len(lhs) != len(rhs) {
			return
		}
		for i, r := range rhs {
			lit, ok := ast.Unparen(r).(*ast.CompositeLit)
			id, isID := lhs[i].(*ast.Ident)
			if !ok || !isID {
				continue
			}
			if obj := info.ObjectOf(id); obj != nil {
				out[lit] = obj
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			bind(n.Lhs, n.Rhs)
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, len(n.Names))
			for i, id := range n.Names {
				lhs[i] = id
			}
			bind(lhs, n.Values)
		}
		return true
	})
	return out
}
