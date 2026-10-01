package engine

import (
	"context"
	"io"
	"testing"

	"github.com/ssgreg/lintuition/internal/config"
	"github.com/ssgreg/lintuition/sdk"
)

// perQuestion is a backend that makes one call per question and reports usage per call.
type perQuestion struct {
	calls int
	cost  float64
}

func (p *perQuestion) Capabilities() sdk.Capabilities {
	return sdk.Capabilities{Kinds: []sdk.Kind{sdk.Noul}, CallsPerQuestion: true, Local: true}
}

func (p *perQuestion) Classify(_ context.Context, req sdk.Request) (sdk.Response, error) {
	var resp sdk.Response
	for _, q := range req.Questions {
		if err := req.Start(); err != nil {
			return sdk.Response{}, err
		}
		p.calls++
		req.Used(sdk.Usage{CostUSD: p.cost})
		y := 0.9
		resp.Answers = append(resp.Answers, sdk.Answer{QuestionID: q.ID, Yes: &y})
	}
	return resp, nil
}

type twoQuestions struct{}

func (twoQuestions) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{ID: "a", Kind: sdk.Noul, Text: "a?"}, {ID: "b", Kind: sdk.Noul, Text: "b?"}}
}
func (twoQuestions) Decide(*sdk.Candidate, map[string]sdk.Answer) sdk.Decision { return sdk.Clean() }

func newJob() *job {
	return &job{linter: sdk.Linter{Name: "l"}, rule: twoQuestions{}, cand: &sdk.Candidate{}}
}

func newRunner(cl sdk.Classifier, dry bool, b config.Budget, votes int) *runner {
	return &runner{cl: cl, clName: "pq", votes: votes, policy: "prose", budget: b, sem: make(chan struct{}, 1), dryRun: dry, log: io.Discard}
}

func TestPlanCountsCallsPerQuestion(t *testing.T) {
	// Two questions, two votes: four calls. A cap of three cannot hold them, in a plan or a run.
	for _, dry := range []bool{true, false} {
		cl := &perQuestion{}
		r := newRunner(cl, dry, config.Budget{MaxRequests: 3}, 2)
		j := newJob()
		r.do(context.Background(), []*job{j})
		if !j.notAsked || j.planned || cl.calls != 0 {
			t.Errorf("dry=%v: a plan or run over the cap must not start: err %v planned %v calls %d", dry, j.err, j.planned, cl.calls)
		}
	}
	cl := &perQuestion{}
	r := newRunner(cl, true, config.Budget{MaxRequests: 4}, 2)
	j := newJob()
	r.do(context.Background(), []*job{j})
	if j.err != nil || !j.planned || r.requests != 4 {
		t.Fatalf("plan within the cap: err %v planned %v reserved %d", j.err, j.planned, r.requests)
	}
}

func TestCostStopsTheNextQuestion(t *testing.T) {
	cl := &perQuestion{cost: 100}
	r := newRunner(cl, false, config.Budget{MaxCostUSD: 100}, 1)
	j := newJob()
	r.do(context.Background(), []*job{j})
	if (j.err == nil && !j.notAsked) || cl.calls != 1 || r.cost != 100 || r.sent != 1 {
		t.Fatalf("the first call spent the budget; the second must not go: err %v calls %d cost %v sent %d", j.err, cl.calls, r.cost, r.sent)
	}
}
