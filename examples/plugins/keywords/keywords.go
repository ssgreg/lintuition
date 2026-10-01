// Package keywords is an example plugin classifier: it answers yes/no questions by looking for
// configured words in the state's text. It is local and deterministic, which makes it a stand-in
// for a real backend in tests and a template for writing one.
//
//	semantic:
//	  classifier: keywords
//	  classifiers:
//	    keywords:
//	      yes: [owner, "@", ticket, "#", "2026"]
package keywords

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ssgreg/lintuition/sdk"
)

// Settings configure the classifier.
type Settings struct {
	// Yes are words whose presence in any state text answers a yes/no question with yes.
	Yes []string `yaml:"yes"`
}

// Classifier is the keywords backend.
type Classifier struct{ yes []string }

func init() {
	sdk.RegisterClassifier(sdk.ClassifierFactory{
		Name:        "keywords",
		Doc:         "answers yes/no questions by configured words in the text (example plugin); local",
		NewSettings: func() any { return &Settings{} },
		New: func(s any) (sdk.Classifier, error) {
			st := s.(*Settings)
			if len(st.Yes) == 0 {
				return nil, errors.New("yes: list at least one word")
			}
			return &Classifier{yes: st.Yes}, nil
		},
	})
}

// Capabilities implements sdk.Classifier: yes/no only, never leaves the machine.
func (c *Classifier) Capabilities() sdk.Capabilities {
	return sdk.Capabilities{Kinds: []sdk.Kind{sdk.Noul}, Local: true}
}

// Classify implements sdk.Classifier.
func (c *Classifier) Classify(_ context.Context, req sdk.Request) (sdk.Response, error) {
	var texts []string
	keys := make([]string, 0, len(req.State))
	for k := range req.State {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		texts = append(texts, strings.ToLower(fmt.Sprint(req.State[k])))
	}
	all := strings.Join(texts, "\n")
	var resp sdk.Response
	for _, q := range req.Questions {
		if q.Kind != sdk.Noul {
			return sdk.Response{}, fmt.Errorf("question %q: only yes/no questions are supported", q.ID)
		}
		yes := 0.05
		for _, w := range c.yes {
			if strings.Contains(all, strings.ToLower(w)) {
				yes = 0.95
				break
			}
		}
		resp.Answers = append(resp.Answers, sdk.Answer{QuestionID: q.ID, Yes: &yes})
	}
	return resp, nil
}
