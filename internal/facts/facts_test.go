package facts

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestWords(t *testing.T) {
	for in, want := range map[string]string{
		"SaveConfigCopy": "save config copy",
		"HTTPServer":     "http server",
		"wantInUse":      "want in use",
		"want_in_use":    "want in use",
		"IOSeconds2":     "io seconds 2",
		"x":              "x",
	} {
		if got := strings.Join(Words(in), " "); got != want {
			t.Errorf("Words(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLevel(t *testing.T) {
	for in, want := range map[string]string{
		"Infof": "info", "Infow": "info", "InfoContext": "info", "Infox": "info", "Println": "unleveled",
		"Warning": "warn", "Errorf": "error", "Fatalln": "fatal", "DPanic": "panic", "Trace": "debug",
		"Log": "", "Info2": "", "Sync": "",
	} {
		got, _ := level(in)
		if got != want {
			t.Errorf("level(%q) = %q, want %q", in, got, want)
		}
	}
}

// check type-checks src (package p, standard library imports only) and returns the info.
func check(t *testing.T, src string) (*ast.File, *types.Info) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("p", fset, []*ast.File{f}, info); err != nil {
		t.Fatal(err)
	}
	return f, info
}

func TestAsLogCall(t *testing.T) {
	f, info := check(t, `package p
import ("log"; "log/slog"; "context")
const m = "con" + "stant"
func g(ctx context.Context, l *slog.Logger, s string) {
	log.Printf("printf %d", 1)
	log.Println()
	log.Print(m)
	log.Print(s)
	slog.Info("slog info")
	slog.ErrorContext(ctx, "slog error")
	l.Warn("method warn")
	slog.Log(ctx, slog.LevelInfo, "unknown level")
	_ = slog.String("k", "v")
}`)
	var got []string
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if lc, ok := AsLogCall(info, c); ok {
				msg := "<dynamic>"
				if lc.MessageKnown {
					msg = lc.Message
				}
				got = append(got, lc.Level+" "+msg)
			}
		}
		return true
	})
	want := "unleveled printf %d\nunleveled <dynamic>\nunleveled constant\nunleveled <dynamic>\ninfo slog info\nerror slog error\nwarn method warn"
	if strings.Join(got, "\n") != want {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), want)
	}
}

func TestFallibleStmt(t *testing.T) {
	f, info := check(t, `package p
type C struct{}
func (C) Do() error { return nil }
func (C) Two() (int, error) { return 0, nil }
func (C) N() int { return 0 }
func g(c C) error {
	err := c.Do()        // yes
	failure := c.Do()    // yes
	_, err = c.Two()     // yes
	n, _ := c.Two()      // no
	err2 := c.N()        // no
	_ = c.Do()           // no
	c.Do()               // no
	if err := c.Do(); err != nil { return err } // yes
	_, _, _, _ = failure, n, err2, err
	return c.Do()        // yes
}`)
	fset := token.NewFileSet()
	_ = fset
	var got []bool
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "g" {
			continue
		}
		for _, s := range fd.Body.List {
			if a, ok := s.(*ast.AssignStmt); ok && a.Tok == token.ASSIGN && len(a.Lhs) == 4 {
				continue // the _, _, _, _ = line
			}
			_, ok := FallibleStmt(info, s)
			got = append(got, ok)
		}
	}
	want := []bool{true, true, true, false, false, false, false, true, true}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("statement %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestIsLogPackage(t *testing.T) {
	for path, want := range map[string]bool{
		"log": true, "log/slog": true, "go.uber.org/zap": true, "github.com/ssgreg/logf": true,
		"example.com/internal/logging": true, "example.com/applogger": true, "k8s.io/klog": true,
		"example.com/logic": false, "example.com/login": false, "example.com/catalog": false, "example.com/dialog": false,
	} {
		name := path[strings.LastIndex(path, "/")+1:]
		if got := isLogPackage(types.NewPackage(path, name)); got != want {
			t.Errorf("isLogPackage(%q) = %v, want %v", path, got, want)
		}
	}
}
