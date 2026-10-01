package enumcomment

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ssgreg/lintuition/sdk"
)

// Comments are this linter's subject, so cases are told apart by the constant they are on rather
// than by a "// N" comment.
func TestExtraction(t *testing.T) {
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
	var got []string
	r := &rule{}
	for _, res := range res {
		for _, c := range res.Result.([]*sdk.Candidate) {
			if len(c.Payload.Source) > 0 || len(c.Payload.Facts) > 0 {
				t.Errorf("%s sends more than the comment: %v %v", c.Subject, c.Payload.Source, c.Payload.Facts)
			}
			if c.Unsupported != "" {
				got = append(got, c.Subject+" unsupported: "+c.Unsupported)
				continue
			}
			var keys []string
			for _, o := range r.Questions(c)[0].Options {
				keys = append(keys, o.Key)
			}
			got = append(got, fmt.Sprintf("%s %q [%s]", c.Subject, c.Payload.Prose["comment"], strings.Join(keys, " ")))
		}
	}
	sort.Strings(got)
	want := []string{
		`A/line unsupported: the comment is on a line of several constants`,
		`K01/line unsupported: the block has more than 20 constants to offer as options`,
		`LocalA/doc "this constant is inside a function." [LocalA LocalB none]`,
		`StateIdle/doc "this constant is a job nobody has picked up yet." [StateIdle StateRunning StateDone none]`,
		`StateRunning/line "the job finished and its result is stored" [StateIdle StateRunning StateDone none]`,
		`none/line unsupported: a constant is named like an answer option`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestQuestionTextIsFixed(t *testing.T) {
	r := &rule{}
	a := r.Questions(&sdk.Candidate{Local: map[string]string{"constants": "A B"}})[0]
	b := r.Questions(&sdk.Candidate{Local: map[string]string{"constants": "X Y Z"}})[0]
	if a.Text != b.Text || len(a.Options) != 3 || len(b.Options) != 4 {
		t.Fatalf("text must be fixed and options per block: %+v %+v", a, b)
	}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
}

func describes(choice string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"describes": {QuestionID: "describes", Choice: choice, Probabilities: map[string]float64{choice: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.8}
	c := &sdk.Candidate{Local: map[string]string{"name": "StateRunning", "comment": "the job finished", "constants": "StateIdle StateRunning StateDone"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{describes("StateDone", 0.9), true, false},
		{describes("StateDone", 0.7), false, true}, // weak shift abstains
		{describes("StateRunning", 0.9), false, false},
		{describes("StateRunning", 0.5), false, true}, // weak agreement abstains
		{describes("none", 0.85), false, false},
		{describes("unclear", 0.9), false, true},
		{map[string]sdk.Answer{"describes": {QuestionID: "describes", Choice: "StateDone"}}, false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, describes("StateDone", 0.9)); d.Message != `comment describes StateDone, not StateRunning: "the job finished"` {
		t.Errorf("message: %s", d.Message)
	}
}
