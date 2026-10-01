package facts

import (
	"go/ast"
	"strings"
	"testing"
)

func TestHasWrapVerb(t *testing.T) {
	for f, want := range map[string]bool{"a: %w": true, "%[1]w": true, "%+w": true, "%%w": false, "100%% done": false, "%v": false} {
		if got := hasWrapVerb(f); got != want {
			t.Errorf("hasWrapVerb(%q) = %v", f, got)
		}
	}
}

func TestErrorCallWraps(t *testing.T) {
	f, info := check(t, `package p
import ("errors"; "fmt")
func g(err error) {
	_ = errors.New("%w")
	_ = fmt.Errorf("%%w")
	_ = fmt.Errorf("x: %[1]w", err)
	_ = fmt.Errorf("x: %v", err)
}`)
	var got []bool
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if ec, ok := AsErrorCall(info, c); ok {
				got = append(got, ec.Wraps)
			}
		}
		return true
	})
	if len(got) != 4 || got[0] || got[1] || !got[2] || got[3] {
		t.Fatalf("wraps: %v, want [false false true false]", got)
	}
}

func TestFields(t *testing.T) {
	f, info := check(t, `package p
import ("log"; "log/slog")
func g(l *slog.Logger, password, user string, n int) {
	log.Println("password", password)
	slog.Info("login", "user", user, slog.Int("n", n))
	slog.Info("value", slog.StringValue(user))
	slog.Info("grouped", slog.Group("auth", slog.String("password", password)))
	slog.With("user", user).Info("chained")
}`)
	var got []string
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if lc, ok := AsLogCall(info, c); ok {
				fs, partial := Fields(info, lc)
				var ks []string
				for _, f := range fs {
					ks = append(ks, f.Key+"="+f.ValueDesc)
				}
				got = append(got, lc.Message+": "+strings.Join(ks, ",")+map[bool]string{true: " (partial)"}[partial])
			}
		}
		return true
	})
	want := []string{
		"password: ", // Println's arguments are not fields
		"login: user=user,n=n",
		"value:  (partial)", // a slog.Value is not a field
		"grouped: auth.password=password",
		"chained: user=user",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
