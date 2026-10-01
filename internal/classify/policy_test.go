package classify

import (
	"errors"
	"strings"
	"testing"

	"github.com/ssgreg/lintuition/sdk"
)

func TestStatePolicy(t *testing.T) {
	var p sdk.Payload
	p.Fact("kind", "counter")
	p.AddProse("help", "Total seconds.")
	if _, err := State(p, Facts); !errors.As(err, new(ErrPolicy)) {
		t.Fatalf("facts policy must refuse prose, got %v", err)
	}
	s, err := State(p, Prose)
	if err != nil || s["help"] != "Total seconds." || s["kind"] != "counter" {
		t.Fatalf("prose policy: %v %v", s, err)
	}
	p.AddSource("code", "x := 1")
	if _, err := State(p, Prose); !errors.As(err, new(ErrPolicy)) {
		t.Fatalf("prose policy must refuse source, got %v", err)
	}
	if _, err := State(p, Source); err != nil {
		t.Fatal(err)
	}
	p.AddProse("kind", "dup")
	if _, err := State(p, Source); err == nil || !strings.Contains(err.Error(), "set twice") {
		t.Fatalf("duplicate key across kinds must fail, got %v", err)
	}
}

func TestCheck(t *testing.T) {
	qs := []sdk.Question{
		{ID: "k", Kind: sdk.Choice, Text: "?", Options: []sdk.Option{{Key: "a"}, {Key: "b"}}},
		{ID: "y", Kind: sdk.Noul, Text: "?"},
		{ID: "s", Kind: sdk.Score, Text: "?", Min: 1, Max: 5},
	}
	ok := []sdk.Answer{{QuestionID: "k", Choice: "unclear"}, {QuestionID: "y", Yes: 0.3}, {QuestionID: "s", Score: 2}}
	if _, err := Check(qs, sdk.Response{Answers: ok}); err != nil {
		t.Fatal(err)
	}
	bad := map[string][]sdk.Answer{
		"has no answer":    ok[:2],
		"not an option":    {{QuestionID: "k", Choice: "c"}, ok[1], ok[2]},
		"unknown option":   {{QuestionID: "k", Choice: "a", Probabilities: map[string]float64{"z": 0.1}}, ok[1], ok[2]},
		"outside [0, 1]":   {ok[0], {QuestionID: "y", Yes: 1.5}, ok[2]},
		"outside [1, 5]":   {ok[0], ok[1], {QuestionID: "s", Score: 9}},
		"answered twice":   append(append([]sdk.Answer{}, ok...), ok[0]),
		"unknown question": append(append([]sdk.Answer{}, ok...), sdk.Answer{QuestionID: "x"}),
	}
	for want, answers := range bad {
		if _, err := Check(qs, sdk.Response{Answers: answers}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q, got %v", want, err)
		}
	}
}
