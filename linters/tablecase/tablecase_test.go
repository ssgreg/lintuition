package tablecase

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
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "b")
	var cs []*sdk.Candidate
	for _, r := range res {
		cs = append(cs, r.Result.([]*sdk.Candidate)...)
	}
	sort.Slice(cs, func(i, j int) bool { return num(caseNo(t, cs[i].Pos)) < num(caseNo(t, cs[j].Pos)) })
	var got []string
	for _, c := range cs {
		n := caseNo(t, c.Pos)
		if c.Unsupported != "" {
			got = append(got, n+" unsupported")
			continue
		}
		got = append(got, fmt.Sprintf("%s %q about=%q %s=%s", n, c.Payload.Prose["case_name"], c.Payload.Facts["about"], c.Local["field"], c.Local["value"]))
	}
	want := []string{
		`1 "rejects an expired token" about="the result of IsExpired" want=true`,
		`2 "fresh token" about="the result of IsExpired" want=false`,
		`3 "named constant" about="the result of IsExpired" want=true`,
		`4 unsupported`,
		`5 "error flag only" about="the result of IsExpired" want=false`,
		`7 "keeps the port in use while attached" about="whether in use" wantInUse=false`,
		`8 unsupported`,
		`9 "external package test" about="the result of IsExpired" want=true`,
		`10 unsupported`,
		`11 "ranged literal" about="the result of IsExpired" want=true`,
		`12 "through t.Run" about="the result of IsExpired" want=false`,
		`13 unsupported`,
		`14 "allowed input" about="the result of Allowed" want=true`,
		`15 "not expired and valid" about="whether expired" wantExpired=false`,
		`15 "not expired and valid" about="whether valid" wantValid=false`,
		`16 unsupported`,
		`17 unsupported`,
		`18 unsupported`,
		`19 unsupported`,
		`20 "through got" about="the result of Allowed" want=true`,
		`21 unsupported`,
		`22 unsupported`,
		`22 unsupported`,
		`23 unsupported`,
		`24 "input normalised" about="the result of Allowed" want=true`,
		`25 unsupported`,
		`26 "len of the table" about="the result of Allowed" want=true`,
		`27 unsupported`,
		`28 unsupported`,
		`29 unsupported`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func outcome(choice string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"outcome": {QuestionID: "outcome", Choice: choice, Probabilities: map[string]float64{choice: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.85}
	c := &sdk.Candidate{Local: map[string]string{"case_name": "rejects an expired token", "field": "want", "value": "true"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{outcome("false", 0.9), true, false},
		{outcome("false", 0.7), false, true},
		{outcome("true", 0.9), false, false},
		{outcome("true", 0.5), false, true},
		{outcome("unstated", 0.9), false, false},
		{outcome("unclear", 0.9), false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, outcome("false", 0.9)); d.Message != `case "rejects an expired token" reads as want false, but the table sets true` {
		t.Errorf("message: %s", d.Message)
	}
}
