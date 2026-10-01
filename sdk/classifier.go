// Package sdk is the small public contract between lintuition, its linters and its classifiers.
//
// The contract is pre-v1: it may change in any minor release until lintuition v1.0.0.
package sdk

import (
	"context"
	"fmt"
)

// Kind is the type of a classifier question.
type Kind string

const (
	// Choice picks one option from a closed set and returns a probability per option.
	Choice Kind = "choice"
	// Noul answers a yes/no question with the probability of yes.
	Noul Kind = "noul"
	// Score places the state on ordered levels and returns a position from 0 to len(Levels)-1.
	Score Kind = "score"
)

// Unclear is the option key every Choice question gets, so the classifier can abstain.
const Unclear = "unclear"

// Option is one answer of a Choice question.
type Option struct {
	Key         string
	Description string
}

// Question is one atomic question about the state of a request.
type Question struct {
	// ID is unique within a request.
	ID   string
	Kind Kind
	// Text is the question. It refers to state fields by name, for example `help`.
	Text string
	// Options are the answers of a Choice question; an Unclear option is added if absent.
	Options []Option
	// Levels are the ordered level descriptions of a Score question, lowest first (2 to 10).
	Levels []string
}

// Validate reports a malformed question.
func (q Question) Validate() error {
	if q.ID == "" || q.Text == "" {
		return fmt.Errorf("question %q: id and text are required", q.ID)
	}
	switch q.Kind {
	case Choice:
		if len(q.Options) < 2 {
			return fmt.Errorf("question %q: a choice needs at least two options", q.ID)
		}
		seen := map[string]bool{}
		for _, o := range q.Options {
			if o.Key == "" || seen[o.Key] {
				return fmt.Errorf("question %q: empty or duplicate option %q", q.ID, o.Key)
			}
			seen[o.Key] = true
		}
	case Noul:
	case Score:
		if len(q.Levels) < 2 || len(q.Levels) > 10 {
			return fmt.Errorf("question %q: a score needs 2 to 10 levels", q.ID)
		}
	default:
		return fmt.Errorf("question %q: unknown kind %q", q.ID, q.Kind)
	}
	return nil
}

// WithUnclear returns the options with an Unclear option appended if it is missing.
func (q Question) WithUnclear() []Option {
	for _, o := range q.Options {
		if o.Key == Unclear {
			return q.Options
		}
	}
	return append(append([]Option(nil), q.Options...), Option{Key: Unclear, Description: "The text does not let you tell."})
}

// Answer is a classifier's typed answer to one question.
type Answer struct {
	QuestionID string
	// Choice is the picked option of a Choice question.
	Choice string
	// Probabilities are per-option probabilities of a Choice question, if the backend provides them.
	Probabilities map[string]float64
	// Yes is the probability of yes for a Noul question; nil for any other kind. A pointer, so a
	// missing value is not read as 0.
	Yes *float64
	// Score is the position of a Score question, from 0 to len(Levels)-1, possibly fractional;
	// nil for any other kind.
	Score *float64
	// Confidence is what the backend reports as its confidence, and ConfidenceMeaning says what that
	// number is. A threshold must name the field it reads; the two are not interchangeable.
	Confidence        *float64
	ConfidenceMeaning ConfidenceMeaning
}

// ConfidenceMeaning records where a confidence number comes from.
type ConfidenceMeaning string

const (
	ConfidenceNone       ConfidenceMeaning = ""
	ConfidenceProvider   ConfidenceMeaning = "provider-probability"
	ConfidenceSelfReport ConfidenceMeaning = "self-reported"
	// ConfidenceSummary is a provider's summary of its own answer distribution; it is not the
	// probability of the picked option and not a measure of correctness.
	ConfidenceSummary ConfidenceMeaning = "provider-summary"
)

// Probability returns the probability of the given option, and false if the backend gave none.
func (a Answer) Probability(option string) (float64, bool) {
	p, ok := a.Probabilities[option]
	return p, ok
}

// Request is one classifier call: one state and the questions about it.
//
// State is the approved, policy-checked payload; a classifier never sees source files or syntax trees.
type Request struct {
	// Linter is the name of the asking linter, for logs, budgets and scripted fakes.
	Linter    string
	State     map[string]any
	Questions []Question
}

// Usage is what a request cost.
type Usage struct {
	InputTokens int
	// CostUSD is the backend's estimate under its price assumption; zero when unknown.
	CostUSD float64
}

// Response holds one answer per question, in any order.
type Response struct {
	Answers []Answer
	Usage   Usage
}

// Capabilities are what a classifier backend supports.
type Capabilities struct {
	Kinds []Kind
	// Probabilities is true when Choice answers carry per-option probabilities.
	Probabilities bool
	// Local is true when requests never leave the machine.
	Local bool
	// MaxStateBytes limits the encoded state of one request; zero means no limit.
	MaxStateBytes int
}

// Supports reports whether the backend answers questions of kind k.
func (c Capabilities) Supports(k Kind) bool {
	for _, x := range c.Kinds {
		if x == k {
			return true
		}
	}
	return false
}

// Classifier answers typed questions over one state.
type Classifier interface {
	Capabilities() Capabilities
	// Classify answers every question of the request or returns an error. A partial response is an error.
	Classify(ctx context.Context, req Request) (Response, error)
}

// ClassifierFactory builds a classifier from its configuration.
type ClassifierFactory struct {
	Name string
	Doc  string
	// NewSettings returns a pointer to a zero settings struct to decode the configuration into;
	// nil means the classifier takes no settings.
	NewSettings func() any
	New         func(settings any) (Classifier, error)
}
