package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// TestOutboundJev runs the sample against a stand-in Jev server and checks what left the machine:
// the Help text and the question, and not the metric name, file names or the linter name.
func TestOutboundJev(t *testing.T) {
	dir := sample(t)
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		var req struct{ State struct{ Help string } }
		json.Unmarshal(b, &req)
		choice := "total"
		switch h := req.State.Help; {
		case strings.Contains(h, "utilization"), strings.Contains(h, "being served"):
			choice = "current"
		case strings.Contains(h, "latency"):
			choice = "distribution"
		}
		io.WriteString(w, `{"answers":{"kind":{"type":"choice","choice":"`+choice+`","confidence":0.8,"probabilities":{"`+choice+`":0.9}}},"usage":{"input_tokens":100}}`)
	}))
	defer srv.Close()
	t.Setenv("LINTUITION_TEST_KEY", "k-secret")
	os.WriteFile(filepath.Join(dir, ".lintuition.yml"), []byte(`version: "2"
semantic:
  classifier: jev
  classifiers:
    jev:
      endpoint: `+srv.URL+`
      api-key-env: LINTUITION_TEST_KEY
  cache:
    dir: `+filepath.Join(dir, "cache")+`
`), 0o600)
	code, out, errs := run("run", "./...")
	if code != 1 || !strings.Contains(out, "metrics.go:10:30: counter Help describes a current value") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if len(bodies) != 4 {
		t.Fatalf("%d requests, want one per candidate", len(bodies))
	}
	all := strings.Join(bodies, "\n")
	for _, never := range []string{"io_seconds_total", "requests_in_flight", "metrics.go", dir, "metric-type-vs-help", "k-secret"} {
		if strings.Contains(all, never) {
			t.Errorf("request bodies contain %q", never)
		}
	}
	if !strings.Contains(all, "Disk I/O utilization.") || !strings.Contains(all, `"unclear"`) {
		t.Errorf("bodies lack the Help or the unclear option:\n%s", all)
	}
	if !strings.Contains(errs, "~$0.000017 at the backend's price assumption") {
		t.Errorf("the summary shows the estimated cost: %s", errs)
	}
}

func TestPreview(t *testing.T) {
	sample(t)
	code, out, errs := run("run", "--dry-run", "--preview", "requests.jsonl", "./...")
	if code != 0 || out != "" {
		t.Fatalf("exit %d %s %s", code, out, errs)
	}
	b, err := os.ReadFile("requests.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 4 {
		t.Fatalf("%d preview lines, want 4", len(lines))
	}
	var first struct {
		Linter string
		File   string
		Line   int
		State  map[string]any
	}
	json.Unmarshal([]byte(lines[0]), &first)
	if first.Linter != "metric-type-vs-help" || first.File != "metrics.go" || first.State["help"] != "Disk I/O utilization." {
		t.Fatalf("preview: %s", lines[0])
	}
	if code, _, errs := run("run", "--preview", "x.jsonl", "./..."); code != 2 || !strings.Contains(errs, "needs --dry-run") {
		t.Fatalf("preview without dry run: %d %s", code, errs)
	}
}

// TestCacheAndVotes: three samples per candidate, a strict majority decides, a second run replays the
// bundle from the cache without new requests, and a changed vote count is a different key.
func TestCacheAndVotes(t *testing.T) {
	dir := sample(t)
	var mu sync.Mutex
	n := 0
	perHelp := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var req struct{ State struct{ Help string } }
		json.Unmarshal(b, &req)
		mu.Lock()
		n++
		perHelp[req.State.Help]++
		k := perHelp[req.State.Help]
		mu.Unlock()
		choice := "total"
		switch h := req.State.Help; {
		case strings.Contains(h, "utilization"):
			choice = "current"
		case strings.Contains(h, "being served"):
			// Disagreeing samples: current, total, distribution in turn.
			choice = []string{"current", "total", "distribution"}[k%3]
		case strings.Contains(h, "latency"):
			choice = "distribution"
		}
		io.WriteString(w, `{"answers":{"kind":{"type":"choice","choice":"`+choice+`","probabilities":{"`+choice+`":0.9}}},"usage":{"input_tokens":10}}`)
	}))
	defer srv.Close()
	t.Setenv("LINTUITION_TEST_KEY", "k")
	cfg := func(votes string) {
		os.WriteFile(filepath.Join(dir, ".lintuition.yml"), []byte(`version: "2"
semantic:
  classifier: jev
  votes: `+votes+`
  classifiers:
    jev: {endpoint: `+srv.URL+`, api-key-env: LINTUITION_TEST_KEY}
  cache:
    dir: `+filepath.Join(dir, "cache")+`
`), 0o600)
	}
	cfg("3")
	code, out, errs := run("run", "--output.json.path", "stdout", "./...")
	if code != 1 || n != 12 {
		t.Fatalf("first run: exit %d, %d requests, want 12 (4 candidates x 3)\n%s", code, n, errs)
	}
	if !strings.Contains(errs, "1 abstained") || !strings.Contains(out, `"samples":3`) {
		t.Fatalf("disagreeing samples must abstain; evidence must show 3 samples\n%s\n%s", out, errs)
	}
	code, out, errs = run("run", "--output.json.path", "stdout", "./...")
	if code != 1 || n != 12 || !strings.Contains(errs, "4 from cache") || !strings.Contains(out, `"replayed":true`) {
		t.Fatalf("second run must replay: exit %d, %d requests\n%s\n%s", code, n, out, errs)
	}
	cfg("1")
	if _, _, errs := run("run", "./..."); n != 16 || strings.Contains(errs, "from cache") {
		t.Fatalf("one vote is a different key: %d requests\n%s", n, errs)
	}
	// Everything is cached now: a budget of one request is not touched.
	if code, _, errs := run("run", "--max-requests", "1", "./..."); code != 1 || n != 16 {
		t.Fatalf("cached run under a tight budget: exit %d, %d requests\n%s", code, n, errs)
	}
}

func TestRetriesCountAgainstBudget(t *testing.T) {
	dir := sample(t)
	var mu sync.Mutex
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	t.Setenv("LINTUITION_TEST_KEY", "k")
	os.WriteFile(filepath.Join(dir, ".lintuition.yml"), []byte(`version: "2"
semantic:
  classifier: jev
  concurrency: 1
  budget: {max-requests: 6}
  classifiers:
    jev: {endpoint: `+srv.URL+`, api-key-env: LINTUITION_TEST_KEY, max-retries: 2}
  cache: {disabled: true}
`), 0o600)
	code, _, errs := run("run", "--output.json.path", "stdout", "./...")
	if code != 2 || attempts > 6 {
		t.Fatalf("exit %d, %d HTTP attempts for a budget of 6\n%s", code, attempts, errs)
	}
}

func TestPreviewOfflineAndCollisions(t *testing.T) {
	dir := sample(t)
	os.WriteFile(filepath.Join(dir, ".lintuition.yml"), []byte("version: \"2\"\nsemantic:\n  classifier: jev\n  classifiers:\n    jev: {api-key-env: LINTUITION_UNSET_KEY}\n"), 0o600)
	code, _, errs := run("run", "--dry-run", "--preview", "req.jsonl", "./...")
	b, _ := os.ReadFile("req.jsonl")
	if code != 0 || !strings.Contains(string(b), `"body":{"model":"jev-latest"`) {
		t.Fatalf("keyless preview: exit %d %s\n%s", code, errs, b)
	}
	if code, _, errs := run("run", "./..."); code != 2 || !strings.Contains(errs, "export LINTUITION_UNSET_KEY") {
		t.Fatalf("a real run without the key must fail early: %d %s", code, errs)
	}
	os.WriteFile("keep.json", []byte("keep"), 0o600)
	if code, _, errs := run("run", "--dry-run", "--preview", "keep.json", "--output.json.path", "./keep.json", "./..."); code != 2 || !strings.Contains(errs, "already gets the json output") {
		t.Fatalf("collision: %d %s", code, errs)
	}
	if b, _ := os.ReadFile("keep.json"); string(b) != "keep" {
		t.Fatalf("a rejected collision overwrote the file: %q", b)
	}
}

func TestEvalOffline(t *testing.T) {
	twinsDir, _ := filepath.Abs("../testdata/twins")
	code, out, errs := run("eval", "--runs", "2", twinsDir)
	// Every marked twin is caught in every run, however many twins the linters bring.
	if code != 0 || !regexp.MustCompile(`caught in every run [1-9]\d*, in some 0, in none 0; 0 unaccounted`).MatchString(out) {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
}

func TestEvalKeepsFindingsApartAndReportsCoverage(t *testing.T) {
	dir, _ := filepath.Abs("../testdata/harness")
	json := filepath.Join(t.TempDir(), "eval.json")
	_, out, errs := run("eval", "--runs", "3", "--json", json, dir, "./evalcov")
	if !strings.Contains(out, "2 unaccounted finding(s)") || strings.Count(out, " 3/3: ") != 2 || strings.Contains(out, "6/3") {
		t.Fatalf("two different findings on one line are two, each in 3 of 3 runs:\n%s\n%s", out, errs)
	}
	if !strings.Contains(out, "metric-type-vs-help") || !strings.Contains(out, "x 3 completed runs (of 3 requested)") {
		t.Fatalf("coverage and completed runs must be reported:\n%s", out)
	}
	b, _ := os.ReadFile(json)
	if !strings.Contains(string(b), `"Unsupported": 3`) || !strings.Contains(string(b), `"runs_completed": 3`) {
		t.Fatalf("JSON must carry coverage (one unsupported candidate a run):\n%s", b)
	}
}
