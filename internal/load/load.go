// Package load loads Go packages and runs the linters' analyzers over them.
package load

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/ssgreg/lintuition/sdk"
)

// Options control package loading.
type Options struct {
	Dir       string
	Patterns  []string
	Tests     bool
	BuildTags []string
	// ModFlag is passed as -mod= when set.
	ModFlag string
}

// Problem is a package that could not be analysed, fully or in part.
type Problem struct {
	Package string
	Err     string
}

// Result is the candidates per analyzer, and every package that could not be analysed.
type Result struct {
	Candidates map[*analysis.Analyzer][]*sdk.Candidate
	Packages   int
	Problems   []Problem
}

const mode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports |
	packages.NeedTypes | packages.NeedTypesSizes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule

// Run loads the packages and runs the analyzers. A package with load or type errors is not analysed
// and is reported as a problem; it is never silently treated as clean.
func Run(ctx context.Context, opts Options, analyzers []*analysis.Analyzer) (*Result, error) {
	cfg := &packages.Config{Context: ctx, Mode: mode, Dir: opts.Dir, Tests: opts.Tests}
	if len(opts.BuildTags) > 0 {
		cfg.BuildFlags = append(cfg.BuildFlags, "-tags="+strings.Join(opts.BuildTags, ","))
	}
	if opts.ModFlag != "" {
		cfg.BuildFlags = append(cfg.BuildFlags, "-mod="+opts.ModFlag)
	}
	patterns := opts.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}
	res := &Result{Candidates: map[*analysis.Analyzer][]*sdk.Candidate{}}
	if len(pkgs) == 0 {
		// An empty match analyses nothing; a mistyped CI target must not pass as clean.
		res.Problems = append(res.Problems, Problem{Package: strings.Join(patterns, " "), Err: "no packages matched"})
		return res, nil
	}
	var good []*packages.Package
	for _, p := range variants(pkgs) {
		if len(p.Errors) > 0 {
			msgs := make([]string, 0, len(p.Errors))
			for _, e := range p.Errors {
				msgs = append(msgs, e.Error())
			}
			res.Problems = append(res.Problems, Problem{Package: p.ID, Err: strings.Join(msgs, "; ")})
			continue
		}
		good = append(good, p)
	}
	res.Packages = len(good)
	if len(good) == 0 || len(analyzers) == 0 {
		return res, nil
	}
	graph, err := checker.Analyze(analyzers, good, nil)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}
	for act := range graph.All() {
		if !act.IsRoot {
			continue
		}
		if act.Err != nil {
			res.Problems = append(res.Problems, Problem{Package: act.Package.ID, Err: fmt.Sprintf("%s: %v", act.Analyzer.Name, act.Err)})
			continue
		}
		cs, _ := act.Result.([]*sdk.Candidate)
		res.Candidates[act.Analyzer] = append(res.Candidates[act.Analyzer], cs...)
	}
	for a, cs := range res.Candidates {
		res.Candidates[a] = dedupe(cs)
	}
	sort.Slice(res.Problems, func(i, j int) bool { return res.Problems[i].Package < res.Problems[j].Package })
	return res, nil
}

// variants drops the package variants that only duplicate others: the generated test main
// (`p.test`), and a package `p` whose test variant `p [p.test]` holds the same files and more.
func variants(pkgs []*packages.Package) []*packages.Package {
	ids := map[string]bool{}
	for _, p := range pkgs {
		ids[p.ID] = true
	}
	var out []*packages.Package
	for _, p := range pkgs {
		if strings.HasSuffix(p.ID, ".test") && p.Name == "main" {
			continue
		}
		if ids[p.ID+" ["+p.ID+".test]"] {
			continue
		}
		out = append(out, p)
	}
	return out
}

func dedupe(cs []*sdk.Candidate) []*sdk.Candidate {
	seen := map[string]bool{}
	out := cs[:0]
	for _, c := range cs {
		k := fmt.Sprintf("%s:%d:%d:%s", c.Pos.Filename, c.Pos.Line, c.Pos.Column, c.Subject)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Pos, out[j].Pos
		if a.Filename != b.Filename {
			return a.Filename < b.Filename
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
	return out
}
