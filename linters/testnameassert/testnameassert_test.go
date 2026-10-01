package testnameassert

import (
	"fmt"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ssgreg/lintuition/sdk"
)

// caseNo reads the "// N" case number from the candidate's source line.
func caseNo(t *testing.T, pos token.Position) string {
	b, err := os.ReadFile(pos.Filename)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Split(string(b), "\n")[pos.Line-1]
	i := strings.Index(line, "// ")
	if i < 0 {
		return "?"
	}
	return strings.Fields(line[i+3:])[0]
}

func num(s string) int {
	var n int
	fmt.Sscan(s, &n)
	return n
}

func TestExtraction(t *testing.T) {
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "c")
	var cs []*sdk.Candidate
	for _, r := range res {
		cs = append(cs, r.Result.([]*sdk.Candidate)...)
	}
	sort.Slice(cs, func(i, j int) bool { return num(caseNo(t, cs[i].Pos)) < num(caseNo(t, cs[j].Pos)) })
	var got []string
	for _, c := range cs {
		n := caseNo(t, c.Pos)
		if c.Unsupported != "" {
			got = append(got, n+" unsupported: "+c.Unsupported)
			continue
		}
		if len(c.Payload.Source) > 0 || len(c.Payload.Prose) != 1 || len(c.Payload.Facts) != 1 {
			t.Errorf("case %s: payload %+v", n, c.Payload)
		}
		got = append(got, fmt.Sprintf("%s %q call=%s expect=%s", n, c.Payload.Prose["test"], c.Payload.Facts["call"], c.Local["expect"]))
	}
	want := []string{
		`1 "validate rejects empty" call=Validate expect=error`,
		`2 "validate accepts token" call=Validate expect=no_error`,
		`3 "parse returns number" call=Parse expect=no_error`,
		`4 "parse fails on letters" call=Parse expect=error`,
		`5 "parse file reads header" call=ParseFile expect=no_error`,
		`6 "validate refuses blank" call=Validate expect=error`,
		`8 unsupported: the test calls Validate more than once`,
		`9 unsupported: the error variable of Validate is written again`,
		`10 unsupported: the error is not asserted`,
		`11 unsupported: the error of Validate is not kept`,
		`12 unsupported: the error is asserted both ways`,
		`13 unsupported: the error is checked in a form not read here`,
		`15 "validate rejects letters" call=Validate expect=error`,
		`16 unsupported: the test name matches several called functions`,
		`17 "store save fails when full" call=Save expect=no_error`,
		`19 "validate in subtest" call=Validate expect=error`,
		`20 unsupported: the error variable of Validate is written again`,
		`21 unsupported: the error is checked in a form not read here`,
		`22 unsupported: the error is checked in a form not read here`,
		`24 unsupported: the error is checked in a form not read here`,
		`25 unsupported: the error is checked in a form not read here`,
		`26 unsupported: the error is checked in a form not read here`,
		`27 unsupported: the error is checked in a form not read here`,
		`28 "validate rejects deferred" call=Validate expect=error`,
		`29 "validate accepts logged" call=Validate expect=no_error`,
		`30 unsupported: the error is checked in a form not read here`,
		`31 unsupported: the error is checked in a form not read here`,
		`32 unsupported: the error is checked in a form not read here`,
		`33 unsupported: the error is checked in a form not read here`,
		`34 unsupported: the error is checked in a form not read here`,
		`35 unsupported: the error is checked in a form not read here`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func nameSays(choice string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"name_says": {QuestionID: "name_says", Choice: choice, Probabilities: map[string]float64{choice: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.9}
	noErr := &sdk.Candidate{Local: map[string]string{"call": "Validate", "expect": "no_error"}}
	wantErr := &sdk.Candidate{Local: map[string]string{"call": "Validate", "expect": "error"}}
	for _, tc := range []struct {
		c               *sdk.Candidate
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{noErr, nameSays("error", 0.95), true, false},
		{noErr, nameSays("error", 0.8), false, true},
		{noErr, nameSays("no_error", 0.95), false, false},
		{noErr, nameSays("no_error", 0.5), false, true},
		{wantErr, nameSays("no_error", 0.95), true, false},
		{wantErr, nameSays("error", 0.95), false, false},
		{wantErr, nameSays("input_only", 0.95), false, false},
		{wantErr, nameSays("input_only", 0.6), false, true},
		{noErr, nameSays("unclear", 0.95), false, true},
	} {
		d := r.Decide(tc.c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%v %+v: got %+v", tc.c.Local, tc.answers, d)
		}
	}
	if d := r.Decide(noErr, nameSays("error", 0.95)); d.Message != "test name expects an error from Validate, but the test fails when Validate returns one" {
		t.Errorf("message: %s", d.Message)
	}
	if d := r.Decide(wantErr, nameSays("no_error", 0.95)); d.Message != "test name expects Validate to succeed, but the test fails when Validate returns no error" {
		t.Errorf("message: %s", d.Message)
	}
}
