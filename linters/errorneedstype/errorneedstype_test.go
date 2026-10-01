package errorneedstype

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
	i := strings.LastIndex(line, "// ")
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
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
	var cs []*sdk.Candidate
	for _, r := range res {
		cs = append(cs, r.Result.([]*sdk.Candidate)...)
	}
	sort.SliceStable(cs, func(i, j int) bool { return num(caseNo(t, cs[i].Pos)) < num(caseNo(t, cs[j].Pos)) })
	var got []string
	for _, c := range cs {
		n := caseNo(t, c.Pos)
		if len(c.Payload.Source) > 0 || len(c.Payload.Facts) > 0 {
			t.Errorf("case %s sends more than the message: %v %v", n, c.Payload.Source, c.Payload.Facts)
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported")
			continue
		}
		got = append(got, fmt.Sprintf("%s %s %q", n, c.Subject, c.Payload.Prose["message"]))
	}
	want := []string{
		`1 c1/0 "user not found"`,
		`2 c2/1 "user %d already exists"`,
		`5 unsupported`,
		`8 c8/0 "user is gone"`,
		`10 c10/func/0 "timeout waiting for lock"`,
		`11 unsupported`,
		`12 c12/0 "load user: %v"`,
		`13 c13/0 "100%w done"`,
		`14 unsupported`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func yes(p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"branchable": {QuestionID: "branchable", Yes: &p}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.8}
	c := &sdk.Candidate{Local: map[string]string{"message": "user not found"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{yes(0.9), true, false},
		{yes(0.8), true, false},
		{yes(0.7), false, true}, // weak yes abstains
		{yes(0.3), false, true}, // weak no abstains
		{yes(0.1), false, false},
		{yes(0.5), false, true}, // unclear
		{map[string]sdk.Answer{"branchable": {QuestionID: "branchable"}}, false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, yes(0.9)); d.Message != `branchable condition returned as a plain string error; consider a sentinel or type: "user not found"` {
		t.Errorf("message: %s", d.Message)
	}
}
