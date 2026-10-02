package docsignature

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
	seen := map[string]bool{}
	for _, r := range res {
		for _, c := range r.Result.([]*sdk.Candidate) {
			// The test variant of the package repeats the non-test files.
			k := c.Pos.String()
			if seen[k] {
				continue
			}
			seen[k] = true
			cs = append(cs, c)
		}
	}
	sort.SliceStable(cs, func(i, j int) bool { return num(caseNo(t, cs[i].Pos)) < num(caseNo(t, cs[j].Pos)) })
	var got []string
	for _, c := range cs {
		n := caseNo(t, c.Pos)
		if len(c.Payload.Source) > 0 || len(c.Payload.Facts) > 0 {
			t.Errorf("case %s sends more than the doc: facts %v, source %v", n, c.Payload.Facts, c.Payload.Source)
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported")
			continue
		}
		got = append(got, fmt.Sprintf("%s %s %q", n, c.Local["claim"], c.Payload.Prose["doc"]))
	}
	want := []string{
		`1 result "the documented function writes the buffer to disk and returns the number of bytes written."`,
		`2 result "the documented function reports whether the buffer is non-empty."`,
		`3 result "the documented function blocks and returns when ctx is done."`,
		`7 result "the documented function returns the size; see GetSizeLimit."`,
		`8 error "the documented function returns an error if the name is empty."`,
		`9 error "the documented function returns ErrNotFound when the key is missing."`,
		`14 unsupported`,
		`15 unsupported`,
		`16 unsupported`,
		`17 unsupported`,
		`18 unsupported`,
		`19 error "the documented function returns an error description."`,
		`20 result "the documented function writes pending data and returns the count."`,
		`21 error "the documented function returns the value or an error."`,
		`26 error "the documented function logs an error when the counter overflows."`,
		`33 result "the documented function returns the fixture path."`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func yes(id string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{id: {QuestionID: id, Yes: &p}}
}

func TestQuestions(t *testing.T) {
	r := &rule{threshold: 0.85}
	for claim, id := range map[string]string{claimResult: "returns_result", claimError: "returns_error"} {
		qs := r.Questions(&sdk.Candidate{Local: map[string]string{"claim": claim}})
		if len(qs) != 1 || qs[0].ID != id || qs[0].Kind != sdk.Noul || !strings.Contains(qs[0].Text, "`doc`") {
			t.Errorf("claim %s: %+v", claim, qs)
		}
		if err := qs[0].Validate(); err != nil {
			t.Errorf("claim %s: %v", claim, err)
		}
	}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.85}
	result := &sdk.Candidate{Local: map[string]string{"name": "Sync", "claim": claimResult}}
	errc := &sdk.Candidate{Local: map[string]string{"name": "Validate", "claim": claimError}}
	for _, tc := range []struct {
		name            string
		c               *sdk.Candidate
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{"result: promised", result, yes("returns_result", 0.95), true, false},
		{"result: at the threshold", result, yes("returns_result", 0.85), true, false},
		{"result: not promised", result, yes("returns_result", 0.05), false, false},
		{"result: no at the threshold", result, yes("returns_result", 0.15), false, false},
		{"result: weak yes abstains", result, yes("returns_result", 0.7), false, true},
		{"result: weak no abstains", result, yes("returns_result", 0.3), false, true},
		{"result: no probability", result, map[string]sdk.Answer{"returns_result": {QuestionID: "returns_result"}}, false, true},
		{"result: the other question's answer is ignored", result, yes("returns_error", 0.99), false, true},
		{"error: promised", errc, yes("returns_error", 0.9), true, false},
		{"error: not promised", errc, yes("returns_error", 0.1), false, false},
		{"error: weak abstains", errc, yes("returns_error", 0.5), false, true},
		{"error: missing answer", errc, map[string]sdk.Answer{}, false, true},
	} {
		d := r.Decide(tc.c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%s: got %+v", tc.name, d)
		}
	}
	if d := r.Decide(result, yes("returns_result", 0.9)); d.Message != "doc of Sync says it returns a result, but Sync returns nothing" {
		t.Errorf("message: %s", d.Message)
	}
	if d := r.Decide(errc, yes("returns_error", 0.9)); d.Message != "doc of Validate says it returns an error, but Validate has no error result" {
		t.Errorf("message: %s", d.Message)
	}
}

func TestWords(t *testing.T) {
	for _, tc := range []struct {
		text          string
		ret, errWords bool
	}{
		{"Run returns when done.", true, false},
		{"Valid Reports Whether x.", true, false},
		{"Next yields the next item.", true, false},
		{"The returned value is cached.", true, false},
		{"Returning early is fine.", true, false},
		{"Close closes it.", false, false},
		{"It logs an error.", false, true},
		{"Errors are collected.", false, true},
		{"err is always nil.", false, true},
		{"See ErrClosed.", false, true},
		{"See errNoRows.", false, true},
		{"Uses errgroup.", false, false},
		{"Errorf builds a message.", false, false},
		{"Prints an errno.", false, false},
		{"A returnable value.", false, false},
	} {
		if got := returnWords.MatchString(tc.text); got != tc.ret {
			t.Errorf("return words in %q: %v", tc.text, got)
		}
		if got := errorWords.MatchString(tc.text); got != tc.errWords {
			t.Errorf("error words in %q: %v", tc.text, got)
		}
	}
}
