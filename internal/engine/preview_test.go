package engine_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	_ "github.com/ssgreg/lintuition/builtin"
	"github.com/ssgreg/lintuition/internal/config"
	"github.com/ssgreg/lintuition/internal/engine"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestPreviewFailureFailsTheRun(t *testing.T) {
	c, err := config.Load("../../examples/sample/.lintuition.yml")
	if err != nil {
		t.Fatal(err)
	}
	res, err := engine.Run(context.Background(), engine.Options{Config: c, Dir: "../../examples/sample", Patterns: []string{"./..."}, DryRun: true, Preview: failingWriter{}})
	if err != nil {
		t.Fatal(err)
	}
	planned := 0
	for _, l := range res.Run.Linters {
		planned += l.Planned
	}
	if !res.Run.Incomplete || planned != 0 || !strings.Contains(strings.Join(res.Run.Problems, "\n"), "disk full") {
		t.Fatalf("incomplete %v, planned %d, problems %v", res.Run.Incomplete, planned, res.Run.Problems)
	}
}
