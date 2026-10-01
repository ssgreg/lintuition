package sentinelname

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
		got = append(got, fmt.Sprintf("%s %s %q %q", n, c.Payload.Facts["name"], c.Payload.Facts["name_words"], c.Payload.Prose["text"]))
	}
	want := []string{
		`1 ErrNotFound "not found" "permission denied"`,
		`2 errClosed "closed" "connection closed"`,
		`3 ErrHTTPTimeout "http timeout" "request timed out"`,
		`4 ErrMissing "missing" "not found"`,
		`5 unsupported`,
		`6 unsupported`,
		`7 ErrPercent "percent" "100%w done"`,
		`14 ErrA "a" "a failed"`,
		`14 ErrB "b" "b failed"`,
		`15 unsupported`,
		`16 unsupported`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func condition(choice string, probs map[string]float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"condition": {QuestionID: "condition", Choice: choice, Probabilities: probs}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.55, margin: 0.2}
	c := &sdk.Candidate{Local: map[string]string{"name": "ErrNotFound", "text": "permission denied"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{condition("different", map[string]float64{"different": 0.7, "same": 0.2}), true, false},
		{condition("different", map[string]float64{"different": 0.55, "same": 0.3}), true, false},
		{condition("different", map[string]float64{"different": 0.5, "same": 0.1}), false, true},          // below the threshold
		{condition("different", map[string]float64{"different": 0.56, "same": 0.44}), false, true},        // within the margin
		{condition("same", map[string]float64{"same": 0.9, "different": 0.05}), false, false},             // clean
		{condition("same", map[string]float64{"same": 0.4, "different": 0.35}), false, true},              // weak clean abstains
		{condition("insufficient", map[string]float64{"insufficient": 0.8}), false, false},                // too vague: no finding
		{condition("unclear", map[string]float64{"unclear": 0.9}), false, true},                           // unclear
		{map[string]sdk.Answer{"condition": {QuestionID: "condition", Choice: "different"}}, false, true}, // no probability
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	d := r.Decide(c, condition("different", map[string]float64{"different": 0.8}))
	if d.Message != `sentinel error name and message describe different conditions: ErrNotFound "permission denied"` {
		t.Errorf("message: %s", d.Message)
	}
}

func TestNameWords(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		ok         bool
	}{
		{"ErrNotFound", "not found", true},
		{"errNoRows", "no rows", true},
		{"Err_Closed", "closed", true},
		{"Err", "", false},
		{"Errata", "", false},
		{"Timeout", "", false},
	} {
		got, ok := nameWords(tc.name)
		if got != tc.want || ok != tc.ok {
			t.Errorf("nameWords(%q) = %q, %v", tc.name, got, ok)
		}
	}
}
