package metrictypevshelp

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ssgreg/lintuition/sdk"
)

func TestExtraction(t *testing.T) {
	res := analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
	var got []string
	for _, r := range res {
		for _, c := range r.Result.([]*sdk.Candidate) {
			if len(c.Payload.Facts) > 0 || len(c.Payload.Source) > 0 {
				t.Errorf("%s: only the Help may be sent, payload %+v", c.Subject, c.Payload)
			}
			if c.Unsupported != "" {
				got = append(got, fmt.Sprintf("%d %s unsupported: %s", c.Pos.Line, c.Subject, c.Unsupported))
				continue
			}
			got = append(got, fmt.Sprintf("%d %s %s %q", c.Pos.Line, c.Subject, c.Local["kind"], c.Payload.Prose["help"]))
		}
	}
	want := []string{
		`15 io_seconds_total counter "Disk I/O utilization."`,
		`16 in_flight gauge "Requests being served now."`,
		`17 latency_seconds histogram "Latency."`,
		`18 size_bytes summary "Sizes."`,
		`19 dynamic_total unsupported: Help is built at run time`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidates:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func choice(key string, p float64) map[string]sdk.Answer {
	return map[string]sdk.Answer{"kind": {QuestionID: "kind", Choice: key, Probabilities: map[string]float64{key: p}}}
}

func TestDecide(t *testing.T) {
	r := &rule{threshold: 0.8}
	counter := &sdk.Candidate{Local: map[string]string{"kind": "counter", "help": "Disk I/O utilization."}}
	gauge := &sdk.Candidate{Local: map[string]string{"kind": "gauge", "help": "x"}}
	cases := []struct {
		c       *sdk.Candidate
		answers map[string]sdk.Answer
		report  bool
		abstain bool
	}{
		{counter, choice("current", 0.9), true, false},
		{counter, choice("total", 0.9), false, false},
		{counter, choice("total", 0.3), false, true},
		{counter, map[string]sdk.Answer{"kind": {QuestionID: "kind", Choice: "total"}}, false, true},
		{counter, choice("current", 0.6), false, true},
		{counter, choice("unclear", 0.9), false, true},
		{counter, map[string]sdk.Answer{"kind": {QuestionID: "kind", Choice: "current"}}, false, true},
		{gauge, choice("current", 0.9), false, false},
		{gauge, choice("distribution", 0.85), true, false},
	}
	for i, tc := range cases {
		d := r.Decide(tc.c, tc.answers)
		if d.Report != tc.report || (d.Abstained != "") != tc.abstain {
			t.Errorf("case %d: got %+v", i, d)
		}
	}
}
