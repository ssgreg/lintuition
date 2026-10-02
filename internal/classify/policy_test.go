package classify

import (
	"errors"
	"math"
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
		{ID: "s", Kind: sdk.Score, Text: "?", Levels: []string{"a", "b", "c", "d", "e"}},
	}
	f := func(v float64) *float64 { return &v }
	ok := []sdk.Answer{{QuestionID: "k", Choice: "unclear"}, {QuestionID: "y", Yes: f(0.3)}, {QuestionID: "s", Score: f(2)}}
	if _, err := Check(qs, sdk.Response{Answers: ok}); err != nil {
		t.Fatal(err)
	}
	bad := map[string][]sdk.Answer{
		"has no answer":          ok[:2],
		"not one of the options": {{QuestionID: "k", Choice: "c"}, ok[1], ok[2]},
		"was not offered":        {{QuestionID: "k", Choice: "a", Probabilities: map[string]float64{"z": 0.1}}, ok[1], ok[2]},
		"outside [0, 1]":         {ok[0], {QuestionID: "y", Yes: f(1.5)}, ok[2]},
		"outside [0, 4]":         {ok[0], ok[1], {QuestionID: "s", Score: f(9)}},
		"exactly a yes":          {ok[0], {QuestionID: "y"}, ok[2]},
		"exactly a score":        {ok[0], ok[1], {QuestionID: "s"}},
		"carries a yes":          {{QuestionID: "k", Choice: "a", Yes: f(1)}, ok[1], ok[2]},
		"answered twice":         append(append([]sdk.Answer{}, ok...), ok[0]),
		"was not asked":          append(append([]sdk.Answer{}, ok...), sdk.Answer{QuestionID: "x"}),
	}
	for want, answers := range bad {
		if _, err := Check(qs, sdk.Response{Answers: answers}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q, got %v", want, err)
		}
	}
}

func TestCheckChoiceMass(t *testing.T) {
	// Two-decimal rounding: each positive entry may be up to 0.005 above its true value, so the
	// sum may exceed 1 by 0.005 per positive entry and no more.
	qs := []sdk.Question{{ID: "k", Kind: sdk.Choice, Text: "?", Options: []sdk.Option{{Key: "a"}, {Key: "b"}, {Key: "c"}, {Key: "d"}}}}
	for _, tc := range []struct {
		ps   map[string]float64
		ok   bool
		want map[string]float64 // what Check returns, when it differs from ps
	}{
		{map[string]float64{"a": 0.6, "b": 0.4}, true, nil},
		{map[string]float64{"a": 0.6, "b": 0.3, "unclear": 0.1}, true, nil},
		{map[string]float64{"a": 0.6}, true, nil},
		{map[string]float64{"a": 0.6, "b": 0.2}, true, nil},
		{nil, true, nil},
		// At the bound: three positive entries, 1.015, the true values may add up to exactly 1.
		{map[string]float64{"a": 0.34, "b": 0.34, "c": 0.335, "d": 0}, true, map[string]float64{"a": 0.34 / 1.015, "b": 0.34 / 1.015, "c": 0.335 / 1.015, "d": 0}},
		{map[string]float64{"a": 0.43, "b": 0.42, "c": 0.16}, true, map[string]float64{"a": 0.43 / 1.01, "b": 0.42 / 1.01, "c": 0.16 / 1.01}},
		// Just past it: 1.02 over three positive entries; the zeros do not widen the allowance.
		{map[string]float64{"a": 0.43, "b": 0.42, "c": 0.17, "d": 0, "unclear": 0}, false, nil},
		{map[string]float64{"a": 0.34, "b": 0.34, "c": 0.336}, false, nil},
		{map[string]float64{"a": 0.6, "b": 0.42}, false, nil},
		{map[string]float64{"a": 0.6, "b": 0.4, "unclear": 0.5}, false, nil},
		{map[string]float64{"a": 1, "b": 1}, false, nil},
	} {
		in := map[string]float64{}
		for k, v := range tc.ps {
			in[k] = v
		}
		got, err := Check(qs, sdk.Response{Answers: []sdk.Answer{{QuestionID: "k", Choice: "a", Probabilities: tc.ps}}})
		if (err == nil) != tc.ok {
			t.Errorf("%v: got %v", tc.ps, err)
			continue
		}
		if err != nil {
			if !strings.Contains(err.Error(), "more than rounding explains") {
				t.Errorf("%v: %v", tc.ps, err)
			}
			continue
		}
		want := tc.want
		if want == nil {
			want = in
		}
		gp := got["k"].Probabilities
		if len(gp) != len(want) {
			t.Errorf("%v: returned %v", tc.ps, gp)
		}
		for k, v := range want {
			if math.Abs(gp[k]-v) > 1e-12 {
				t.Errorf("%v: %s returned %v, want %v", tc.ps, k, gp[k], v)
			}
		}
		for k, v := range in {
			if tc.ps[k] != v {
				t.Errorf("%v: Check changed the caller's map", tc.ps)
			}
		}
	}
}

func TestFactsMustBeStructural(t *testing.T) {
	for _, v := range []any{`t.Errorf("DO_NOT_EXPORT")`, "a\nb", "x := 1", map[string]int{}} {
		var p sdk.Payload
		p.Fact("assertions", v)
		if _, err := State(p, Source); err == nil {
			t.Errorf("fact %q must be refused", v)
		}
	}
	var p sdk.Payload
	p.Fact("about", "the result of IsExpired")
	p.Fact("n", 3)
	p.Fact("ok", true)
	if _, err := State(p, Facts); err != nil {
		t.Fatal(err)
	}
}
