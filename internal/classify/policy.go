// Package classify turns candidates into classifier requests under the payload policy and checks
// what comes back.
package classify

import (
	"fmt"
	"math"
	"sort"

	"github.com/ssgreg/lintuition/sdk"
)

// Policies, from the strictest. The default is Prose.
const (
	// Facts sends structural facts only.
	Facts = "facts"
	// Prose adds person-written text (comments, metric Help, log messages). That text leaves the machine.
	Prose = "prose"
	// Source adds Go source text.
	Source = "source"
)

// ErrPolicy is returned when a candidate needs a payload kind the policy does not allow. The
// candidate is skipped whole: asking over a stripped payload would be asking over missing evidence.
type ErrPolicy struct{ Kind string }

func (e ErrPolicy) Error() string {
	return fmt.Sprintf("needs %s, which semantic.payload does not allow", e.Kind)
}

// State builds the outbound state of a payload, or refuses it under the policy.
func State(p sdk.Payload, policy string) (map[string]any, error) {
	if len(p.Source) > 0 && policy != Source {
		return nil, ErrPolicy{Kind: "source"}
	}
	if len(p.Prose) > 0 && policy == Facts {
		return nil, ErrPolicy{Kind: "prose"}
	}
	state := map[string]any{}
	add := func(k string, v any) error {
		if _, dup := state[k]; dup {
			return fmt.Errorf("payload key %q is set twice", k)
		}
		state[k] = v
		return nil
	}
	for _, k := range sortedKeys(p.Facts) {
		if err := add(k, p.Facts[k]); err != nil {
			return nil, err
		}
	}
	for _, k := range sortedKeys(p.Prose) {
		if err := add(k, p.Prose[k]); err != nil {
			return nil, err
		}
	}
	for _, k := range sortedKeys(p.Source) {
		if err := add(k, p.Source[k]); err != nil {
			return nil, err
		}
	}
	return state, nil
}

// Check validates a response against the questions it answers and returns the answers by question ID.
// A missing, duplicate, unknown or out-of-domain answer is an error, never a guess.
func Check(qs []sdk.Question, resp sdk.Response) (map[string]sdk.Answer, error) {
	byID := map[string]sdk.Question{}
	for _, q := range qs {
		byID[q.ID] = q
	}
	out := map[string]sdk.Answer{}
	for _, a := range resp.Answers {
		q, ok := byID[a.QuestionID]
		if !ok {
			return nil, fmt.Errorf("answer to unknown question %q", a.QuestionID)
		}
		if _, dup := out[a.QuestionID]; dup {
			return nil, fmt.Errorf("question %q answered twice", a.QuestionID)
		}
		if err := checkAnswer(q, a); err != nil {
			return nil, fmt.Errorf("question %q: %w", q.ID, err)
		}
		out[a.QuestionID] = a
	}
	for _, q := range qs {
		if _, ok := out[q.ID]; !ok {
			return nil, fmt.Errorf("question %q has no answer", q.ID)
		}
	}
	return out, nil
}

func checkAnswer(q sdk.Question, a sdk.Answer) error {
	prob := func(p float64) bool { return !math.IsNaN(p) && p >= 0 && p <= 1 }
	if a.Confidence != nil && !prob(*a.Confidence) {
		return fmt.Errorf("confidence %v is outside [0, 1]", *a.Confidence)
	}
	switch q.Kind {
	case sdk.Choice:
		domain := map[string]bool{}
		for _, o := range q.WithUnclear() {
			domain[o.Key] = true
		}
		if !domain[a.Choice] {
			return fmt.Errorf("choice %q is not an option", a.Choice)
		}
		for k, p := range a.Probabilities {
			if !domain[k] {
				return fmt.Errorf("probability for unknown option %q", k)
			}
			if !prob(p) {
				return fmt.Errorf("probability %v of %q is outside [0, 1]", p, k)
			}
		}
	case sdk.Noul:
		if !prob(a.Yes) {
			return fmt.Errorf("yes probability %v is outside [0, 1]", a.Yes)
		}
	case sdk.Score:
		if math.IsNaN(a.Score) || a.Score < q.Min || a.Score > q.Max {
			return fmt.Errorf("score %v is outside [%v, %v]", a.Score, q.Min, q.Max)
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
