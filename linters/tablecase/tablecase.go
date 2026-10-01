// Package tablecase checks the rows of table-driven tests: a case whose name says one boolean outcome
// while its expectation field says the other, as when a row is copied and the want is not flipped.
//
//	{name: "rejects an expired token", token: expired, want: true},   // name says false
//
// A person wrote both the name and the want, so the table is evidence nobody had to derive. The
// classifier reads only the case name and what the boolean is about; Go code compares its answer
// with the constant in the table.
package tablecase

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"sort"
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
			elem, isMap := rowType(pass.TypesInfo.TypeOf(lit))
			if elem == nil {
				return true
			}
			for _, el := range lit.Elts {
				var key ast.Expr
				row := el
				if kv, ok := el.(*ast.KeyValueExpr); ok && isMap {
					key, row = kv.Key, kv.Value
				}
				if u, ok := row.(*ast.UnaryExpr); ok {
					row = u.X
				}
				rl, ok := row.(*ast.CompositeLit)
				if !ok {
					continue
				}
				out = append(out, rowCandidates(pass, elem, key, rl, testedFunc(pass, fd, lit, vars[lit]))...)
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

func rowCandidates(pass *analysis.Pass, st *types.Struct, key ast.Expr, rl *ast.CompositeLit, tested *types.Func) []*sdk.Candidate {
	info := pass.TypesInfo
	caseName := ""
	if key != nil {
		caseName, _ = facts.ConstString(info, key)
	}
	type want struct {
		field string
		value bool
	}
	var wants []want
	for _, e := range rl.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		id, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		f := field(st, id.Name)
		if f == nil {
			continue
		}
		if nameFields[strings.ToLower(id.Name)] && caseName == "" {
			caseName, _ = facts.ConstString(info, kv.Value)
			continue
		}
		b, isBool := f.Type().Underlying().(*types.Basic)
		// wantErr says whether an error is expected, not what the result is.
		if !isBool || b.Kind() != types.Bool || !wantRE.MatchString(id.Name) || strings.Contains(id.Name, "Err") {
			continue
		}
		if v, ok := facts.ConstBool(info, kv.Value); ok {
			wants = append(wants, want{id.Name, v})
		}
	}
	if strings.TrimSpace(caseName) == "" {
		return nil
	}
	var out []*sdk.Candidate
	for _, w := range wants {
		c := &sdk.Candidate{
			Pos:     pass.Fset.Position(rl.Pos()),
			Subject: caseName,
			Local:   map[string]string{"case_name": caseName, "field": w.field, "value": fmt.Sprint(w.value)},
		}
		about, ok := aboutField(w.field, tested)
		if !ok {
			c.Unsupported = "the expectation is the tested function's result, and no tested function returning bool was found"
			out = append(out, c)
			continue
		}
		c.Payload.AddProse("case_name", caseName)
		c.Payload.Fact("about", about)
		out = append(out, c)
	}
	return out
}

func field(st *types.Struct, name string) *types.Var {
	for i := 0; i < st.NumFields(); i++ {
		if st.Field(i).Name() == name {
			return st.Field(i)
		}
	}
	return nil
}

// aboutField says what an expectation field is about: wantInUse -> "whether in use"; want -> "the
// result of IsExpired" when the test is named after a bool-returning function.
func aboutField(name string, tested *types.Func) (string, bool) {
	if !generic[name] {
		words := facts.Words(name)
		for len(words) > 0 && (words[0] == "want" || words[0] == "expect" || words[0] == "expected") {
			words = words[1:]
		}
		if len(words) > 0 {
			return "whether " + strings.Join(words, " "), true
		}
	}
	if tested == nil {
		return "", false
	}
	res := tested.Type().(*types.Signature).Results()
	for i := 0; i < res.Len(); i++ {
		if b, ok := res.At(i).Type().Underlying().(*types.Basic); ok && b.Kind() == types.Bool {
			return "the result of " + tested.Name(), true
		}
	}
	return "", false
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

// rangeBodies returns the bodies of the loops that range over the table: over its variable, or
// over the literal itself.
func rangeBodies(info *types.Info, body *ast.BlockStmt, lit *ast.CompositeLit, table types.Object) []*ast.BlockStmt {
	var out []*ast.BlockStmt
	ast.Inspect(body, func(n ast.Node) bool {
		rs, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		if ast.Unparen(rs.X) == lit {
			out = append(out, rs.Body)
		} else if id, ok := ast.Unparen(rs.X).(*ast.Ident); ok && table != nil && info.ObjectOf(id) == table {
			out = append(out, rs.Body)
		}
		return true
	})
	return out
}

// testedFunc is the function a table's rows are run through: a function or method called inside a
// loop over the table (subtest closures included), defined in the package under test, whose name the
// test name contains. A helper the loop also calls (New, Equal) is not it unless the test is named
// after it; a table nobody ranges over binds to nothing. The longest such name wins.
func testedFunc(pass *analysis.Pass, fd *ast.FuncDecl, lit *ast.CompositeLit, table types.Object) *types.Func {
	testName := strings.ToLower(fd.Name.Name)
	under := strings.TrimSuffix(pass.Pkg.Path(), "_test")
	var found []*types.Func
	for _, rb := range rangeBodies(pass.TypesInfo, fd.Body, lit, table) {
		collectCalls(pass, rb, testName, under, &found)
	}
	if len(found) == 0 {
		return nil
	}
	sort.SliceStable(found, func(i, j int) bool { return len(found[i].Name()) > len(found[j].Name()) })
	return found[0]
}

func collectCalls(pass *analysis.Pass, body *ast.BlockStmt, testName, under string, found *[]*types.Func) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn := facts.Callee(pass.TypesInfo, call)
		if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != under || len(fn.Name()) < 3 {
			return true
		}
		if strings.Contains(testName, strings.ToLower(fn.Name())) {
			*found = append(*found, fn)
		}
		return true
	})
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
