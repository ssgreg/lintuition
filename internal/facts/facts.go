// Package facts holds typed extraction helpers shared by the built-in linters. Every helper resolves
// identifiers through go/types, never by spelling: a variable named err is not an error, and a
// renamed error is still one.
package facts

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"
	"unicode"

	"golang.org/x/tools/go/types/typeutil"
)

var errorType = types.Universe.Lookup("error").Type()

// IsError reports whether t is the error interface.
func IsError(t types.Type) bool { return t != nil && types.Identical(t, errorType) }

// Callee returns the function or method a call statically calls, or nil (a func value, a conversion,
// a builtin).
func Callee(info *types.Info, call *ast.CallExpr) *types.Func {
	fn, _ := typeutil.Callee(info, call).(*types.Func)
	return fn
}

// ErrorSlots returns the indices of the error results of a signature.
func ErrorSlots(sig *types.Signature) []int {
	var out []int
	for i := 0; i < sig.Results().Len(); i++ {
		if IsError(sig.Results().At(i).Type()) {
			out = append(out, i)
		}
	}
	return out
}

// ConstString returns the constant string value of an expression, through constants and
// concatenation, and false when it is not a constant string.
func ConstString(info *types.Info, e ast.Expr) (string, bool) {
	tv, ok := info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// ConstBool returns the constant bool value of an expression, through named constants.
func ConstBool(info *types.Info, e ast.Expr) (bool, bool) {
	tv, ok := info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Bool {
		return false, false
	}
	return constant.BoolVal(tv.Value), true
}

// Fallible is a statement whose call can fail: it returns an error the statement keeps.
type Fallible struct {
	Callee *types.Func
	Call   *ast.CallExpr
}

// FallibleStmt returns the fallible call of a statement, if the statement is one of:
//
//	err := f()            x, err = f()          (any name; the slot type decides)
//	if err := f(); ...    (the init statement)
//	return f()            (the caller gets the error)
//
// A call whose error result is discarded with _ or not assigned is not fallible here: the code
// does not look at the failure, so a log line before it claims nothing the code checks.
func FallibleStmt(info *types.Info, s ast.Stmt) (Fallible, bool) {
	switch s := s.(type) {
	case *ast.IfStmt:
		if s.Init != nil {
			return FallibleStmt(info, s.Init)
		}
	case *ast.AssignStmt:
		if len(s.Rhs) != 1 {
			return Fallible{}, false
		}
		call, ok := ast.Unparen(s.Rhs[0]).(*ast.CallExpr)
		if !ok {
			return Fallible{}, false
		}
		fn := Callee(info, call)
		if fn == nil {
			return Fallible{}, false
		}
		sig := fn.Type().(*types.Signature)
		for _, i := range ErrorSlots(sig) {
			if i < len(s.Lhs) && !isBlank(s.Lhs[i]) {
				return Fallible{Callee: fn, Call: call}, true
			}
		}
	case *ast.ReturnStmt:
		if len(s.Results) != 1 {
			return Fallible{}, false
		}
		call, ok := ast.Unparen(s.Results[0]).(*ast.CallExpr)
		if !ok {
			return Fallible{}, false
		}
		if fn := Callee(info, call); fn != nil && len(ErrorSlots(fn.Type().(*types.Signature))) > 0 {
			return Fallible{Callee: fn, Call: call}, true
		}
	}
	return Fallible{}, false
}

func isBlank(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "_"
}

// Words splits an identifier into lower-case words: SaveConfigCopy -> save, config, copy;
// HTTPServer -> http, server.
func Words(ident string) []string {
	var out []string
	rs := []rune(ident)
	start := 0
	flush := func(end int) {
		if end > start {
			out = append(out, strings.ToLower(string(rs[start:end])))
		}
		start = end
	}
	for i := 1; i < len(rs); i++ {
		prev, cur := rs[i-1], rs[i]
		next := rune(0)
		if i+1 < len(rs) {
			next = rs[i+1]
		}
		switch {
		case cur == '_' || cur == '-':
			flush(i)
			start = i + 1
		case unicode.IsLower(prev) && unicode.IsUpper(cur):
			flush(i)
		case unicode.IsUpper(prev) && unicode.IsUpper(cur) && unicode.IsLower(next):
			flush(i)
		case unicode.IsLetter(prev) != unicode.IsLetter(cur) && cur != '_' && prev != '_':
			flush(i)
		}
	}
	flush(len(rs))
	return out
}

// IsTest reports whether fd is a top-level TestXxx function in a _test.go file.
func IsTest(fset *token.FileSet, fd *ast.FuncDecl) bool {
	if !strings.HasPrefix(fd.Name.Name, "Test") || fd.Recv != nil {
		return false
	}
	return strings.HasSuffix(fset.Position(fd.Pos()).Filename, "_test.go")
}
