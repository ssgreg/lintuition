package logkeyrole

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
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
	var cs []*sdk.Candidate
	for _, r := range res {
		cs = append(cs, r.Result.([]*sdk.Candidate)...)
	}
	sort.Slice(cs, func(i, j int) bool { return num(caseNo(t, cs[i].Pos)) < num(caseNo(t, cs[j].Pos)) })
	var got []string
	for _, c := range cs {
		n := caseNo(t, c.Pos)
		if len(c.Payload.Source) > 0 || len(c.Payload.Prose) > 0 {
			t.Errorf("case %s sends more than facts: %+v", n, c.Payload)
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported: "+c.Unsupported)
			continue
		}
		got = append(got, fmt.Sprintf("%s %v", n, c.Payload.Facts["fields"]))
	}
	want := []string{
		`1 [key remaining_attempts, value elapsedAttempts, type int]`,
		`5 [key remaining, value s.elapsed, type int key sum, value s.inner.total, type int]`,
		`6 unsupported: the log fields are not readable`,
		`8 unsupported: a field key cannot be sent as a fact`,
		`9 [key deadline, value started, type int]`,
		`12 [key path, value name, type string]`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func role(choice string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"role": {QuestionID: "role", Choice: choice, Probabilities: map[string]float64{choice: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.85}
	c := &sdk.Candidate{Local: map[string]string{"pairs": "remaining_attempts (elapsedAttempts)"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{role("conflict", 0.9), true, false},
		{role("conflict", 0.7), false, true},
		{role("same", 0.9), false, false},
		{role("same", 0.6), false, true},
		{role("insufficient", 0.95), false, true},
		{role("unclear", 0.9), false, true},
		{map[string]sdk.Answer{"role": {QuestionID: "role", Choice: "conflict"}}, false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, role("conflict", 0.9)); d.Message != `log key names a different quantity than its value: remaining_attempts (elapsedAttempts)` {
		t.Errorf("message: %s", d.Message)
	}
	two := &sdk.Candidate{Local: map[string]string{"pairs": "remaining (s.elapsed), sum (s.inner.total)"}}
	if d := r.Decide(two, role("conflict", 0.9)); d.Message != `a log key names a different quantity than its value: one of remaining (s.elapsed), sum (s.inner.total)` {
		t.Errorf("message: %s", d.Message)
	}
}
