// Package logsensitive finds a log call whose structured fields log a secret as is:
//
//	slog.Info("user logged in", "password", req.Password)
//
// Code reads the fields through go/types (field constructors, slog's key, value pairs) and keeps
// only those whose value could hold a secret: not a literal (it is in the source already), not a
// bool, time or duration, and not the result of a redacting call (Redact, Mask, Hash, SHA256, ...).
// The classifier is asked what role the most sensitive of the remaining values serves; the key and
// the identifiers the value comes from are sent, never the value. A field value the analyzer cannot
// name, a key a fact cannot carry, or fields it cannot read at all make the call unsupported.
package logsensitive

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ssgreg/lintuition/internal/facts"
	"github.com/ssgreg/lintuition/sdk"
)

// Name is the linter name.
const Name = "log-sensitive-field"

// Settings configure the linter.
type Settings struct {
	// Threshold is the minimum probability of "secret" for a finding. The default 0.85 is the
	// prototype's; it is not yet validated on a labelled set.
	Threshold *float64 `yaml:"threshold"`
}

// Analyzer extracts log calls with structured fields that could hold a secret.
var Analyzer = &analysis.Analyzer{
	Name:       "logsensitive",
	Doc:        "extract log calls with structured fields whose values could hold a secret",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: sdk.CandidatesType,
	Run:        run,
}

func init() {
	sdk.RegisterLinter(sdk.Linter{
		Name:        Name,
		Doc:         "a structured log field that logs a secret (token, password, key material) as is",
		Standard:    true,
		Version:     "1",
		Analyzer:    Analyzer,
		NewSettings: func() any { return &Settings{} },
		New: func(s any) (sdk.Rule, error) {
			t, err := sdk.Threshold(s.(*Settings).Threshold, 0.85)
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
	ins.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		if c := candidate(pass, n.(*ast.CallExpr)); c != nil {
			c.Subject = facts.EnclosingFunc(stack) + "/" + c.Subject
			out = append(out, c)
		}
		return true
	})
	return out, nil
}

func candidate(pass *analysis.Pass, call *ast.CallExpr) *sdk.Candidate {
	lc, ok := facts.AsLogCall(pass.TypesInfo, call)
	if !ok {
		return nil
	}
	fields, partial := facts.Fields(pass.TypesInfo, lc)
	c := &sdk.Candidate{Pos: pass.Fset.Position(call.Pos()), Local: map[string]string{}}
	var keys, descs []string
	unnamed := false
	for _, f := range fields {
		if !mayHoldSecret(pass.TypesInfo, f) {
			continue
		}
		d, ok := facts.FieldFact(f)
		if !ok {
			c.Subject = "fields"
			c.Unsupported = "a field key cannot be sent as a fact"
			return c
		}
		if f.ValueDesc == "an expression" {
			unnamed = true
		}
		keys = append(keys, f.Key)
		descs = append(descs, d)
	}
	switch {
	case len(keys) == 0 && partial:
		c.Subject = "fields"
		c.Unsupported = "the log fields are not readable"
		return c
	case len(keys) == 0:
		return nil
	case unnamed:
		c.Subject = strings.Join(keys, ",")
		c.Unsupported = "a field value is an expression the analyzer cannot name"
		return c
	}
	c.Subject = strings.Join(keys, ",")
	c.Local["keys"] = strings.Join(keys, ", ")
	c.Payload.Fact("fields", descs)
	if lc.MessageKnown {
		c.Payload.AddProse("message", lc.Message)
	}
	return c
}

// mayHoldSecret reports whether a field's value could be a secret worth asking about. A literal is
// in the source already; a bool, a time or a duration cannot carry one; a redacting call's result
// is not the secret it was made from.
func mayHoldSecret(info *types.Info, f facts.LogField) bool {
	if f.Literal {
		return false
	}
	switch t := types.Unalias(info.TypeOf(f.Value)).(type) {
	case *types.Basic:
		if t.Info()&(types.IsBoolean|types.IsFloat|types.IsComplex) != 0 {
			return false
		}
	case *types.Named:
		if p := t.Obj().Pkg(); p != nil && p.Path() == "time" && (t.Obj().Name() == "Time" || t.Obj().Name() == "Duration") {
			return false
		}
	}
	if call, ok := ast.Unparen(f.Value).(*ast.CallExpr); ok {
		if fn := facts.Callee(info, call); fn != nil && redacts(fn.Name()) {
			return false
		}
	}
	return true
}

var redactPrefixes = []string{"redact", "mask", "hash", "hmac", "obfusc", "scrub", "sanitiz", "censor", "fingerprint", "encrypt", "digest", "anonymi", "elide"}

var redactWords = map[string]bool{"md5": true, "sha1": true, "sha256": true, "sha512": true, "sha": true}

// redacts reports whether a function's name says it redacts, masks or hashes its argument:
// Redact, MaskToken, HashPassword, SHA256, (*url.URL).Redacted. It is a name heuristic over the
// resolved callee: a redactor under another name is not recognised and its result is asked about.
func redacts(name string) bool {
	for _, w := range facts.Words(name) {
		if redactWords[w] {
			return true
		}
		for _, p := range redactPrefixes {
			if strings.HasPrefix(w, p) {
				return true
			}
		}
	}
	return false
}

type rule struct{ threshold float64 }

func (r *rule) Questions(*sdk.Candidate) []sdk.Question {
	return []sdk.Question{{
		ID:   "role",
		Kind: sdk.Choice,
		Text: "What role does the most sensitive value in `fields` serve? A key named after a secret whose value is only its name, id, type or timestamp is metadata, not a secret.",
		Options: []sdk.Option{
			{Key: "secret", Description: "The value itself is an authentication or recovery secret: token, password, key material, credential."},
			{Key: "personal", Description: "Personal information: e-mail, name, address, phone."},
			{Key: "public", Description: "A public identifier or metadata, including names and ids of secrets."},
		},
	}}
}

func (r *rule) Decide(c *sdk.Candidate, answers map[string]sdk.Answer) sdk.Decision {
	a := answers["role"]
	if a.Choice == sdk.Unclear {
		return sdk.Abstain("the fields do not let the classifier tell what the values are")
	}
	p, ok := a.Probability(a.Choice)
	if !ok {
		return sdk.Abstain("the classifier gave no probability for its answer")
	}
	if p < r.threshold {
		return sdk.Abstain(fmt.Sprintf("%s at %.2f is below the threshold %.2f", a.Choice, p, r.threshold))
	}
	if a.Choice != "secret" {
		return sdk.Clean()
	}
	keys := c.Local["keys"]
	if strings.Contains(keys, ", ") {
		return sdk.Report("log field value looks like a secret: one of %s", keys)
	}
	return sdk.Report("log field value looks like a secret: %s", keys)
}
