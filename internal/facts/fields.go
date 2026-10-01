package facts

import (
	"go/ast"
	"go/types"
	"strings"
)

// LogField is one structured field of a log call: a key and the value logged under it. The value is
// described by its shape, never by its content: a literal's text is not kept.
type LogField struct {
	Key string
	// Value is the value expression, for local checks only.
	Value ast.Expr
	// ValueDesc names where the value comes from, from identifiers only: "tst.ArchivePassword",
	// "the result of Redact", "a literal", "an expression".
	ValueDesc string
	// Type is the value's type, package-qualified by name: "string", "time.Duration".
	Type string
	// Literal is true when the value is a constant written in the code.
	Literal bool
}

// Fields returns the structured fields of a log call, and whether some arguments could not be read
// as fields (partial). Fields are read only where the logger defines them:
//   - field constructors returning a field type (slog.Attr, zap.Field, logf.Field): slog.String("k", v),
//     zap.Error(err), logf.String("k", v); slog.Group("g", ...) contributes its fields as g.k;
//   - alternating key, value arguments, only for log/slog and zap's sugared *w methods;
//   - fields added along a call chain: slog.With(...), logrus WithField("k", v), zerolog Str("k", v).
//
// The arguments of Print, Println, logrus.Info(args ...) and printf formats are not fields.
func Fields(info *types.Info, lc LogCall) (fields []LogField, partial bool) {
	// The calls the logging call is made on, innermost first: slog.With(...).WithGroup("g").Info(...)
	// gives With, WithGroup. Groups apply to what is added after them, as at run time.
	var chain []*ast.CallExpr
	for x := lc.Call.Fun; ; {
		sel, ok := ast.Unparen(x).(*ast.SelectorExpr)
		if !ok {
			break
		}
		inner, ok := ast.Unparen(sel.X).(*ast.CallExpr)
		if !ok {
			break
		}
		chain = append([]*ast.CallExpr{inner}, chain...)
		x = inner.Fun
	}
	prefix := ""
	for _, inner := range chain {
		fn := Callee(info, inner)
		if fn == nil || !isLogPackage(fn.Pkg()) {
			continue
		}
		switch {
		case fn.Name() == "WithGroup":
			g, ok := "", len(inner.Args) == 1
			if ok {
				g, ok = ConstString(info, inner.Args[0])
			}
			if !ok {
				partial = true // a group whose name is not known: the keys cannot be told
				continue
			}
			if g != "" {
				prefix += g + "."
			}
		case fn.Name() == "With":
			f, p := readArgs(info, inner.Args, alternates(fn), prefix)
			fields, partial = append(fields, f...), partial || p
		case len(inner.Args) == 2:
			if k, ok := ConstString(info, inner.Args[0]); ok {
				fields = append(fields, field(info, prefix+k, inner.Args[1]))
			}
		}
	}
	if lc.MessageArg >= 0 && !strings.HasSuffix(lc.Func.Name(), "f") && structured(lc.Func) {
		f, p := readArgs(info, lc.Call.Args[lc.MessageArg+1:], alternates(lc.Func), prefix)
		fields, partial = append(fields, f...), partial || p
	}
	return fields, partial
}

// structured reports whether a logging function takes fields after its message: alternating key,
// value arguments, or a variadic parameter of a field type. Print(v ...any) and logrus.Info(args
// ...any) take values to print, not fields.
func structured(fn *types.Func) bool {
	if alternates(fn) {
		return true
	}
	sig := fn.Type().(*types.Signature)
	if !sig.Variadic() {
		return false
	}
	last := sig.Params().At(sig.Params().Len() - 1).Type()
	sl, ok := last.(*types.Slice)
	return ok && isFieldType(sl.Elem())
}

// alternates reports whether a logging function takes alternating key, value arguments.
func alternates(fn *types.Func) bool {
	if fn.Pkg() == nil {
		return false
	}
	switch fn.Pkg().Path() {
	case "log/slog":
		return true
	case "go.uber.org/zap":
		return strings.HasSuffix(fn.Name(), "w") || fn.Name() == "With"
	}
	return false
}

func readArgs(info *types.Info, args []ast.Expr, alternating bool, prefix string) (out []LogField, partial bool) {
	for i := 0; i < len(args); i++ {
		a := ast.Unparen(args[i])
		if fs, ok := fieldCall(info, a, prefix); ok {
			out = append(out, fs...)
			continue
		}
		if alternating {
			if k, ok := ConstString(info, a); ok && i+1 < len(args) {
				out = append(out, field(info, prefix+k, args[i+1]))
				i++
				continue
			}
		}
		partial = true
	}
	return out, partial
}

// fieldTypes are the types a field constructor returns, by package and name.
var fieldTypes = map[string]bool{
	"log/slog.Attr": true, "go.uber.org/zap/zapcore.Field": true, "go.uber.org/zap.Field": true,
	"github.com/ssgreg/logf.Field": true,
}

// fieldCall reads a field constructor: F("key", value) or zap.Error(err) style constructors whose key
// is their name, returning a field type; slog.Group adds its fields under its key.
func fieldCall(info *types.Info, e ast.Expr, prefix string) ([]LogField, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	fn := Callee(info, call)
	if fn == nil || !isLogPackage(fn.Pkg()) {
		return nil, false
	}
	res := fn.Type().(*types.Signature).Results()
	if res.Len() != 1 || !isFieldType(res.At(0).Type()) {
		return nil, false
	}
	if fn.Pkg().Path() == "log/slog" && fn.Name() == "Group" && len(call.Args) >= 1 {
		g, ok := ConstString(info, call.Args[0])
		if !ok {
			return nil, false
		}
		// An empty group name inlines its fields, as slog does.
		inner := prefix
		if g != "" {
			inner = prefix + g + "."
		}
		var out []LogField
		for _, a := range call.Args[1:] {
			fs, ok := fieldCall(info, ast.Unparen(a), inner)
			if !ok {
				return nil, false
			}
			out = append(out, fs...)
		}
		return out, true
	}
	switch len(call.Args) {
	case 1:
		return []LogField{field(info, prefix+strings.ToLower(fn.Name()), call.Args[0])}, true
	case 2:
		if k, ok := ConstString(info, call.Args[0]); ok {
			return []LogField{field(info, prefix+k, call.Args[1])}, true
		}
	}
	return nil, false
}

func isFieldType(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || n.Obj().Pkg() == nil {
		return false
	}
	return fieldTypes[n.Obj().Pkg().Path()+"."+n.Obj().Name()]
}

func field(info *types.Info, key string, v ast.Expr) LogField {
	f := LogField{Key: key, Value: v, ValueDesc: Describe(info, v)}
	if t := info.TypeOf(v); t != nil {
		f.Type = types.TypeString(t, func(p *types.Package) string { return p.Name() })
	}
	if tv, ok := info.Types[v]; ok && tv.Value != nil {
		f.Literal = true
	}
	return f
}

// Describe names where an expression's value comes from using identifiers only, never literal
// content: x, cfg.Password, the result of Redact, a literal, an expression.
func Describe(info *types.Info, e ast.Expr) string {
	e = ast.Unparen(e)
	if tv, ok := info.Types[e]; ok && tv.Value != nil {
		return "a literal"
	}
	if s, ok := selectorPath(e); ok {
		return s
	}
	switch e := e.(type) {
	case *ast.CallExpr:
		if fn := Callee(info, e); fn != nil {
			return "the result of " + fn.Name()
		}
		if s, ok := selectorPath(e.Fun); ok {
			return "the result of " + s
		}
	case *ast.UnaryExpr:
		return Describe(info, e.X)
	case *ast.StarExpr:
		return Describe(info, e.X)
	case *ast.IndexExpr:
		if s, ok := selectorPath(e.X); ok {
			return "an element of " + s
		}
	}
	return "an expression"
}

// selectorPath renders an identifier or a chain of field selections: a, a.b, a.b.c.
func selectorPath(e ast.Expr) (string, bool) {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return e.Name, true
	case *ast.SelectorExpr:
		if s, ok := selectorPath(e.X); ok {
			return s + "." + e.Sel.Name, true
		}
	}
	return "", false
}
