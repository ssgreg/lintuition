package severeunderstated

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

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
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
	var cs []*sdk.Candidate
	for _, r := range res {
		cs = append(cs, r.Result.([]*sdk.Candidate)...)
	}
	sort.Slice(cs, func(i, j int) bool { return num(caseNo(t, cs[i].Pos)) < num(caseNo(t, cs[j].Pos)) })
	var got []string
	for _, c := range cs {
		n := caseNo(t, c.Pos)
		if len(c.Payload.Source) > 0 {
			t.Errorf("case %s sends source: %+v", n, c.Payload)
		}
		for k := range c.Payload.Prose {
			if k != "message" {
				t.Errorf("case %s sends unexpected prose %s", n, k)
			}
		}
		for k := range c.Payload.Facts {
			if k != "level" && k != "branch" {
				t.Errorf("case %s sends an unexpected fact %s", n, k)
			}
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported: "+c.Unsupported)
			continue
		}
		if c.Payload.Facts["level"] != c.Local["level"] {
			t.Errorf("case %s: level fact %v, local %s", n, c.Payload.Facts["level"], c.Local["level"])
		}
		line := fmt.Sprintf("%s %s %q", n, c.Local["level"], c.Payload.Prose["message"])
		if b, ok := c.Payload.Facts["branch"]; ok {
			list, isList := b.([]string)
			if !isList || len(list) == 0 {
				t.Errorf("case %s: branch is %T %v, want a non-empty []string", n, b, b)
			}
			line += " branch=" + strings.Join(list, " | ")
		}
		// Every candidate the analyzer asks about must pass the payload policy as it is sent.
		if _, err := classify.State(c.Payload, classify.Prose); err != nil {
			t.Errorf("case %s: the payload is refused: %v", n, err)
		}
		got = append(got, line)
	}
	want := []string{
		`1 info "queue full, dropping events"`,
		`4 unsupported: the log message is not a constant string`,
		`5 debug "upload lost after restart"`,
		`9 info "session expired, work discarded"`,
		`12 info "no saved state, starting empty" branch=error is fs.ErrNotExist`,
		`13 info "state file unreadable, skipped"`,
		`14 info "state file unreadable, skipped"`,
		`15 debug "snapshot file missing, nothing to load" branch=os.IsNotExist`,
		`16 debug "snapshot file missing, nothing to load"`,
		`17 info "stream ended, partial record discarded" branch=error is io.EOF`,
		`18 info "stream broke, partial record discarded"`,
		`19 info "stream ended, partial record discarded" branch=error is io.EOF`,
		`20 debug "send aborted, the batch is dropped" branch=error is context.Canceled`,
		`21 info "matched a local error value"`,
		`22 info "request timed out, reply lost" branch=error is context.DeadlineExceeded`,
		`23 info "upload cut short, chunk lost" branch=error is io.ErrUnexpectedEOF`,
		`24 info "upload cut short, chunk lost"`,
		`25 info "worker exits, jobs keep running" branch=context done`,
		`26 info "worker exits, jobs keep running"`,
		`27 info "interrupted, stopping without waiting" branch=receive from a channel of os.Signal`,
		`28 info "second interrupt, stopping without waiting" branch=callback passed along with a signal value`,
		`29 info "second interrupt, stopping without waiting"`,
		`30 info "interrupted, stopping without waiting"`,
		`31 info "shutdown cut the sync short, changes lost" branch=error is context.Canceled | context done`,
		`33 info "no saved state, starting empty"`,
		`35 info "no saved state, starting empty" branch=os.IsNotExist`,
		`37 info "no saved state, starting empty" branch=os.IsNotExist`,
		`40 info "record vanished before it was read" branch=error is a.ErrLost`,
		`41 info "worker exits, jobs keep running"`,
		`42 info "node not ready, request queued" branch=error is a.errNotReady`,
		`43 info "no saved state, starting empty" branch=os.IsNotExist`,
		`44 info "buffered rows were thrown away"`,
		`45 info "buffered rows were thrown away"`,
		`46 info "buffered rows were thrown away"`,
		`47 info "buffered rows were thrown away"`,
		`48 info "buffered rows were thrown away"`,
		`49 info "buffered rows were thrown away"`,
		`50 info "buffered rows were thrown away"`,
		`51 info "buffered rows were thrown away"`,
		`52 info "buffered rows were thrown away"`,
		`53 info "buffered rows were thrown away"`,
		`54 info "buffered rows were thrown away" branch=error is context.Canceled`,
		`55 info "buffered rows were thrown away"`,
		`56 info "buffered rows were thrown away" branch=context done`,
		`57 info "buffered rows were thrown away" branch=context done`,
		`58 info "buffered rows were thrown away" branch=callback passed along with a signal value`,
		`59 info "buffered rows were thrown away"`,
		`60 info "buffered rows were thrown away"`,
		`61 info "buffered rows were thrown away"`,
		`62 info "buffered rows were thrown away"`,
		`63 info "buffered rows were thrown away" branch=error is context.Canceled`,
		`64 info "buffered rows were thrown away"`,
		`65 info "buffered rows were thrown away"`,
		`66 info "buffered rows were thrown away" branch=error is context.Canceled`,
		`67 info "buffered rows were thrown away"`,
		`68 info "buffered rows were thrown away"`,
		`69 info "buffered rows were thrown away" branch=error is context.Canceled`,
		`70 info "buffered rows were thrown away"`,
		`71 info "buffered rows were thrown away" branch=error is context.Canceled`,
		`72 info "buffered rows were thrown away"`,
		`73 info "buffered rows were thrown away"`,
		`74 info "buffered rows were thrown away"`,
		`75 info "buffered rows were thrown away" branch=context done`,
		`76 info "buffered rows were thrown away"`,
		`77 info "buffered rows were thrown away"`,
		`78 info "buffered rows were thrown away"`,
		`79 info "buffered rows were thrown away"`,
		`80 info "buffered rows were thrown away"`,
		`81 info "buffered rows were thrown away" branch=error is context.Canceled`,
		`82 info "buffered rows were thrown away" branch=error is context.Canceled`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func answers(choice string, p float64, yes *float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{
		"consequence": {QuestionID: "consequence", Choice: choice, Probabilities: map[string]float64{choice: p}},
		"on_purpose":  {QuestionID: "on_purpose", Yes: yes},
	}
}

func f(v float64) *float64 { return &v }

// split answers "consequence" with the given distribution, picking its most probable option, and
// says the loss was not meant.
func split(ps map[string]float64) map[string]sdk.Answer {
	var choice string
	for k, p := range ps {
		if choice == "" || p > ps[choice] || (p == ps[choice] && k < choice) {
			choice = k
		}
	}
	return map[string]sdk.Answer{
		"consequence": {QuestionID: "consequence", Choice: choice, Probabilities: ps},
		"on_purpose":  {QuestionID: "on_purpose", Yes: f(0.1)},
	}
}

func TestQuestions(t *testing.T) {
	qs := (&rule{threshold: 0.85}).Questions(&sdk.Candidate{})
	if len(qs) != 2 || qs[0].ID != "consequence" || qs[0].Kind != sdk.Choice || qs[1].ID != "on_purpose" || qs[1].Kind != sdk.Noul {
		t.Fatalf("questions: %+v", qs)
	}
	for _, q := range qs {
		if err := q.Validate(); err != nil {
			t.Error(err)
		}
		if !strings.Contains(q.Text, "`message`") {
			t.Errorf("%s does not name the message field: %s", q.ID, q.Text)
		}
	}
	for _, fact := range []string{"context done", "receive from a channel of os.Signal", "callback passed along with a signal value"} {
		if !strings.Contains(qs[0].Text, fact) {
			t.Errorf("consequence does not show the branch fact %q", fact)
		}
	}
	if strings.Contains(qs[0].Text, "arrived") || strings.Contains(qs[0].Text, "established") {
		t.Errorf("consequence claims more than the facts show: %s", qs[0].Text)
	}
	for _, field := range []string{"`level`", "`branch`"} {
		if !strings.Contains(qs[0].Text, field) {
			t.Errorf("consequence does not name %s", field)
		}
	}
	var keys []string
	for _, o := range qs[0].WithUnclear() {
		keys = append(keys, o.Key)
	}
	if got := strings.Join(keys, " "); got != "routine expected_absence requested_stop recovery inconvenience unintended_loss unclear" {
		t.Errorf("options: %s", got)
	}
	// Every option but the loss is a clean answer, and the sum in Decide reads exactly those.
	if got := strings.Join(cleanOptions, " "); got != "routine expected_absence requested_stop recovery inconvenience" {
		t.Errorf("clean options: %s", got)
	}
	// The question text is the same for every candidate: what differs goes in the payload.
	other := (&rule{threshold: 0.5}).Questions(&sdk.Candidate{Local: map[string]string{"message": "x", "level": "debug"}})
	if fmt.Sprint(other) != fmt.Sprint(qs) {
		t.Error("the questions depend on the candidate")
	}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.85}
	strict := &rule{threshold: 0.95}
	c := &sdk.Candidate{Local: map[string]string{"level": "info", "message": "queue full, dropping events"}}
	for _, tc := range []struct {
		name            string
		r               *rule
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{"loss, not meant", r, answers("unintended_loss", 0.9, f(0.1)), true, false},
		{"loss at the threshold", r, answers("unintended_loss", 0.85, f(0.1)), true, false},
		{"loss just below the threshold", r, answers("unintended_loss", 0.84, f(0.1)), false, true},
		{"loss, weak", r, answers("unintended_loss", 0.7, f(0.1)), false, true},
		{"strict threshold: 0.9 abstains", strict, answers("unintended_loss", 0.9, f(0.1)), false, true},
		{"strict threshold: 0.96 reports", strict, answers("unintended_loss", 0.96, f(0.1)), true, false},
		{"meant at the no bound reports", r, answers("unintended_loss", 0.9, f(0.3)), true, false},
		{"meant just above the no bound abstains", r, answers("unintended_loss", 0.9, f(0.31)), false, true},
		{"meant in between abstains", r, answers("unintended_loss", 0.9, f(0.5)), false, true},
		{"meant just below the yes bound abstains", r, answers("unintended_loss", 0.9, f(0.69)), false, true},
		{"meant at the yes bound is clean", r, answers("unintended_loss", 0.9, f(0.7)), false, false},
		{"meant is clean", r, answers("unintended_loss", 0.9, f(0.8)), false, false},
		{"on purpose probability missing", r, answers("unintended_loss", 0.9, nil), false, true},
		{"on purpose missing entirely", r, map[string]sdk.Answer{"consequence": answers("unintended_loss", 0.9, nil)["consequence"]}, false, true},
		{"routine is clean", r, answers("routine", 0.9, f(0.1)), false, false},
		{"routine, weak, abstains", r, answers("routine", 0.6, f(0.1)), false, true},
		{"expected absence is clean", r, answers("expected_absence", 0.9, f(0.1)), false, false},
		{"requested stop is clean", r, answers("requested_stop", 0.9, f(0.1)), false, false},
		{"requested stop at the threshold is clean", r, answers("requested_stop", 0.85, f(0.1)), false, false},
		{"recovery is clean", r, answers("recovery", 0.9, f(0.1)), false, false},
		{"recovery under the strict threshold abstains", strict, answers("recovery", 0.9, f(0.1)), false, true},
		{"inconvenience is clean", r, answers("inconvenience", 0.95, f(0.1)), false, false},
		{"a clean answer does not read on purpose", r, answers("routine", 0.9, nil), false, false},
		{"unclear abstains", r, answers("unclear", 0.95, f(0.1)), false, true},
		{"consequence probabilities missing", r, map[string]sdk.Answer{"consequence": {QuestionID: "consequence", Choice: "unintended_loss"}, "on_purpose": {QuestionID: "on_purpose", Yes: f(0.1)}}, false, true},
		{"clean choice with no probabilities abstains", r, map[string]sdk.Answer{"consequence": {QuestionID: "consequence", Choice: "routine"}, "on_purpose": {QuestionID: "on_purpose", Yes: f(0.1)}}, false, true},
		{"loss probability of another option only", r, map[string]sdk.Answer{"consequence": {QuestionID: "consequence", Choice: "unintended_loss", Probabilities: map[string]float64{"routine": 0.9}}, "on_purpose": {QuestionID: "on_purpose", Yes: f(0.1)}}, false, true},
		{"absence and stop split, together confident", r, split(map[string]float64{"expected_absence": 0.5, "requested_stop": 0.4, "unintended_loss": 0.1}), false, false},
		{"routine and recovery split, together confident", r, split(map[string]float64{"routine": 0.45, "recovery": 0.45, "unintended_loss": 0.1}), false, false},
		{"split with loss mass, together below the threshold", r, split(map[string]float64{"requested_stop": 0.5, "routine": 0.2, "unintended_loss": 0.3}), false, true},
		{"split with unclear mass, together below the threshold", r, split(map[string]float64{"routine": 0.5, "recovery": 0.3, "unclear": 0.2}), false, true},
		{"split confident at 0.90, the strict threshold abstains", strict, split(map[string]float64{"routine": 0.6, "recovery": 0.3, "unintended_loss": 0.1}), false, true},
		{"split confident at 0.97, the strict threshold is clean", strict, split(map[string]float64{"routine": 0.6, "recovery": 0.37, "unintended_loss": 0.03}), false, false},
		{"loss picked by a hair is not rescued by the clean sum", r, split(map[string]float64{"unintended_loss": 0.4, "routine": 0.35, "recovery": 0.25}), false, true},
		{"mass of exactly 1 is taken as it is", r, split(map[string]float64{"routine": 0.43, "recovery": 0.42, "unintended_loss": 0.15}), false, false},
		{"five clean options adding to the threshold in decimals", r, split(map[string]float64{"routine": 0.32, "expected_absence": 0.13, "requested_stop": 0.34, "recovery": 0.04, "inconvenience": 0.02, "unintended_loss": 0, "unclear": 0.15}), false, false},
		{"five clean options a cent below the threshold", r, split(map[string]float64{"routine": 0.32, "expected_absence": 0.13, "requested_stop": 0.33, "recovery": 0.04, "inconvenience": 0.02, "unintended_loss": 0, "unclear": 0.16}), false, true},
		{"consequence missing entirely", r, map[string]sdk.Answer{"on_purpose": {QuestionID: "on_purpose", Yes: f(0.1)}}, false, true},
	} {
		d := tc.r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%s: got %+v", tc.name, d)
		}
	}
	if d := r.Decide(c, answers("unintended_loss", 0.9, f(0.1))); d.Message != `unintended loss logged at info level: "queue full, dropping events"` {
		t.Errorf("message: %s", d.Message)
	}
}

// TestFixtureTextsCarryNoHints checks that no message in the twins or the showcase carries a
// fixture explanation: the message is what is sent, and a hint in it would make a live evaluation
// measure the hint instead of the text.
func TestFixtureTextsCarryNoHints(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/twins/severeunderstated/*.go")
	files = append(files, "../../examples/showcase/logs.go")
	fset := token.NewFileSet()
	for _, name := range files {
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			for _, hint := range []string{"Defect", "Fixed twin", "Negative", "severe-event-understated", "want"} {
				if strings.Contains(lit.Value, hint) {
					t.Errorf("%s: %s carries %q", fset.Position(lit.Pos()), lit.Value, hint)
				}
			}
			return true
		})
		// An explanation is a detached comment: a doc comment would reach the linters that read
		// doc comments, and through them a classifier.
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Doc != nil && strings.Contains(name, "twins") {
				t.Errorf("%s: %s has a doc comment; keep the explanation detached", fset.Position(fd.Pos()), fd.Name.Name)
			}
		}
	}
}

// TestUnformattedFallthrough reads source gofmt has not touched, where an empty statement follows
// fallthrough: the case it falls into still names nothing. The fixture lives in a string so that
// gofmt over the repository leaves it as it is.
func TestUnformattedFallthrough(t *testing.T) {
	const src = `package p

import (
	"context"
	"errors"
	"io"
	"log/slog"
)

func tagged(err error) {
	switch err {
	case io.EOF:
		fallthrough; ;
	case context.Canceled:
		slog.Info("open batch thrown away", "err", err)
	}
}

func tagless(err error) {
	switch {
	case err == io.EOF:
		fallthrough; ;
	case errors.Is(err, context.Canceled):
		slog.Info("open batch dropped", "err", err)
	}
}

func control(err error) {
	switch err {
	case io.EOF:
		;
	case context.Canceled:
		slog.Info("open batch discarded", "err", err)
	}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("p", fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatal(err)
	}
	pass := &analysis.Pass{Fset: fset, Files: []*ast.File{f}, Pkg: pkg, TypesInfo: info,
		ResultOf: map[*analysis.Analyzer]any{inspect.Analyzer: inspector.New([]*ast.File{f})}}
	out, err := run(pass)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range out.([]*sdk.Candidate) {
		b, _ := c.Payload.Facts["branch"].([]string)
		got[c.Local["message"]] = strings.Join(b, ", ")
	}
	want := map[string]string{
		"open batch thrown away": "",
		"open batch dropped":     "",
		"open batch discarded":   "error is context.Canceled",
	}
	if len(got) != len(want) {
		t.Errorf("candidates: %v", got)
	}
	for k, v := range want {
		if g, ok := got[k]; !ok || g != v {
			t.Errorf("%s: got %q, want %q", k, g, v)
		}
	}
}

// TestDecideCheckedMass runs answers through the shared answer check, as the engine does, and
// decides each many times over maps built in different orders: one answer must always give one
// decision, also when its clean options add up to the threshold exactly in decimals.
func TestDecideCheckedMass(t *testing.T) {
	r := &rule{threshold: 0.85}
	c := &sdk.Candidate{Local: map[string]string{"level": "info", "message": "m"}}
	keys := []string{"routine", "expected_absence", "requested_stop", "recovery", "inconvenience", "unintended_loss", "unclear"}
	for _, tc := range []struct {
		name    string
		probs   []float64
		abstain bool
	}{
		{"clean options at the threshold", []float64{0.32, 0.13, 0.34, 0.04, 0.02, 0, 0.15}, false},
		{"clean options a cent below", []float64{0.32, 0.13, 0.33, 0.04, 0.02, 0, 0.16}, true},
		{"clean options well above", []float64{0.5, 0.1, 0.2, 0.05, 0.05, 0.05, 0.05}, false},
	} {
		rng := rand.New(rand.NewSource(1))
		for trial := 0; trial < 50; trial++ {
			ps := map[string]float64{}
			for _, i := range rng.Perm(len(keys)) {
				ps[keys[i]] = tc.probs[i]
			}
			ans := split(ps)
			checked, err := classify.Check(r.Questions(c), sdk.Response{Answers: []sdk.Answer{ans["consequence"], ans["on_purpose"]}})
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			for i := 0; i < 2000; i++ {
				if d := r.Decide(c, checked); (d.Abstained != "") != tc.abstain || d.Report {
					t.Fatalf("%s, trial %d, decision %d: %+v", tc.name, trial, i, d)
				}
			}
		}
	}
	// Mass that rounding cannot explain never reaches Decide: the check refuses it.
	ans := split(map[string]float64{"routine": 0.43, "recovery": 0.42, "unintended_loss": 0.18})
	if _, err := classify.Check(r.Questions(c), sdk.Response{Answers: []sdk.Answer{ans["consequence"], ans["on_purpose"]}}); err == nil {
		t.Error("an overfull distribution passed the check")
	}
}
