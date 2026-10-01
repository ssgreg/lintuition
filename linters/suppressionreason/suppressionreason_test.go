package suppressionreason

import (
	"fmt"
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

func TestExtraction(t *testing.T) {
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "e")
	var cs []*sdk.Candidate
	for _, r := range res {
		cs = append(cs, r.Result.([]*sdk.Candidate)...)
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].Pos.Line < cs[j].Pos.Line })
	var got []string
	for _, c := range cs {
		if c.Unsupported != "" {
			got = append(got, fmt.Sprintf("%d unsupported: %s", c.Pos.Line, c.Unsupported))
			continue
		}
		if len(c.Payload.Source) > 0 || len(c.Payload.Prose) != 1 {
			t.Errorf("line %d: payload %+v", c.Pos.Line, c.Payload)
		}
		got = append(got, fmt.Sprintf("%d %s %q suppressed=%q", c.Pos.Line, c.Payload.Facts["linter"], c.Payload.Prose["rationale"], c.Payload.Facts["suppressed"]))
	}
	errcheck := ` suppressed="an error returned by a call is not checked"`
	want := []string{
		`7 errcheck "a close error on a read-only file loses nothing"` + errcheck,
		`8 gosec "the path comes from the operator's own config" suppressed="a security weakness, such as hard-coded credentials, injection, weak crypto or unsafe file permissions"`,
		`14 unsupported: the directive names several linters; which one the reason addresses is not established`,
		`15 unsupported: no description of what mylinter reports`,
		`16 suppression-rationale "checked by hand" suppressed="a nolint reason that explains something other than what the suppressed linter reports"`,
		`17 unsupported: the description of quoted-doc is not a plain fact`,
		`18 lll "a URL cannot be wrapped // see the style guide" suppressed="a line is longer than the configured limit"`,
		`21 errcheck "spaced directive"` + errcheck,
		`22 errcheck "the buffer is small // errors from this best-effort cleanup are deliberately ignored"` + errcheck,
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
		{about("elsewhere", 0.9), true, false},
		{about("elsewhere", 0.75), false, true},
		{about("addresses", 0.9), false, false},
		{about("addresses", 0.5), false, true},
		{about("unclear", 0.9), false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, about("elsewhere", 0.9)); d.Message != `nolint rationale is about something other than what errcheck reports: "the file is small"` {
		t.Errorf("message: %s", d.Message)
	}
}

func TestReasonKeepsTheWholeComment(t *testing.T) {
	for in, want := range map[string]string{
		"the buffer is small // errors from this cleanup are ignored": "the buffer is small // errors from this cleanup are ignored",
		"a URL // see https://example.com/x":                          "a URL // see https://example.com/x",
		"cleanup only // want \"a twin mark\"":                        "cleanup only",
		"the mark // want \"is cut\" `only at the end`  ":             "the mark",
		"a want in the middle // want \"x\" is kept":                  "a want in the middle // want \"x\" is kept",
		"// want \"only a mark\"":                                     "",
		"// wanted is a word":                                         "// wanted is a word",
	} {
		if got := reasonOf(in); got != want {
			t.Errorf("reasonOf(%q) = %q, want %q", in, got, want)
		}
	}
}
