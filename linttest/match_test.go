package linttest

import (
	"regexp"
	"testing"

	"github.com/ssgreg/lintuition/internal/report"
)

func TestMatchIsOrderIndependent(t *testing.T) {
	issues := []report.Issue{{Text: "counter Help describes a current value"}, {Text: "gauge Help describes a running total"}}
	broad, specific := regexp.MustCompile(`Help describes`), regexp.MustCompile(`^counter`)
	for _, pats := range [][]*regexp.Regexp{{broad, specific}, {specific, broad}} {
		for p, i := range match(pats, issues) {
			if i < 0 {
				t.Errorf("pattern %q unmatched with order %v", pats[p], pats)
			}
		}
	}
}
