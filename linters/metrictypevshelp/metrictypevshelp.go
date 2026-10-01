// Package metrictypevshelp checks that a Prometheus metric's Help describes the kind of value its type
// records: a counter's Help a running total, a gauge's a current value, a histogram's or summary's a
// distribution. promlint checks metric names; this checks the person-written Help against the type.
//
// The classifier reads only the Help text. The metric name is not sent: a classifier trusts a name such
// as `..._total` over the text it is asked about.
package metrictypevshelp

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "metric-type-vs-help"

const promPkg = "github.com/prometheus/client_golang/prometheus"

// optsKind maps the options type to the value its metric records.
var optsKind = map[string]string{
	"CounterOpts":   "counter",
	"GaugeOpts":     "gauge",
	"HistogramOpts": "histogram",
	"SummaryOpts":   "summary",
}

// expected is the Help answer each metric kind is consistent with.
var expected = map[string]string{
	"counter":   "total",
	"gauge":     "current",
	"histogram": "distribution",
	"summary":   "distribution",
}

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of the classifier's answer for a finding (default 0.8).
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts one candidate per metric options literal with a constant Help.
var Analyzer = &analysis.Analyzer{
	Name:       "metrictypevshelp",
	Doc:        "extract Prometheus metric options with their Help text",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "Prometheus metric Help that describes a different kind of value than the metric type records",
		Standard:    true,
		Analyzer:    Analyzer,
		NewSettings: func() any { return &Settings{} },
		New: func(s any) (sdk.Rule, error) {
			t, err := sdk.Threshold(s.(*Settings).Threshold, 0.8)
			if err != nil {
				return nil, err
			}
			return &rule{threshold: t}, nil
		},
	})
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	var out []*sdk.Candidate
	ins.Preorder([]ast.Node{(*ast.CompositeLit)(nil)}, func(n ast.Node) {
		lit := n.(*ast.CompositeLit)
		kind, ok := metricKind(pass.TypesInfo.TypeOf(lit))
		if !ok {
			return
		}
		var help, name string
		helpKnown := false
		for _, el := range lit.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			v, isConst := constString(pass.TypesInfo, kv.Value)
			switch key.Name {
			case "Help":
				help, helpKnown = v, isConst
			case "Name":
				name = v
			}
		}
		c := &sdk.Candidate{
			Pos:     pass.Fset.Position(lit.Pos()),
			Subject: name,
			Local:   map[string]string{"kind": kind, "help": help},
		}
		switch {
		case !hasHelp(lit):
			// No Help at all is promlint's business.
			return
		case !helpKnown:
			c.Unsupported = "Help is built at run time"
			out = append(out, c)
			return
		case strings.TrimSpace(help) == "":
			return
		}
		c.Payload.AddProse("help", help)
		out = append(out, c)
	})
	return out, nil
}

func hasHelp(lit *ast.CompositeLit) bool {
	for _, el := range lit.Elts {
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Help" {
				return true
			}
		}
	}
	return false
}

// metricKind resolves the literal's type by object identity, so a renamed import or a local alias of
// the prometheus package still matches and an unrelated type named CounterOpts does not.
func metricKind(t types.Type) (string, bool) {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return "", false
	}
	obj := named.Obj()
	if obj.Pkg() == nil || obj.Pkg().Path() != promPkg {
		return "", false
	}
	k, ok := optsKind[obj.Name()]
	return k, ok
}

func constString(info *types.Info, e ast.Expr) (string, bool) {
	tv, ok := info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "kind",
		Kind: sdk.Choice,
		Text: "What kind of value does the metric help text `help` describe?",
		Options: []sdk.Option{
			{Key: "total", Description: "A running total since start that only goes up: number of X performed, total bytes, total seconds spent."},
			{Key: "current", Description: "A value that can go up and down, or a state: current number of X, in progress, size now, info, health."},
			{Key: "distribution", Description: "A distribution of observed values: latency, sizes per request, buckets."},
		},
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["kind"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the Help does not say what kind of value it is")
	}
	kind := c.Local["kind"]
	if a.Choice == expected[kind] {
		return sdk.Clean()
	}
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("answer %s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	return sdk.Report("%s Help describes %s, not %s: %q", kind, describe[a.Choice], describe[expected[kind]], clip(c.Local["help"], 80))
}

var describe = map[string]string{
	"total":        "a running total",
	"current":      "a current value",
	"distribution": "a distribution",
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
