// Package humanunit finds a printf-style message whose wording names one unit for a number while
// the code passes the number in another:
//
//	log.Printf("request took %.0f ms", elapsed.Seconds()) // the value is in seconds
//
// Code finds the shape and knows the unit: a call of a printf-style function (its name ends in f,
// its last two parameters are a format string and ...any, such as fmt.Printf, log.Printf, a
// logger's Infof or fmt.Errorf) whose argument for a verb is a call of time.Duration's Seconds,
// Milliseconds, Microseconds, Nanoseconds, Minutes or Hours, possibly under a conversion to a basic
// numeric type or math.Round, Floor, Ceil or Trunc. A conversion to a named type is not followed: its
// String or Format method may print another number. Each verb is bound to its argument by position,
// as fmt binds it. The classifier reads only the format and the verb, and says which unit the
// wording gives that number; Go code compares it with the method called. A Duration printed with %v
// or %s prints its own unit and is not asked. A format built at run time, an explicit argument
// index or a verb that cannot be bound is unsupported.
package humanunit

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "human-unit-contradiction"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of the stated unit for a decision either way (default
	// 0.85, from the prototype; not yet validated on a labelled set).
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts printf verbs bound to a time.Duration unit method.
var Analyzer = &analysis.Analyzer{
	Name:       "humanunit",
	Doc:        "extract printf verbs whose argument is a time.Duration converted to a unit",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a printf message names a different unit than the duration value it prints",
		Standard:    true,
		Version:     "2",
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

// units maps time.Duration's methods to the unit they return.
var units = map[string]string{
	"Hours":        "hours",
	"Minutes":      "minutes",
	"Seconds":      "seconds",
	"Milliseconds": "milliseconds",
	"Microseconds": "microseconds",
	"Nanoseconds":  "nanoseconds",
}

var rounding = map[string]bool{"Round": true, "Floor": true, "Ceil": true, "Trunc": true}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	var out []*sdk.Candidate
	ins.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		for _, c := range candidates(pass, n.(*ast.CallExpr)) {
			c.Subject = enclosing(stack) + "/" + c.Subject
			out = append(out, c)
		}
		return true
	})
	return out, nil
}

// enclosing names the outermost function declaration on the stack ((T).M or F), with "/func" for
// each closure inside it.
func enclosing(stack []ast.Node) string {
	name := ""
	for _, n := range stack {
		switch n := n.(type) {
		case *ast.FuncDecl:
			name = n.Name.Name
			if n.Recv != nil && len(n.Recv.List) > 0 {
				name = "(" + types.ExprString(n.Recv.List[0].Type) + ")." + name
			}
		case *ast.FuncLit:
			name += "/func"
		}
	}
	return name
}

// formatIndex returns the index of the format parameter of a printf-style function: its name ends
// in f, and its last two parameters are a string and ...any.
func formatIndex(fn *types.Func) (int, bool) {
	if fn == nil || !strings.HasSuffix(fn.Name(), "f") {
		return 0, false
	}
	sig := fn.Type().(*types.Signature)
	n := sig.Params().Len()
	if !sig.Variadic() || n < 2 {
		return 0, false
	}
	sl, ok := sig.Params().At(n - 1).Type().(*types.Slice)
	if !ok {
		return 0, false
	}
	if it, ok := sl.Elem().Underlying().(*types.Interface); !ok || !it.Empty() {
		return 0, false
	}
	if b, ok := sig.Params().At(n - 2).Type().Underlying().(*types.Basic); !ok || b.Kind() != types.String {
		return 0, false
	}
	return n - 2, true
}

func candidates(pass *analysis.Pass, call *ast.CallExpr) []*sdk.Candidate {
	fn := facts.Callee(pass.TypesInfo, call)
	fi, ok := formatIndex(fn)
	if !ok || call.Ellipsis.IsValid() || len(call.Args) <= fi {
		return nil
	}
	args := call.Args[fi+1:]
	// The unit arguments, by position among the format's arguments.
	unitAt := map[int]string{}
	for i, a := range args {
		if u, ok := unitOf(pass.TypesInfo, a); ok {
			unitAt[i] = u
		}
	}
	if len(unitAt) == 0 {
		return nil
	}
	unsupported := func(i int, why string) *sdk.Candidate {
		return &sdk.Candidate{
			Pos:         pass.Fset.Position(args[i].Pos()),
			Subject:     fmt.Sprintf("%s/arg%d", fn.Name(), i+1),
			Unsupported: why,
		}
	}
	var out []*sdk.Candidate
	format, known := facts.ConstString(pass.TypesInfo, call.Args[fi])
	var verbs []verb
	var perr string
	if known {
		verbs, perr = parseVerbs(format)
	}
	bound := map[int]bool{}
	for k, v := range verbs {
		if perr != "" {
			break
		}
		u, ok := unitAt[v.arg]
		if !ok {
			continue
		}
		bound[v.arg] = true
		c := &sdk.Candidate{
			Pos:     pass.Fset.Position(args[v.arg].Pos()),
			Subject: fmt.Sprintf("%s/verb%d", fn.Name(), k+1),
			Local:   map[string]string{"unit": u, "format": format, "verb": v.text},
		}
		desc := v.text
		if len(verbs) > 1 {
			desc = fmt.Sprintf("%s (verb %d of %d)", v.text, k+1, len(verbs))
		}
		c.Payload.AddProse("format", format)
		c.Payload.AddProse("verb", desc)
		out = append(out, c)
	}
	for i := range args {
		if _, ok := unitAt[i]; !ok || bound[i] {
			continue
		}
		switch {
		case !known:
			out = append(out, unsupported(i, "the format is not a constant string"))
		case perr != "":
			out = append(out, unsupported(i, perr))
		default:
			out = append(out, unsupported(i, "no verb of the format is bound to this argument"))
		}
	}
	return out
}

// unitOf returns the unit of an argument that is a call of a time.Duration unit method, through
// numeric conversions and math rounding.
func unitOf(info *types.Info, e ast.Expr) (string, bool) {
	for {
		call, ok := ast.Unparen(e).(*ast.CallExpr)
		if !ok {
			return "", false
		}
		if tv, ok := info.Types[call.Fun]; ok && tv.IsType() {
			// Only a conversion to a basic numeric type prints the same number: float64(d.Seconds()).
			// A named type may have its own String or Format method, and string(n) is not a number.
			b, basic := types.Unalias(tv.Type).(*types.Basic)
			if !basic || b.Info()&types.IsNumeric == 0 || len(call.Args) != 1 {
				return "", false
			}
			e = call.Args[0]
			continue
		}
		fn := facts.Callee(info, call)
		if fn == nil || fn.Pkg() == nil {
			return "", false
		}
		if fn.Pkg().Path() == "math" && rounding[fn.Name()] && len(call.Args) == 1 {
			e = call.Args[0]
			continue
		}
		sig := fn.Type().(*types.Signature)
		if sig.Recv() == nil || fn.Pkg().Path() != "time" || !isDuration(sig.Recv().Type()) {
			return "", false
		}
		u, ok := units[fn.Name()]
		return u, ok
	}
}

func isDuration(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "time" && n.Obj().Name() == "Duration"
}

// verb is one formatting verb of a format and the index of the argument it prints.
type verb struct {
	text string
	arg  int
}

// parseVerbs returns the verbs of a printf format in order, each with the argument it consumes,
// following fmt's own scanning (doPrintf): flags, then a width (* consumes an argument), then a
// precision when a '.' is not the last byte, then the verb rune. A percent verb, decorated or not
// (%5%), prints '%' and consumes no argument. A format that ends inside a verb prints %!(NOVERB)
// and consumes nothing, so the verbs before it stay bound. An explicit argument index makes the
// binding something this parser does not follow, and returns a reason instead.
func parseVerbs(format string) ([]verb, string) {
	const index = "the format uses an explicit argument index"
	var out []verb
	arg := 0
	end := len(format)
	for i := 0; i < end; {
		if format[i] != '%' {
			i++
			continue
		}
		start := i
		i++
		for i < end && strings.IndexByte("#0+- ", format[i]) >= 0 {
			i++
		}
		if i < end && format[i] == '[' {
			return nil, index
		}
		if i < end && format[i] == '*' {
			arg++
			i++
		} else {
			for i < end && format[i] >= '0' && format[i] <= '9' {
				i++
			}
		}
		if i+1 < end && format[i] == '.' {
			i++
			if format[i] == '[' {
				return nil, index
			}
			if format[i] == '*' {
				arg++
				i++
			} else {
				for i < end && format[i] >= '0' && format[i] <= '9' {
					i++
				}
			}
		}
		if i < end && format[i] == '[' {
			return nil, index
		}
		if i >= end {
			break // %!(NOVERB)
		}
		r, size := utf8.DecodeRuneInString(format[i:])
		i += size
		if r == '%' {
			continue
		}
		out = append(out, verb{text: format[start:i], arg: arg})
		arg++
	}
	return out, ""
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "unit",
		Kind: sdk.Choice,
		Text: "Which unit does the wording of `format` say the number next to the verb `verb` is in?",
		Options: []sdk.Option{
			{Key: "seconds", Description: "Seconds (s, sec, seconds)."},
			{Key: "milliseconds", Description: "Milliseconds (ms, msec)."},
			{Key: "microseconds", Description: "Microseconds (us, µs)."},
			{Key: "nanoseconds", Description: "Nanoseconds (ns)."},
			{Key: "minutes", Description: "Minutes (m, min)."},
			{Key: "hours", Description: "Hours (h, hr)."},
			{Key: "unspecified", Description: "The wording names no unit for that number."},
		},
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["unit"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the wording does not let the classifier tell the unit")
	}
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	// Weak support abstains whichever way the answer goes.
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice == "unspecified" || a.Choice == c.Local["unit"] {
		return sdk.Clean()
	}
	return sdk.Report("text says %s, the value is in %s: %q", a.Choice, c.Local["unit"], clip(c.Local["format"], 80))
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
