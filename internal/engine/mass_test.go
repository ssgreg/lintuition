package engine

import (
	"fmt"
	"testing"

	"github.com/ssgreg/lintuition/internal/classify"
	_ "github.com/ssgreg/lintuition/linters/destructiveadvice"
	_ "github.com/ssgreg/lintuition/linters/enumcomment"
	"github.com/ssgreg/lintuition/sdk"
)

func builtinRule(t *testing.T, name string) sdk.Rule {
	t.Helper()
	for _, l := range sdk.Linters() {
		if l.Name == name {
			r, err := l.New(l.NewSettings())
			if err != nil {
				t.Fatal(err)
			}
			return r
		}
	}
	t.Fatalf("no linter %s", name)
	return nil
}

// replayDecisions stores samples in a cache, reads them back and decides n times, counting the
// outcomes: the same cached answers must always decide the same way.
func replayDecisions(t *testing.T, rule sdk.Rule, c *sdk.Candidate, samples [][]sdk.Answer, n int) map[string]int {
	t.Helper()
	req := sdk.Request{Questions: rule.Questions(c)}
	cache := &classify.Cache{Dir: t.TempDir()}
	key, err := classify.Key("test", "1", len(samples), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Put(key, classify.Bundle{Samples: samples}); err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for range n {
		b, ok := cache.Lookup(key, len(samples), req.Questions)
		if !ok {
			t.Fatal("cache miss")
		}
		j := &job{rule: rule, cand: c}
		(&runner{}).decide(j, req, b.Samples)
		switch {
		case j.err != nil:
			t.Fatal(j.err)
		case j.decision.Report:
			out["report"]++
		case j.decision.Abstained != "":
			out["abstain"]++
		default:
			out["clean"]++
		}
	}
	return out
}

// TestReplayAtTheThresholdIsStable: twenty constants, the picked one at exactly enum-comment-shift's
// threshold 0.8 and nineteen at 0.01 besides "none". The decimal sum is exactly 1; float addition in
// map order is not, and must not tip the answer into normalization and below the threshold.
func TestReplayAtTheThresholdIsStable(t *testing.T) {
	rule := builtinRule(t, "enum-comment-shift")
	names := ""
	ps := map[string]float64{"C00": 0.8, "none": 0.01, "unclear": 0}
	for i := range 20 {
		k := fmt.Sprintf("C%02d", i)
		names += k + " "
		if i > 0 {
			ps[k] = 0.01
		}
	}
	c := &sdk.Candidate{Local: map[string]string{"name": "C01", "comment": "the job is done", "constants": names}}
	s := []sdk.Answer{{QuestionID: "describes", Choice: "C00", Probabilities: ps}}
	for _, votes := range []int{1, 3} {
		samples := make([][]sdk.Answer, votes)
		for i := range samples {
			samples[i] = s
		}
		if got := replayDecisions(t, rule, c, samples, 500); got["report"] != 500 {
			t.Errorf("%d vote(s): %v", votes, got)
		}
	}
}

// TestReplayOfRoundedExcess: 0.43 + 0.42 + 0.16 is rounding past 1; normalized, the clean options
// hold 0.8416, below 0.85, on every replay and with three votes as with one.
func TestReplayOfRoundedExcess(t *testing.T) {
	rule := builtinRule(t, "destructive-remediation")
	c := &sdk.Candidate{Local: map[string]string{"text": "the store is broken, drop it"}}
	y := 0.1
	s := []sdk.Answer{
		{QuestionID: "advice", Choice: "safe_advice", Probabilities: map[string]float64{"safe_advice": 0.43, "own_action": 0.42, "destructive_advice": 0.16}},
		{QuestionID: "states_loss", Yes: &y},
	}
	for _, votes := range []int{1, 3} {
		samples := make([][]sdk.Answer, votes)
		for i := range samples {
			samples[i] = s
		}
		if got := replayDecisions(t, rule, c, samples, 200); got["abstain"] != 200 {
			t.Errorf("%d vote(s): %v", votes, got)
		}
	}
	if s[0].Probabilities["safe_advice"] != 0.43 {
		t.Error("the samples were changed")
	}
}
