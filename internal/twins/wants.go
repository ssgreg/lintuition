package twins

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/ssgreg/lintuition/internal/report"
)

var (
	wantRE    = regexp.MustCompile("// want ((?:\\s*(?:`[^`]*`|\"(?:[^\"\\\\]|\\\\.)*\"))+)")
	patternRE = regexp.MustCompile("`[^`]*`|\"(?:[^\"\\\\]|\\\\.)*\"")
)

// readWants collects the want comments of the analysed files, the same set the patterns, build tags
// and test selection gave the engine, keyed by file:line relative to dir.
func ReadWants(dir string, files []string) (map[string][]*regexp.Regexp, error) {
	out := map[string][]*regexp.Regexp{}
	for _, p := range files {
		if err := readFileWants(dir, p, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func readFileWants(dir, p string, out map[string][]*regexp.Regexp) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	rel, _ := filepath.Rel(dir, p)
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		m := wantRE.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		for _, q := range patternRE.FindAllString(m[1], -1) {
			s, err := strconv.Unquote(q)
			if err != nil {
				return fmt.Errorf("%s:%d: bad want: %v", rel, n, err)
			}
			re, err := regexp.Compile(s)
			if err != nil {
				return fmt.Errorf("%s:%d: bad want: %v", rel, n, err)
			}
			k := fmt.Sprintf("%s:%d", rel, n)
			out[k] = append(out[k], re)
		}
	}
	return sc.Err()
}

// match pairs patterns with findings one to one, as many as possible (a maximum bipartite
// matching), so the order of the patterns on a line does not matter. It returns, for each pattern,
// the index of its finding or -1.
func Match(pats []*regexp.Regexp, issues []report.Issue) []int {
	ofIssue := make([]int, len(issues))
	for i := range ofIssue {
		ofIssue[i] = -1
	}
	var try func(p int, seen []bool) bool
	try = func(p int, seen []bool) bool {
		for i, is := range issues {
			if seen[i] || !pats[p].MatchString(is.Text) {
				continue
			}
			seen[i] = true
			if ofIssue[i] < 0 || try(ofIssue[i], seen) {
				ofIssue[i] = p
				return true
			}
		}
		return false
	}
	for p := range pats {
		try(p, make([]bool, len(issues)))
	}
	out := make([]int, len(pats))
	for p := range out {
		out[p] = -1
	}
	for i, p := range ofIssue {
		if p >= 0 {
			out[p] = i
		}
	}
	return out
}
