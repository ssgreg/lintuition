// Package tablecase checks the rows of table-driven tests: a case whose name says one boolean outcome
// while its expectation field says the other, as when a row is copied and the want is not flipped.
//
//	{name: "rejects an expired token", token: expired, want: true},   // name says false
//
// A person wrote both the name and the want, so the table is evidence nobody had to derive. The
// classifier reads only the case name and what the boolean is about; Go code compares its answer
// with the constant in the table.
//
// A row is checked only when the loop over its table compares the expectation, as is, with a
// result: `got := F(tt.in); got != tt.want`, `F(tt.in) != tt.want`, or an Equal assertion. A
// generic want must be compared with a call's bool result, which names what it is about. A
// negated or otherwise transformed comparison, or comparisons with different calls, make the row
// unsupported rather than guessed.
package tablecase

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "table-case-vs-expectation"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of the name's answer for a finding (default 0.85, from
	// the prototype; not yet validated on a labelled set).
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts one candidate per boolean expectation of a named table row.
var Analyzer = &analysis.Analyzer{
	Name:       "tablecase",
	Doc:        "extract named table-test rows with a constant boolean expectation",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a table test case whose name says the opposite of its boolean expectation",
		Standard:    true,
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

var (
	nameFields = map[string]bool{"name": true, "desc": true, "description": true, "title": true, "case": true, "scenario": true}
	wantRE     = regexp.MustCompile(`^(want|expected|expect|exp|ok|result)$|^(want|expect|expected)[A-Z_]\w*$`)
	// generic expectation fields hold the tested function's own result.
	generic = map[string]bool{"want": true, "expected": true, "expect": true, "exp": true, "ok": true, "result": true}
)

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	var out []*sdk.Candidate
	ins.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fd := n.(*ast.FuncDecl)
		if fd.Body == nil || !isTest(pass, fd) {
			return
		}
		vars := tableVars(pass.TypesInfo, fd.Body)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			st, isMap := rowType(pass.TypesInfo.TypeOf(lit))
			if st == nil {
				return true
			}
			b := bindings(pass, fd.Body, lit, vars[lit])
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
					out = append(out, rowCandidates(pass, st, key, rl, b)...)
				}
			}
			return true
		})
	})
	return out, nil
}

func isTest(pass *analysis.Pass, fd *ast.FuncDecl) bool {
	if !strings.HasPrefix(fd.Name.Name, "Test") || fd.Recv != nil {
		return false
	}
	return strings.HasSuffix(pass.Fset.Position(fd.Pos()).Filename, "_test.go")
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
	b, ok := f.Type().Underlying().(*types.Basic)
	// wantErr says whether an error is expected, not what the result is.
	return ok && b.Kind() == types.Bool && wantRE.MatchString(f.Name()) && !strings.Contains(f.Name(), "Err")
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
func bindings(pass *analysis.Pass, body *ast.BlockStmt, lit *ast.CompositeLit, table types.Object) map[string]*binding {
	info := pass.TypesInfo
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
			f, ok := fieldOf(info, fieldSide, rowObj)
			if !ok {
				return
			}
			b := out[f.Name()]
			if b == nil {
				b = &binding{}
				out[f.Name()] = b
			}
			callee, transformed := resultOf(info, other, defs)
			switch {
			case transformed:
				b.conflict = "the expectation is compared in a transformed form"
			case b.callee != nil && callee != nil && b.callee != callee:
				b.conflict = "the expectation is compared with results of different calls"
			case callee != nil:
				b.callee = callee
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
	}
	return out
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

// callDefs maps a variable to the bool-returning call it was defined from in the loop body:
// got := F(tt.in), or got, err := F(tt.in) at F's bool result.
func callDefs(info *types.Info, body *ast.BlockStmt) map[types.Object]*types.Func {
	out := map[types.Object]*types.Func{}
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 {
			return true
		}
		call, ok := ast.Unparen(as.Rhs[0]).(*ast.CallExpr)
		if !ok {
			return true
		}
		fn := facts.Callee(info, call)
		if fn == nil {
			return true
		}
		res := fn.Type().(*types.Signature).Results()
		for i, l := range as.Lhs {
			id, ok := l.(*ast.Ident)
			if !ok || i >= res.Len() || !isBool(res.At(i).Type()) {
				continue
			}
			if obj := info.ObjectOf(id); obj != nil {
				out[obj] = fn
			}
		}
		return true
	})
	return out
}

// resultOf says which call's bool result an expression is, directly or through a variable defined
// from it; transformed is true for a negation, which flips what the expectation means.
func resultOf(info *types.Info, e ast.Expr, defs map[types.Object]*types.Func) (*types.Func, bool) {
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
		return defs[info.ObjectOf(e)], false
	}
	return nil, false
}

func isBool(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.Bool
}

func rowCandidates(pass *analysis.Pass, st *types.Struct, key ast.Expr, rl *ast.CompositeLit, binds map[string]*binding) []*sdk.Candidate {
	info := pass.TypesInfo
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
		return nil
	}
	var out []*sdk.Candidate
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !isExpectation(f) {
			continue
		}
		expr := values[f.Name()]
		pos := rl.Pos()
		if expr != nil {
			pos = expr.Pos()
		}
		c := &sdk.Candidate{
			Pos: pass.Fset.Position(pos),
			// The field is part of the identity: two expectations of one row are two questions.
			Subject: caseName + "#" + f.Name(),
			Local:   map[string]string{"case_name": caseName, "field": f.Name()},
		}
		b := binds[f.Name()]
		var value bool
		switch {
		case b == nil:
			if expr == nil {
				continue // an unused, unset field says nothing
			}
			c.Unsupported = "the expectation is not compared, as is, in a loop over the table"
		case b.conflict != "":
			c.Unsupported = b.conflict
		case generic[f.Name()] && b.callee == nil:
			c.Unsupported = "the expectation is not compared with the result of a call"
		case expr == nil && keyed:
			value = false // omitted in a keyed literal: Go's zero value
		case expr == nil:
			c.Unsupported = "the row does not set the expectation"
		default:
			v, ok := facts.ConstBool(info, expr)
			if !ok {
				c.Unsupported = "the expectation is not a constant"
			}
			value = v
		}
		if c.Unsupported != "" {
			out = append(out, c)
			continue
		}
		about := "the result of " + calleeName(b)
		if !generic[f.Name()] {
			about = aboutName(f.Name())
		}
		c.Local["value"] = fmt.Sprint(value)
		c.Payload.AddProse("case_name", caseName)
		c.Payload.Fact("about", about)
		out = append(out, c)
	}
	return out
}

func calleeName(b *binding) string {
	if b == nil || b.callee == nil {
		return ""
	}
	return b.callee.Name()
}

// aboutName says what a named expectation is about: wantInUse -> "whether in use".
func aboutName(name string) string {
	words := facts.Words(name)
	for len(words) > 0 && (words[0] == "want" || words[0] == "expect" || words[0] == "expected") {
		words = words[1:]
	}
	return "whether " + strings.Join(words, " ")
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

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "outcome",
		Kind: sdk.Choice,
		Text: "The boolean in this test case is about `about`. Reading only the case name `case_name`, does the name say that boolean is true or false?",
		Options: []sdk.Option{
			{Key: "true", Description: "The name states it outright (conflicts, expired, keeps it in use, is missing)."},
			{Key: "false", Description: "The name states the opposite outright (does not conflict, not expired, deletes it)."},
			{Key: "unstated", Description: "The name only describes the input or the setup and says nothing about this boolean."},
		},
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["outcome"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the case name does not say")
	}
	// Weak support abstains whichever way the answer goes.
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice == "unstated" || a.Choice == c.Local["value"] {
		return sdk.Clean()
	}
	return sdk.Report("case %q reads as %s %s, but the table sets %s", c.Local["case_name"], c.Local["field"], a.Choice, c.Local["value"])
}
