package normalaterror

import (
	"fmt"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ssgreg/lintuition/internal/facts"
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
		if len(c.Payload.Source) > 0 || len(c.Payload.Prose) > 1 {
			t.Errorf("case %s sends more than the message and facts: %+v", n, c.Payload)
		}
		for k, v := range c.Payload.Facts {
			if k != "error" && k != "error_check" {
				t.Errorf("case %s sends an unknown fact %s", n, k)
			}
			if s, ok := v.(string); !ok || !facts.FactSafe(s) {
				t.Errorf("case %s: fact %s=%v is not a safe string", n, k, v)
			}
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported: "+c.Unsupported)
			continue
		}
		line := fmt.Sprintf("%s %s %q", n, c.Local["level"], c.Payload.Prose["message"])
		if e, ok := c.Payload.Facts["error"]; ok {
			line += fmt.Sprintf(" error=%v", e)
		}
		if e, ok := c.Payload.Facts["error_check"]; ok {
			line += fmt.Sprintf(" check=%q", e)
		}
		if _, ok := c.Payload.Facts["error"]; ok != (c.Local["error"] != "") {
			t.Errorf("case %s: the error fact and Local[error]=%q disagree", n, c.Local["error"])
		}
		if c.Local["error"] == errorIdentified {
			line += " identified"
		}
		got = append(got, line)
	}
	want := []string{
		`1 error "cache miss"`,
		`4 unsupported: the log message is not a constant string`,
		`5 fatal "client went away: %s"`,
		`9 error "retry scheduled"`,
		`14 error "request canceled by client" error=attached`,
		`15 error "planned failover completed successfully"`,
		`16 error "listener close" error=attached check="not nil"`,
		`17 error "client went away" error=attached check="is context.Canceled" identified`,
		`18 error "applying config" error=attached`,
		`19 error "stopping endpoint: %v" error=attached`,
		`20 error "renewing certificates" error=attached`,
		`21 error "shutting down exporter" error=attached`,
		`22 error "accept loop" error=attached check="not nil, is not net.ErrClosed"`,
		`23 error "stop hook" error=attached check="not nil"`,
		`24 error "reading frames" error=attached check="not nil, is not io.EOF"`,
		`25 error "flushing buffer" error=attached check="not nil"`,
		`26 error "peer closed the stream" error=attached check="is io.EOF" identified`,
		`27 error "no state file yet" error=attached check="os.IsNotExist" identified`,
		`28 error "request deadline passed" error=attached check="is context.DeadlineExceeded" identified`,
		`29 error "end of input" error=attached check="is io.EOF" identified`,
		`30 error "stream ended" error=attached`,
		`31 error "closing socket" error=attached`,
		`32 error "second attempt" error=attached`,
		`33 error "filling result" error=attached`,
		`34 error "reset path" error=attached`,
		`35 error "async report" error=attached`,
		`36 error "stopping admin endpoint" error=attached check="not nil"`,
		`37 error "waiting for shutdowns" error=attached`,
		`38 error "closing both ends" error=attached`,
		`39 error "request dropped"`,
		`40 error "custom type path" error=attached`,
		`41 error "nil attached"`,
		`42 error "forced close" error=attached`,
		`43 error "goto path" error=attached`,
		`44 error "guard that stays" error=attached`,
		`45 error "local target" error=attached`,
		`46 error "other variable" error=attached`,
		`47 error "handler returned" error=attached check="is not context.Canceled"`,
		`50 error "listener close" error=attached`,
		`51 error "retry scheduled" error=attached`,
		`52 error "draining channel" error=attached check="not nil"`,
		`53 fatal "binding port" error=attached check="not nil"`,
		`54 error "shadowed in init" error=attached check="not nil"`,
		`55 error "listener gone" error=attached check="is net.ErrClosed, not nil" identified`,
		`56 error "after reassign" error=attached check="is io.EOF" identified`,
		`57 error "short redeclare" error=attached`,
		`58 error "plain reassign" error=attached`,
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
		// The threshold is inclusive.
		{event("routine", 0.9), true, false},
		{event("routine", 0.89), false, true},
		{event("degradation", 0.89), false, true},
		{event("failure", 0.9), false, false},
		// A probability for another option is not the chosen one's.
		{map[string]sdk.Answer{"event": {QuestionID: "event", Choice: "routine", Probabilities: map[string]float64{"failure": 0.95}}}, false, true},
		// No answer at all abstains.
		{map[string]sdk.Answer{}, false, true},
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
		"terror alert": false, "can notify": false, "planned failover completed successfully": false,
		"failback started": false, "Failures: 3": true,
	} {
		if got := namesFailure(msg); got != want {
			t.Errorf("namesFailure(%q) = %v", msg, got)
		}
	}
}

// with adds an answer to the operation question with probability p for "action".
func with(a map[string]sdk.Answer, p float64) map[string]sdk.Answer {
	choice := "action"
	if p < 0.5 {
		choice = "outcome"
	}
	out := map[string]sdk.Answer{"operation": {QuestionID: "operation", Choice: choice, Probabilities: map[string]float64{"action": p, "outcome": 1 - p}}}
	for k, v := range a {
		out[k] = v
	}
	return out
}

// TestDecideWithError covers a call that carries an error: an operation named with an error the
// code did not identify is clean whatever the event answer; otherwise the event question decides.
func TestDecideWithError(t *testing.T) {
	r := &rule{threshold: 0.9}
	unidentified := &sdk.Candidate{Local: map[string]string{"level": "error", "message": "closing the listener", "error": errorUnidentified}}
	identified := &sdk.Candidate{Local: map[string]string{"level": "error", "message": "streaming results", "error": errorIdentified}}
	for _, tc := range []struct {
		name            string
		c               *sdk.Candidate
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{"operation named, routine read: clean", unidentified, with(event("routine", 0.97), 0.9), false, false},
		{"operation named at the bar: clean", unidentified, with(event("routine", 0.97), operationOnly), false, false},
		{"operation named, unclear event: clean", unidentified, with(event("unclear", 0.9), 0.95), false, false},
		{"operation named, no event answer: clean", unidentified, with(nil, 0.95), false, false},
		{"says what happened: the event decides", unidentified, with(event("routine", 0.95), 0.1), true, false},
		{"below the bar: the event decides", unidentified, with(event("routine", 0.95), 0.69), true, false},
		{"below the bar, low event: abstain", unidentified, with(event("routine", 0.8), 0.5), false, true},
		{"below the bar, failure: clean", unidentified, with(event("failure", 0.95), 0.2), false, false},
		{"no operation answer: the event decides", unidentified, event("routine", 0.95), true, false},
		{"no operation answer, unclear: abstain", unidentified, event("unclear", 0.95), false, true},
		{"operation answer without a probability: the event decides", unidentified, map[string]sdk.Answer{
			"operation": {QuestionID: "operation", Choice: "action"}, "event": event("routine", 0.95)["event"],
		}, true, false},
		{"operation unclear: the event decides", unidentified, map[string]sdk.Answer{
			"operation": {QuestionID: "operation", Choice: "unclear", Probabilities: map[string]float64{"unclear": 0.9}}, "event": event("routine", 0.95)["event"],
		}, true, false},
		// An identified error is not the news: the operation answer, if any, is not read.
		{"identified: the event decides", identified, with(event("routine", 0.95), 0.95), true, false},
		{"identified, low: abstain", identified, with(event("routine", 0.85), 0.95), false, true},
		{"identified, failure: clean", identified, event("failure", 0.95), false, false},
	} {
		d := r.Decide(tc.c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%s: got %+v", tc.name, d)
		}
	}
}

// TestQuestions checks that the question text is fixed: the same for every candidate, whatever its
// facts, and that its options are the ones Decide reads.
func TestQuestions(t *testing.T) {
	r := &rule{threshold: 0.9}
	plain := r.Questions(&sdk.Candidate{})
	withFacts := &sdk.Candidate{Local: map[string]string{"error": errorIdentified}}
	withFacts.Payload.Fact("error", "attached")
	withFacts.Payload.Fact("error_check", "is io.EOF")
	other := r.Questions(withFacts)
	unidentified := r.Questions(&sdk.Candidate{Local: map[string]string{"error": errorUnidentified}})
	if len(plain) != 1 || len(other) != 1 || plain[0].Text != other[0].Text || plain[0].ID != "event" || plain[0].Kind != sdk.Choice {
		t.Fatalf("questions: %+v vs %+v", plain, other)
	}
	if len(unidentified) != 2 || unidentified[0].Text != plain[0].Text || unidentified[1].ID != "operation" || unidentified[1].Kind != sdk.Choice {
		t.Fatalf("questions for an unidentified error: %+v", unidentified)
	}
	if strings.Contains(unidentified[1].Text, "`error") {
		t.Errorf("the operation question reads more than the message: %s", unidentified[1].Text)
	}
	var keys []string
	for _, o := range plain[0].Options {
		keys = append(keys, o.Key)
	}
	if strings.Join(keys, ",") != "routine,degradation,failure" {
		t.Errorf("options: %v", keys)
	}
	for _, ref := range []string{"`message`", "`error`", "`error_check`"} {
		if !strings.Contains(plain[0].Text, ref) {
			t.Errorf("the question does not name %s", ref)
		}
	}
}
