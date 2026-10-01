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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

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
	keyEnv   string
	endpoint string
	model    string
	key      string
	client   *http.Client
	retries  int
	price    float64
	limiter  *limiter
	sleep    func(context.Context, time.Duration) error
}

// New validates the settings and returns the backend. getenv reads the API key.
func New(s Settings, getenv func(string) string) (*Classifier, error) {
	c := &Classifier{endpoint: s.Endpoint, model: s.Model, retries: 4, price: DefaultPricePerMTok, sleep: sleepCtx}
	if c.endpoint == "" {
		c.endpoint = DefaultEndpoint
	}
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("endpoint: %w", err)
	}
	// Plain HTTP only to a loopback address (a test server or a local proxy).
	if u.Scheme != "https" && !(u.Scheme == "http" && isLoopback(u.Hostname())) {
		return nil, fmt.Errorf("endpoint %q: https is required", c.endpoint)
	}
	if c.model == "" {
		c.model = DefaultModel
	}
	env := s.APIKeyEnv
	if env == "" {
		env = DefaultKeyEnv
	}
	// A missing key is reported by Ready, so config verify and a dry-run preview work offline.
	c.key, c.keyEnv = getenv(env), env
	timeout := 60 * time.Second
	if s.Timeout != "" {
		if timeout, err = time.ParseDuration(s.Timeout); err != nil || timeout <= 0 {
			return nil, fmt.Errorf("timeout %q: must be a positive duration", s.Timeout)
		}
	}
	c.client = &http.Client{
		Timeout: timeout,
		// Never follow a redirect: it would resend the body and the key to a place New did not check.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	if s.MaxRetries != nil {
		if *s.MaxRetries < 0 || *s.MaxRetries > 10 {
			return nil, fmt.Errorf("max-retries %d: must be 0 to 10", *s.MaxRetries)
		}
		c.retries = *s.MaxRetries
	}
	rpm := s.RequestsPerMinute
	if rpm == 0 {
		rpm = 1000
	}
	if rpm < 0 {
		return nil, fmt.Errorf("requests-per-minute %d: must be positive", rpm)
	}
	c.limiter = newLimiter(rpm)
	if s.PricePerMTok != nil {
		if p := *s.PricePerMTok; math.IsNaN(p) || math.IsInf(p, 0) || p < 0 {
			return nil, fmt.Errorf("price-per-mtok %v: must be a finite number, 0 or more", p)
		}
		c.price = *s.PricePerMTok
	}
	return c, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Ready reports whether the backend can send: the API key must be set.
func (c *Classifier) Ready() error {
	if c.key == "" {
		return fmt.Errorf("the API key is not set: export %s", c.keyEnv)
	}
	return nil
}

// Identity names what decides an answer besides the request, for the answer cache: the endpoint
// and the model. A moving alias such as jev-latest is bounded by the cache TTL.
func (c *Classifier) Identity() string { return "jev|" + c.endpoint + "|" + c.model }

// Capabilities implements sdk.Classifier.
func (c *Classifier) Capabilities() sdk.Capabilities {
	return sdk.Capabilities{Kinds: []sdk.Kind{sdk.Choice, sdk.Noul, sdk.Score}, Probabilities: true}
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
	raw, err := c.post(ctx, body, req.Retry)
	if err != nil {
		return sdk.Response{}, err
	}
	if err := uniqueAnswerKeys(raw); err != nil {
		return sdk.Response{}, err
	}
	var wr wireResponse
	if err := json.Unmarshal(raw, &wr); err != nil {
		// The decoder's message can quote the body; keep only the category.
		return sdk.Response{}, errors.New("the response is not valid JSON of the expected shape")
	}
	asked := map[string]bool{}
	for _, q := range req.Questions {
		asked[q.ID] = true
	}
	for id := range wr.Answers {
		if !asked[id] {
			return sdk.Response{}, errors.New("the response answers a question that was not asked")
		}
	}
	resp := sdk.Response{Usage: sdk.Usage{
		InputTokens: wr.Usage.InputTokens,
		CostUSD:     float64(wr.Usage.InputTokens) * c.price / 1e6,
	}}
	for _, q := range req.Questions {
		wa, ok := wr.Answers[q.ID]
		if !ok {
			continue // Check reports the missing answer
		}
		a, err := convert(q, wa)
		if err != nil {
			// q.ID is ours; err names only the category, never a remote value.
			return sdk.Response{}, fmt.Errorf("question %q: %w", q.ID, err)
		}
		resp.Answers = append(resp.Answers, a)
	}
	return resp, nil
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

// uniqueAnswerKeys refuses a response whose answers object names a question twice; a JSON decoder
// would silently keep the last one.
func uniqueAnswerKeys(raw []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return errors.New("the response is not a JSON object")
	}
	ans, ok := top["answers"]
	if !ok {
		return errors.New("the response has no answers")
	}
	dec := json.NewDecoder(bytes.NewReader(ans))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return errors.New("the response's answers are not an object")
	}
	seen := map[string]bool{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return errors.New("the response's answers are malformed")
		}
		k, _ := t.(string)
		if seen[k] {
			return errors.New("the response answers one question twice")
		}
		seen[k] = true
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return errors.New("the response's answers are malformed")
		}
	}
	return nil
}

// apiError is a failure the service reported, including a redirect, which is never followed. It never carries the request or response body: a
// service may echo the input, and the input may be private.
type apiError struct {
	Status int
}

func (e apiError) Error() string {
	return fmt.Sprintf("HTTP %d %s", e.Status, http.StatusText(e.Status))
}

// post sends the body, retrying 429, 5xx and transport errors. Every attempt after the first is
// first cleared with spend (the run's budget); a refusal stops the retries with its error.
func (c *Classifier) post(ctx context.Context, body []byte, spend func() error) ([]byte, error) {
	var last error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 && spend != nil {
			if err := spend(); err != nil {
				return nil, fmt.Errorf("%v; retry not sent: %w", last, err)
			}
		}
		if err := c.limiter.wait(ctx, c.sleep); err != nil {
			return nil, err
		}
		raw, retryAfter, err := c.once(ctx, body)
		if err == nil {
			return raw, nil
		}
		last = err
		var ae apiError
		retryable := !errors.As(err, &ae) || ae.Status == http.StatusTooManyRequests || ae.Status >= 500
		if !retryable || attempt == c.retries || ctx.Err() != nil {
			break
		}
		wait := retryAfter
		if wait == 0 {
			// Exponential backoff with jitter: 0.5-1.5 s, 1-3 s, 2-6 s, ...
			base := time.Duration(1<<attempt) * time.Second
			wait = base/2 + time.Duration(rand.Int64N(int64(base)))
		}
		// The server's delay is honoured as given; the run's context bounds the total wait.
		if dl, ok := ctx.Deadline(); ok && time.Until(dl) < wait {
			return nil, fmt.Errorf("%v; the requested retry delay of %s exceeds the run's remaining time", last, wait.Round(time.Second))
		}
		if err := c.sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
	return nil, last
}

func (c *Classifier) once(ctx context.Context, body []byte) ([]byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		// url.Error names the endpoint and the cause; the key travels in a header and is not in it.
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retryAfter(resp.Header.Get("Retry-After"), time.Now()), apiError{Status: resp.StatusCode}
	}
	return raw, 0, nil
}

// retryAfter parses Retry-After in both forms HTTP allows (RFC 9110, 10.2.3): delay seconds or an
// HTTP date. Zero means none was given, or the date has passed.
func retryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	if s, err := strconv.ParseFloat(v, 64); err == nil {
		if s <= 0 || math.IsNaN(s) || math.IsInf(s, 0) {
			return 0
		}
		return time.Duration(s * float64(time.Second))
	}
	if t, err := http.ParseTime(v); err == nil && t.After(now) {
		return t.Sub(now)
	}
	return 0
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// limiter spaces requests evenly to a rate per minute.
type limiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
	now      func() time.Time
}

func newLimiter(rpm int) *limiter {
	return &limiter{interval: time.Minute / time.Duration(rpm), now: time.Now}
}

func (l *limiter) wait(ctx context.Context, sleep func(context.Context, time.Duration) error) error {
	l.mu.Lock()
	now := l.now()
	at := l.next
	if at.Before(now) {
		at = now
	}
	l.next = at.Add(l.interval)
	l.mu.Unlock()
	if d := at.Sub(now); d > 0 {
		return sleep(ctx, d)
	}
	return nil
}
