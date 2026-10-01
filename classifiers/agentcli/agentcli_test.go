package agentcli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestAnswerStrictness(t *testing.T) {
	noul := sdk.Question{ID: "y", Kind: sdk.Noul, Text: "?"}
	for name, raw := range map[string]string{
		"null probability": `{"answer":"B","probabilities":{"A":null,"B":1}}`,
		"duplicate key":    `{"answer":"A","answer":"B","probabilities":{"A":0.5,"B":0.5}}`,
		"trailing value":   `{"answer":"A","probabilities":{"A":0.6,"B":0.4}} {"answer":"B"}`,
		"not the max":      `{"answer":"B","probabilities":{"A":0.53,"B":0.52}}`,
	} {
		if _, err := answer(noul, json.RawMessage(raw)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	// A tie is a maximum, whichever label comes first.
	for _, raw := range []string{`{"answer":"B","probabilities":{"A":0.4995,"B":0.4995}}`, `{"answer":"A","probabilities":{"B":0.4995,"A":0.4995}}`} {
		if _, err := answer(noul, json.RawMessage(raw)); err != nil {
			t.Errorf("%s: a tied maximum is a valid answer: %v", raw, err)
		}
	}
}

func script(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755)
	return p
}

func TestIdentity(t *testing.T) {
	a := script(t, "a", `echo "1.0 (Claude Code)"`)
	b := script(t, "b", `echo "1.0 (Claude Code)"`)
	ca, _ := New("claude-code", Settings{Command: a})
	cb, _ := New("claude-code", Settings{Command: b})
	if ca.Identity() == "" || ca.Identity() == cb.Identity() {
		t.Errorf("two executables with one version string must not share answers: %q %q", ca.Identity(), cb.Identity())
	}
	slow := script(t, "slow", `sleep 30; echo "1.0"`)
	cs, _ := New("claude-code", Settings{Command: slow})
	done := make(chan string, 1)
	go func() { done <- cs.Identity() }()
	select {
	case id := <-done:
		if id != "" {
			t.Errorf("a version that cannot be read in time must give no identity, got %q", id)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("version discovery is not bounded")
	}
}

func TestUsageAccounting(t *testing.T) {
	c, _ := New("claude-code", Settings{})
	for name, tc := range map[string]struct {
		out     string
		unknown bool
		cost    float64
	}{
		"cost":         {`{"structured_output":{"answer":"A","probabilities":{"A":1,"B":0,"C":0}},"total_cost_usd":0.1,"usage":{"input_tokens":5}}`, false, 0.1},
		"missing cost": {`{"structured_output":{"answer":"A","probabilities":{"A":1,"B":0,"C":0}},"usage":{"input_tokens":5}}`, true, 0},
		"null cost":    {`{"structured_output":{},"total_cost_usd":null}`, true, 0},
		"negative":     {`{"structured_output":{},"total_cost_usd":-0.1}`, true, 0},
	} {
		_, u, _ := c.parse(t.TempDir(), []byte(tc.out))
		if u.Unknown != tc.unknown || u.CostUSD != tc.cost {
			t.Errorf("%s: usage %+v", name, u)
		}
	}
	cx, _ := New("codex", Settings{AcceptAgentTools: true})
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "answer.json"), []byte(`{}`), 0o600)
	if _, u, _ := cx.parse(dir, []byte(`{"type":"item.completed"}`)); !u.Unknown {
		t.Error("codex without a turn.completed usage must be unknown, not free")
	}
	// A run that fails still reports what it cost.
	c.run = func(context.Context, string, []string) ([]byte, error) {
		return []byte(`{"is_error":true,"total_cost_usd":0.1,"usage":{"input_tokens":100}}`), errors.New("exit 1")
	}
	var used []sdk.Usage
	_, err := c.Classify(context.Background(), sdk.Request{State: map[string]any{}, Questions: []sdk.Question{q}, Used: func(u sdk.Usage) { used = append(used, u) }})
	if err == nil || len(used) != 1 || used[0].CostUSD != 0.1 {
		t.Fatalf("failed run: %v %+v", err, used)
	}
}

func TestCodexNeedsConsent(t *testing.T) {
	fake := script(t, "codex", `echo codex-cli 1.0`)
	c, _ := New("codex", Settings{Command: fake})
	if err := c.Ready(); err == nil || !strings.Contains(err.Error(), "accept-agent-tools") {
		t.Fatalf("codex must refuse without consent: %v", err)
	}
	c, _ = New("codex", Settings{Command: fake, AcceptAgentTools: true})
	if err := c.Ready(); err != nil {
		t.Fatal(err)
	}
	sc, _ := schema(q)
	args, _ := c.args(t.TempDir(), "P", sc)
	joined := strings.Join(args, " ")
	for _, want := range []string{"--ignore-user-config", "project_doc_max_bytes=0", "features.shell_tool=false", "features.multi_agent=false"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args lack %q", want)
		}
	}
	home, err := codexHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(home)
	if len(entries) != 1 || entries[0].Name() != "auth.json" || entries[0].Type()&os.ModeSymlink == 0 {
		t.Fatalf("CODEX_HOME must hold only a link to the login: %v", entries)
	}
}
