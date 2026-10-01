package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// jevModule writes a one-file module and a config that asks a stand-in Jev server, which answers
// every question with answer(state).
func jevModule(t *testing.T, linter, src string, answer func(state map[string]any) string) {
	t.Helper()
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var req struct {
			State map[string]any `json:"state"`
		}
		json.Unmarshal(b, &req)
		io.WriteString(w, `{"answers":`+answer(req.State)+`,"usage":{"input_tokens":10}}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("WAVE1B_TEST_KEY", "k")
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module p\n\ngo 1.26\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "p.go"), []byte(src), 0o600)
	os.WriteFile(filepath.Join(dir, ".lintuition.yml"), []byte(`version: "2"
linters:
  default: none
  enable: [`+linter+`]
semantic:
  classifier: jev
  cache: {disabled: true}
  classifiers:
    jev:
      endpoint: `+srv.URL+`
      api-key-env: WAVE1B_TEST_KEY
      max-retries: 0
`), 0o600)
	t.Chdir(dir)
}

func choice(q, c, probs string) string {
	return `{"` + q + `":{"type":"choice","choice":"` + c + `","confidence":0.8,"probabilities":` + probs + `}}`
}

// A sparse answer must not read the missing "same" as zero: near the threshold it abstains like the
// full near tie does, while a clear margin still reports.
func TestSentinelMarginNeedsTheCompetingProbability(t *testing.T) {
	src := "package p\n\nimport \"errors\"\n\nvar ErrNotFound = errors.New(\"permission denied\")\n"
	for _, tc := range []struct {
		name, probs string
		code        int
	}{
		{"sentinel-sparse-margin", `{"different":0.56}`, 0},
		{"sentinel-full-near-tie", `{"different":0.56,"same":0.44}`, 0},
		{"sentinel-clear-margin", `{"different":0.8,"same":0.1}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jevModule(t, "sentinel-name-vs-text", src, func(map[string]any) string { return choice("condition", "different", tc.probs) })
			code, out, errs := run("run", "./...")
			if code != tc.code {
				t.Fatalf("exit %d, want %d\n%s%s", code, tc.code, out, errs)
			}
		})
	}
}
