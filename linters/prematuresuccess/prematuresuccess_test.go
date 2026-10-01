package prematuresuccess

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
	var got []string
	var cs []*sdk.Candidate
	for _, r := range res {
		cs = append(cs, r.Result.([]*sdk.Candidate)...)
	}
	sort.Slice(cs, func(i, j int) bool { return num(caseNo(t, cs[i].Pos)) < num(caseNo(t, cs[j].Pos)) })
	{
		for _, c := range cs {
			n := caseNo(t, c.Pos)
			if c.Unsupported != "" {
				got = append(got, n+" unsupported")
				continue
			}
			if len(c.Payload.Source) > 0 {
				t.Errorf("case %s sends source: %v", n, c.Payload.Source)
			}
			got = append(got, fmt.Sprintf("%s %s %q", n, c.Payload.Facts["next_call"], c.Payload.Prose["message"]))
		}
	}
	want := []string{
		`1 SaveConfig "config saved to disk"`,
		`3 SaveConfig "config saved to disk"`,
		`4 SaveConfig "config saved to disk"`,
		`8 unsupported`,
		`10 SaveConfigCopy "config saved to disk"`,
		`13 SaveConfig "config saved to disk"`,
		`14 unsupported`,
		`15 SaveConfig "config saved to disk"`,
		`16 unsupported`,
		`18 unsupported`,
		`19 unsupported`,
		`20 unsupported`,
		`21 unsupported`,
		`22 SaveConfig "config saved to disk"`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func claim(choice string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"claim": {QuestionID: "claim", Choice: choice, Probabilities: map[string]float64{choice: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.6}
	c := &sdk.Candidate{Local: map[string]string{"next_call": "SaveConfig", "message": "config saved to disk"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{claim("completed", 0.61), true, false},
		{claim("completed", 0.55), false, true},
		{claim("starting", 0.8), false, false},
		{claim("starting", 0.3), false, true},
		{claim("progress", 0.9), false, false},
		{claim("unclear", 0.3), false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, claim("completed", 0.7)); d.Message != `success logged before SaveConfig has returned: "config saved to disk"` {
		t.Errorf("message: %s", d.Message)
	}
}

func TestSharesWord(t *testing.T) {
	for _, tc := range []struct {
		msg, callee string
		want        bool
	}{
		{"config saved to disk", "SaveConfig", true},
		{"configs saved", "SaveConfig", true},
		{"cache warmed", "SaveConfig", false},
		{"uploaded the archive", "UploadArchive", true},
		{"done", "Do", false},
	} {
		if got := sharesWord(tc.msg, tc.callee); got != tc.want {
			t.Errorf("sharesWord(%q, %q) = %v", tc.msg, tc.callee, got)
		}
	}
}
