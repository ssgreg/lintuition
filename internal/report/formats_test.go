package report

import (
	"bytes"
	"encoding/json"
	"go/token"
	"reflect"
	"testing"
)

func TestGitHubActionsEscaping(t *testing.T) {
	var b bytes.Buffer
	is := []Issue{{FromLinter: "a,b:c", Text: "line one\n::error::injected 100%", Pos: token.Position{Filename: "dir,x/f:1.go", Line: 3, Column: 4}, Severity: "error"}}
	if err := writeGitHubActions(&b, is, Run{}); err != nil {
		t.Fatal(err)
	}
	want := "::error file=dir%2Cx/f%3A1.go,line=3,col=4,title=a%2Cb%3Ac::line one%0A::error::injected 100%25\n"
	if b.String() != want {
		t.Fatalf("got  %q\nwant %q", b.String(), want)
	}
}

func TestSARIFLocations(t *testing.T) {
	for in, want := range map[string]string{
		"metrics.go":       "metrics.go",
		"dir/metric #%.go": "dir/metric%20%23%25.go",
		"/abs/a b.go":      "file:///abs/a%20b.go",
	} {
		if got := sarifURI(in); got != want {
			t.Errorf("sarifURI(%q) = %q, want %q", in, got, want)
		}
	}
	is := Issue{Pos: token.Position{Column: 13}, SourceLines: []string{"var café = prometheus.CounterOpts{}"}}
	if got := utf16Column(is); got != 12 {
		t.Errorf("utf16 column %d, want 12 (é is two bytes, one UTF-16 unit)", got)
	}
	is = Issue{Pos: token.Position{Column: 7}, SourceLines: []string{"x := \"😀\" + y"}}
	if got := utf16Column(is); got != 7 {
		t.Errorf("utf16 column before the emoji %d, want 7", got)
	}
	is = Issue{Pos: token.Position{Column: 12}, SourceLines: []string{"x := \"😀\" + y"}}
	if got := utf16Column(is); got != 10 {
		t.Errorf("utf16 column after the emoji %d, want 10 (4 bytes, 2 units)", got)
	}
}

func TestSARIFEmptyRulesIsAnArray(t *testing.T) {
	var b bytes.Buffer
	if err := writeSARIF(&b, nil, Run{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b.Bytes(), []byte(`"rules": []`)) || bytes.Contains(b.Bytes(), []byte("null")) {
		t.Fatalf("rules must be an empty array:\n%s", b.String())
	}
}

// markBuiltin marks a linter built-in for one test, as package builtin does for the real ones.
func markBuiltin(t *testing.T, linter string) {
	t.Helper()
	MarkBuiltin(linter)
	t.Cleanup(func() { delete(builtins, linter) })
}

func TestGitHubActionsDocLink(t *testing.T) {
	markBuiltin(t, "premature-success")
	var b bytes.Buffer
	run := Run{Linters: []LinterStatus{{Name: "premature-success", DocURL: DocURL("v0.2.0", "premature-success")}}}
	is := []Issue{
		{FromLinter: "premature-success", Text: "says saved\n::error::injected", Pos: token.Position{Filename: "f.go", Line: 3, Column: 4}},
		{FromLinter: "todo-owner", Text: "no owner", Pos: token.Position{Filename: "f.go", Line: 5, Column: 1}},
	}
	if err := writeGitHubActions(&b, is, run); err != nil {
		t.Fatal(err)
	}
	want := "::warning file=f.go,line=3,col=4,title=premature-success::says saved%0A::error::injected" +
		"%0Ahttps://github.com/ssgreg/lintuition/blob/v0.2.0/docs/linters.md#premature-success\n" +
		"::warning file=f.go,line=5,col=1,title=todo-owner::no owner\n"
	if b.String() != want {
		t.Fatalf("got  %q\nwant %q", b.String(), want)
	}
}

func TestSARIFHelpURI(t *testing.T) {
	markBuiltin(t, "doc-vs-signature")
	var b bytes.Buffer
	run := Run{Linters: []LinterStatus{
		{Name: "doc-vs-signature", Enabled: true, DocURL: DocURL("v0.2.0", "doc-vs-signature")},
		{Name: "todo-owner", Enabled: true, DocURL: DocURL("v0.2.0", "todo-owner")},
	}}
	is := []Issue{{FromLinter: "unknown", Text: "x", Pos: token.Position{Filename: "f.go", Line: 1}}}
	if err := writeSARIF(&b, is, run); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Runs []struct {
			Tool struct {
				Driver struct {
					Rules []map[string]any
				}
			}
		}
	}
	if err := json.Unmarshal(b.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	got := map[string]any{}
	for _, r := range doc.Runs[0].Tool.Driver.Rules {
		got[r["id"].(string)] = r["helpUri"]
	}
	want := map[string]any{
		"doc-vs-signature": "https://github.com/ssgreg/lintuition/blob/v0.2.0/docs/linters.md#doc-vs-signature",
		"todo-owner":       nil,
		"unknown":          nil,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("helpUri by rule %v, want %v", got, want)
	}
}

func TestDocURL(t *testing.T) {
	markBuiltin(t, "premature-success")
	for _, c := range []struct{ version, ref string }{
		{"v0.2.0", "v0.2.0"},
		{"v1.0.0-rc.1", "v1.0.0-rc.1"},
		{"", "main"},
		{"(devel)", "main"},
		{"v0.2.1-0.20261005120000-abcdefabcdef", "main"},
		{"v0.0.0-20261005120000-abcdefabcdef", "main"},
		{"v0.2.0+dirty", "main"},
		{"v0.2.1-0.20261005120000-abcdefabcdef+dirty", "main"},
		{"v0.2.1-SNAPSHOT-abcdef1", "main"},
		{"v0.2", "main"},
	} {
		want := "https://github.com/ssgreg/lintuition/blob/" + c.ref + "/docs/linters.md#premature-success"
		if got := DocURL(c.version, "premature-success"); got != want {
			t.Errorf("DocURL(%q) = %q, want %q", c.version, got, want)
		}
	}
	if got := DocURL("v0.2.0", "todo-owner"); got != "" {
		t.Errorf("a plugin linter got a link: %q", got)
	}
	// A plugin may take a built-in's name in a binary that leaves builtin out; it is not marked.
	if got := DocURL("v0.2.0", "metric-type-vs-help"); got != "" {
		t.Errorf("a plugin named like a built-in got the built-in's link: %q", got)
	}
}
