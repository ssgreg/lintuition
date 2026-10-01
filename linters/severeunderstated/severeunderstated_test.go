package severeunderstated

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
		if len(c.Payload.Source) > 0 || len(c.Payload.Facts) > 0 {
			t.Errorf("case %s sends more than the message: %+v", n, c.Payload)
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported: "+c.Unsupported)
			continue
		}
		got = append(got, fmt.Sprintf("%s %s %q", n, c.Local["level"], c.Payload.Prose["message"]))
	}
	want := []string{
		`1 info "queue full, dropping events"`,
		`4 unsupported: the log message is not a constant string`,
		`5 debug "upload lost after restart"`,
		`9 info "session expired, work discarded"`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func answers(choice string, p float64, yes *float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{
		"consequence": {QuestionID: "consequence", Choice: choice, Probabilities: map[string]float64{choice: p}},
		"on_purpose":  {QuestionID: "on_purpose", Yes: yes},
	}
}

func f(v float64) *float64 { return &v }

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.85}
	c := &sdk.Candidate{Local: map[string]string{"level": "info", "message": "queue full, dropping events"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{answers("unintended_loss", 0.9, f(0.1)), true, false},
		{answers("unintended_loss", 0.9, f(0.3)), true, false},
		{answers("unintended_loss", 0.7, f(0.1)), false, true},
		{answers("unintended_loss", 0.9, f(0.8)), false, false},
		{answers("unintended_loss", 0.9, f(0.5)), false, true},
		{answers("unintended_loss", 0.9, nil), false, true},
		{answers("routine", 0.9, f(0.1)), false, false},
		{answers("routine", 0.6, f(0.1)), false, true},
		{answers("inconvenience", 0.95, f(0.1)), false, false},
		{answers("unclear", 0.95, f(0.1)), false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, answers("unintended_loss", 0.9, f(0.1))); d.Message != `unintended loss logged at info level: "queue full, dropping events"` {
		t.Errorf("message: %s", d.Message)
	}
}
