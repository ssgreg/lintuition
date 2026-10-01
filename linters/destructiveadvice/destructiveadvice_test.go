package destructiveadvice

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
		if len(c.Payload.Source) > 0 {
			t.Errorf("case %s sends source: %+v", n, c.Payload)
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported: "+c.Unsupported)
			continue
		}
		got = append(got, fmt.Sprintf("%s %s %q", n, c.Payload.Facts["kind"], c.Payload.Prose["text"]))
	}
	want := []string{
		`1 error message "index is corrupt; delete the data directory and restart"`,
		`2 error message "config invalid: %w; reset it with --reset"`,
		`4 unsupported: the error message is not a constant string`,
		`5 log message "cache is stale, wipe it and restart"`,
		`7 error message "run rm -rf /var/lib/app to recover"`,
		`8 unsupported: the log message is not a constant string`,
		`9 log message "removed 3 stale sessions"`,
		`11 error message "droplet count %d"`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func answers(choice string, p float64, yes *float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{
		"advice":      {QuestionID: "advice", Choice: choice, Probabilities: map[string]float64{choice: p}},
		"states_loss": {QuestionID: "states_loss", Yes: yes},
	}
}

func f(v float64) *float64 { return &v }

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.85}
	c := &sdk.Candidate{Local: map[string]string{"text": "delete the data directory"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{answers("destructive_advice", 0.9, f(0.1)), true, false},
		{answers("destructive_advice", 0.9, f(0.3)), true, false},
		{answers("destructive_advice", 0.7, f(0.1)), false, true},
		{answers("destructive_advice", 0.9, f(0.8)), false, false},
		{answers("destructive_advice", 0.9, f(0.5)), false, true},
		{answers("destructive_advice", 0.9, nil), false, true},
		{answers("safe_advice", 0.9, f(0.1)), false, false},
		{answers("no_advice", 0.95, f(0.1)), false, false},
		{answers("no_advice", 0.5, f(0.1)), false, true},
		{answers("unclear", 0.95, f(0.1)), false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, answers("destructive_advice", 0.9, f(0.1))); d.Message != `advises a destructive step without saying what is lost: "delete the data directory"` {
		t.Errorf("message: %s", d.Message)
	}
}

func TestDestructiveWord(t *testing.T) {
	for text, want := range map[string]bool{
		"delete the cache": true, "Reset your password": true, "run rm -rf x": true, "reinstall the agent": true,
		"file not found": false, "firmware update": false, "remote host": false, "transform failed": false,
	} {
		if got := destructiveWord(text); got != want {
			t.Errorf("destructiveWord(%q) = %v", text, got)
		}
	}
}
