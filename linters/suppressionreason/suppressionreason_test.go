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

// caseNo is the number of the case function cN the candidate's line is in.
func caseNo(t *testing.T, pos token.Position) string {
	b, err := os.ReadFile(pos.Filename)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	for i := pos.Line - 1; i >= 0; i-- {
		if rest, ok := strings.CutPrefix(lines[i], "func c"); ok {
			n, _, _ := strings.Cut(rest, "(")
			return n
		}
	}
	return "?"
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
		n := caseNo(t, c.Pos)
		if c.Unsupported != "" {
			if len(c.Payload.Facts)+len(c.Payload.Prose)+len(c.Payload.Source) > 0 {
				t.Errorf("case %s: an unsupported candidate carries a payload %+v", n, c.Payload)
			}
			got = append(got, fmt.Sprintf("%s unsupported: %s", n, c.Unsupported))
			continue
		}
		if len(c.Payload.Source) > 0 || len(c.Payload.Prose) != 1 {
			t.Errorf("case %s: payload %+v", n, c.Payload)
		}
		for k := range c.Payload.Facts {
			if k != "linter" && k != "rule" && k != "suppressed" {
				t.Errorf("case %s sends an unexpected fact %s", n, k)
			}
		}
		line := fmt.Sprintf("%s %s", n, c.Payload.Facts["linter"])
		if r, ok := c.Payload.Facts["rule"]; ok {
			line += fmt.Sprintf(" rule=%s", r)
		}
		got = append(got, line+fmt.Sprintf(" %q suppressed=%q", c.Payload.Prose["rationale"], c.Payload.Facts["suppressed"]))
	}
	errcheck := ` suppressed="an error returned by a call is not checked"`
	gosec := ` suppressed="a security weakness, such as hard-coded credentials, injection, weak crypto or unsafe file permissions"`
	staticcheck := ` suppressed="a likely bug, a deprecated API or code that can be simplified"`
	revive := ` suppressed="a style, naming or documentation convention is not followed"`
	want := []string{
		`1 errcheck "a close error on a read-only file loses nothing"` + errcheck,
		`2 gosec "the path comes from the operator's own config"` + gosec,
		`6 unsupported: the directive names several linters; which one the reason addresses is not established`,
		`7 unsupported: no description of what mylinter reports`,
		`8 suppression-rationale "checked by hand" suppressed="a nolint reason that explains something other than what the suppressed linter reports"`,
		`9 unsupported: the description of quoted-doc is not a plain fact`,
		`10 lll "a URL cannot be wrapped // see the style guide" suppressed="a line is longer than the configured limit"`,
		`13 errcheck "spaced directive"` + errcheck,
		`14 errcheck "the buffer is small // errors from this best-effort cleanup are deliberately ignored"` + errcheck,
		`15 gosec rule=G304 "G304: a constant file name under the build directory" suppressed="File path provided as taint input"`,
		`16 gosec rule=G204 "the binary name is a constant (G204)" suppressed="Audit use of command execution"`,
		`17 gosec rule=G304 "G304 and again G304: a file name this program generated" suppressed="File path provided as taint input"`,
		`18 unsupported: the reason names several gosec rules; which one it addresses is not established`,
		`19 unsupported: no description of what gosec rule G999 reports`,
		`20 unsupported: no description of what gosec rule G307 reports`,
		`21 gosec "g304 does not apply, G3040 is no rule, xG304 is a word, GOFILE is an environment variable"` + gosec,
		`22 staticcheck rule=SA1019 "SA1019: the replacement needs a newer Go than we support" suppressed="Using a deprecated function, variable, constant or field"`,
		`23 staticcheck rule=S1000 "S1000: the select is kept for the timeout case added next" suppressed="Use plain channel send or receive instead of single-case select"`,
		`23 staticcheck rule=ST1003 "ST1003: the name follows the wire format" suppressed="Poorly chosen identifier"`,
		`23 staticcheck rule=QF1001 "QF1001: the condition reads as the spec states it" suppressed="Apply De Morgan law"`,
		`24 staticcheck "G304 is not ours to fix here"` + staticcheck,
		`25 staticcheck "SA10190 is a ticket number"` + staticcheck,
		`26 revive rule=var-naming "var-naming: the name mirrors the protocol field" suppressed="Naming rules"`,
		`27 revive rule=unused-receiver "disable-line:unused-receiver" suppressed="Suggests to rename or remove unused method receivers"`,
		`27 revive rule=unused-parameter "revive:disable-next-line:unused-parameter the hook signature is fixed" suppressed="Suggests to rename or remove unused function parameters"`,
		`28 revive "note: the exported name is part of the public API"` + revive,
		`29 revive "the range loop needs the index"` + revive,
		`30 unsupported: no description of what revive rule no-such-rule reports`,
		`31 errcheck "G104 already covers it; a close error on a read-only file loses nothing"` + errcheck,
		`32 unsupported: the reason names several revive rules; which one it addresses is not established`,
		`33 revive rule=unused-receiver "disable-line:unused-receiver,unused-receiver the plugin ABI fixes it" suppressed="Suggests to rename or remove unused method receivers"`,
		`34 unsupported: the reason names several revive rules; which one it addresses is not established`,
		`35 unsupported: no description of what revive rule range-extra2 reports`,
		`36 unsupported: no description of what revive rule Unused_Receiver reports`,
		`37 revive rule=unused-receiver "revive:disable:unused-receiver the plugin ABI needs the receiver" suppressed="Suggests to rename or remove unused method receivers"`,
		`38 unsupported: the reason names several revive rules; which one it addresses is not established`,
		`39 unsupported: no description of what revive rule use-any reports`,
		`39 unsupported: no description of what revive rule use-any reports`,
		`40 unsupported: no description of what revive rule brand-new-check reports`,
		`41 unsupported: the reason names several revive rules; which one it addresses is not established`,
		`42 revive "todo: the schema decides this name"` + revive,
		`42 revive "exported:the colon has no blank after it"` + revive,
		`43 gosec "the fixture G304.golden is readable by every test on purpose"` + gosec,
		`43 staticcheck "the file SA1019.txt is the fixture for this test"` + staticcheck,
		`43 gosec "the mode follows ticket SEC-G306 on shared fixtures"` + gosec,
		`43 gosec "the name éG304 comes from the generator"` + gosec,
		`43 gosec "written under testdata/G304/ by the generator"` + gosec,
		`44 gosec rule=G306 "G306: public fixtures, see ticket SEC-G304" suppressed="Poor file permissions used when writing to a file"`,
		`45 staticcheck rule=SA1019 "kept until the next major release, see SA1019." suppressed="Using a deprecated function, variable, constant or field"`,
		`45 gosec rule=G306 "a constant mode [\"G306\"] for public fixtures" suppressed="Poor file permissions used when writing to a file"`,
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
	for linter, tb := range ruleTables {
		if _, ok := golangci[linter]; !ok {
			t.Errorf("%s has a rule table but no general description", linter)
		}
		if len(tb.rules) == 0 {
			t.Errorf("%s: empty rule table", linter)
		}
		for id, d := range tb.rules {
			if !factRE.MatchString(d) || d == "" {
				t.Errorf("%s %s: %q cannot be sent as a fact", linter, id, d)
			}
			// Every ID in a table is one the reason pattern finds: first and inside a sentence for
			// an ID, first and after revive's disable syntax for a revive rule name.
			reasons := []string{id + ": fine", "fine, see " + id}
			if linter == "revive" {
				reasons[1] = "disable-line:" + id
			}
			for _, reason := range reasons {
				if r, one := citedRule(linter, reason); r != id || !one {
					t.Errorf("%s: %q reads as rule %q", linter, reason, r)
				}
			}
		}
	}
	// Reassigned IDs stay out: a reason naming them may mean the old check.
	for _, id := range []string{"G105", "G113", "G307"} {
		if _, ok := gosecRules[id]; ok {
			t.Errorf("gosec %s is in the table", id)
		}
	}
}

func TestCitedRule(t *testing.T) {
	for _, tc := range []struct {
		linter, reason, rule string
		one                  bool
	}{
		{"gosec", "G304: a fixed path", "G304", true},
		{"gosec", "a fixed path (G304), still G304", "G304", true},
		{"gosec", "G304 and G703: a fixed path", "", false},
		{"gosec", "g304: lower case", "", true},
		{"gosec", "G3040 is not a rule", "", true},
		{"gosec", "a fixed path", "", true},
		{"staticcheck", "SA1019: kept for old clients", "SA1019", true},
		{"staticcheck", "S1000 then ST1003", "", false},
		{"staticcheck", "SA1019 and G304", "SA1019", true},
		{"revive", "exported: part of the API", "exported", true},
		{"revive", "disable-next-line:unused-receiver", "unused-receiver", true},
		{"revive", "revive:disable-line:unknown-thing", "unknown-thing", true},
		{"revive", "todo: rename later", "", true},
		{"revive", "exported: retained API // var-naming: the wire field", "", false},
		{"revive", "see exported: not first", "", true},
		{"gosec", "the fixture G304.json is public", "", true},
		{"gosec", "see BUG-G304 and testdata/G304/x", "", true},
		{"gosec", "G306: public, see BUG-G304", "G306", true},
		{"gosec", "a constant name (G204).", "G204", true},
		{"gosec", "G304,G703: one path", "", false},
		{"revive", "disable-line:unused-receiver,unused-parameter both", "", false},
		{"revive", "disable-line:range-extra2 x", "range-extra2", true},
		{"revive", "disable-line:Bad_Name x", "Bad_Name", true},
		{"revive", "use-any: an undescribed rule is still cited", "use-any", true},
		{"revive", "brand-new-check: a hyphenated heading reads as a rule", "brand-new-check", true},
		{"revive", "exported,var-naming: two rules", "", false},
		{"errcheck", "G104: errcheck has no rule table", "", true},
		{"lll", "SA1019", "", true},
		// The reason as the analyzer sees it: a twin's want mark is already cut.
		{"gosec", reasonOf("G304: a fixed path // want \"G703\""), "G304", true},
	} {
		if r, one := citedRule(tc.linter, tc.reason); r != tc.rule || one != tc.one {
			t.Errorf("citedRule(%s, %q) = %q, %v; want %q, %v", tc.linter, tc.reason, r, one, tc.rule, tc.one)
		}
	}
}

func TestQuestions(t *testing.T) {
	qs := (&rule{threshold: 0.9}).Questions(&sdk.Candidate{})
	if len(qs) != 1 || qs[0].ID != "about" || qs[0].Kind != sdk.Choice {
		t.Fatalf("questions: %+v", qs)
	}
	q := qs[0]
	if err := q.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"`suppressed`", "`rationale`"} {
		if !strings.Contains(q.Text, field) {
			t.Errorf("the question does not name %s: %s", field, q.Text)
		}
	}
	var keys []string
	for _, o := range q.WithUnclear() {
		keys = append(keys, o.Key)
	}
	if strings.Join(keys, ",") != "addresses,elsewhere,unclear" {
		t.Errorf("options: %v", keys)
	}
	// The text is fixed: the same for every candidate.
	other := (&rule{threshold: 0.9}).Questions(&sdk.Candidate{Local: map[string]string{"linter": "gosec"}})
	if other[0].Text != q.Text {
		t.Error("the question text depends on the candidate")
	}
}

func about(choice string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"about": {QuestionID: "about", Choice: choice, Probabilities: map[string]float64{choice: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.9}
	c := &sdk.Candidate{Local: map[string]string{"linter": "errcheck", "rationale": "the file is small"}}
	for _, tc := range []struct {
		name            string
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{"elsewhere above the threshold", about("elsewhere", 0.95), true, false},
		{"elsewhere at the threshold", about("elsewhere", 0.9), true, false},
		{"elsewhere just below the threshold", about("elsewhere", 0.89), false, true},
		{"elsewhere at the old 0.8 default", about("elsewhere", 0.8), false, true},
		{"addresses above the threshold", about("addresses", 0.95), false, false},
		{"addresses below the threshold", about("addresses", 0.5), false, true},
		{"unclear however sure", about("unclear", 0.99), false, true},
		{"no probability at all", map[string]sdk.Answer{"about": {QuestionID: "about", Choice: "elsewhere"}}, false, true},
		{"no probability for the chosen option", map[string]sdk.Answer{"about": {QuestionID: "about", Choice: "elsewhere", Probabilities: map[string]float64{"addresses": 0.05}}}, false, true},
		{"no answer", map[string]sdk.Answer{}, false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%s: got %+v", tc.name, d)
		}
	}
	if d := r.Decide(c, about("elsewhere", 0.95)); d.Message != `nolint rationale is about something other than what errcheck reports: "the file is small"` {
		t.Errorf("message: %s", d.Message)
	}
	// A reason that names a rule is reported against that rule.
	g := &sdk.Candidate{Local: map[string]string{"linter": "gosec", "rule": "G304", "rationale": "G304: legacy code"}}
	if d := r.Decide(g, about("elsewhere", 0.95)); d.Message != `nolint rationale is about something other than gosec G304, the rule it names: "G304: legacy code"` {
		t.Errorf("message: %s", d.Message)
	}
}

func TestDefaultThreshold(t *testing.T) {
	var l sdk.Linter
	for _, x := range sdk.Linters() {
		if x.Name == Name {
			l = x
		}
	}
	if l.Version != "3" {
		t.Errorf("version %s", l.Version)
	}
	rl, err := l.New(&Settings{})
	if err != nil {
		t.Fatal(err)
	}
	if th := rl.(*rule).threshold; th != 0.9 {
		t.Errorf("default threshold %v", th)
	}
	bad := 1.5
	if _, err := l.New(&Settings{Threshold: &bad}); err == nil {
		t.Error("a threshold above 1 is accepted")
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
