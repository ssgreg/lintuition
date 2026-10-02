package destructiveadvice

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
		`32 log message "drop the old shard" level=warn next_call=store.Drop`,
		`33 unsupported: the log message is not a constant string`,
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
