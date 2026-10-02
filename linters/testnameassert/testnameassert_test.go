package testnameassert

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

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
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "c")
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
		if len(c.Payload.Source) > 0 || len(c.Payload.Prose) != 2 || len(c.Payload.Facts) != 2 {
			t.Errorf("case %s: payload %+v", n, c.Payload)
		}
		line := fmt.Sprintf("%s %q call=%s expect=%s", n, c.Payload.Prose["test"], c.Payload.Facts["call"], c.Local["expect"])
		switch su := c.Payload.Facts["setup"]; su {
		case "the test passes an error value to the call as an argument":
			line += " error_argument"
		case "no error argument observed":
		default:
			t.Errorf("case %s: setup %q", n, su)
		}
		// Every supported payload must pass the policy check the engine applies before sending.
		if _, err := classify.State(c.Payload, classify.Prose); err != nil {
			t.Errorf("case %s: %v", n, err)
		}
		if d := c.Payload.Prose["doc"]; d != "" {
			line += fmt.Sprintf(" doc=%q", d)
		}
		got = append(got, line)
	}
	want := []string{
		`1 "validate rejects empty" call=Validate expect=error`,
		`2 "validate accepts token" call=Validate expect=no_error`,
		`3 "parse returns number" call=Parse expect=no_error`,
		`4 "parse fails on letters" call=Parse expect=error`,
		`5 "parse file reads header" call=ParseFile expect=no_error`,
		`6 "validate refuses blank" call=Validate expect=error`,
		`8 unsupported: the test calls Validate more than once`,
		`9 unsupported: the error variable of Validate is written again`,
		`10 unsupported: the error is not asserted`,
		`11 unsupported: the error of Validate is not kept`,
		`12 unsupported: the error is asserted both ways`,
		`13 unsupported: the error is checked in a form not read here`,
		`15 "validate rejects letters" call=Validate expect=error`,
		`16 unsupported: the test name matches several called functions`,
		`17 "store save fails when full" call=Save expect=no_error`,
		`19 "validate in subtest" call=Validate expect=error`,
		`20 unsupported: the error variable of Validate is written again`,
		`21 unsupported: the error is checked in a form not read here`,
		`22 unsupported: the error is checked in a form not read here`,
		`24 unsupported: the error is checked in a form not read here`,
		`25 unsupported: the error is checked in a form not read here`,
		`26 unsupported: the error is checked in a form not read here`,
		`27 unsupported: the error is checked in a form not read here`,
		`28 "validate rejects deferred" call=Validate expect=error`,
		`29 "validate accepts logged" call=Validate expect=no_error`,
		`30 unsupported: the error is checked in a form not read here`,
		`31 unsupported: the error is checked in a form not read here`,
		`32 unsupported: the error is checked in a form not read here`,
		`33 unsupported: the error is checked in a form not read here`,
		`34 unsupported: the error is checked in a form not read here`,
		`35 unsupported: the error is checked in a form not read here`,
		`36 unsupported: the error is checked in a form not read here`,
		`37 unsupported: the error is checked in a form not read here`,
		`38 unsupported: the error is checked in a form not read here`,
		`39 "validate accepts converted" call=Validate expect=no_error`,
		`40 "validate accepts error text" call=Validate expect=no_error`,
		`41 unsupported: the error is checked in a form not read here`,
		`42 unsupported: the error is checked in a form not read here`,
		`43 "validate accepts documented" call=Validate expect=no_error doc="this test checks that a plain token passes; TestValidateAcceptsDocumentedToo\nis a different name and stays."`,
		`44 "validate accepts spaced" call=Validate expect=no_error doc="Every mention is masked: see this test (this test)."`,
		`45 "wrap nil error" call=Wrap expect=no_error error_argument`,
		`46 "join nothing" call=Join expect=no_error`,
		`47 "collect empty" call=Collect expect=no_error`,
		`48 "store restore keeps cause" call=Restore expect=error error_argument`,
		`49 "validate accepts undocumented" call=Validate expect=no_error`,
		`50 "join one" call=Join expect=no_error error_argument`,
		`51 "join spread" call=Join expect=no_error`,
		`52 "store join through expression" call=Join expect=no_error`,
		`53 "store join through expression one" call=Join expect=no_error error_argument`,
		`54 "store join through expression spread" call=Join expect=no_error`,
		`55 "store prefix through expression" call=Prefix expect=no_error`,
		`56 "store prefix direct one" call=Prefix expect=no_error error_argument`,
		`57 "unwrap explicit" call=Unwrap expect=no_error error_argument`,
		`58 "unwrap inferred" call=Unwrap expect=error error_argument`,
		`59 "consume forwarded" call=Consume expect=no_error error_argument`,
		`60 "tally forwarded" call=Tally expect=no_error`,
		`61 "alias nil error" call=Alias expect=no_error error_argument`,
		`62 "wrap a very long function name that goes on and on and on until it is longer than any ordinary name would be nil error" call=WrapAVeryLongFunctionNameThatGoesOnAndOnAndOnUntilItIsLongerThanAnyOrdinaryNameWouldBe expect=no_error error_argument`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func nameSays(choice string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"name_says": {QuestionID: "name_says", Choice: choice, Probabilities: map[string]float64{choice: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.9}
	noErr := &sdk.Candidate{Local: map[string]string{"call": "Validate", "expect": "no_error"}}
	wantErr := &sdk.Candidate{Local: map[string]string{"call": "Validate", "expect": "error"}}
	for _, tc := range []struct {
		c               *sdk.Candidate
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{noErr, nameSays("error", 0.95), true, false},
		{noErr, nameSays("error", 0.8), false, true},
		{noErr, nameSays("no_error", 0.95), false, false},
		{noErr, nameSays("no_error", 0.5), false, true},
		{wantErr, nameSays("no_error", 0.95), true, false},
		{wantErr, nameSays("error", 0.95), false, false},
		{wantErr, nameSays("input_only", 0.95), false, false},
		{wantErr, nameSays("input_only", 0.6), false, true},
		{noErr, nameSays("unclear", 0.95), false, true},
		// At the threshold the answer counts; just below it abstains.
		{noErr, nameSays("error", 0.9), true, false},
		{noErr, nameSays("error", 0.89), false, true},
		// A name that describes the input or an arranged failure is clean whichever way the test asserts.
		{noErr, nameSays("input_only", 0.99), false, false},
		{wantErr, nameSays("input_only", 0.99), false, false},
		// A choice without its probability abstains, as does an answer with none at all.
		{noErr, map[string]sdk.Answer{"name_says": {QuestionID: "name_says", Choice: "error", Probabilities: map[string]float64{"no_error": 0.95}}}, false, true},
		{noErr, map[string]sdk.Answer{}, false, true},
	} {
		d := r.Decide(tc.c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%v %+v: got %+v", tc.c.Local, tc.answers, d)
		}
	}
	if d := r.Decide(noErr, nameSays("error", 0.95)); d.Message != "test name expects an error from Validate, but the test fails when Validate returns one" {
		t.Errorf("message: %s", d.Message)
	}
	if d := r.Decide(wantErr, nameSays("no_error", 0.95)); d.Message != "test name expects Validate to succeed, but the test fails when Validate returns no error" {
		t.Errorf("message: %s", d.Message)
	}
}

// TestQuestionIsFixed checks that the question does not depend on the candidate and refers to every
// field the payload sends, so a request carries text a person wrote only as state, never as
// instructions.
func TestQuestionIsFixed(t *testing.T) {
	r := &rule{threshold: 0.9}
	a := r.Questions(&sdk.Candidate{Local: map[string]string{"test": "TestA"}})
	b := r.Questions(&sdk.Candidate{Local: map[string]string{"test": "TestB"}})
	if fmt.Sprint(a) != fmt.Sprint(b) || len(a) != 1 {
		t.Fatalf("questions differ or are not one: %v / %v", a, b)
	}
	for _, field := range []string{"`test`", "`doc`", "`call`", "`setup`"} {
		if !strings.Contains(a[0].Text, field) {
			t.Errorf("question does not refer to %s: %s", field, a[0].Text)
		}
	}
	if err := a[0].Validate(); err != nil {
		t.Fatal(err)
	}
}

// TestFixtureDocsCarryNoHints checks that the twins' and the showcase's explanations ("Defect: ...",
// "test-name-vs-assertion: ...") are not part of a test's doc comment, which this linter sends: they
// would tell the classifier the answer and make a live evaluation measure the hint.
func TestFixtureDocsCarryNoHints(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/twins/testname/*_test.go")
	files = append(files, "../../examples/showcase/tests_test.go")
	if len(files) < 2 {
		t.Fatalf("fixtures not found: %v", files)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Doc == nil {
				continue
			}
			for _, hint := range []string{"Defect", "Fixed twin", "Negative", "test-name-vs-assertion", "want"} {
				if strings.Contains(fd.Doc.Text(), hint) {
					t.Errorf("%s: the doc of %s carries %q", name, fd.Name.Name, hint)
				}
			}
		}
	}
}
