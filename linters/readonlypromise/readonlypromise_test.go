package readonlypromise

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
		if len(c.Payload.Source) > 0 {
			t.Errorf("case %s sends source: %v", n, c.Payload.Source)
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported")
			continue
		}
		got = append(got, fmt.Sprintf("%s %s %q %q", n, c.Local["path"], c.Payload.Facts["target"], c.Payload.Prose["doc"]))
	}
	want := []string{
		`1 q.head "the receiver q, a Queue" "the documented function returns the next item without altering the queue."`,
		`3 q.hits "the receiver q, a Queue" "the documented function adds the values and does not modify xs."`,
		`4 xs[0] "the parameter xs, a slice" "the documented function doubles every value, leaving xs unchanged."`,
		`5 calls "package-level state, the variable calls" "the documented function is pure."`,
		`7 unsupported`,
		`8 q.items "the receiver q, a Queue" "the documented function appends v to the queue."`,
		`10 q.head "the receiver q, a Queue" "the documented function is read-only."`,
		`11 q.hits "the receiver q, a Queue" "the documented function returns the head; it never changes the queue, but records a hit."`,
		`12 dst "the parameter dst, a slice" "the documented function writes into dst, leaving src unchanged."`,
		`13 q.head "the receiver q, a Queue" "the documented function rewinds q and clears s, without modifying anything else."`,
		`13 s[0] "the parameter s, a Stack" "the documented function rewinds q and clears s, without modifying anything else."`,
		`15 q.head "the receiver q, a Queue" "the documented function moves the head; m itself is left as it was."`,
		`16 q.head "the receiver q, a Queue" "the documented function rewinds q and schedules a later write to p, leaving both unchanged now."`,
		`16 unsupported`,
		`17 unsupported`,
		`18 *p "the parameter p" "the documented function sets *p, then replaces p, leaving p unchanged."`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func yes(p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"promises_unchanged": {QuestionID: "promises_unchanged", Yes: &p}}
}

func TestQuestions(t *testing.T) {
	qs := (&rule{threshold: 0.85}).Questions(&sdk.Candidate{})
	if len(qs) != 1 || qs[0].Validate() != nil || qs[0].Kind != sdk.Noul || !strings.Contains(qs[0].Text, "`target`") {
		t.Fatalf("%+v", qs)
	}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.7}
	c := &sdk.Candidate{Local: map[string]string{"name": "Peek", "root": "q", "path": "q.head"}}
	for _, tc := range []struct {
		name            string
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{"promised", yes(0.95), true, false},
		{"at the threshold", yes(0.7), true, false},
		{"not promised", yes(0.05), false, false},
		{"no at the threshold", yes(0.3), false, false},
		{"weak yes abstains", yes(0.65), false, true},
		{"weak no abstains", yes(0.35), false, true},
		{"no probability", map[string]sdk.Answer{"promises_unchanged": {QuestionID: "promises_unchanged"}}, false, true},
		{"missing answer", map[string]sdk.Answer{}, false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%s: got %+v", tc.name, d)
		}
	}
	if d := r.Decide(c, yes(0.9)); d.Message != "doc of Peek promises to leave q unchanged, but Peek writes q.head" {
		t.Errorf("message: %s", d.Message)
	}
	g := &sdk.Candidate{Local: map[string]string{"name": "Count", "root": "calls", "path": "calls"}}
	if d := r.Decide(g, yes(0.9)); d.Message != "doc of Count promises to leave calls unchanged, but Count writes calls" {
		t.Errorf("global message: %s", d.Message)
	}
}
