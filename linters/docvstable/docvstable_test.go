package docvstable

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
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "d")
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
		if len(c.Payload.Source) > 0 || len(c.Payload.Facts) > 0 {
			t.Errorf("case %s: only the doc and the case name may be sent, payload %+v", n, c.Payload)
		}
		got = append(got, fmt.Sprintf("%s %s %q doc=%q %s=%s", n, c.Local["function"], c.Payload.Prose["situation"], c.Payload.Prose["doc"], c.Local["field"], c.Local["value"]))
	}
	want := []string{
		`1 IsExpired "deadline passed an hour ago" doc="this function reports whether the token's deadline has passed. this function never looks at the clock." want=true`,
		`2 IsExpired "deadline tomorrow" doc="this function reports whether the token's deadline has passed. this function never looks at the clock." want=false`,
		`3 unsupported: the expectation is not a constant`,
		`7 unsupported: the expectation is compared in a transformed form`,
		`8 unsupported: the expectation is compared with a value not established as one call's result`,
		`9 unsupported: the expectation is compared with results of different calls`,
		`10 Valid "fresh token" doc="this function reports whether the token can still be used." want=true`,
		`11 Generic "empty string" doc="this function reports whether v is the zero value." want=true`,
		`12 unsupported: the doc of IsExpired is not in this package`,
		`13 unsupported: the row's expectation is written in a loop over the table`,
		`14 unsupported: the table is written after it is built`,
		`14 unsupported: the table is written after it is built`,
		`15 unsupported: the table is written after it is built`,
		`16 unsupported: the row's expectation is written in a loop over the table`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func condition(choice string, p float64) map[string]sdk.Answer {
	return answer(choice, p, 0.8)
}

// answer is a condition answer with the probability that the case name states the facts.
func answer(choice string, p, stated float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{
		"condition": {QuestionID: "condition", Choice: choice, Probabilities: map[string]float64{choice: p}},
		"stated":    {QuestionID: "stated", Yes: &stated},
	}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.8}
	wantFalse := &sdk.Candidate{Local: map[string]string{"function": "IsExpired", "case_name": "deadline passed", "value": "false"}}
	wantTrue := &sdk.Candidate{Local: map[string]string{"function": "IsExpired", "case_name": "deadline tomorrow", "value": "true"}}
	onlyCondition := func(choice string, p float64) map[string]sdk.Answer {
		a := condition(choice, p)
		delete(a, "stated")
		return a
	}
	for _, tc := range []struct {
		c               *sdk.Candidate
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{wantFalse, condition("met", 0.9), true, false},
		{wantFalse, condition("met", 0.7), false, true},
		{wantFalse, condition("not_met", 0.9), false, false},
		{wantFalse, condition("not_met", 0.5), false, true},
		{wantTrue, condition("not_met", 0.85), true, false},
		{wantTrue, condition("met", 0.95), false, false},
		{wantTrue, condition("not_covered", 0.95), false, false},
		{wantTrue, condition("unclear", 0.95), false, true},
		// At the threshold the answer counts; just below it abstains.
		{wantFalse, condition("met", 0.8), true, false},
		{wantFalse, condition("met", 0.79), false, true},
		// A contradiction in a name that only labels its input abstains; the gate is 0.7 sure of no.
		{wantTrue, answer("not_met", 0.95, 0.2), false, true},
		{wantTrue, answer("not_met", 0.95, 0.3), false, true},
		{wantTrue, answer("not_met", 0.95, 0.31), true, false},
		{wantFalse, answer("met", 0.95, 0.05), false, true},
		// The gate never turns an agreeing or uncovered answer into an abstention.
		{wantTrue, answer("met", 0.95, 0.05), false, false},
		{wantTrue, answer("not_covered", 0.95, 0.05), false, false},
		// A contradiction without the stated answer abstains; an agreeing one does not need it.
		{wantTrue, onlyCondition("not_met", 0.95), false, true},
		{wantTrue, onlyCondition("met", 0.95), false, false},
		// A choice without its probability abstains, as does no answer at all.
		{wantTrue, map[string]sdk.Answer{"condition": {QuestionID: "condition", Choice: "not_met", Probabilities: map[string]float64{"met": 0.9}}}, false, true},
		{wantTrue, map[string]sdk.Answer{}, false, true},
	} {
		d := r.Decide(tc.c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%v %+v: got %+v", tc.c.Local, tc.answers, d)
		}
	}
	if d := r.Decide(wantFalse, condition("met", 0.9)); d.Message != `doc of IsExpired implies true for case "deadline passed", the test expects false` {
		t.Errorf("message: %s", d.Message)
	}
	if d := r.Decide(wantTrue, answer("not_met", 0.95, 0.2)); d.Abstained != "the case name may not state the facts the doc's condition depends on (0.20)" {
		t.Errorf("abstention: %s", d.Abstained)
	}
}

// TestQuestionsAreFixed checks that the questions do not depend on the candidate, refer only to the
// fields the payload sends, and are well formed.
func TestQuestionsAreFixed(t *testing.T) {
	r := &rule{threshold: 0.8}
	a := r.Questions(&sdk.Candidate{Local: map[string]string{"case_name": "a"}})
	b := r.Questions(&sdk.Candidate{Local: map[string]string{"case_name": "b"}})
	if fmt.Sprint(a) != fmt.Sprint(b) || len(a) != 2 {
		t.Fatalf("questions differ or are not two: %v / %v", a, b)
	}
	for _, q := range a {
		if err := q.Validate(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(q.Text, "`doc`") || !strings.Contains(q.Text, "`situation`") {
			t.Errorf("%s does not refer to doc and situation: %s", q.ID, q.Text)
		}
	}
}
