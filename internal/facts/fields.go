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

// Fields returns the structured fields of a log call: field constructors such as zap.String("k", v),
// slog.Int("k", n) or logf.String("k", v); slog's alternating key, value arguments; and fields added
// along a call chain (logrus WithField("k", v), zerolog Str("k", v)). Arguments it cannot read as
// fields are left out; a printf argument is not a field.
func Fields(info *types.Info, lc LogCall) []LogField {
	var out []LogField
	args := lc.Call.Args
	start := lc.MessageArg + 1
	if lc.MessageArg < 0 {
		start = len(args)
	}
	printf := strings.HasSuffix(lc.Func.Name(), "f")
	for i := start; i < len(args) && !printf; i++ {
		a := ast.Unparen(args[i])
		if f, ok := fieldCall(info, a); ok {
			out = append(out, f)
			continue
		}
		// slog style: "key", value.
		if k, ok := ConstString(info, a); ok && i+1 < len(args) {
			out = append(out, field(info, k, args[i+1]))
			i++
		}
	}
	// Fields along the chain the call is made on.
	x := lc.Call.Fun
	for {
		sel, ok := ast.Unparen(x).(*ast.SelectorExpr)
		if !ok {
			break
		}
		inner, ok := ast.Unparen(sel.X).(*ast.CallExpr)
		if !ok {
			break
		}
		if fn := Callee(info, inner); fn != nil && isLogPackage(fn.Pkg()) && len(inner.Args) == 2 {
			if k, ok := ConstString(info, inner.Args[0]); ok {
				out = append(out, field(info, k, inner.Args[1]))
			}
		}
		x = inner.Fun
	}
	return out
}

// fieldCall reads a field constructor of a logger package: F("key", value) returning a value, or
// zap.Error(err) style constructors whose key is their name.
func fieldCall(info *types.Info, e ast.Expr) (LogField, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return LogField{}, false
	}
	fn := Callee(info, call)
	if fn == nil || !isLogPackage(fn.Pkg()) || fn.Type().(*types.Signature).Results().Len() != 1 {
		return LogField{}, false
	}
	switch len(call.Args) {
	case 1:
		return field(info, strings.ToLower(fn.Name()), call.Args[0]), true
	case 2:
		if k, ok := ConstString(info, call.Args[0]); ok {
			return field(info, k, call.Args[1]), true
		}
	}
	return LogField{}, false
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
