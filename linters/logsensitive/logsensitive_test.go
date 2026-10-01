package logsensitive

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
	var cs []*sdk.Candidate
	for _, r := range res {
		cs = append(cs, r.Result.([]*sdk.Candidate)...)
	}
	sort.SliceStable(cs, func(i, j int) bool { return num(caseNo(t, cs[i].Pos)) < num(caseNo(t, cs[j].Pos)) })
	var got []string
	for _, c := range cs {
		n := caseNo(t, c.Pos)
		if fmt.Sprint(c.Payload) != "" && strings.Contains(fmt.Sprint(c.Payload), "hunter2") {
			t.Errorf("case %s sends a literal's value: %v", n, c.Payload)
		}
		if len(c.Payload.Source) > 0 {
			t.Errorf("case %s sends source: %v", n, c.Payload.Source)
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported: "+c.Unsupported)
			continue
		}
		got = append(got, fmt.Sprintf("%s %q %v", n, c.Payload.Prose["message"], c.Payload.Facts["fields"]))
	}
	want := []string{
		`1 "user logged in" [key password, value cfg.Password, type string]`,
		`4 "login" [key jwt, value tok, type string]`,
		`6 unsupported: the log fields are not readable`,
		`8 "login" [key user, value u, type string]`,
		`9 unsupported: a field value is an expression the analyzer cannot name`,
		`11 unsupported: a field key cannot be sent as a fact`,
		`12 "" [key secret, value s, type string]`,
		`14 "key loaded" [key key, value key, type slice of byte key cfg, value p, type pointer to a.Config]`,
		`15 "login" [key user, value u, type string]`,
		`15 unsupported: some log fields are not readable; only the readable ones are asked about`,
		`16 "login" [key request_id, value requestID, type string]`,
		`16 unsupported: some log fields are not readable; only the readable ones are asked about`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func role(choice string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"role": {QuestionID: "role", Choice: choice, Probabilities: map[string]float64{choice: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.85}
	c := &sdk.Candidate{Local: map[string]string{"keys": "password"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{role("secret", 0.9), true, false},
		{role("secret", 0.7), false, true},
		{role("public", 0.9), false, false},
		{role("public", 0.5), false, true},
		{role("personal", 0.95), false, false},
		{role("unclear", 0.9), false, true},
		{map[string]sdk.Answer{"role": {QuestionID: "role", Choice: "secret"}}, false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, role("secret", 0.9)); d.Message != `log field value looks like a secret: password` {
		t.Errorf("message: %s", d.Message)
	}
	two := &sdk.Candidate{Local: map[string]string{"keys": "user, token"}}
	if d := r.Decide(two, role("secret", 0.9)); d.Message != `log field value looks like a secret: one of user, token` {
		t.Errorf("message: %s", d.Message)
	}
}

func TestRedacts(t *testing.T) {
	for name, want := range map[string]bool{
		"Redact": true, "MaskToken": true, "HashPassword": true, "SHA256": true, "Redacted": true,
		"ShareLink": false, "Token": false, "String": false, "Hmac": true,
	} {
		if got := redacts(name); got != want {
			t.Errorf("redacts(%q) = %v", name, got)
		}
	}
}
