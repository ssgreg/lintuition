package facts

import (
	"go/ast"
	"go/types"
	"regexp"
	"strings"
)

// factRE mirrors the payload policy's check of a structural fact string, so an extractor can tell
// in advance that a fact would be refused and make the candidate unsupported instead.
var factRE = regexp.MustCompile(`^[A-Za-z0-9_ .,/()-]{0,120}$`)

// FactSafe reports whether s can be sent as a structural fact.
func FactSafe(s string) bool { return factRE.MatchString(s) }

// FieldFact describes a log field as a structural fact, from its key, the identifiers its value
// comes from and its type, never the value itself: "key password, value cfg.Password, type
// string". The type is left out when it cannot be written in a fact ([]byte, map[string]any); false
// means the key or the value description cannot be.
func FieldFact(f LogField) (string, bool) {
	s := "key " + f.Key + ", value " + f.ValueDesc
	if t := typeFact(f.Type); t != "" {
		s += ", type " + t
	}
	return s, FactSafe(s)
}

// typeFact rewrites a type string into fact characters: *pkg.T -> pointer to pkg.T, []byte ->
// slice of byte; anything else that does not fit is dropped.
func typeFact(t string) string {
	switch {
	case strings.HasPrefix(t, "*"):
		if in := typeFact(t[1:]); in != "" {
			return "pointer to " + in
		}
		return ""
	case strings.HasPrefix(t, "[]"):
		if in := typeFact(t[2:]); in != "" {
			return "slice of " + in
		}
		return ""
	}
	if FactSafe(t) {
		return t
	}
	return ""
}

// EnclosingFunc names the outermost function declaration on an inspector stack ((T).M or F), with
// "/func" for each closure inside it; "" at package level.
func EnclosingFunc(stack []ast.Node) string {
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
