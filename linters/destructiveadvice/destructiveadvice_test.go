package destructiveadvice

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ssgreg/lintuition/classifiers/jev"
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
	sort.Slice(cs, func(i, j int) bool {
		a, b := num(caseNo(t, cs[i].Pos)), num(caseNo(t, cs[j].Pos))
		return a < b || a == b && cs[i].Pos.Line < cs[j].Pos.Line
	})
	var got []string
	for _, c := range cs {
		n := caseNo(t, c.Pos)
		if len(c.Payload.Source) > 0 {
			t.Errorf("case %s sends source: %+v", n, c.Payload)
		}
		for k := range c.Payload.Facts {
			if k != "kind" && k != "level" && k != "next_call" {
				t.Errorf("case %s sends an unexpected fact %s", n, k)
			}
		}
		if c.Unsupported != "" {
			got = append(got, n+" unsupported: "+c.Unsupported)
			continue
		}
		line := fmt.Sprintf("%s %s %q", n, c.Payload.Facts["kind"], c.Payload.Prose["text"])
		for _, k := range []string{"level", "next_call"} {
			if v, ok := c.Payload.Facts[k]; ok {
				line += fmt.Sprintf(" %s=%v", k, v)
			}
		}
		got = append(got, line)
	}
	want := []string{
		`1 error message "index is corrupt; delete the data directory and restart"`,
		`2 error message "config invalid: %w; reset it with --reset"`,
		`3 error message "file not found"`,
		`4 unsupported: the error message is not a constant string`,
		`5 log message "cache is stale, wipe it and restart" level=error`,
		`7 error message "run rm -rf /var/lib/app to recover"`,
		`11 error message "droplet count %d"`,
		`13 error message "format the data volume and restart"`,
		`14 error message "overwrite the database with an empty file"`,
		`15 error message "run mkfs.ext4 on the data volume to recover"`,
		`18 log message "staging copy is stale, it has to be dropped" level=warn next_call=store.Drop`,
		`19 log message "journal is damaged; wipe it and restart" level=error`,
		`20 log message "remove the lock file by hand" level=warn`,
		`21 log message "purge the queue and resubmit" level=warn`,
		`22 log message "dropping the orphaned table" level=warn next_call=store.Drop`,
		`23 log message "clearing the temporary directory" level=warn next_call=os.RemoveAll`,
		`24 log message "reinstall the agent with --clean" level=unleveled`,
		`26 log message "erase the old snapshot" level=warn next_call=store.Drop`,
		`27 log message "reset the replica state" level=warn`,
		`28 log message "format the cache disk and start again" level=fatal`,
		`29 error message "drop the index and rebuild it"`,
		`30 log message "wipe the workspace" level=warn`,
		`31 log message "all replicas must be dropped" level=warn next_call=dropper.DropAll`,
		`32 log message "drop the old shard" level=warn`,
		`33 unsupported: the log message is not a constant string`,
		`34 log message "erase the build cache" level=warn`,
		`35 log message "erase the build cache" level=warn`,
		`36 log message "erase the build cache" level=warn`,
		`37 log message "erase the build cache" level=warn`,
		`38 log message "erase the build cache and start over" level=fatal`,
		`39 log message "erase the build cache and start over" level=panic`,
		`40 log message "erase the build cache" level=warn`,
		`41 log message "erase the build cache" level=warn`,
		`42 log message "erase the build cache" level=warn`,
		`43 log message "erase the build cache" level=warn`,
		`44 log message "erase the build cache" level=warn`,
		`45 log message "the cache entries are stale and get dropped" level=warn next_call=store.Load`,
		`46 log message "the cache entries are stale and get dropped" level=warn`,
		`47 log message "erase the build cache" level=warn`,
		`48 log message "the cache entries are stale and get dropped" level=warn next_call=a.removeAll`,
		`49 log message "erase the build cache" level=warn`,
		`51 log message "erase the build cache" level=warn`,
		`51 log message "and again" level=error next_call=store.Drop`,
		`52 log message "erase the build cache" level=warn`,
		`53 log message "erase the build cache" level=warn`,
		`54 log message "erase the build cache" level=warn`,
		`55 log message "erase the build cache" level=warn`,
		`56 log message "erase the build cache" level=warn`,
		`57 log message "erase the build cache" level=warn`,
		`58 log message "erase the build cache" level=warn`,
		`59 log message "erase the build cache" level=warn`,
		`60 log message "the cache entries are stale and get dropped" level=warn next_call=a.dropPath`,
		`61 log message "the cache entries are stale and get dropped" level=warn next_call=a.dropPtr`,
		`62 log message "the cache entries are stale and get dropped" level=warn next_call=a.dropPath`,
		`63 log message "the cache entries are stale and get dropped" level=warn next_call=valueStore.Drop`,
		`64 log message "the cache entries are stale and get dropped" level=warn next_call=store.Drop`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func answers(choice string, p float64, yes *float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{
		"advice":      {QuestionID: "advice", Choice: choice, Probabilities: map[string]float64{choice: p}},
		"states_loss": {QuestionID: "states_loss", Yes: yes},
	}
}

func f(v float64) *float64 { return &v }

// split answers "advice" with the given distribution, picking its most probable option, and says
// nothing is lost.
func split(ps map[string]float64) map[string]sdk.Answer {
	var choice string
	for k, p := range ps {
		if choice == "" || p > ps[choice] {
			choice = k
		}
	}
	return map[string]sdk.Answer{
		"advice":      {QuestionID: "advice", Choice: choice, Probabilities: ps},
		"states_loss": {QuestionID: "states_loss", Yes: f(0.1)},
	}
}

func TestQuestions(t *testing.T) {
	qs := (&rule{threshold: 0.85}).Questions(&sdk.Candidate{})
	if len(qs) != 2 || qs[0].ID != "advice" || qs[0].Kind != sdk.Choice || qs[1].ID != "states_loss" || qs[1].Kind != sdk.Noul {
		t.Fatalf("questions: %+v", qs)
	}
	for _, q := range qs {
		if err := q.Validate(); err != nil {
			t.Error(err)
		}
		if !strings.Contains(q.Text, "`text`") {
			t.Errorf("%s does not name the text field: %s", q.ID, q.Text)
		}
	}
	for _, field := range []string{"`kind`", "`level`", "`next_call`"} {
		if !strings.Contains(qs[0].Text, field) {
			t.Errorf("advice does not name %s", field)
		}
	}
	var keys []string
	for _, o := range qs[0].WithUnclear() {
		keys = append(keys, o.Key)
	}
	if got := strings.Join(keys, " "); got != "destructive_advice safe_advice own_action no_advice unclear" {
		t.Errorf("options: %s", got)
	}
	// The question text is the same for every candidate: what differs goes in the payload.
	other := (&rule{threshold: 0.5}).Questions(&sdk.Candidate{Local: map[string]string{"text": "x", "kind": "log message"}})
	if fmt.Sprint(other) != fmt.Sprint(qs) {
		t.Error("the questions depend on the candidate")
	}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.85}
	strict := &rule{threshold: 0.95}
	c := &sdk.Candidate{Local: map[string]string{"text": "delete the data directory"}}
	for _, tc := range []struct {
		name            string
		r               *rule
		answers         map[string]sdk.Answer
		report, abstain bool
	}{
		{"destructive, loss not stated", r, answers("destructive_advice", 0.9, f(0.1)), true, false},
		{"destructive at the threshold", r, answers("destructive_advice", 0.85, f(0.1)), true, false},
		{"destructive just below the threshold", r, answers("destructive_advice", 0.84, f(0.1)), false, true},
		{"destructive, weak", r, answers("destructive_advice", 0.7, f(0.1)), false, true},
		{"strict threshold: 0.9 abstains", strict, answers("destructive_advice", 0.9, f(0.1)), false, true},
		{"strict threshold: 0.96 reports", strict, answers("destructive_advice", 0.96, f(0.1)), true, false},
		{"loss at the no bound reports", r, answers("destructive_advice", 0.9, f(0.3)), true, false},
		{"loss just above the no bound abstains", r, answers("destructive_advice", 0.9, f(0.31)), false, true},
		{"loss in between abstains", r, answers("destructive_advice", 0.9, f(0.5)), false, true},
		{"loss just below the yes bound abstains", r, answers("destructive_advice", 0.9, f(0.69)), false, true},
		{"loss at the yes bound is clean", r, answers("destructive_advice", 0.9, f(0.7)), false, false},
		{"loss stated is clean", r, answers("destructive_advice", 0.9, f(0.8)), false, false},
		{"loss probability missing", r, answers("destructive_advice", 0.9, nil), false, true},
		{"states_loss missing entirely", r, map[string]sdk.Answer{"advice": answers("destructive_advice", 0.9, nil)["advice"]}, false, true},
		{"own action is clean", r, answers("own_action", 0.9, f(0.1)), false, false},
		{"own action at the threshold is clean", r, answers("own_action", 0.85, f(0.1)), false, false},
		{"own action, weak, abstains", r, answers("own_action", 0.6, f(0.1)), false, true},
		{"own action under the strict threshold abstains", strict, answers("own_action", 0.9, f(0.1)), false, true},
		{"safe advice is clean", r, answers("safe_advice", 0.9, f(0.1)), false, false},
		{"no advice is clean", r, answers("no_advice", 0.95, f(0.1)), false, false},
		{"no advice, weak, abstains", r, answers("no_advice", 0.5, f(0.1)), false, true},
		{"unclear abstains", r, answers("unclear", 0.95, f(0.1)), false, true},
		{"advice probabilities missing", r, map[string]sdk.Answer{"advice": {QuestionID: "advice", Choice: "destructive_advice"}, "states_loss": {QuestionID: "states_loss", Yes: f(0.1)}}, false, true},
		{"advice probability of another option only", r, map[string]sdk.Answer{"advice": {QuestionID: "advice", Choice: "destructive_advice", Probabilities: map[string]float64{"no_advice": 0.9}}, "states_loss": {QuestionID: "states_loss", Yes: f(0.1)}}, false, true},
		{"no advice and own action split, together confident", r, split(map[string]float64{"no_advice": 0.59, "own_action": 0.41}), false, false},
		{"safe advice and no advice split, together confident", r, split(map[string]float64{"safe_advice": 0.5, "no_advice": 0.45, "destructive_advice": 0.05}), false, false},
		{"split with destructive mass, together below the threshold", r, split(map[string]float64{"no_advice": 0.45, "own_action": 0.3, "destructive_advice": 0.25}), false, true},
		{"split with unclear mass, together below the threshold", r, split(map[string]float64{"own_action": 0.5, "no_advice": 0.3, "unclear": 0.2}), false, true},
		{"split confident at 0.90, the strict threshold abstains", strict, split(map[string]float64{"no_advice": 0.6, "own_action": 0.3, "destructive_advice": 0.1}), false, true},
		{"split confident at 0.97, the strict threshold is clean", strict, split(map[string]float64{"no_advice": 0.6, "own_action": 0.37, "destructive_advice": 0.03}), false, false},
		{"destructive picked by a hair is not rescued by the clean sum", r, split(map[string]float64{"destructive_advice": 0.4, "no_advice": 0.35, "own_action": 0.25}), false, true},
		{"advice missing entirely", r, map[string]sdk.Answer{"states_loss": {QuestionID: "states_loss", Yes: f(0.1)}}, false, true},
	} {
		d := tc.r.Decide(c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("%s: got %+v", tc.name, d)
		}
	}
	if d := r.Decide(c, answers("destructive_advice", 0.9, f(0.1))); d.Message != `advises a destructive step without saying what is lost: "delete the data directory"` {
		t.Errorf("message: %s", d.Message)
	}
}

// TestFixtureTextsCarryNoHints checks that no message in the twins or the showcase carries a
// fixture explanation: the message is what is sent, and a hint in it would make a live evaluation
// measure the hint instead of the text.
func TestFixtureTextsCarryNoHints(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/twins/destructiveadvice/*.go")
	files = append(files, "../../examples/showcase/logs.go")
	fset := token.NewFileSet()
	for _, name := range files {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			for _, hint := range []string{"Defect", "Fixed twin", "Negative", "destructive-remediation", "want"} {
				if strings.Contains(lit.Value, hint) {
					t.Errorf("%s: %s carries %q", fset.Position(lit.Pos()), lit.Value, hint)
				}
			}
			return true
		})
	}
}

// TestJevDistributionMass runs Jev answers through the adapter and the shared validation into the
// rule: an overfull distribution never reaches the clean sum, a valid split does.
func TestJevDistributionMass(t *testing.T) {
	for _, tc := range []struct {
		name, advice string
		valid, clean bool
	}{
		{"overfull, clean picked", `"choice":"safe_advice","probabilities":{"safe_advice":0.6,"own_action":0.4,"destructive_advice":0.5}`, false, false},
		{"overfull, destructive mass hidden", `"choice":"safe_advice","probabilities":{"safe_advice":0.5,"own_action":0.4,"destructive_advice":0.9}`, false, false},
		{"valid split", `"choice":"no_advice","probabilities":{"no_advice":0.59,"own_action":0.41,"destructive_advice":0,"safe_advice":0,"unclear":0}`, true, true},
		{"partial, below the threshold", `"choice":"no_advice","probabilities":{"no_advice":0.6,"own_action":0.2}`, true, false},
		{"past rounding: 1.02 over three positive entries", `"choice":"safe_advice","probabilities":{"safe_advice":0.43,"own_action":0.42,"destructive_advice":0.17,"no_advice":0,"unclear":0}`, false, false},
		{"within rounding, raw clean 0.85 normalizes to 0.84 and abstains", `"choice":"safe_advice","probabilities":{"safe_advice":0.43,"own_action":0.42,"destructive_advice":0.16,"no_advice":0,"unclear":0}`, true, false},
		{"within rounding, normalized still clean", `"choice":"no_advice","probabilities":{"no_advice":0.51,"own_action":0.5,"destructive_advice":0,"safe_advice":0,"unclear":0}`, true, true},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"answers":{"advice":{"type":"choice",`+tc.advice+`},"states_loss":{"type":"noul","noul":0.1}},"usage":{"input_tokens":1}}`)
		}))
		backend, err := jev.New(jev.Settings{Endpoint: srv.URL, Model: "stub"}, func(string) string { return "test-key" })
		if err != nil {
			t.Fatal(err)
		}
		r := &rule{threshold: 0.85}
		c := &sdk.Candidate{Local: map[string]string{"text": "x"}}
		qs := r.Questions(c)
		resp, err := backend.Classify(context.Background(), sdk.Request{Questions: qs})
		srv.Close()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		byID, err := classify.Check(qs, resp)
		if (err == nil) != tc.valid {
			t.Errorf("%s: validation %v", tc.name, err)
			continue
		}
		if err != nil {
			continue
		}
		d := r.Decide(c, byID)
		if clean := !d.Report && d.Abstained == ""; clean != tc.clean || d.Report {
			t.Errorf("%s: %+v", tc.name, d)
		}
	}
}

// TestDecideGrid walks every distribution over the five answers on a 0.05 grid, the classifier
// picking its most probable option: a clean result never hides more than 0.15 of destructive mass,
// and destructive advice at 0.85 or more with no stated loss is always reported.
func TestDecideGrid(t *testing.T) {
	r := &rule{threshold: 0.85}
	keys := []string{"destructive_advice", "safe_advice", "own_action", "no_advice", "unclear"}
	n := 0
	var walk func(i, left int, v []int)
	walk = func(i, left int, v []int) {
		if i == len(keys)-1 {
			v = append(v, left)
			ps := map[string]float64{}
			best := 0
			for j, x := range v {
				ps[keys[j]] = float64(x) / 20
				if x > v[best] {
					best = j
				}
			}
			d := r.Decide(&sdk.Candidate{}, map[string]sdk.Answer{
				"advice":      {QuestionID: "advice", Choice: keys[best], Probabilities: ps},
				"states_loss": {QuestionID: "states_loss", Yes: f(0.1)},
			})
			if v[0] >= 17 && !d.Report {
				t.Errorf("%v: destructive advice not reported: %+v", ps, d)
			}
			if !d.Report && d.Abstained == "" && v[0] > 3 {
				t.Errorf("%v: clean with destructive mass %.2f", ps, ps["destructive_advice"])
			}
			n++
			return
		}
		for x := 0; x <= left; x++ {
			walk(i+1, left-x, append(v, x))
		}
	}
	walk(0, 20, nil)
	if n != 10626 {
		t.Errorf("walked %d distributions", n)
	}
}
