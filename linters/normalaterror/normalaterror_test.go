package normalaterror

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/internal/classify"
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
		for k := range c.Payload.Facts {
			if k != "error" && k != "error_check" {
				t.Errorf("case %s sends an unknown fact %s", n, k)
			}
		}
		// Every payload must pass the policy check a real run applies before sending.
		if _, err := classify.State(c.Payload, classify.Prose); c.Unsupported == "" && err != nil {
			t.Errorf("case %s: the payload would be refused: %v", n, err)
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
			line += fmt.Sprintf(" check=%q", strings.Join(e.([]string), ", "))
		}
		if _, ok := c.Payload.Facts["error"]; ok != (c.Local["error"] != "") {
			t.Errorf("case %s: the error fact and Local[error]=%q disagree", n, c.Local["error"])
		}
		if e := c.Local["error"]; e != "" {
			line += " " + e
		}
		got = append(got, line)
	}
	want := []string{
		`1 error "cache miss"`,
		`4 unsupported: the log message is not a constant string`,
		`5 fatal "client went away: %s"`,
		`9 error "retry scheduled"`,
		`14 error "request canceled by client" error=attached unproven`,
		`15 error "planned failover completed successfully"`,
		`16 error "listener close" error=attached check="not nil" failed`,
		`17 error "client went away" error=attached check="is context.Canceled" identified`,
		`18 error "applying config" error=attached unproven`,
		`19 error "stopping endpoint: %v" error=attached unproven`,
		`20 error "renewing certificates" error=attached unproven`,
		`21 error "shutting down exporter" error=attached unproven`,
		`22 error "accept loop" error=attached check="not nil, is not net.ErrClosed" failed`,
		`23 error "stop hook" error=attached check="not nil" failed`,
		`24 error "reading frames" error=attached check="not nil, is not io.EOF" failed`,
		`25 error "flushing buffer" error=attached check="not nil" failed`,
		`26 error "peer closed the stream" error=attached check="is io.EOF" identified`,
		`27 error "no state file yet" error=attached check="os.IsNotExist" identified`,
		`28 error "request deadline passed" error=attached check="is context.DeadlineExceeded" identified`,
		`29 error "end of input" error=attached check="is io.EOF" identified`,
		`30 error "stream ended" error=attached unproven`,
		`31 error "closing socket" error=attached unproven`,
		`32 error "second attempt" error=attached unproven`,
		`33 error "filling result" error=attached unproven`,
		`34 error "reset path" error=attached unproven`,
		`35 error "async report" error=attached unproven`,
		`36 error "stopping admin endpoint" error=attached check="not nil" failed`,
		`37 error "waiting for shutdowns" error=attached unproven`,
		`38 error "closing both ends" error=attached unproven`,
		`39 error "request dropped"`,
		`40 error "custom type path" error=attached unproven`,
		`41 error "nil attached"`,
		`42 error "forced close" error=attached unproven`,
		`43 error "goto path" error=attached unproven`,
		`44 error "guard that stays" error=attached unproven`,
		`45 error "local target" error=attached unproven`,
		`46 error "other variable" error=attached unproven`,
		`47 error "handler returned" error=attached check="is not context.Canceled" unproven`,
		`50 error "listener close" error=attached unproven`,
		`51 error "retry scheduled" error=attached unproven`,
		`52 error "draining channel" error=attached check="not nil" failed`,
		`53 fatal "binding port" error=attached check="not nil" failed`,
		`54 error "shadowed in init" error=attached check="not nil" failed`,
		`55 error "listener gone" error=attached check="is net.ErrClosed, not nil" identified`,
		`56 error "after reassign" error=attached check="is io.EOF" identified`,
		`57 error "short redeclare" error=attached unproven`,
		`58 error "plain reassign" error=attached unproven`,
		`61 error "streaming rows" error=attached unproven`,
		`62 error "streaming columns" error=attached check="is context.Canceled" identified`,
		`63 error "handling stop" error=attached unproven`,
		`64 error "counting frames" error=attached check="not nil" failed`,
		`65 error "draining input" error=attached check="is io.EOF" identified`,
		`66 error "closing the listener" error=context.Canceled identified`,
		`67 error "closing the listener" error=attached unproven`,
		`68 error "config reload" error=attached check="nil" nil`,
		`69 error "reading stream" error=attached check="not nil, is not context.DeadlineExceeded, is not context.Canceled, is not io.ErrUnexpectedEOF, is not io.ErrClosedPipe, is not io.ErrShortBuffer" failed`,
		`70 error "decoding frame" error=attached check="not nil" failed`,
		`71 error "deferred rewrite" error=attached unproven`,
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

// TestDecideWithError covers a call that carries an error: an action named with an error the code
// checked is set and did not single out is clean whatever the event answer; with one not shown to
// be set it abstains; otherwise the event question decides.
func TestDecideWithError(t *testing.T) {
	r := &rule{threshold: 0.9}
	failed := &sdk.Candidate{Local: map[string]string{"level": "error", "message": "closing the listener", "error": errorFailed}}
	unproven := &sdk.Candidate{Local: map[string]string{"level": "error", "message": "closing the listener", "error": errorUnproven}}
	known := &sdk.Candidate{Local: map[string]string{"level": "error", "message": "config reload", "error": errorNil}}
	identified := &sdk.Candidate{Local: map[string]string{"level": "error", "message": "streaming results", "error": errorIdentified}}
	for _, tc := range []struct {
		name            string
		c               *sdk.Candidate
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{"operation named, routine read: clean", failed, with(event("routine", 0.97), 0.9), false, false},
		{"operation named at the bar: clean", failed, with(event("routine", 0.97), operationOnly), false, false},
		{"operation named, unclear event: clean", failed, with(event("unclear", 0.9), 0.95), false, false},
		{"operation named, no event answer: clean", failed, with(nil, 0.95), false, false},
		{"says what happened: the event decides", failed, with(event("routine", 0.95), 0.1), true, false},
		{"below the bar: the event decides", failed, with(event("routine", 0.95), 0.69), true, false},
		{"below the bar, low event: abstain", failed, with(event("routine", 0.8), 0.5), false, true},
		{"below the bar, failure: clean", failed, with(event("failure", 0.95), 0.2), false, false},
		{"no operation answer: the event decides", failed, event("routine", 0.95), true, false},
		{"no operation answer, unclear: abstain", failed, event("unclear", 0.95), false, true},
		{"operation answer without a probability: the event decides", failed, map[string]sdk.Answer{
			"operation": {QuestionID: "operation", Choice: "action"}, "event": event("routine", 0.95)["event"],
		}, true, false},
		{"operation unclear: the event decides", failed, map[string]sdk.Answer{
			"operation": {QuestionID: "operation", Choice: "unclear", Probabilities: map[string]float64{"unclear": 0.9}}, "event": event("routine", 0.95)["event"],
		}, true, false},
		// An identified error is not the news: the operation answer, if any, is not read.
		{"identified: the event decides", identified, with(event("routine", 0.95), 0.95), true, false},
		{"identified, low: abstain", identified, with(event("routine", 0.85), 0.95), false, true},
		{"identified, failure: clean", identified, event("failure", 0.95), false, false},
		// An error not shown to be set: an action reading abstains instead of going clean.
		{"unproven, action: abstain", unproven, with(event("routine", 0.97), 0.95), false, true},
		{"unproven, action, failure read: abstain", unproven, with(event("failure", 0.97), 0.95), false, true},
		{"unproven, outcome: the event decides", unproven, with(event("routine", 0.95), 0.05), true, false},
		{"unproven, outcome, low: abstain", unproven, with(event("routine", 0.6), 0.05), false, true},
		// A known nil error reports no failure: the operation answer is not read.
		{"nil, action: the event decides", known, with(event("routine", 0.95), 0.99), true, false},
		{"nil, failure: clean", known, with(event("failure", 0.95), 0.99), false, false},
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
	unidentified := r.Questions(&sdk.Candidate{Local: map[string]string{"error": errorFailed}})
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

// TestExtractedDecisions runs extracted candidates through Decide with the answers that made the
// old false findings: the event read as routine, the message read as an action. Whether the
// override applies must follow from what extraction established.
func TestExtractedDecisions(t *testing.T) {
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
	byCase := map[string]*sdk.Candidate{}
	for _, r := range res {
		for _, c := range r.Result.([]*sdk.Candidate) {
			byCase[caseNo(t, c.Pos)] = c
		}
	}
	r := &rule{threshold: 0.9}
	answers := with(event("routine", 0.99), 0.99)
	for n, want := range map[string]string{
		"16": "clean",   // err != nil: an action with a set error reads as a failure
		"68": "report",  // err == nil: no failure, the routine answer stands
		"66": "report",  // context.Canceled by name: identified, the routine answer stands
		"17": "report",  // errors.Is(err, context.Canceled): identified
		"18": "abstain", // no check: not shown to be set
		"47": "abstain", // only what it is not
		"1":  "report",  // no error at all
	} {
		c := byCase[n]
		if c == nil {
			t.Fatalf("case %s: no candidate", n)
		}
		qs := r.Questions(c)
		asked := map[string]sdk.Answer{}
		for _, q := range qs {
			asked[q.ID] = answers[q.ID]
		}
		d := r.Decide(c, asked)
		got := "clean"
		switch {
		case d.Report:
			got = "report"
		case d.Abstained != "":
			got = "abstain"
		}
		if got != want {
			t.Errorf("case %s (%s): got %s, want %s", n, c.Local["error"], got, want)
		}
	}
}

// TestUnformattedFallthrough covers "fallthrough; ;", which gofmt rewrites and so cannot live in
// testdata: the empty statements after it do not hide the incoming edge, in a switch with a tag
// and without one.
func TestUnformattedFallthrough(t *testing.T) {
	const src = `package p

import (
	"errors"
	"io"
	"log/slog"
)

func tagged(err error) {
	switch err {
	case io.ErrUnexpectedEOF:
		fallthrough; ;
	case io.EOF:
		slog.Error("reading input", "err", err)
	}
}

func tagless(err error, short bool) {
	switch {
	case short:
		fallthrough; ;
	case errors.Is(err, io.EOF):
		slog.Error("reading tail", "err", err)
	}
}

func control(err error) {
	switch err {
	case io.ErrUnexpectedEOF:
		;
	case io.EOF:
		slog.Error("reading head", "err", err)
	}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("p", fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatal(err)
	}
	pass := &analysis.Pass{Fset: fset, Files: []*ast.File{f}, Pkg: pkg, TypesInfo: info,
		ResultOf: map[*analysis.Analyzer]any{inspect.Analyzer: inspector.New([]*ast.File{f})}}
	out, err := run(pass)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range out.([]*sdk.Candidate) {
		checks, _ := c.Payload.Facts["error_check"].([]string)
		got[c.Local["message"]] = strings.Join(checks, ", ") + "/" + c.Local["error"]
	}
	want := map[string]string{
		"reading input": "/unproven",
		"reading tail":  "/unproven",
		"reading head":  "is io.EOF/identified",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}
