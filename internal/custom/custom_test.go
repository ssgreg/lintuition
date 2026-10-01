package custom

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ManifestName)
	os.WriteFile(p, []byte(body), 0o600)
	return p
}

func TestManifestValidation(t *testing.T) {
	for body, want := range map[string]string{
		"plugins: [{module: a.com/x, version: v1.0.0}]\n":                           "exactly one of version and path",
		"version: v0.1.0\npath: .\nplugins: [{module: a.com/x, version: v1.0.0}]\n": "exactly one of version and path",
		"version: latest\nplugins: [{module: a.com/x, version: v1.0.0}]\n":          "semantic version",
		"version: v0.1.0\n": "list at least one",
		"version: v0.1.0\nplugins: [{module: a.com/x}]\n":                                              "exactly one of version and path",
		"version: v0.1.0\nplugins: [{module: a.com/x, version: v1.0.0}, {module: a.com/x, path: .}]\n": "listed twice",
		"version: v0.1.0\nplugins: [{module: a.com/x, import: b.com/y, version: v1.0.0}]\n":            "not inside module",
		"version: v0.1.0\nplugins: [{module: a.com/x, version: v1.0.0, extra: 1}]\n":                   "field extra not found",
		"version: v0.1.0\nname: ../evil\nplugins: [{module: a.com/x, version: v1.0.0}]\n":              "name",
	} {
		if _, err := Load(write(t, body)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want %q", body, err, want)
		}
	}
}

func TestGenerate(t *testing.T) {
	m, err := Load(write(t, "version: v0.1.0\nplugins:\n  - {module: example.com/p, import: example.com/p/lint, version: v1.2.3}\n  - {module: example.com/q, path: /src/q}\n"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := m.Generate(dir); err != nil {
		t.Fatal(err)
	}
	mod, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
	main, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	for _, want := range []string{"github.com/ssgreg/lintuition v0.1.0", "example.com/p v1.2.3", `replace example.com/q => "/src/q"`} {
		if !strings.Contains(string(mod), want) {
			t.Errorf("go.mod lacks %q:\n%s", want, mod)
		}
	}
	for _, want := range []string{`_ "example.com/p/lint"`, `_ "example.com/q"`, `_ "github.com/ssgreg/lintuition/builtin"`, `"example.com/p v1.2.3"`} {
		if !strings.Contains(string(main), want) {
			t.Errorf("main.go lacks %q:\n%s", want, main)
		}
	}
}

// TestBuildWithExamplePlugins builds a real custom binary from the two example plugin modules and
// runs it: the external linter asks the external classifier, through the public sdk only.
func TestBuildWithExamplePlugins(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}
	root, _ := filepath.Abs("../..")
	dest := t.TempDir()
	m, err := Load(write(t, "path: "+root+"\ndestination: "+dest+"\nplugins:\n"+
		"  - {module: example.com/lintuition-todo-owner, path: "+filepath.Join(root, "examples/plugins/todoowner")+"}\n"+
		"  - {module: example.com/lintuition-keywords, path: "+filepath.Join(root, "examples/plugins/keywords")+"}\n"))
	if err != nil {
		t.Fatal(err)
	}
	var log strings.Builder
	bin, err := m.Build(context.Background(), &log)
	if err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	out, err := exec.Command(bin, "version", "--json").Output()
	if err != nil {
		t.Fatal(err)
	}
	var v struct{ Linters, Classifiers, Plugins []string }
	json.Unmarshal(out, &v)
	if !strings.Contains(strings.Join(v.Linters, ","), "todo-owner") || !strings.Contains(strings.Join(v.Classifiers, ","), "keywords") || len(v.Plugins) != 2 {
		t.Fatalf("version: %s", out)
	}
	cmd := exec.Command(bin, "run", "./...")
	cmd.Dir = filepath.Join(root, "testdata/plugins")
	res, err := cmd.CombinedOutput()
	if code := cmd.ProcessState.ExitCode(); code != 1 || !strings.Contains(string(res), "todo.go:3:1: TODO names no owner, ticket or date (todo-owner)") || strings.Contains(string(res), "todo.go:6") {
		t.Fatalf("exit %d\n%s", code, res)
	}
}
