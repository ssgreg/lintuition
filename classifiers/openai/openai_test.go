package openai

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ssgreg/lintuition/sdk"
)

// stub answers each completion with first-token log probabilities picked from the prompt.
func stub(t *testing.T, top func(prompt string) map[string]float64) (*httptest.Server, *[]map[string]any) {
	var mu sync.Mutex
	var bodies []map[string]any
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			w.WriteHeader(404)
			return
		}
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(b, &body)
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
		msgs := body["messages"].([]any)
		prompt := msgs[len(msgs)-1].(map[string]any)["content"].(string)
		var tl []map[string]any
		for tok, p := range top(prompt) {
			tl = append(tl, map[string]any{"token": tok, "logprob": math.Log(p)})
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"logprobs": map[string]any{"content": []any{map[string]any{"top_logprobs": tl}}}}},
			"usage":   map[string]any{"prompt_tokens": 100},
		})
	}))
	t.Cleanup(s.Close)
	return s, &bodies
}

func newTest(t *testing.T, url string) *Classifier {
	c, err := New(Settings{BaseURL: url + "/v1", Model: "m"}, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	c.hc.Sleep = func(context.Context, time.Duration) error { return nil }
	return c
}

var req = sdk.Request{
	State: map[string]any{"help": "Disk I/O utilization."},
	Questions: []sdk.Question{
		{ID: "kind", Kind: sdk.Choice, Text: "What does `help` describe?", Options: []sdk.Option{{Key: "total", Description: "a total"}, {Key: "current", Description: "now"}}},
		{ID: "yes", Kind: sdk.Noul, Text: "Is `help` short?"},
		{ID: "lvl", Kind: sdk.Score, Text: "How clear is `help`?", Levels: []string{"unclear", "somewhat", "clear"}},
	},
}

func TestAnswersFromLogprobs(t *testing.T) {
	s, bodies := stub(t, func(p string) map[string]float64 {
		switch {
		case strings.Contains(p, "describe?"):
			return map[string]float64{"B": 0.6, " A": 0.2, "C": 0.1, "Hello": 0.1} // C is unclear
		case strings.Contains(p, "short?"):
			return map[string]float64{"A": 0.3, "B": 0.6}
		default:
			return map[string]float64{"0": 0.2, "2": 0.8}
		}
	})
	c := newTest(t, s.URL)
	resp, err := c.Classify(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(*bodies) != 3 {
		t.Fatalf("%d completions, want one per question", len(*bodies))
	}
	by := map[string]sdk.Answer{}
	for _, a := range resp.Answers {
		by[a.QuestionID] = a
	}
	k := by["kind"]
	if k.Choice != "current" || math.Abs(k.Probabilities["current"]-0.6/0.9) > 1e-9 || math.Abs(k.Probabilities["unclear"]-0.1/0.9) > 1e-9 {
		t.Errorf("choice: %+v (letter mass renormalized, a non-letter token ignored)", k)
	}
	if math.Abs(*by["yes"].Yes-1.0/3) > 1e-9 {
		t.Errorf("noul: yes %v, want 1/3", *by["yes"].Yes)
	}
	if math.Abs(*by["lvl"].Score-1.6) > 1e-9 {
		t.Errorf("score: %v, want the expected level 1.6", *by["lvl"].Score)
	}
	first := (*bodies)[0]
	if first["model"] != "m" || first["logprobs"] != true || first["max_tokens"].(float64) != 1 || first["temperature"].(float64) != 0 {
		t.Errorf("request: %v", first)
	}
	prompt := first["messages"].([]any)[1].(map[string]any)["content"].(string)
	if !strings.Contains(prompt, `{"help":"Disk I/O utilization."}`) || !strings.Contains(prompt, "C: The text does not let you tell.") {
		t.Errorf("prompt:\n%s", prompt)
	}
	if !c.Capabilities().Local || c.Ready() != nil {
		t.Error("a loopback server is local and needs no key")
	}
}

func TestRefusesNonAnswers(t *testing.T) {
	s, _ := stub(t, func(string) map[string]float64 { return map[string]float64{"Sure": 0.9, "A": 0.1} })
	_, err := newTest(t, s.URL).Classify(context.Background(), sdk.Request{State: req.State, Questions: req.Questions[:1]})
	if err == nil || !strings.Contains(err.Error(), "min-letter-mass") {
		t.Fatalf("a model that does not answer with an option must fail: %v", err)
	}
}

func TestRemoteNeedsKeyAndModel(t *testing.T) {
	if _, err := New(Settings{}, func(string) string { return "" }); err == nil {
		t.Error("model is required")
	}
	c, err := New(Settings{Model: "gpt"}, func(string) string { return "" })
	if err != nil || c.Ready() == nil || c.Capabilities().Local {
		t.Fatalf("the default OpenAI endpoint is remote and needs a key: %v", err)
	}
	if _, err := New(Settings{Model: "m", BaseURL: "http://api.example.com/v1"}, func(string) string { return "k" }); err == nil {
		t.Error("plain http to a remote host must be refused")
	}
	a, _ := New(Settings{Model: "m"}, func(string) string { return "key-a" })
	b, _ := New(Settings{Model: "m"}, func(string) string { return "key-b" })
	if a.Identity() == b.Identity() || strings.Contains(a.Identity(), "key-a") {
		t.Error("cache identity must differ by key and never hold it")
	}
}

func TestExtraQuestionsSpendBudget(t *testing.T) {
	s, bodies := stub(t, func(string) map[string]float64 { return map[string]float64{"A": 1} })
	c := newTest(t, s.URL)
	r := req
	r.Questions = []sdk.Question{{ID: "a", Kind: sdk.Noul, Text: "?"}, {ID: "b", Kind: sdk.Noul, Text: "?"}}
	r.Retry = func() error { return context.Canceled }
	if _, err := c.Classify(context.Background(), r); err == nil || len(*bodies) != 1 {
		t.Fatalf("the second question's completion must be cleared with the budget: %v, %d sent", err, len(*bodies))
	}
}
