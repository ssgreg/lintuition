package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/ssgreg/lintuition/sdk"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".lintuition.yml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadRejects(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"unknown top key":    {"version: \"2\"\nlinter: {}\n", "field linter not found"},
		"unknown nested key": {"version: \"2\"\nrun:\n  timeuot: 1m\n", "field timeuot not found"},
		"v1 exclude-rules":   {"version: \"2\"\nissues:\n  exclude-rules: []\n", "field exclude-rules not found"},
		"missing version":    {"linters: {}\n", `must be "2"`},
		"bad default":        {"version: \"2\"\nlinters:\n  default: most\n", "not one of standard"},
		"bad payload":        {"version: \"2\"\nsemantic:\n  payload: facts-only\n", "not one of facts, prose, source"},
		"reserved exit code": {"version: \"2\"\nrun:\n  issues-exit-code: 2\n", "reserved"},
		"bad duration":       {"version: \"2\"\nrun:\n  timeout: soon\n", "invalid duration"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(write(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Linters.Default != "standard" || c.Semantic.Payload != PayloadProse || c.IssuesExitCode() != 1 || !c.Tests() {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.Output.Formats.Text == nil || c.Output.Formats.Text.Path != "stdout" {
		t.Fatal("text to stdout is the default output")
	}
}

func TestFindUpwards(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, ".lintuition.yml")
	if err := os.WriteFile(p, []byte("version: \"2\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := Find(sub)
	if err != nil || got != p {
		t.Fatalf("Find = %q, %v; want %q", got, err, p)
	}
}

type testSettings struct {
	Threshold float64 `yaml:"threshold"`
}

type nopRule struct{}

func (nopRule) Questions(*sdk.Candidate) []sdk.Question                   { return nil }
func (nopRule) Decide(*sdk.Candidate, map[string]sdk.Answer) sdk.Decision { return sdk.Clean() }

func registry() []sdk.Linter {
	mk := func(name string, std bool) sdk.Linter {
		return sdk.Linter{
			Name: name, Standard: std,
			Analyzer:    &analysis.Analyzer{Name: strings.ReplaceAll(name, "-", ""), Doc: "x", ResultType: sdk.CandidatesType, Run: func(*analysis.Pass) (any, error) { return nil, nil }},
			NewSettings: func() any { return &testSettings{} },
			New:         func(any) (sdk.Rule, error) { return nopRule{}, nil },
		}
	}
	return []sdk.Linter{mk("alpha", true), mk("beta", false)}
}

func names(es []Enabled) string {
	var s []string
	for _, e := range es {
		s = append(s, e.Linter.Name)
	}
	return strings.Join(s, ",")
}

func TestSelect(t *testing.T) {
	cases := []struct{ body, want, err string }{
		{"version: \"2\"\n", "alpha", ""},
		{"version: \"2\"\nlinters:\n  default: all\n", "alpha,beta", ""},
		{"version: \"2\"\nlinters:\n  default: none\n  enable: [beta]\n", "beta", ""},
		{"version: \"2\"\nlinters:\n  default: all\n  disable: [alpha]\n", "beta", ""},
		{"version: \"2\"\nlinters:\n  enable: [gamma]\n", "", `unknown linter "gamma"`},
		{"version: \"2\"\nlinters:\n  enable: [beta]\n  disable: [beta]\n", "", "both enabled and disabled"},
		{"version: \"2\"\nlinters:\n  settings:\n    gamma: {}\n", "", `unknown linter "gamma"`},
		{"version: \"2\"\nlinters:\n  settings:\n    alpha:\n      treshold: 1\n", "", "field treshold not found"},
	}
	for _, tc := range cases {
		c, err := Load(write(t, tc.body))
		if err != nil {
			t.Fatal(err)
		}
		got, err := c.Select(registry())
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("%q: got %v, want error %q", tc.body, err, tc.err)
			}
			continue
		}
		if err != nil || names(got) != tc.want {
			t.Errorf("%q: got %q, %v; want %q", tc.body, names(got), err, tc.want)
		}
	}
}
