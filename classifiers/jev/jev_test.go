package jev

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ssgreg/lintuition/sdk"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

// server answers with the handler and records every body it got.
type server struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []string
	auth   []string
}

func newServer(t *testing.T, h func(n int, body map[string]any, w http.ResponseWriter)) *server {
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.bodies = append(s.bodies, string(b))
		s.auth = append(s.auth, r.Header.Get("Authorization"))
		n := len(s.bodies)
		s.mu.Unlock()
		var body map[string]any
		json.Unmarshal(b, &body)
		h(n, body, w)
	}))
	t.Cleanup(s.Close)
	return s
}

func newTest(t *testing.T, url string) *Classifier {
	t.Helper()
	c, err := New(Settings{Endpoint: url, Model: "jev-test"}, env(map[string]string{DefaultKeyEnv: "k-123"}))
	if err != nil {
		t.Fatal(err)
	}
	c.sleep = func(context.Context, time.Duration) error { return nil }
	return c
}

var req = sdk.Request{
	Linter: "secret-linter-name",
	State:  map[string]any{"help": "Disk I/O utilization."},
	Questions: []sdk.Question{
		{ID: "kind", Kind: sdk.Choice, Text: "What does `help` describe?", Options: []sdk.Option{{Key: "total", Description: "a total"}, {Key: "current", Description: "now"}}},
		{ID: "yes", Kind: sdk.Noul, Text: "Is `help` short?"},
		{ID: "lvl", Kind: sdk.Score, Text: "How clear is `help`?", Levels: []string{"unclear", "somewhat", "clear"}},
	},
}

const okResp = `{"model":"jev-1.13.0","answers":{
 "kind":{"type":"choice","choice":"current","confidence":0.53,"probabilities":{"total":0.3,"current":0.65,"unclear":0.05}},
 "yes":{"type":"noul","noul":0.94},
 "lvl":{"type":"score","score":1.4,"confidence":0.4,"probabilities":{"0":0.1,"1":0.4,"2":0.5}}},
 "usage":{"input_tokens":1000000,"output_tokens":20}}`

func TestRequestAndResponse(t *testing.T) {
	s := newServer(t, func(_ int, _ map[string]any, w http.ResponseWriter) { io.WriteString(w, okResp) })
	resp, err := newTest(t, s.URL).Classify(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Model     string
		State     map[string]any
		Questions map[string]struct {
			Type         string
			Instructions string
			Criteria     any
		}
	}
	json.Unmarshal([]byte(s.bodies[0]), &body)
	if body.Model != "jev-test" || body.State["help"] != "Disk I/O utilization." || s.auth[0] != "Bearer k-123" {
		t.Fatalf("body %+v auth %q", body, s.auth[0])
	}
	crit, _ := body.Questions["kind"].Criteria.(map[string]any)
	if crit["unclear"] == nil || crit["total"] != "a total" {
		t.Errorf("every choice must offer unclear: %v", crit)
	}
	if lv, _ := body.Questions["lvl"].Criteria.([]any); len(lv) != 3 || body.Questions["yes"].Criteria != nil {
		t.Errorf("score levels / noul criteria: %+v", body.Questions)
	}
	if strings.Contains(s.bodies[0], "secret-linter-name") {
		t.Error("the linter name must not be sent")
	}
	by := map[string]sdk.Answer{}
	for _, a := range resp.Answers {
		by[a.QuestionID] = a
	}
	if p, _ := by["kind"].Probability("current"); by["kind"].Choice != "current" || p != 0.65 || *by["kind"].Confidence != 0.53 || by["kind"].ConfidenceMeaning != sdk.ConfidenceSummary {
		t.Errorf("choice: %+v", by["kind"])
	}
	if *by["yes"].Yes != 0.94 || *by["lvl"].Score != 1.4 {
		t.Errorf("noul/score: %+v %+v", by["yes"], by["lvl"])
	}
	if resp.Usage.InputTokens != 1000000 || resp.Usage.CostUSD != DefaultPricePerMTok {
		t.Errorf("usage: %+v", resp.Usage)
	}
}

func TestRetries(t *testing.T) {
	s := newServer(t, func(n int, _ map[string]any, w http.ResponseWriter) {
		switch n {
		case 1:
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.WriteHeader(http.StatusBadGateway)
		default:
			io.WriteString(w, okResp)
		}
	})
	c := newTest(t, s.URL)
	var waits []time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	if _, err := c.Classify(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(s.bodies) != 3 || len(waits) == 0 || waits[0] != 2*time.Second {
		t.Fatalf("requests %d, waits %v", len(s.bodies), waits)
	}
}

func TestErrorsDoNotLeakPayloadOrKey(t *testing.T) {
	s := newServer(t, func(_ int, body map[string]any, w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": "bad input", "echo": body})
	})
	_, err := newTest(t, s.URL).Classify(context.Background(), req)
	if err == nil || len(s.bodies) != 1 {
		t.Fatalf("a 400 must fail without retries: %v, %d requests", err, len(s.bodies))
	}
	if strings.Contains(err.Error(), "utilization") || strings.Contains(err.Error(), "k-123") {
		t.Fatalf("error leaks payload or key: %v", err)
	}
}

func TestBadAnswers(t *testing.T) {
	for _, body := range []string{
		`{"answers":{"kind":{"type":"noul","noul":0.5}}}`,
		`{"answers":{"kind":{"type":"choice"}}}`,
		`not json`,
	} {
		s := newServer(t, func(int, map[string]any, http.ResponseWriter) {})
		s.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, body) })
		if _, err := newTest(t, s.URL).Classify(context.Background(), req); err == nil {
			t.Errorf("%s: want an error", body)
		}
	}
}

func TestSettings(t *testing.T) {
	keyless, err := New(Settings{}, env(nil))
	if err != nil {
		t.Fatalf("a missing key must not stop construction (config verify, preview): %v", err)
	}
	if err := keyless.Ready(); err == nil || !strings.Contains(err.Error(), "export TYPESAFE_API_KEY") {
		t.Errorf("missing key: %v", err)
	}
	if _, err := keyless.Classify(context.Background(), req); err == nil {
		t.Error("classify without a key must fail")
	}
	k := env(map[string]string{"MY_KEY": "x"})
	if _, err := New(Settings{APIKeyEnv: "MY_KEY", Endpoint: "http://api.example.com/v1"}, k); err == nil {
		t.Error("plain http to a remote host must be refused")
	}
	neg := -1
	if _, err := New(Settings{APIKeyEnv: "MY_KEY", MaxRetries: &neg}, k); err == nil {
		t.Error("negative retries must be refused")
	}
	c, err := New(Settings{APIKeyEnv: "MY_KEY"}, k)
	if err != nil || c.endpoint != DefaultEndpoint || c.model != DefaultModel || c.Capabilities().Local {
		t.Fatalf("defaults: %+v %v", c, err)
	}
}

func TestLimiterSpacesRequests(t *testing.T) {
	l := newLimiter(60) // one a second
	now := time.Unix(0, 0)
	l.now = func() time.Time { return now }
	var waits []time.Duration
	sleep := func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	for range 3 {
		l.wait(context.Background(), sleep)
	}
	if len(waits) != 2 || waits[0] != time.Second || waits[1] != 2*time.Second {
		t.Fatalf("waits %v", waits)
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	var second bool
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { second = true }))
	defer other.Close()
	for _, code := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect, http.StatusFound} {
		s := newServer(t, func(_ int, _ map[string]any, w http.ResponseWriter) {
			w.Header().Set("Location", other.URL+"/collect")
			w.WriteHeader(code)
		})
		_, err := newTest(t, s.URL).Classify(context.Background(), req)
		if err == nil || second || len(s.bodies) != 1 {
			t.Fatalf("%d: err %v, followed %v, %d attempts", code, err, second, len(s.bodies))
		}
	}
}

func TestRetriesSpendBudget(t *testing.T) {
	s := newServer(t, func(_ int, _ map[string]any, w http.ResponseWriter) { w.WriteHeader(http.StatusTooManyRequests) })
	c := newTest(t, s.URL)
	spent := 0
	r := req
	r.Retry = func() error {
		if spent == 1 {
			return errors.New("budget reached")
		}
		spent++
		return nil
	}
	_, err := c.Classify(context.Background(), r)
	if err == nil || !strings.Contains(err.Error(), "budget reached") || len(s.bodies) != 2 {
		t.Fatalf("err %v, %d attempts; want 2 (the first plus one paid retry)", err, len(s.bodies))
	}
}

func TestMalformedAnswersRejectedWithoutQuotingThem(t *testing.T) {
	const marker = "PRIVATE_RESPONSE_MARKER"
	for name, body := range map[string]string{
		"unrequested": `{"answers":{"kind":{"type":"choice","choice":"current","probabilities":{"current":1}},"yes":{"type":"noul","noul":0.5},"lvl":{"type":"score","score":1},"alien":{"type":"noul","noul":0.1}}}`,
		"duplicate":   `{"answers":{"kind":{"type":"choice","choice":"total"},"kind":{"type":"choice","choice":"current"},"yes":{"type":"noul","noul":0.5},"lvl":{"type":"score","score":1}}}`,
		"mixed":       `{"answers":{"kind":{"type":"choice","choice":"current","noul":0.5},"yes":{"type":"noul","noul":0.5},"lvl":{"type":"score","score":1}}}`,
		"wrong type":  `{"answers":{"kind":{"type":"` + marker + `"},"yes":{"type":"noul","noul":0.5},"lvl":{"type":"score","score":1}}}`,
		"bad json":    `{"answers":{"kind":` + marker,
	} {
		s := newServer(t, func(int, map[string]any, http.ResponseWriter) {})
		s.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, body) })
		_, err := newTest(t, s.URL).Classify(context.Background(), req)
		if err == nil {
			t.Errorf("%s: want an error", name)
			continue
		}
		if strings.Contains(err.Error(), marker) {
			t.Errorf("%s: error quotes the response: %v", name, err)
		}
	}
}

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for v, want := range map[string]time.Duration{
		"120":  120 * time.Second,
		"1.5":  1500 * time.Millisecond,
		"0":    0,
		"-3":   0,
		"soon": 0,
		now.Add(2 * time.Minute).Format(http.TimeFormat):  2 * time.Minute,
		now.Add(-2 * time.Minute).Format(http.TimeFormat): 0,
	} {
		if got := retryAfter(v, now); got != want {
			t.Errorf("retryAfter(%q) = %v, want %v", v, got, want)
		}
	}
	// A delay longer than the run has left fails at once instead of retrying early.
	s := newServer(t, func(_ int, _ map[string]any, w http.ResponseWriter) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	c := newTest(t, s.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.Classify(ctx, req); err == nil || !strings.Contains(err.Error(), "exceeds the run's remaining time") || len(s.bodies) != 1 {
		t.Fatalf("err %v, %d attempts", err, len(s.bodies))
	}
}
