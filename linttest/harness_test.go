package linttest_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/ssgreg/lintuition/linttest"
)

// recorder is a testing.TB that records failures instead of failing, so the harness's own
// failures can be asserted.
type recorder struct {
	testing.TB
	mu     sync.Mutex
	errors []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	panic(r)
}

func (r *recorder) Fatal(args ...any) { r.Fatalf("%s", fmt.Sprint(args...)) }

func runRecorded(t *testing.T, dir string, patterns ...string) []string {
	r := &recorder{TB: t}
	func() {
		defer func() {
			if p := recover(); p != nil && p != r {
				panic(p)
			}
		}()
		linttest.Run(r, dir, patterns...)
	}()
	return r.errors
}

func TestHarnessSeesPastIssueLimits(t *testing.T) {
	errs := runRecorded(t, "../testdata/harness", "./limits")
	if len(errs) != 1 || !strings.Contains(errs[0], "limits/limits.go:11: unexpected finding") {
		t.Fatalf("the unannotated fourth finding must fail the harness, got %q", errs)
	}
}

func TestHarnessReadsOnlySelectedPackages(t *testing.T) {
	errs := runRecorded(t, "../testdata/harness", "./other")
	if len(errs) != 1 || !strings.Contains(errs[0], "other/other.go:6: no finding") {
		t.Fatalf("selected package: got %q", errs)
	}
	for _, e := range runRecorded(t, "../testdata/harness", "./limits") {
		if strings.Contains(e, "other/") {
			t.Fatalf("an unselected package's want was demanded: %q", e)
		}
	}
}
