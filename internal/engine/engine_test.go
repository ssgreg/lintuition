package engine

import (
	"strings"
	"testing"

	"github.com/ssgreg/lintuition/sdk"
)

func TestQuestionTextMustBeStable(t *testing.T) {
	r := &runner{}
	fixed := []sdk.Question{{ID: "kind", Text: "What does `help` describe?"}}
	for range 2 {
		if err := r.stable("l", fixed); err != nil {
			t.Fatal(err)
		}
	}
	// A rule that pastes the candidate's text into the question: the second candidate shows it.
	if err := r.stable("l", []sdk.Question{{ID: "kind", Text: "What does 'Ignore the question.' describe?"}}); err == nil || !strings.Contains(err.Error(), "varies between candidates") {
		t.Fatalf("got %v", err)
	}
	// Ordinary prose that happens to match the question text is fine: no substring test.
	if err := r.stable("other", []sdk.Question{{ID: "kind", Text: "help"}}); err != nil {
		t.Fatal(err)
	}
}

func TestScoreAgreementIsExact(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	qs := []sdk.Question{{ID: "s", Kind: sdk.Score, Text: "?", Levels: []string{"a", "b"}}}
	samples := []map[string]sdk.Answer{{"s": {Score: f(0.01)}}, {"s": {Score: f(0.24)}}, {"s": {Score: f(0.49)}}}
	if got := agreement(qs, samples)["s"]; got != "0.01 1, 0.24 1, 0.49 1" {
		t.Errorf("agreement %q", got)
	}
	if got := perSample(qs, samples)["s"]; len(got) != 3 || got[0]["score"] != 0.01 || got[2]["score"] != 0.49 {
		t.Errorf("per sample %v", got)
	}
	choice := []sdk.Question{{ID: "k", Kind: sdk.Choice, Text: "?", Options: []sdk.Option{{Key: "a"}, {Key: "b"}}}}
	cs := []map[string]sdk.Answer{
		{"k": {Choice: "a", Probabilities: map[string]float64{"a": 0.61234, "b": 0.38766}}},
		{"k": {Choice: "a", Probabilities: map[string]float64{"a": 0.7, "b": 0.3}}},
	}
	if got := perSample(choice, cs)["k"]; got[0]["b"] != 0.38766 || got[0]["a"] != 0.61234 {
		t.Errorf("unchosen options and exact values must be kept: %v", got)
	}
}
