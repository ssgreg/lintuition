package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// jevStub answers every request with the choice fn picks from the Help, counting HTTP attempts.
type jevStub struct {
	*httptest.Server
	mu    sync.Mutex
	calls int
}

func newJevStub(t *testing.T, fn func(help, auth string) (status int, choice string)) *jevStub {
	s := &jevStub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var req struct{ State struct{ Help string } }
		json.Unmarshal(b, &req)
		s.mu.Lock()
		s.calls++
		s.mu.Unlock()
		status, choice := fn(req.State.Help, r.Header.Get("Authorization"))
		if status != 200 {
			w.WriteHeader(status)
			return
		}
		io.WriteString(w, `{"answers":{"kind":{"type":"choice","choice":"`+choice+`","probabilities":{"`+choice+`":0.9}}},"usage":{"input_tokens":1000000}}`)
	}))
	t.Cleanup(s.Close)
	return s
}

func onlyUtilization(t *testing.T, dir string) {
	t.Helper()
	// One candidate: keep only the counter whose Help is the utilization text.
	os.WriteFile(filepath.Join(dir, "metrics.go"), []byte(`package sample

import prom "github.com/prometheus/client_golang/prometheus"

var ioSeconds = prom.NewCounter(prom.CounterOpts{Name: "io_seconds_total", Help: "Disk I/O utilization."})
`), 0o600)
}

func jevConfig(t *testing.T, dir, url, extra string) {
	t.Helper()
	os.WriteFile(filepath.Join(dir, ".lintuition.yml"), []byte(`version: "2"
semantic:
  classifier: jev
  classifiers:
    jev: {endpoint: `+url+`, api-key-env: LINTUITION_TEST_KEY, max-retries: 0, price-per-mtok: 100}
  cache: {dir: `+filepath.Join(dir, "cache")+`}
`+extra), 0o600)
}

func TestCostCapStopsLaterVotes(t *testing.T) {
	dir := sample(t)
	onlyUtilization(t, dir)
	s := newJevStub(t, func(string, string) (int, string) { return 200, "current" })
	t.Setenv("LINTUITION_TEST_KEY", "k")
	jevConfig(t, dir, s.URL, "  votes: 3\n  budget: {max-cost-usd: 1}\n")
	code, _, errs := run("run", "./...")
	if code != 2 || s.calls != 1 || !strings.Contains(errs, "max-cost-usd") {
		t.Fatalf("exit %d, %d calls; the first $100 reply must stop the other votes\n%s", code, s.calls, errs)
	}
}

func TestRequestsCountAttemptsNotReservations(t *testing.T) {
	dir := sample(t)
	onlyUtilization(t, dir)
	s := newJevStub(t, func(string, string) (int, string) { return 400, "" })
	t.Setenv("LINTUITION_TEST_KEY", "k")
	jevConfig(t, dir, s.URL, "  votes: 3\n")
	code, out, errs := run("run", "--output.json.path", "stdout", "./...")
	var rep struct {
		Report struct {
			Lintuition struct{ Stats struct{ Requests int } }
		}
	}
	json.Unmarshal([]byte(out), &rep)
	if code != 2 || s.calls != 1 || rep.Report.Lintuition.Stats.Requests != 1 {
		t.Fatalf("exit %d, %d calls, reported %d requests\n%s", code, s.calls, rep.Report.Lintuition.Stats.Requests, errs)
	}
}

func TestCacheIsScopedToTheAccount(t *testing.T) {
	dir := sample(t)
	onlyUtilization(t, dir)
	s := newJevStub(t, func(_, auth string) (int, string) {
		if auth == "Bearer key-a" {
			return 200, "current"
		}
		return 200, "total"
	})
	jevConfig(t, dir, s.URL, "")
	t.Setenv("LINTUITION_TEST_KEY", "key-a")
	if code, _, errs := run("run", "./..."); code != 1 || s.calls != 1 {
		t.Fatalf("account a: exit %d, %d calls\n%s", code, s.calls, errs)
	}
	t.Setenv("LINTUITION_TEST_KEY", "key-b")
	if code, _, errs := run("run", "./..."); code != 0 || s.calls != 2 {
		t.Fatalf("account b must not replay a's answers: exit %d, %d calls\n%s", code, s.calls, errs)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".lintuition.yml")); strings.Contains(string(b), "key-a") {
		t.Fatal("sanity")
	}
	filepath.Walk(filepath.Join(dir, "cache"), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			if b, _ := os.ReadFile(p); strings.Contains(string(b), "key-") {
				t.Errorf("%s holds the key", p)
			}
		}
		return nil
	})
}

func TestInvalidCacheRecordIsRefreshed(t *testing.T) {
	dir := sample(t)
	onlyUtilization(t, dir)
	s := newJevStub(t, func(string, string) (int, string) { return 200, "current" })
	t.Setenv("LINTUITION_TEST_KEY", "k")
	jevConfig(t, dir, s.URL, "  votes: 3\n")
	if code, _, _ := run("run", "./..."); code != 1 || s.calls != 3 {
		t.Fatalf("prime: exit %d, %d calls", code, s.calls)
	}
	// Cut the record to one sample, as a corrupted or foreign record might be.
	filepath.Walk(filepath.Join(dir, "cache"), func(p string, info os.FileInfo, err error) error {
		if err == nil && strings.HasSuffix(p, ".json") {
			var bd map[string]any
			b, _ := os.ReadFile(p)
			json.Unmarshal(b, &bd)
			bd["samples"] = bd["samples"].([]any)[:1]
			nb, _ := json.Marshal(bd)
			os.WriteFile(p, nb, 0o600)
		}
		return nil
	})
	if code, _, errs := run("run", "--dry-run", "./..."); code != 0 || !strings.Contains(errs, "0 cached") {
		t.Fatalf("dry run must not count a short record as cached: %d %s", code, errs)
	}
	code, out, errs := run("run", "--output.json.path", "stdout", "./...")
	if code != 1 || s.calls != 6 || !strings.Contains(out, `"agreement":{"kind":"current 3"}`) || !strings.Contains(out, `"linter_version":"1"`) {
		t.Fatalf("a short record must be refreshed with three samples: exit %d, %d calls\n%s\n%s", code, s.calls, out, errs)
	}
}

func TestCacheCleanKeepsForeignFiles(t *testing.T) {
	dir := sample(t)
	cache := filepath.Join(dir, "shared")
	os.MkdirAll(cache, 0o700)
	os.WriteFile(filepath.Join(cache, "README.md"), []byte("keep"), 0o600)
	os.WriteFile(filepath.Join(dir, ".lintuition.yml"), []byte("version: \"2\"\nsemantic:\n  cache: {dir: "+cache+"}\n"), 0o600)
	if code, out, errs := run("cache", "clean"); code != 0 || !strings.Contains(out, "removed 0") {
		t.Fatalf("exit %d %s %s", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(cache, "README.md")); err != nil {
		t.Fatal("cache clean removed a file it did not write")
	}
}

func TestCostAtCapStopsTheNextSample(t *testing.T) {
	dir := sample(t)
	onlyUtilization(t, dir)
	s := newJevStub(t, func(string, string) (int, string) { return 200, "current" })
	t.Setenv("LINTUITION_TEST_KEY", "k")
	// One reply costs exactly the cap ($100 at 1M tokens and price 100).
	jevConfig(t, dir, s.URL, "  votes: 3\n  budget: {max-cost-usd: 100}\n")
	if code, _, errs := run("run", "./..."); code != 2 || s.calls != 1 {
		t.Fatalf("exit %d, %d calls; a budget spent exactly must stop the next sample\n%s", code, s.calls, errs)
	}
}

func TestQueuedCandidateChecksCostFirst(t *testing.T) {
	dir := sample(t)
	s := newJevStub(t, func(string, string) (int, string) { return 200, "current" })
	t.Setenv("LINTUITION_TEST_KEY", "k")
	jevConfig(t, dir, s.URL, "  concurrency: 1\n  budget: {max-cost-usd: 1}\n")
	if code, _, errs := run("run", "./..."); code != 2 || s.calls != 1 {
		t.Fatalf("exit %d, %d calls; candidates queued behind the first must not be sent once it spent the budget\n%s", code, s.calls, errs)
	}
}

func TestFingerprintsTellFunctionsApart(t *testing.T) {
	dir := sample(t)
	os.WriteFile(filepath.Join(dir, "metrics.go"), []byte(`package sample

import prom "github.com/prometheus/client_golang/prometheus"

var a = prom.CounterOpts{Help: "Disk I/O utilization."}
var b = prom.CounterOpts{Help: "Disk I/O utilization."}
`), 0o600)
	fps := func() []string {
		_, out, _ := run("run", "--output.json.path", "stdout", "./...")
		var rep struct {
			Issues []struct{ Fingerprint string }
		}
		json.Unmarshal([]byte(out), &rep)
		var fs []string
		for _, is := range rep.Issues {
			fs = append(fs, is.Fingerprint)
		}
		return fs
	}
	first := fps()
	if len(first) != 2 || first[0] == first[1] {
		t.Fatalf("two identical-looking findings need two fingerprints: %v", first)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "metrics.go"))
	os.WriteFile(filepath.Join(dir, "metrics.go"), []byte(strings.Replace(string(b), "var a", "// an unrelated comment\n\nvar a", 1)), 0o600)
	if again := fps(); strings.Join(again, ",") != strings.Join(first, ",") {
		t.Fatalf("fingerprints must survive unrelated line moves: %v then %v", first, again)
	}
}

func TestYAMLOnlyFormat(t *testing.T) {
	dir := sample(t)
	appendConfig(t, dir, "output:\n  formats:\n    sarif:\n      path: stdout\n")
	code, out, errs := run("run", "./...")
	if code != 1 || !strings.HasPrefix(strings.TrimSpace(out), "{") || strings.Contains(out, "metrics.go:10:30:") {
		t.Fatalf("only sarif on stdout: exit %d\n%s\n%s", code, out, errs)
	}
}
