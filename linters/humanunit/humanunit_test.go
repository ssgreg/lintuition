package humanunit

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
		if len(c.Payload.Source) > 0 || len(c.Payload.Facts) > 0 {
			t.Errorf("case %s sends more than the format and verb: %v %v", n, c.Payload.Source, c.Payload.Facts)
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported: "+c.Unsupported)
			continue
		}
		got = append(got, fmt.Sprintf("%s %s %s %q %q", n, c.Subject, c.Local["unit"], c.Payload.Prose["format"], c.Payload.Prose["verb"]))
	}
	want := []string{
		`1 cases/Printf/verb1 seconds "request took %.0f ms" "%.0f"`,
		`2 cases/Printf/verb3 milliseconds "retry %d of %d in %d ms\n" "%d (verb 3 of 3)"`,
		`3 cases/Infof/verb1 minutes "waited %d s" "%d"`,
		`4 cases/Wrapf/verb1 hours "timed out after %.1f min" "%.1f"`,
		`5 cases/Sprintf/verb1 microseconds "%*d us" "%*d"`,
		`6 cases/Sprintf/verb1 nanoseconds "100%% done in %d ns" "%d"`,
		`9 unsupported: the format is not a constant string`,
		`10 unsupported: the format uses an explicit argument index`,
		`11 unsupported: no verb of the format is bound to this argument`,
		`16 cases/Printf/verb1 seconds "took %.1f s and %d ms" "%.1f (verb 1 of 2)"`,
		`16 cases/Printf/verb2 milliseconds "took %.1f s and %d ms" "%d (verb 2 of 2)"`,
		`17 cases/Sprintf/verb1 seconds "%5% seconds; one %f seconds; two %d ms" "%f (verb 1 of 2)"`,
		`17 cases/Sprintf/verb2 milliseconds "%5% seconds; one %f seconds; two %d ms" "%d (verb 2 of 2)"`,
		`20 cases/Sprintf/verb1 seconds "elapsed %d s" "%d"`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestParseVerbs(t *testing.T) {
	for _, tc := range []struct {
		format, want string
	}{
		{"a %d b %s", "%d@0 %s@1"},
		{"%-8.3f|%+v", "%-8.3f@0 %+v@1"},
		{"%*.*f %d", "%*.*f@2 %d@3"},
		{"100%% %x", "%x@0"},
		{"%[2]d", "error"},
		{"%.[1]d", "error"},
		{"trailing %", ""},
		{"%d then %-", "%d@0"},            // %!(NOVERB) consumes nothing
		{"%5% s %f s %d ms", "%f@0 %d@1"}, // a decorated percent consumes nothing
		{"%*% %d", "%d@1"},                // but its * width does
		{"%-08.3% %x", "%x@0"},
		{"%5. %d", "%5. @0 %d@1"}, // a '.' that is not the last byte starts a precision; here ' ' is the verb
		{"%5.", "%5.@0"},          // a last '.' is the verb
		{"%é %d", "%é@0 %d@1"},    // the verb is a rune
		{"%!d %d", "%!@0 %d@1"},
	} {
		vs, perr := parseVerbs(tc.format)
		var parts []string
		for _, v := range vs {
			parts = append(parts, fmt.Sprintf("%s@%d", v.text, v.arg))
		}
		got := strings.Join(parts, " ")
		if perr != "" {
			got = "error"
		}
		if got != tc.want {
			t.Errorf("parseVerbs(%q) = %s", tc.format, got)
		}
	}
}

// TestParseVerbsMatchesFmt checks the argument count against fmt itself: with exactly as many
// arguments as the parser binds, fmt reports neither a missing nor an extra one.
func TestParseVerbsMatchesFmt(t *testing.T) {
	for _, f := range []string{
		"a %d b %s", "%-8.3f|%+v", "%*.*f %d", "100%% %x", "%5% s %f s %d ms", "%*% %d",
		"%-08.3% %x", "%5. %d", "%é %d", "%!d %d", "%d then %-", "%.*d %d",
	} {
		vs, perr := parseVerbs(f)
		if perr != "" {
			t.Fatalf("%q: %s", f, perr)
		}
		n := 0
		if len(vs) > 0 {
			n = vs[len(vs)-1].arg + 1
		}
		// A * width or precision after the last verb consumes too; none of these formats has one.
		args := make([]any, n)
		for i := range args {
			args[i] = 7
		}
		out := fmt.Sprintf(f, args...)
		if strings.Contains(out, "%!(EXTRA") || strings.Contains(out, "(MISSING)") {
			t.Errorf("%q with %d args: %s", f, n, out)
		}
		if n > 0 {
			if out := fmt.Sprintf(f, args[:n-1]...); !strings.Contains(out, "MISSING") && !strings.Contains(out, "BADWIDTH") && !strings.Contains(out, "BADPREC") {
				t.Errorf("%q: one argument fewer than %d is not missing: %s", f, n, out)
			}
		}
	}
}

func unit(choice string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"unit": {QuestionID: "unit", Choice: choice, Probabilities: map[string]float64{choice: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.85}
	c := &sdk.Candidate{Local: map[string]string{"unit": "seconds", "format": "request took %.0f ms", "verb": "%.0f"}}
	for _, tc := range []struct {
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{unit("milliseconds", 0.9), true, false},
		{unit("milliseconds", 0.8), false, true}, // weak contradiction abstains
		{unit("seconds", 0.95), false, false},
		{unit("seconds", 0.6), false, true}, // weak agreement abstains
		{unit("unspecified", 0.9), false, false},
		{unit("unclear", 0.9), false, true},
		{map[string]sdk.Answer{"unit": {QuestionID: "unit", Choice: "minutes"}}, false, true},
	} {
		d := r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%+v: got %+v", tc.answers, d)
		}
	}
	if d := r.Decide(c, unit("milliseconds", 0.9)); d.Message != `text says milliseconds, the value is in seconds: "request took %.0f ms"` {
		t.Errorf("message: %s", d.Message)
	}
}
