package normalaterror

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
		`1 error "cache miss"`,
		`4 unsupported: the log message is not a constant string`,
		`5 fatal "client went away: %s"`,
		`9 error "retry scheduled"`,
		`14 error "request canceled by client"`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func event(choice string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"event": {QuestionID: "event", Choice: choice, Probabilities: map[string]float64{choice: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.9}
	c := &sdk.Candidate{Local: map[string]string{"level": "error", "message": "cache miss"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{event("routine", 0.95), true, false},
		{event("routine", 0.8), false, true},
		{event("failure", 0.95), false, false},
		{event("failure", 0.5), false, true},
		{event("degradation", 0.92), false, false},
		{event("unclear", 0.95), false, true},
		{map[string]sdk.Answer{"event": {QuestionID: "event", Choice: "routine"}}, false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, event("routine", 0.95)); d.Message != `routine event logged at error level: "cache miss"` {
		t.Errorf("message: %s", d.Message)
	}
}

func TestNamesFailure(t *testing.T) {
	for msg, want := range map[string]bool{
		"failed to save": true, "Save failure": true, "ERROR: x": true, "cannot open": true, "could not dial": true,
		"request timed out": true, "cache miss": false, "client went away": false, "retry scheduled": false,
		"terror alert": false, "can notify": false,
	} {
		if got := namesFailure(msg); got != want {
			t.Errorf("namesFailure(%q) = %v", msg, got)
		}
	}
}
