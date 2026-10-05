package enumcomment

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ssgreg/lintuition/sdk"
)

// caseNo reads the case number from the "// N" comment above the const block around pos, or above
// the function that holds it. A comment inside the block would be a candidate itself, so the number
// sits outside.
func caseNo(t *testing.T, pos token.Position) int {
	b, err := os.ReadFile(pos.Filename)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	for i := pos.Line - 1; i > 0; i-- {
		if strings.TrimSpace(lines[i]) != "const (" {
			continue
		}
		for j := i - 1; j >= 0 && j >= i-2; j-- {
			var n int
			if _, err := fmt.Sscanf(strings.TrimSpace(lines[j]), "// %d", &n); err == nil {
				return n
			}
		}
		t.Fatalf("%s: no case number above the block", pos)
	}
	t.Fatalf("%s: not in a const block", pos)
	return 0
}

func TestExtraction(t *testing.T) {
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
	type line struct {
		n int
		s string
	}
	var got []line
	r := &rule{}
	for _, res := range res {
		for _, c := range res.Result.([]*sdk.Candidate) {
			n := caseNo(t, c.Pos)
			if len(c.Payload.Source) > 0 || len(c.Payload.Facts) > 0 {
				t.Errorf("case %d %s sends more than the comment: %v %v", n, c.Subject, c.Payload.Source, c.Payload.Facts)
			}
			if c.Unsupported != "" {
				got = append(got, line{n, fmt.Sprintf("%d %s unsupported: %s", n, c.Subject, c.Unsupported)})
				continue
			}
			if c.Local["comment"] == "" || c.Local["name"] == "" {
				t.Errorf("case %d %s: local facts missing: %v", n, c.Subject, c.Local)
			}
			var keys []string
			for _, o := range r.Questions(c)[0].Options {
				keys = append(keys, o.Key)
			}
			got = append(got, line{n, fmt.Sprintf("%d %s %q [%s]", n, c.Subject, c.Payload.Prose["comment"], strings.Join(keys, " "))})
		}
	}
	sort.SliceStable(got, func(i, j int) bool {
		if got[i].n != got[j].n {
			return got[i].n < got[j].n
		}
		return got[i].s < got[j].s
	})
	var gs []string
	for _, l := range got {
		gs = append(gs, l.s)
	}
	want := []string{
		`1 StateIdle/doc "a job nobody has picked up yet" [StateIdle StateRunning StateDone none]`,
		`1 StateRunning/line "the job finished and its result is stored" [StateIdle StateRunning StateDone none]`,
		`4 A/line unsupported: the comment is on a line of several constants`,
		`6 none/line unsupported: a constant is named like an answer option`,
		`6 other/line unsupported: a constant is named like an answer option`,
		`7 unclear/line unsupported: a constant is named like an answer option`,
		`8 LocalA/doc "inside a function" [LocalA LocalB none]`,
		`9 ModeFast/doc "this constant skips the checksum; ModeFast is the default." [ModeFast ModeSafe none]`,
		`9 ModeSafe/doc "this constant: verifies the checksum of every block." [ModeFast ModeSafe none]`,
		`10 PhaseCopying/doc "this constant means every block was copied and checked." [PhaseCopying PhaseChecked none]`,
		`11 debug/doc "Enable extra checks while developing." [debug trace none]`,
		`11 trace/doc "If trace is set, debugging output is printed." [debug trace none]`,
		`12 StepCopying/doc "StepVerified means every block was checked." [StepCopying StepVerified none]`,
		`13 ReadOnly/doc "ReadWrite permits reads and writes, and everything ReadOnly permits." [ReadOnly ReadWrite none]`,
		`14 anyIdle/doc unsupported: the comment opens with several constants' names`,
		`14 readIdle/doc unsupported: the comment opens with several constants' names`,
		`14 writeIdle/doc unsupported: the comment opens with several constants' names`,
		`15 LevelHigh/doc "this constant is like LevelLow, but louder." [LevelLow LevelHigh none]`,
		`15 LevelLow/doc "this constant is quiet." [LevelLow LevelHigh none]`,
		`16 P/doc "A placeholder until the value is known." [P Q none]`,
		`16 Q/doc "P is a letter here." [P Q none]`,
		`17 Run/doc "Running jobs are counted here." [Run Stop none]`,
		`18 Ready/doc "PréReady describes a queued job." [ÉtatPrêt StatoΩ Ready PréReady none]`,
		`18 StatoΩ/doc "this constant is the idle state." [ÉtatPrêt StatoΩ Ready PréReady none]`,
		`18 ÉtatPrêt/doc "this constant means the worker can take a job." [ÉtatPrêt StatoΩ Ready PréReady none]`,
		`19 limitBody/doc unsupported: the comment matches another constant's comment but for one word that names no constant`,
		`19 limitLink/doc unsupported: the comment matches another constant's comment but for one word that names no constant`,
		`19 limitTitle/doc unsupported: the comment matches another constant's comment but for one word that names no constant`,
		`20 codeOne/doc unsupported: the comment matches another constant's comment word for word`,
		`20 codeSix/line "a value of its own" [codeOne codeTwo codeSix none]`,
		`20 codeTwo/line unsupported: the comment matches another constant's comment word for word`,
		`21 WorkRunning/doc "the operation completed successfully" [WorkRunning WorkDone none]`,
		`22 CopyDone/doc "this constant means the copy completed successfully." [CopyRunning CopyDone none]`,
		`22 CopyRunning/doc "CopyDone means the copy completed successfully." [CopyRunning CopyDone none]`,
		`23 AccessRead/doc "the operation permits writes" [AccessWrite AccessRead none]`,
		`23 AccessWrite/doc "the operation permits reads" [AccessWrite AccessRead none]`,
		`24 KindForBody/line "body of ForStmt" [KindForDone KindIfDone KindForBody none]`,
		`24 KindForDone/line unsupported: the comment matches another constant's comment but for one word that names no constant`,
		`24 KindIfDone/line unsupported: the comment matches another constant's comment but for one word that names no constant`,
		`25 readFast/line unsupported: the comment matches another constant's comment but for one word that names no constant`,
		`25 writeFast/line unsupported: the comment matches another constant's comment but for one word that names no constant`,
		`26 inTimeout/line "read timeout" [inTimeout outTimeout none]`,
		`26 outTimeout/line "write timeout" [inTimeout outTimeout none]`,
		`27 kindBody/line "body of switch" [kindHead kindTail kindBody none]`,
		`27 kindHead/line "head of loop" [kindHead kindTail kindBody none]`,
		`27 kindTail/line "block after loop" [kindHead kindTail kindBody none]`,
		`28 SlotL/doc unsupported: the comment matches another constant's comment but for one word that names no constant`,
		`28 SlotR/doc unsupported: the comment matches another constant's comment but for one word that names no constant`,
		`29 ruleRead/line unsupported: the comment matches another constant's comment but for one word that names no constant`,
		`29 ruleWrite/line unsupported: the comment matches another constant's comment but for one word that names no constant`,
		`30 FlagOn/doc "this constant is set by default." [FlagOn FlagOff none]`,
		`30 FlagOn/line "turned off by the operator" [FlagOn FlagOff none]`,
		`32 KindPrint/line "behaves like fmt.Print" [KindPrint KindPrintf none]`,
		`32 KindPrintf/line "behaves like fmt.Printf" [KindPrint KindPrintf none]`,
		`33 K01/line unsupported: the block has more than 20 constants to offer as options`,
		`33 K02/line unsupported: the block has more than 20 constants to offer as options`,
		`34 OptCompiledFiles/doc "this constant adds CompiledFiles." [OptExportFile OptCompiledFiles OptSyntaxTree none]`,
		`34 OptExportFile/doc "this constant adds ExportFile." [OptExportFile OptCompiledFiles OptSyntaxTree none]`,
		`34 OptSyntaxTree/doc "this constant adds Syntax." [OptExportFile OptCompiledFiles OptSyntaxTree none]`,
	}
	if strings.Join(gs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(gs, "\n"), strings.Join(want, "\n"))
	}
}

func TestWords(t *testing.T) {
	names := map[string]bool{"StateIdle": true, "trace": true, "ÉtatPrêt": true, "Run": true, "PréRun": true, "A": true, "KA": true, "KB": true}
	for _, tc := range []struct {
		text, subj string
		group      bool
	}{
		{"StateIdle is a job nobody has picked up.", "StateIdle", false},
		{"StateIdle: waiting.", "StateIdle", false},
		{"If trace is set, output is printed.", "", false},
		{"ÉtatPrêt means ready.", "ÉtatPrêt", false},
		{"PréRun describes a queued job.", "PréRun", false},
		{"Running jobs are counted.", "", false},
		{"StateIdleSince is the start.", "", false},
		{"A placeholder.", "", false},
		{"KA and KB cut a stalled connection.", "KA", true},
		{"KA, KB: both restart.", "KA", true},
		{"KA, and KB too.", "KA", true},
		{"KA or KB, whichever is first.", "KA", true},
		{"KA and then more.", "KA", false},
		{"[KA] is linked.", "", false},
		{"", "", false},
	} {
		subj, group := subject(tokenize(tc.text), names)
		if subj != tc.subj || group != tc.group {
			t.Errorf("subject(%q) = %q %v", tc.text, subj, group)
		}
	}
	if got := fmt.Sprint(template(tokenize("Same rule as KA, for (reads); 5 ÉtatPrêt."), names)); got != "[same rule as \x00 for reads 5 \x00] [Same rule as KA for reads 5 ÉtatPrêt]" {
		t.Errorf("template: %q", got)
	}
	for in, want := range map[string]string{
		"KindForDone":    "[kind for done]",
		"maxURLLenRunes": "[max url len runes]",
		"read_fast":      "[read fast]",
		"HTTPServer":     "[http server]",
		"ÉtatPrêt":       "[état prêt]",
		"K01":            "[k01]",
	} {
		if got := fmt.Sprint(splitName(in)); got != want {
			t.Errorf("splitName(%s) = %s, want %s", in, got, want)
		}
	}
	parts := nameParts([]string{"AccessRead", "AccessWrite", "KindPrint", "KindPrintf", "NeedFiles", "NeedEmbedFiles", "NeedCompiledGoFiles", "NeedExportFile", "KindForBody", "KindForDone"})
	if got := singles("reads", nameParts([]string{"AccessRead", "readFast"})); got != "" {
		t.Errorf("reads fits two constants, got %q", got)
	}
	for w, want := range map[string]string{
		"reads":           "AccessRead",
		"writes":          "AccessWrite",
		"print":           "KindPrint",
		"printf":          "KindPrintf",
		"read":            "AccessRead",
		"access":          "",
		"readers":         "",
		"512":             "",
		"ExportFile":      "NeedExportFile",
		"CompiledGoFiles": "NeedCompiledGoFiles",
		"EmbedFiles":      "NeedEmbedFiles",
		"Files":           "", // NeedFiles and two more
		"ForStmt":         "", // for fits both Kind constants, stmt fits none
		"Stmt":            "",
	} {
		if got := singles(w, parts); got != want {
			t.Errorf("singles(%s) = %q, want %q", w, got, want)
		}
	}
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"a b c", "a b c", -1},
		{"a b c", "a x c", 1},
		{"a b c", "x y c", -2},
		{"a b", "a b c", -2},
	} {
		if got := differing(strings.Fields(tc.a), strings.Fields(tc.b)); got != tc.want {
			t.Errorf("differing(%q, %q) = %d", tc.a, tc.b, got)
		}
	}
}

// Want comments cannot sit in the analysistest fixture, which would expect diagnostics for them, so
// this block is parsed here.
func TestWantComments(t *testing.T) {
	src := "package p\nconst (\n" +
		"WantA = 1 // want `comment describes WantB, not WantA`\n" +
		"WantB = 2 // want \"comment describes \\\"x\\\", not WantB\"\n" +
		"WantC = 3 // wanted by the scheduler\n" +
		"WantD = 4 // want more retries here\n" +
		"// want `a doc comment is the constant's comment`\n" +
		"WantE = 5\n" +
		"WantF = 6 // want `a pattern` and more\n" +
		")\n"
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range block(&analysis.Pass{Fset: fs}, f.Decls[0].(*ast.GenDecl)) {
		got = append(got, fmt.Sprintf("%s %q %s", c.Subject, c.Payload.Prose["comment"], c.Unsupported))
	}
	want := []string{
		`WantC/line "wanted by the scheduler" `,
		`WantD/line "want more retries here" `,
		"WantE/doc \"want `a doc comment is the constant's comment`\" ",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// bigBlock is one const block of n constants, each with a doc comment that opens with its name.
func bigBlock(t testing.TB, n int) (*analysis.Pass, *ast.GenDecl) {
	var src strings.Builder
	src.WriteString("package p\nconst (\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&src, "// Value%04d is a generated value.\nValue%04d = %d\n", i, i, i)
	}
	src.WriteString(")\n")
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "p.go", src.String(), parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return &analysis.Pass{Fset: fs}, f.Decls[0].(*ast.GenDecl)
}

// A block over the options bound is unsupported before any word work: its cost grows with the
// number of comments, not with comments times names.
func TestLargeBlockIsCheap(t *testing.T) {
	pass, gd := bigBlock(t, 400)
	if cs := block(pass, gd); len(cs) != 400 || cs[0].Unsupported == "" {
		t.Fatalf("%d candidates, first %+v", len(cs), cs[0])
	}
	if a := testing.AllocsPerRun(5, func() { block(pass, gd) }); a > 400*20 {
		t.Errorf("%.0f allocations for a 400-constant block", a)
	}
	// A block at the bound compares every pair of comments, and still stays small.
	pass, gd = bigBlock(t, maxConstants)
	if a := testing.AllocsPerRun(5, func() { block(pass, gd) }); a > maxConstants*60 {
		t.Errorf("%.0f allocations for a %d-constant block", a, maxConstants)
	}
}

func BenchmarkBlock(b *testing.B) {
	for _, n := range []int{maxConstants, 100, 400} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			pass, gd := bigBlock(b, n)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				block(pass, gd)
			}
		})
	}
}

func TestQuestionTextIsFixed(t *testing.T) {
	r := &rule{}
	a := r.Questions(&sdk.Candidate{Local: map[string]string{"constants": "A B"}})[0]
	b := r.Questions(&sdk.Candidate{Local: map[string]string{"constants": "X Y Z"}})[0]
	if a.Text != b.Text || len(a.Options) != 3 || len(b.Options) != 4 {
		t.Fatalf("text must be fixed and options per block: %+v %+v", a, b)
	}
	// Only the constants' options vary, and only by the identifier.
	if a.Options[2] != b.Options[3] {
		t.Errorf("none differs between blocks: %+v %+v", a.Options, b.Options)
	}
	if strings.ReplaceAll(a.Options[0].Description, "A", "X") != b.Options[0].Description {
		t.Errorf("a constant's option says more than its name: %q", a.Options[0].Description)
	}
	if !strings.Contains(a.Text, "`comment`") {
		t.Errorf("the question does not name the comment field: %s", a.Text)
	}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
}

func dist(choice string, ps map[string]float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"describes": {QuestionID: "describes", Choice: choice, Probabilities: ps}}
}

func describes(choice string, p float64) map[string]sdk.Answer {
	return dist(choice, map[string]float64{choice: p})
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.8}
	c := &sdk.Candidate{Local: map[string]string{"name": "StateRunning", "comment": "the job finished", "constants": "StateIdle StateRunning StateDone"}}
	const (
		report = iota
		clean
		abstain
	)
	for _, tc := range []struct {
		answers map[string]sdk.Answer
		want    int
	}{
		{describes("StateDone", 0.9), report},
		{describes("StateDone", 0.8), report}, // at the threshold
		{describes("StateDone", 0.79), abstain},
		{describes("StateDone", 0.7), abstain}, // weak shift abstains
		{describes("StateIdle", 0.95), report},
		{describes("StateRunning", 0.9), clean},
		{describes("StateRunning", 0.5), abstain}, // weak agreement abstains
		{describes("none", 0.85), clean},
		{describes("none", 0.6), abstain},
		{describes("unclear", 0.9), abstain},
		// the answer has no probability for its choice
		{map[string]sdk.Answer{"describes": {QuestionID: "describes", Choice: "StateDone"}}, abstain},
		{dist("StateDone", map[string]float64{"StateRunning": 0.9}), abstain},
		// a full distribution that adds up to 1
		{dist("StateDone", map[string]float64{"StateDone": 0.85, "StateRunning": 0.1, "none": 0.05}), report},
		{dist("StateRunning", map[string]float64{"StateRunning": 0.82, "StateDone": 0.18}), clean},
		// agreement and none are not added up: a split between them abstains
		{dist("none", map[string]float64{"none": 0.5, "StateRunning": 0.45, "StateDone": 0.05}), abstain},
		// a pick that contradicts the rest of its distribution abstains either way
		{dist("StateDone", map[string]float64{"StateDone": 0.85, "StateRunning": 0.4}), abstain},
		{dist("StateRunning", map[string]float64{"StateRunning": 0.9, "StateDone": 0.25}), abstain},
		{dist("StateDone", map[string]float64{"StateDone": 0.85, "StateRunning": 0.2}), report}, // at 1-threshold
	} {
		d := r.Decide(c, tc.answers)
		got := clean
		switch {
		case d.Report:
			got = report
		case d.Abstained != "":
			got = abstain
		}
		if got != tc.want {
			t.Errorf("%+v: got %+v", tc.answers["describes"], d)
		}
	}
	// At a lowered threshold the pick must still lead every other option.
	low := &rule{threshold: 0.4}
	if d := low.Decide(c, dist("StateDone", map[string]float64{"StateDone": 0.45, "StateRunning": 0.5, "none": 0.05})); d.Abstained == "" {
		t.Errorf("a pick that does not lead decided: %+v", d)
	}
	if d := low.Decide(c, dist("StateDone", map[string]float64{"StateDone": 0.45, "StateRunning": 0.45, "none": 0.1})); d.Abstained == "" {
		t.Errorf("a tie decided: %+v", d)
	}
	if d := low.Decide(c, dist("StateDone", map[string]float64{"StateDone": 0.5, "StateRunning": 0.4, "none": 0.1})); !d.Report {
		t.Errorf("a leading pick at 0.5 over 0.4 did not report: %+v", d)
	}
	if d := r.Decide(c, describes("StateDone", 0.9)); d.Message != `comment describes StateDone, not StateRunning: "the job finished"` {
		t.Errorf("message: %s", d.Message)
	}
	long := &sdk.Candidate{Local: map[string]string{"name": "StateRunning", "comment": strings.Repeat("x", 100)}}
	if d := r.Decide(long, describes("StateDone", 0.9)); !strings.HasSuffix(d.Message, strings.Repeat("x", 80)+`..."`) {
		t.Errorf("long comment is not clipped: %s", d.Message)
	}
}

func TestThresholdSetting(t *testing.T) {
	for _, l := range sdk.Linters() {
		if l.Name != Name {
			continue
		}
		if l.Version != "3" {
			t.Errorf("version %s", l.Version)
		}
		rl, err := l.New(l.NewSettings())
		if err != nil || rl.(*rule).threshold != 0.8 {
			t.Fatalf("default threshold: %v %v", rl, err)
		}
		v := 0.9
		rl, err = l.New(&Settings{Threshold: &v})
		if err != nil || rl.(*rule).threshold != 0.9 {
			t.Fatalf("threshold 0.9: %v %v", rl, err)
		}
		return
	}
	t.Fatal("not registered")
}
