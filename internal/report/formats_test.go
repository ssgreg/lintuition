package report

import (
	"bytes"
	"go/token"
	"testing"
)

func TestGitHubActionsEscaping(t *testing.T) {
	var b bytes.Buffer
	is := []Issue{{FromLinter: "a,b:c", Text: "line one\n::error::injected 100%", Pos: token.Position{Filename: "dir,x/f:1.go", Line: 3, Column: 4}, Severity: "error"}}
	if err := writeGitHubActions(&b, is); err != nil {
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
