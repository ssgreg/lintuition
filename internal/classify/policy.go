// Package classify turns candidates into classifier requests under the payload policy and checks
// what comes back.
package classify

import (
	"errors"
	"fmt"
	"math"
	"regexp"
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
		if err := checkFact(k, p.Facts[k]); err != nil {
			return nil, err
		}
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
			return nil, errors.New("an answer to a question that was not asked")
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

// roundingPerOption is how far above its true value one reported option probability may be: a
// backend that rounds to two decimals is off by up to 0.005.
const roundingPerOption = 0.005

func checkAnswer(q sdk.Question, a sdk.Answer) error {
	prob := func(p float64) bool { return !math.IsNaN(p) && p >= 0 && p <= 1 }
	if a.Confidence != nil && !prob(*a.Confidence) {
		return fmt.Errorf("confidence %v is outside [0, 1]", *a.Confidence)
	}
	switch q.Kind {
	case sdk.Choice:
		if a.Yes != nil || a.Score != nil {
			return fmt.Errorf("a choice answer carries a yes or score value")
		}
		domain := map[string]bool{}
		for _, o := range q.WithUnclear() {
			domain[o.Key] = true
		}
		if !domain[a.Choice] {
			return errors.New("the choice is not one of the options")
		}
		sum := 0.0
		for k, p := range a.Probabilities {
			if !domain[k] {
				return errors.New("a probability for an option that was not offered")
			}
			if !prob(p) {
				return errors.New("an option probability is outside [0, 1]")
			}
			sum += p
		}
		// The options are mutually exclusive, so their probabilities add up to at most 1, give or
		// take rounding of each one; more is a contradictory distribution that a rule adding
		// options together would read as confidence. Less is allowed: a backend may report part of
		// the mass.
		if limit := 1 + roundingPerOption*float64(len(domain)) + 1e-9; sum > limit {
			return fmt.Errorf("the option probabilities add up to %.3f, more than 1", sum)
		}
	case sdk.Noul:
		if a.Yes == nil || a.Choice != "" || a.Score != nil || len(a.Probabilities) > 0 {
			return fmt.Errorf("a yes/no answer needs exactly a yes probability")
		}
		if !prob(*a.Yes) {
			return fmt.Errorf("yes probability %v is outside [0, 1]", *a.Yes)
		}
	case sdk.Score:
		if a.Score == nil || a.Choice != "" || a.Yes != nil || len(a.Probabilities) > 0 {
			return fmt.Errorf("a score answer needs exactly a score")
		}
		if top := float64(len(q.Levels) - 1); math.IsNaN(*a.Score) || *a.Score < 0 || *a.Score > top {
			return fmt.Errorf("score %v is outside [0, %v]", *a.Score, top)
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

// factRE is what a structural fact string may look like: identifiers and short descriptions built
// from them ("the result of IsExpired", "counter"). Quotes, braces, operators and newlines mean
// source text was put in the wrong field.
var factRE = regexp.MustCompile(`^[A-Za-z0-9_ .,/()-]{0,120}$`)

// checkFact refuses a fact that is not a scalar or a plain short string. Trusted extractor code can
// still mislabel source as a fact; this catches the common shapes, it is not a sandbox.
func checkFact(k string, v any) error {
	switch v := v.(type) {
	case bool, int, int64, float64:
		return nil
	case string:
		if !factRE.MatchString(v) {
			return fmt.Errorf("fact %q does not look structural (%q); send it as prose or source", k, v)
		}
		return nil
	case []string:
		for _, s := range v {
			if err := checkFact(k, s); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("fact %q has unsupported type %T", k, v)
}
