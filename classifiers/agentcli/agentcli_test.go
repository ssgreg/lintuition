package agentcli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ssgreg/lintuition/sdk"
)

var q = sdk.Question{ID: "kind", Kind: sdk.Choice, Text: "What does `help` describe?",
	Options: []sdk.Option{{Key: "total", Description: "a total"}, {Key: "current", Description: "now"}}}

func TestAnswer(t *testing.T) {
	a, err := answer(q, json.RawMessage(`{"answer":"B","probabilities":{"A":0.1,"B":0.8,"C":0.1}}`))
	if err != nil || a.Choice != "current" || a.Probabilities["current"] != 0.8 || a.ConfidenceMeaning != sdk.ConfidenceSelfReport {
		t.Fatalf("%+v %v", a, err)
	}
	for name, raw := range map[string]string{
		"not offered":  `{"answer":"Z","probabilities":{"A":0.1,"B":0.8,"C":0.1}}`,
		"missing prob": `{"answer":"B","probabilities":{"A":0.2,"B":0.8}}`,
		"extra field":  `{"answer":"B","probabilities":{"A":0.1,"B":0.8,"C":0.1},"note":"x"}`,
		"bad sum":      `{"answer":"B","probabilities":{"A":0.5,"B":0.8,"C":0.5}}`,
		"not the max":  `{"answer":"A","probabilities":{"A":0.1,"B":0.8,"C":0.1}}`,
		"out of range": `{"answer":"B","probabilities":{"A":-0.1,"B":1.0,"C":0.1}}`,
		"extra option": `{"answer":"B","probabilities":{"A":0.1,"B":0.8,"C":0.1,"D":0}}`,
	} {
		if _, err := answer(q, json.RawMessage(raw)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	y, err := answer(sdk.Question{ID: "y", Kind: sdk.Noul, Text: "?"}, json.RawMessage(`{"answer":"A","probabilities":{"A":0.7,"B":0.3}}`))
	if err != nil || *y.Yes != 0.7 {
		t.Fatalf("noul: %+v %v", y, err)
	}
}

func TestClaudeArgsHaveNoTools(t *testing.T) {
	c, _ := New("claude-code", Settings{Model: "haiku"})
	sc, _ := schema(q)
	args, _ := c.args(t.TempDir(), "PROMPT", sc)
	joined := strings.Join(args, " ")
	for _, want := range []string{"-p", "--tools  ", "--strict-mcp-config", "--no-session-persistence", "--setting-sources  ", "--json-schema", "--model haiku"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args lack %q: %s", want, joined)
		}
	}
	if args[len(args)-1] != "PROMPT" {
		t.Error("the prompt goes last")
	}
}

func TestCodexRun(t *testing.T) {
	c, err := New("codex", Settings{ReasoningEffort: "low"})
	if err != nil {
		t.Fatal(err)
	}
	var gotArgs []string
	c.run = func(_ context.Context, dir string, args []string) ([]byte, error) {
		gotArgs = args
		os.WriteFile(filepath.Join(dir, "answer.json"), []byte(`{"answer":"A","probabilities":{"A":0.9,"B":0.05,"C":0.05}}`), 0o600)
		return []byte(`{"type":"turn.completed","usage":{"input_tokens":14000}}` + "\n"), nil
	}
	var used []sdk.Usage
	resp, err := c.Classify(context.Background(), sdk.Request{State: map[string]any{"help": "h"}, Questions: []sdk.Question{q}, Used: func(u sdk.Usage) { used = append(used, u) }})
	if err != nil || resp.Answers[0].Choice != "total" || len(used) != 1 || used[0].InputTokens != 14000 {
		t.Fatalf("%+v %v %+v", resp, err, used)
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"exec", "--sandbox read-only", "--ephemeral", "--output-schema", `model_reasoning_effort="low"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("args lack %q", want)
		}
	}
	if !strings.HasPrefix(gotArgs[len(gotArgs)-1], Harness) {
		t.Error("codex has no system prompt flag: the harness must lead the prompt")
	}
	if !c.Capabilities().CallsPerQuestion || c.Capabilities().Local {
		t.Error("one run per question, and it leaves the machine")
	}
}

func TestSettings(t *testing.T) {
	if _, err := New("claude-code", Settings{ReasoningEffort: "low"}); err == nil {
		t.Error("reasoning-effort is for codex only")
	}
	if _, err := New("codex", Settings{MaxParallel: 99}); err == nil {
		t.Error("max-parallel is bounded")
	}
}
