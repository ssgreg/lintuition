// Package jev is a classifier backend for TypeSafe System One models (Jev), https://typesafe.ai.
//
//	semantic:
//	  classifier: jev
//	  classifiers:
//	    jev:
//	      model: jev-latest           # pin a version (jev-1.13.0) for reproducible results
//	      api-key-env: TYPESAFE_API_KEY
//
// Requests leave the machine. The adapter sends the policy-approved state and the questions, and
// nothing else: no file names, positions or linter names.
package jev

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"

	"github.com/ssgreg/lintuition/internal/httpx"
	"github.com/ssgreg/lintuition/sdk"
)

// Defaults.
const (
	DefaultEndpoint = "https://api.typesafe.ai/v1/systemone"
	DefaultModel    = "jev-latest"
	DefaultKeyEnv   = "TYPESAFE_API_KEY"
	// DefaultPricePerMTok is the published input price in USD per million tokens when this adapter
	// was written; output tokens are not billed. It is an assumption for cost estimates and budgets,
	// not a bill: set price-per-mtok when it changes.
	DefaultPricePerMTok = 0.042
)

// Settings configure the backend.
type Settings struct {
	Endpoint  string `yaml:"endpoint"`
	Model     string `yaml:"model"`
	APIKeyEnv string `yaml:"api-key-env"`
	// Timeout per HTTP attempt (default 60s).
	Timeout string `yaml:"timeout"`
	// MaxRetries for 429 and 5xx responses and transport errors (default 4).
	MaxRetries *int `yaml:"max-retries"`
	// RequestsPerMinute caps the request rate (default 1000, under the service's 1200).
	RequestsPerMinute int `yaml:"requests-per-minute"`
	// PricePerMTok is the input price used for cost estimates and budgets.
	PricePerMTok *float64 `yaml:"price-per-mtok"`
	// Account scopes the answer cache; by default a one-way hash of the API key does.
	Account string `yaml:"account"`
}

func init() {
	sdk.RegisterClassifier(sdk.ClassifierFactory{
		Name:        "jev",
		Doc:         "TypeSafe System One models (Jev); remote, needs an API key.",
		NewSettings: func() any { return &Settings{} },
		New:         func(s any) (sdk.Classifier, error) { return New(*s.(*Settings), os.Getenv) },
	})
}

// Classifier is the Jev backend.
type Classifier struct {
	account string
	keyEnv  string
	model   string
	key     string
	price   float64
	hc      *httpx.Client
}

// New validates the settings and returns the backend. getenv reads the API key.
func New(s Settings, getenv func(string) string) (*Classifier, error) {
	endpoint := s.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	hc, err := httpx.New(endpoint, httpx.Options{Timeout: s.Timeout, MaxRetries: s.MaxRetries, RequestsPerMinute: s.RequestsPerMinute})
	if err != nil {
		return nil, err
	}
	c := &Classifier{account: s.Account, model: s.Model, price: DefaultPricePerMTok, hc: hc}
	if c.model == "" {
		c.model = DefaultModel
	}
	env := s.APIKeyEnv
	if env == "" {
		env = DefaultKeyEnv
	}
	// A missing key is reported by Ready, so config verify and a dry-run preview work offline.
	c.key, c.keyEnv = getenv(env), env
	if s.PricePerMTok != nil {
		if p := *s.PricePerMTok; math.IsNaN(p) || math.IsInf(p, 0) || p < 0 {
			return nil, fmt.Errorf("price-per-mtok %v: must be a finite number, 0 or more", p)
		}
		c.price = *s.PricePerMTok
	}
	return c, nil
}

// Ready reports whether the backend can send: the API key must be set.
func (c *Classifier) Ready() error {
	if c.key == "" {
		return fmt.Errorf("the API key is not set: export %s", c.keyEnv)
	}
	return nil
}

// Identity names what decides an answer besides the request, for the answer cache: the endpoint,
// the model and the account. The account is the account setting, or else a one-way hash of the API
// key, so answers never cross accounts and the key is never stored. A moving alias such as
// jev-latest is bounded by the cache TTL.
func (c *Classifier) Identity() string {
	acct := c.account
	if acct == "" {
		sum := sha256.Sum256([]byte("lintuition-cache-scope\x00" + c.key))
		acct = "key:" + hex.EncodeToString(sum[:8])
	}
	return "jev|" + c.hc.Endpoint() + "|" + c.model + "|" + acct
}

// Model is the public model identity for report evidence.
func (c *Classifier) Model() string { return c.model }

// Capabilities implements sdk.Classifier.
func (c *Classifier) Capabilities() sdk.Capabilities {
	return sdk.Capabilities{Kinds: []sdk.Kind{sdk.Choice, sdk.Noul, sdk.Score}, Probabilities: true, CostKnown: c.price > 0}
}

// wire types: https://typesafe.ai, POST /v1/systemone.
type wireRequest struct {
	Model     string                  `json:"model"`
	State     map[string]any          `json:"state"`
	Questions map[string]wireQuestion `json:"questions"`
}

type wireQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	// Criteria is option -> description for a choice, a list of levels for a score.
	Criteria any `json:"criteria,omitempty"`
}

type wireAnswer struct {
	Type          string             `json:"type"`
	Choice        *string            `json:"choice"`
	Noul          *float64           `json:"noul"`
	Score         *float64           `json:"score"`
	Confidence    *float64           `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

type wireResponse struct {
	Model   string                `json:"model"`
	Answers map[string]wireAnswer `json:"answers"`
	Usage   struct {
		InputTokens int `json:"input_tokens"`
	} `json:"usage"`
}

// Body is the exact request body for req; exported for payload preview and tests.
func (c *Classifier) Body(req sdk.Request) ([]byte, error) {
	w := wireRequest{Model: c.model, State: req.State, Questions: map[string]wireQuestion{}}
	for _, q := range req.Questions {
		wq := wireQuestion{Type: string(q.Kind), Instructions: q.Text}
		switch q.Kind {
		case sdk.Choice:
			crit := map[string]string{}
			for _, o := range q.WithUnclear() {
				crit[o.Key] = o.Description
			}
			wq.Criteria = crit
		case sdk.Score:
			wq.Criteria = q.Levels
		}
		w.Questions[q.ID] = wq
	}
	return json.Marshal(w)
}

// Classify implements sdk.Classifier.
func (c *Classifier) Classify(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	body, err := c.Body(req)
	if err != nil {
		return sdk.Response{}, err
	}
	if err := c.Ready(); err != nil {
		return sdk.Response{}, err
	}
	raw, err := c.hc.Post(ctx, body, http.Header{"Authorization": {"Bearer " + c.key}}, req.Retry, req.Start)
	if err != nil {
		return sdk.Response{}, err
	}
	// A 200 response cost money whatever its answers: account it first, unknown when it says
	// nothing usable. The usage also goes back in the response, for callers without Used.
	usage := responseUsage(raw, c.price)
	if req.Used != nil {
		req.Used(usage)
	}
	if err := httpx.NoDuplicateKeys(raw); err != nil {
		return sdk.Response{Usage: usage}, err
	}
	var wr wireResponse
	if err := json.Unmarshal(raw, &wr); err != nil {
		// The decoder's message can quote the body; keep only the category.
		return sdk.Response{Usage: usage}, errors.New("the response is not valid JSON of the expected shape")
	}
	asked := map[string]bool{}
	for _, q := range req.Questions {
		asked[q.ID] = true
	}
	for id := range wr.Answers {
		if !asked[id] {
			return sdk.Response{Usage: usage}, errors.New("the response answers a question that was not asked")
		}
	}
	resp := sdk.Response{Usage: usage}
	for _, q := range req.Questions {
		wa, ok := wr.Answers[q.ID]
		if !ok {
			continue // Check reports the missing answer
		}
		a, err := convert(q, wa)
		if err != nil {
			// q.ID is ours; err names only the category, never a remote value.
			return sdk.Response{Usage: usage}, fmt.Errorf("question %q: %w", q.ID, err)
		}
		resp.Answers = append(resp.Answers, a)
	}
	return resp, nil
}

// responseUsage reads the input tokens of a response. Missing, null, negative, or a body with a
// repeated key (whose last value a decoder would trust) is unknown.
func responseUsage(raw []byte, price float64) sdk.Usage {
	if httpx.NoDuplicateKeys(raw) != nil {
		return sdk.Usage{Unknown: true}
	}
	var u struct {
		Usage struct {
			InputTokens *int `json:"input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &u) != nil || u.Usage.InputTokens == nil || *u.Usage.InputTokens < 0 {
		return sdk.Usage{Unknown: true}
	}
	n := *u.Usage.InputTokens
	return sdk.Usage{InputTokens: n, CostUSD: float64(n) * price / 1e6}
}

// convert maps a wire answer to the SDK, refusing a wrong type or value fields of another kind
// rather than dropping them. Errors never quote remote values.
func convert(q sdk.Question, wa wireAnswer) (sdk.Answer, error) {
	if wa.Type != string(q.Kind) {
		return sdk.Answer{}, fmt.Errorf("the answer's type is not %s", q.Kind)
	}
	switch q.Kind {
	case sdk.Choice:
		if wa.Noul != nil || wa.Score != nil {
			return sdk.Answer{}, errors.New("a choice answer carries a noul or score value")
		}
	case sdk.Noul:
		if wa.Choice != nil || wa.Score != nil || len(wa.Probabilities) > 0 {
			return sdk.Answer{}, errors.New("a noul answer carries other values")
		}
	case sdk.Score:
		// Score answers come with a distribution over the levels; it is metadata, not a choice.
		if wa.Choice != nil || wa.Noul != nil {
			return sdk.Answer{}, errors.New("a score answer carries a choice or noul value")
		}
		wa.Probabilities = nil
	}
	a := sdk.Answer{QuestionID: q.ID}
	if wa.Confidence != nil {
		a.Confidence, a.ConfidenceMeaning = wa.Confidence, sdk.ConfidenceSummary
	}
	switch q.Kind {
	case sdk.Choice:
		if wa.Choice == nil {
			return sdk.Answer{}, errors.New("no choice")
		}
		a.Choice, a.Probabilities = *wa.Choice, wa.Probabilities
	case sdk.Noul:
		a.Yes = wa.Noul
	case sdk.Score:
		a.Score = wa.Score
	}
	return a, nil
}
