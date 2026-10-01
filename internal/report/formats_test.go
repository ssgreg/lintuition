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
