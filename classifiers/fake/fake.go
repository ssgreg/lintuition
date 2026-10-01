// Package fake is a deterministic, local classifier that answers from scripted rules. It is for tests,
// demos and offline CI; it never sends anything anywhere.
//
//	semantic:
//	  classifier: fake
//	  classifiers:
//	    fake:
//	      unmatched: unclear        # or: error
//	      rules:
//	        - linter: metric-type-vs-help
//	          question: kind
//	          when: "seconds spent"     # substring of the encoded state
//	          choice: total
//	          probability: 0.95
package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ssgreg/lintuition/sdk"
)

// Settings configure the fake.
type Settings struct {
	// Unmatched is what a question with no matching rule gets: unclear (the default) or error.
	Unmatched string `yaml:"unmatched"`
	Rules     []Rule `yaml:"rules"`
}

// Rule answers a question when every set field matches. The first matching rule wins.
type Rule struct {
	Linter   string `yaml:"linter"`
	Question string `yaml:"question"`
	// When is a substring of the request state encoded as JSON with sorted keys.
	When        string   `yaml:"when"`
	Choice      string   `yaml:"choice"`
	Probability *float64 `yaml:"probability"`
	Yes         *float64 `yaml:"yes"`
	Score       *float64 `yaml:"score"`
}

func init() {
	sdk.RegisterClassifier(sdk.ClassifierFactory{
		Name:        "fake",
		Doc:         "Deterministic scripted answers for tests and offline runs; local.",
		NewSettings: func() any { return &Settings{} },
		New:         func(s any) (sdk.Classifier, error) { return New(*s.(*Settings)) },
	})
}

// Classifier is the fake backend.
type Classifier struct{ s Settings }

// New validates the settings and returns the fake.
func New(s Settings) (*Classifier, error) {
	switch s.Unmatched {
	case "":
		s.Unmatched = "unclear"
	case "unclear", "error":
	default:
		return nil, fmt.Errorf("unmatched: %q is not one of unclear, error", s.Unmatched)
	}
	for i, r := range s.Rules {
		n := 0
		for _, set := range []bool{r.Choice != "", r.Yes != nil, r.Score != nil} {
			if set {
				n++
			}
		}
		if n != 1 {
			return nil, fmt.Errorf("rules[%d]: set exactly one of choice, yes, score", i)
		}
	}
	return &Classifier{s: s}, nil
}

// Capabilities implements sdk.Classifier.
func (c *Classifier) Capabilities() sdk.Capabilities {
	return sdk.Capabilities{Kinds: []sdk.Kind{sdk.Choice, sdk.Noul, sdk.Score}, Probabilities: true, Local: true}
}

// Classify implements sdk.Classifier.
func (c *Classifier) Classify(_ context.Context, req sdk.Request) (sdk.Response, error) {
	// encoding/json sorts map keys, so the encoding is stable.
	b, err := json.Marshal(req.State)
	if err != nil {
		return sdk.Response{}, err
	}
	state := string(b)
	var resp sdk.Response
	for _, q := range req.Questions {
		a, ok := c.match(req.Linter, q, state)
		if !ok {
			if c.s.Unmatched == "error" {
				return sdk.Response{}, fmt.Errorf("no rule answers %s/%s", req.Linter, q.ID)
			}
			a = unclear(q)
		}
		resp.Answers = append(resp.Answers, a)
	}
	resp.Usage.InputTokens = len(state) / 4
	return resp, nil
}

func (c *Classifier) match(linter string, q sdk.Question, state string) (sdk.Answer, bool) {
	for _, r := range c.s.Rules {
		if (r.Linter != "" && r.Linter != linter) || (r.Question != "" && r.Question != q.ID) ||
			(r.When != "" && !strings.Contains(state, r.When)) {
			continue
		}
		a := sdk.Answer{QuestionID: q.ID}
		switch {
		case r.Choice != "" && q.Kind == sdk.Choice:
			p := 1.0
			if r.Probability != nil {
				p = *r.Probability
			}
			a.Choice = r.Choice
			a.Probabilities = spread(q, r.Choice, p)
		case r.Yes != nil && q.Kind == sdk.Noul:
			a.Yes = r.Yes
		case r.Score != nil && q.Kind == sdk.Score:
			a.Score = r.Score
		default:
			continue
		}
		return a, true
	}
	return sdk.Answer{}, false
}

func unclear(q sdk.Question) sdk.Answer {
	a := sdk.Answer{QuestionID: q.ID}
	switch q.Kind {
	case sdk.Choice:
		a.Choice = sdk.Unclear
		a.Probabilities = spread(q, sdk.Unclear, 1)
	case sdk.Noul:
		half := 0.5
		a.Yes = &half
	case sdk.Score:
		mid := (q.Min + q.Max) / 2
		a.Score = &mid
	}
	return a
}

// spread gives the picked option p and shares the rest evenly.
func spread(q sdk.Question, pick string, p float64) map[string]float64 {
	opts := q.WithUnclear()
	out := map[string]float64{}
	rest := (1 - p) / float64(len(opts)-1)
	for _, o := range opts {
		out[o.Key] = rest
	}
	out[pick] = p
	return out
}
