package suppressionreason

import (
	"fmt"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ssgreg/lintuition/sdk"
)

func init() {
	// A plugin linter whose Doc cannot be sent as a fact.
	sdk.RegisterLinter(sdk.Linter{
		Name:     "quoted-doc",
		Doc:      `a "quoted" doc`,
		Analyzer: &analysis.Analyzer{Name: "quoteddoc", Doc: "x", ResultType: sdk.CandidatesType, Run: func(*analysis.Pass) (any, error) { return nil, nil }},
		New:      func(any) (sdk.Rule, error) { return nil, nil },
	})
}

// caseNo reads the last "// N" case number from the candidate's source line.
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
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "e")
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
		if len(c.Payload.Source) > 0 || len(c.Payload.Prose) != 1 {
			t.Errorf("case %s: payload %+v", n, c.Payload)
		}
		got = append(got, fmt.Sprintf("%s %s %q suppressed=%q", n, c.Payload.Facts["linter"], c.Payload.Prose["rationale"], c.Payload.Facts["suppressed"]))
	}
	want := []string{
		`1 errcheck "a close error on a read-only file loses nothing" suppressed="an error returned by a call is not checked"`,
		`2 gosec "the path comes from the operator's own config" suppressed="a security weakness, such as hard-coded credentials, injection, weak crypto or unsafe file permissions"`,
		`6 unsupported: the directive names several linters; which one the reason addresses is not established`,
		`7 unsupported: no description of what mylinter reports`,
		`8 suppression-rationale "checked by hand" suppressed="a nolint reason that explains something other than what the suppressed linter reports"`,
		`9 unsupported: the description of quoted-doc is not a plain fact`,
		`10 lll "a URL cannot be wrapped" suppressed="a line is longer than the configured limit"`,
		`13 errcheck "spaced directive" suppressed="an error returned by a call is not checked"`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDescriptionsAreFacts(t *testing.T) {
	for name, d := range golangci {
		if !factRE.MatchString(d) {
			t.Errorf("%s: %q cannot be sent as a fact", name, d)
		}
	}
}

func about(choice string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"about": {QuestionID: "about", Choice: choice, Probabilities: map[string]float64{choice: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.8}
	c := &sdk.Candidate{Local: map[string]string{"linter": "errcheck", "rationale": "the file is small"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{about("other", 0.9), true, false},
		{about("other", 0.75), false, true},
		{about("that_risk", 0.9), false, false},
		{about("that_risk", 0.5), false, true},
		{about("unclear", 0.9), false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, about("other", 0.9)); d.Message != `nolint rationale is about something other than what errcheck reports: "the file is small"` {
		t.Errorf("message: %s", d.Message)
	}
}
