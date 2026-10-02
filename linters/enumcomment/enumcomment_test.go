package enumcomment

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

// caseNo reads the case number from the "// N" comment above the const block around pos, or above
// the function that holds it. A comment inside the block would be a candidate itself, so the number
// sits outside.
func caseNo(t *testing.T, pos token.Position) int {
	b, err := os.ReadFile(pos.Filename)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	for i := pos.Line - 1; i > 0; i-- {
		if strings.TrimSpace(lines[i]) != "const (" {
			continue
		}
		for j := i - 1; j >= 0 && j >= i-2; j-- {
			var n int
			if _, err := fmt.Sscanf(strings.TrimSpace(lines[j]), "// %d", &n); err == nil {
				return n
			}
		}
		t.Fatalf("%s: no case number above the block", pos)
	}
	t.Fatalf("%s: not in a const block", pos)
	return 0
}

func TestExtraction(t *testing.T) {
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
	type line struct {
		n int
		s string
	}
	var got []line
	r := &rule{}
	for _, res := range res {
		for _, c := range res.Result.([]*sdk.Candidate) {
			n := caseNo(t, c.Pos)
			if len(c.Payload.Source) > 0 || len(c.Payload.Facts) > 0 {
				t.Errorf("case %d %s sends more than the comment: %v %v", n, c.Subject, c.Payload.Source, c.Payload.Facts)
			}
			if c.Unsupported != "" {
				got = append(got, line{n, fmt.Sprintf("%d %s unsupported: %s", n, c.Subject, c.Unsupported)})
				continue
			}
			if c.Payload.Prose["comment"] != c.Local["comment"] {
				t.Errorf("case %d %s: the comment is not sent as written: %q", n, c.Subject, c.Payload.Prose["comment"])
			}
			var keys []string
			for _, o := range r.Questions(c)[0].Options {
				keys = append(keys, o.Key)
			}
			got = append(got, line{n, fmt.Sprintf("%d %s %q [%s]", n, c.Subject, c.Payload.Prose["comment"], strings.Join(keys, " "))})
		}
	}
	sort.SliceStable(got, func(i, j int) bool {
		if got[i].n != got[j].n {
			return got[i].n < got[j].n
		}
		return got[i].s < got[j].s
	})
	var gs []string
	for _, l := range got {
		gs = append(gs, l.s)
	}
	want := []string{
		`1 StateIdle/doc "a job nobody has picked up yet" [StateIdle StateRunning StateDone none]`,
		`1 StateRunning/line "the job finished and its result is stored" [StateIdle StateRunning StateDone none]`,
		`4 A/line unsupported: the comment is on a line of several constants`,
		`6 none/line unsupported: a constant is named like an answer option`,
		`7 unclear/line unsupported: a constant is named like an answer option`,
		`8 LocalA/doc "inside a function" [LocalA LocalB none]`,
		`10 debug/doc "Enable extra checks while developing." [debug trace none]`,
		`12 PhaseCopying/doc "PhaseVerified means every block was checked." [PhaseCopying PhaseVerified none]`,
		`13 P/doc "A placeholder until the value is known." [P Q none]`,
		`14 Run/doc "Running jobs are counted here." [Run Stop none]`,
		`15 limitBody/doc unsupported: the comment differs from another comment of the block in one word at most`,
		`15 limitLink/doc unsupported: the comment differs from another comment of the block in one word at most`,
		`15 limitTitle/doc unsupported: the comment differs from another comment of the block in one word at most`,
		`16 codeOne/doc unsupported: the comment differs from another comment of the block in one word at most`,
		`16 codeSix/line "a value of its own" [codeOne codeTwo codeSix none]`,
		`16 codeTwo/line unsupported: the comment differs from another comment of the block in one word at most`,
		`17 inTimeout/line "read timeout" [inTimeout outTimeout none]`,
		`17 outTimeout/line "write timeout" [inTimeout outTimeout none]`,
		`18 kindBody/line "body of switch" [kindHead kindTail kindBody none]`,
		`18 kindHead/line "head of loop" [kindHead kindTail kindBody none]`,
		`18 kindTail/line "block after loop" [kindHead kindTail kindBody none]`,
		`19 ruleRead/line unsupported: the comment differs from another comment of the block in one word at most`,
		`19 ruleWrite/line unsupported: the comment differs from another comment of the block in one word at most`,
		`20 FlagOn/line "turned off by the operator" [FlagOn FlagOff none]`,
		`22 K01/line unsupported: the block has more than 20 constants to offer as options`,
	}
	if strings.Join(gs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(gs, "\n"), strings.Join(want, "\n"))
	}
}

func TestNamesWord(t *testing.T) {
	for _, tc := range []struct {
		text, name string
		want       bool
	}{
		{"StateIdle is a job nobody has picked up.", "StateIdle", true},
		{"If trace is set, output is printed.", "trace", true},
		{"see [StateIdle].", "StateIdle", true},
		{"Running jobs are counted.", "Run", false},
		{"StateIdleSince is the start of the wait.", "StateIdle", false},
		{"A placeholder.", "A", false},
		{"no mention here", "StateIdle", false},
	} {
		if got := namesWord(tc.text, tc.name); got != tc.want {
			t.Errorf("namesWord(%q, %q) = %v", tc.text, tc.name, got)
		}
	}
}

func TestNearDuplicate(t *testing.T) {
	names := []string{"KA", "KB", "KC"}
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"block after for", "block after if", true},
		{"block after for", "body of for", false},
		{"Read timeout.", "read timeout", true},
		{"read timeout", "write timeout", false}, // two words
		{"like KA, for reads", "like KC for writes", true},
		{"like KA", "like KB", true}, // the same once names are one placeholder
		{"one two three", "one two three four", false},
		{"", "", false},
	} {
		words := [][]string{template(tc.a, names), template(tc.b, names)}
		if got := nearDuplicate(words, 0); got != tc.want {
			t.Errorf("nearDuplicate(%q, %q) = %v", tc.a, tc.b, got)
		}
	}
}

func TestQuestionTextIsFixed(t *testing.T) {
	r := &rule{}
	a := r.Questions(&sdk.Candidate{Local: map[string]string{"constants": "A B"}})[0]
	b := r.Questions(&sdk.Candidate{Local: map[string]string{"constants": "X Y Z"}})[0]
	if a.Text != b.Text || len(a.Options) != 3 || len(b.Options) != 4 {
		t.Fatalf("text must be fixed and options per block: %+v %+v", a, b)
	}
	// Only the constants' options vary, and only by the identifier.
	if a.Options[2] != b.Options[3] {
		t.Errorf("none differs between blocks: %+v %+v", a.Options, b.Options)
	}
	if strings.ReplaceAll(a.Options[0].Description, "A", "X") != b.Options[0].Description {
		t.Errorf("a constant's option says more than its name: %q", a.Options[0].Description)
	}
	if !strings.Contains(a.Text, "`comment`") {
		t.Errorf("the question does not name the comment field: %s", a.Text)
	}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
}

func dist(choice string, ps map[string]float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"describes": {QuestionID: "describes", Choice: choice, Probabilities: ps}}
}

func describes(choice string, p float64) map[string]sdk.Answer {
	return dist(choice, map[string]float64{choice: p})
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.8}
	c := &sdk.Candidate{Local: map[string]string{"name": "StateRunning", "comment": "the job finished", "constants": "StateIdle StateRunning StateDone"}}
	const (
		report = iota
		clean
		abstain
	)
	for _, tc := range []struct {
		answers map[string]sdk.Answer
		want    int
	}{
		{describes("StateDone", 0.9), report},
		{describes("StateDone", 0.8), report}, // at the threshold
		{describes("StateDone", 0.79), abstain},
		{describes("StateDone", 0.7), abstain}, // weak shift abstains
		{describes("StateIdle", 0.95), report},
		{describes("StateRunning", 0.9), clean},
		{describes("StateRunning", 0.5), abstain}, // weak agreement abstains
		{describes("none", 0.85), clean},
		{describes("none", 0.6), abstain},
		{describes("unclear", 0.9), abstain},
		// the answer has no probability for its choice
		{map[string]sdk.Answer{"describes": {QuestionID: "describes", Choice: "StateDone"}}, abstain},
		{dist("StateDone", map[string]float64{"StateRunning": 0.9}), abstain},
		// a full distribution that adds up to 1
		{dist("StateDone", map[string]float64{"StateDone": 0.85, "StateRunning": 0.1, "none": 0.05}), report},
		{dist("StateRunning", map[string]float64{"StateRunning": 0.82, "StateDone": 0.18}), clean},
		// agreement and none are not added up: a split between them abstains
		{dist("none", map[string]float64{"none": 0.5, "StateRunning": 0.45, "StateDone": 0.05}), abstain},
		// a pick that contradicts the rest of its distribution abstains either way
		{dist("StateDone", map[string]float64{"StateDone": 0.85, "StateRunning": 0.4}), abstain},
		{dist("StateRunning", map[string]float64{"StateRunning": 0.9, "StateDone": 0.25}), abstain},
		{dist("StateDone", map[string]float64{"StateDone": 0.85, "StateRunning": 0.2}), report}, // at 1-threshold
	} {
		d := r.Decide(c, tc.answers)
		got := clean
		switch {
		case d.Report:
			got = report
		case d.Abstained != "":
			got = abstain
		}
		if got != tc.want {
			t.Errorf("%+v: got %+v", tc.answers["describes"], d)
		}
	}
	if d := r.Decide(c, describes("StateDone", 0.9)); d.Message != `comment describes StateDone, not StateRunning: "the job finished"` {
		t.Errorf("message: %s", d.Message)
	}
	long := &sdk.Candidate{Local: map[string]string{"name": "StateRunning", "comment": strings.Repeat("x", 100)}}
	if d := r.Decide(long, describes("StateDone", 0.9)); !strings.HasSuffix(d.Message, strings.Repeat("x", 80)+`..."`) {
		t.Errorf("long comment is not clipped: %s", d.Message)
	}
}

func TestThresholdSetting(t *testing.T) {
	for _, l := range sdk.Linters() {
		if l.Name != Name {
			continue
		}
		if l.Version != "2" {
			t.Errorf("version %s", l.Version)
		}
		rl, err := l.New(l.NewSettings())
		if err != nil || rl.(*rule).threshold != 0.8 {
			t.Fatalf("default threshold: %v %v", rl, err)
		}
		v := 0.9
		rl, err = l.New(&Settings{Threshold: &v})
		if err != nil || rl.(*rule).threshold != 0.9 {
			t.Fatalf("threshold 0.9: %v %v", rl, err)
		}
		return
	}
	t.Fatal("not registered")
}
