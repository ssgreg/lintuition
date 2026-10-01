// Package config decodes and validates the lintuition configuration.
//
// The shape follows golangci-lint v2 (version, run, linters, issues, severity, output) for the keys
// lintuition supports; everything about classifiers lives in the `semantic` extension. Decoding is
// strict: an unknown key at any level, including linter and classifier settings, is an error.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Basenames are the config files looked up from the working directory upwards, in order.
var Basenames = []string{".lintuition.yml", ".lintuition.yaml"}

// Config is the whole configuration.
type Config struct {
	Version  string   `yaml:"version"`
	Run      Run      `yaml:"run"`
	Linters  Linters  `yaml:"linters"`
	Issues   Issues   `yaml:"issues"`
	Severity Severity `yaml:"severity"`
	Output   Output   `yaml:"output"`
	Semantic Semantic `yaml:"semantic"`

	// Path is where the config was read from; empty for the defaults.
	Path string `yaml:"-"`
}

// Run is the `run` section.
type Run struct {
	Timeout   Duration `yaml:"timeout"`
	Tests     *bool    `yaml:"tests"`
	BuildTags []string `yaml:"build-tags"`
	// IssuesExitCode is the exit code when findings are reported (default 1).
	IssuesExitCode *int `yaml:"issues-exit-code"`
	// ModulesDownloadMode is passed to the go command as -mod (readonly, vendor, mod).
	ModulesDownloadMode string `yaml:"modules-download-mode"`
	// RelativePathMode is how report paths are made relative: cfg, wd, gomod, gitroot.
	RelativePathMode string `yaml:"relative-path-mode"`
}

// Linters is the `linters` section.
type Linters struct {
	// Default is the base set: standard, all, none or fast.
	Default    string               `yaml:"default"`
	Enable     []string             `yaml:"enable"`
	Disable    []string             `yaml:"disable"`
	Settings   map[string]yaml.Node `yaml:"settings"`
	Exclusions Exclusions           `yaml:"exclusions"`
}

// Exclusions is `linters.exclusions`.
type Exclusions struct {
	// Generated is lax, strict or disable.
	Generated string          `yaml:"generated"`
	Paths     []string        `yaml:"paths"`
	Rules     []ExclusionRule `yaml:"rules"`
}

// ExclusionRule drops issues matching all of its set fields.
type ExclusionRule struct {
	Path       string   `yaml:"path"`
	PathExcept string   `yaml:"path-except"`
	Text       string   `yaml:"text"`
	Source     string   `yaml:"source"`
	Linters    []string `yaml:"linters"`
}

// Issues is the `issues` section.
type Issues struct {
	MaxIssuesPerLinter *int  `yaml:"max-issues-per-linter"`
	MaxSameIssues      *int  `yaml:"max-same-issues"`
	UniqByLine         *bool `yaml:"uniq-by-line"`
}

// Severity is the `severity` section.
type Severity struct {
	Default string         `yaml:"default"`
	Rules   []SeverityRule `yaml:"rules"`
}

// SeverityRule sets the severity of issues matching all of its set fields.
type SeverityRule struct {
	Severity string   `yaml:"severity"`
	Path     string   `yaml:"path"`
	Text     string   `yaml:"text"`
	Linters  []string `yaml:"linters"`
}

// Output is the `output` section.
type Output struct {
	Formats Formats `yaml:"formats"`
	// ShowStats prints the run summary to stderr (default true). Problems that make a run
	// incomplete are printed whatever it says.
	ShowStats *bool `yaml:"show-stats"`
}

// Formats is `output.formats`: each set format writes to its own path (stdout, stderr or a file).
type Formats struct {
	Text *TextFormat `yaml:"text"`
	JSON *PathFormat `yaml:"json"`
}

// TextFormat is `output.formats.text`.
type TextFormat struct {
	Path             string `yaml:"path"`
	PrintLinterName  *bool  `yaml:"print-linter-name"`
	PrintIssuedLines *bool  `yaml:"print-issued-lines"`
}

// PathFormat is a format with only a destination.
type PathFormat struct {
	Path string `yaml:"path"`
}

// Semantic is the lintuition extension: classifier, payload policy, budgets.
type Semantic struct {
	// Classifier is the name of the backend to use.
	Classifier  string               `yaml:"classifier"`
	Classifiers map[string]yaml.Node `yaml:"classifiers"`
	// Payload is what may leave the machine: facts (structural facts only), prose (the default: facts
	// plus person-written text such as comments and metric Help) or source (adds Go source text).
	Payload string `yaml:"payload"`
	// LocalOnly refuses any backend that sends requests off the machine.
	LocalOnly bool   `yaml:"local-only"`
	Budget    Budget `yaml:"budget"`
	// Concurrency is the number of requests in flight.
	Concurrency int `yaml:"concurrency"`
	// Votes is the number of independent samples per candidate (odd, 1 to 9; default 1). A strict
	// majority must agree, or the candidate abstains. Each sample is a request.
	Votes int   `yaml:"votes"`
	Cache Cache `yaml:"cache"`
}

// Cache configures the answer cache. Only backends that declare an identity are cached.
type Cache struct {
	Disabled bool `yaml:"disabled"`
	// Dir defaults to the user cache directory.
	Dir string `yaml:"dir"`
	// TTL bounds the age of a cached answer (default 168h); a moving model alias needs one.
	TTL Duration `yaml:"ttl"`
}

// Budget caps one run. A run that hits a cap is incomplete.
type Budget struct {
	MaxRequests int     `yaml:"max-requests"`
	MaxCostUSD  float64 `yaml:"max-cost-usd"`
}

// Payload policies, from the strictest.
const (
	PayloadFacts  = "facts"
	PayloadProse  = "prose"
	PayloadSource = "source"
)

// Duration decodes "5m" style durations.
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("line %d: %w", n.Line, err)
	}
	*d = Duration(v)
	return nil
}

// Find returns the first config file from dir upwards, or "" if there is none.
func Find(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		for _, b := range Basenames {
			p := filepath.Join(dir, b)
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// Load reads and strictly decodes a config file; an empty path returns the defaults.
func Load(path string) (*Config, error) {
	c := &Config{}
	if path == "" {
		c.Version = "2"
	} else {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := decodeStrict(bytes.NewReader(b), c); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		// Absolute, so cfg-relative report paths do not depend on how -c was spelled.
		if c.Path, err = filepath.Abs(path); err != nil {
			return nil, err
		}
	}
	c.applyDefaults()
	if err := c.validate(); err != nil {
		if path != "" {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return nil, err
	}
	return c, nil
}

func decodeStrict(r io.Reader, v any) error {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	// A second document would be silently ignored, and with it whatever policy it sets.
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return fmt.Errorf("line %d: only one YAML document is allowed", extra.Line)
	}
	return nil
}

// DecodeNode strictly decodes a settings node into v.
func DecodeNode(n yaml.Node, v any) error {
	// yaml.v3 has no KnownFields on Node.Decode; re-encode and decode through a strict decoder.
	b, err := yaml.Marshal(&n)
	if err != nil {
		return err
	}
	return decodeStrict(bytes.NewReader(b), v)
}

func (c *Config) applyDefaults() {
	if c.Linters.Default == "" {
		c.Linters.Default = "standard"
	}
	if c.Linters.Exclusions.Generated == "" {
		c.Linters.Exclusions.Generated = "lax"
	}
	if c.Semantic.Payload == "" {
		c.Semantic.Payload = PayloadProse
	}
	if c.Run.RelativePathMode == "" {
		c.Run.RelativePathMode = "wd"
	}
	if c.Output.Formats.Text == nil && c.Output.Formats.JSON == nil {
		c.Output.Formats.Text = &TextFormat{Path: "stdout"}
	}
	if c.Semantic.Concurrency == 0 {
		c.Semantic.Concurrency = 8
	}
	if c.Semantic.Votes == 0 {
		c.Semantic.Votes = 1
	}
	if c.Semantic.Cache.TTL == 0 {
		c.Semantic.Cache.TTL = Duration(168 * time.Hour)
	}
}

func (c *Config) validate() error {
	if c.Version != "2" {
		return fmt.Errorf(`version: must be "2", got %q`, c.Version)
	}
	switch c.Linters.Default {
	case "standard", "all", "none", "fast":
	default:
		return fmt.Errorf("linters.default: %q is not one of standard, all, none, fast", c.Linters.Default)
	}
	switch c.Linters.Exclusions.Generated {
	case "lax", "strict", "disable":
	default:
		return fmt.Errorf("linters.exclusions.generated: %q is not one of lax, strict, disable", c.Linters.Exclusions.Generated)
	}
	switch c.Semantic.Payload {
	case PayloadFacts, PayloadProse, PayloadSource:
	default:
		return fmt.Errorf("semantic.payload: %q is not one of facts, prose, source", c.Semantic.Payload)
	}
	switch c.Run.RelativePathMode {
	case "cfg", "wd", "gomod", "gitroot":
	default:
		return fmt.Errorf("run.relative-path-mode: %q is not one of cfg, wd, gomod, gitroot", c.Run.RelativePathMode)
	}
	switch c.Run.ModulesDownloadMode {
	case "", "readonly", "vendor", "mod":
	default:
		return fmt.Errorf("run.modules-download-mode: %q is not one of readonly, vendor, mod", c.Run.ModulesDownloadMode)
	}
	if c.Run.IssuesExitCode != nil && (*c.Run.IssuesExitCode < 0 || *c.Run.IssuesExitCode == 2 || *c.Run.IssuesExitCode > 255) {
		return fmt.Errorf("run.issues-exit-code: %d is reserved or out of range (2 means an incomplete run)", *c.Run.IssuesExitCode)
	}
	if c.Semantic.Budget.MaxRequests < 0 {
		return errors.New("semantic.budget.max-requests: must not be negative")
	}
	if cost := c.Semantic.Budget.MaxCostUSD; math.IsNaN(cost) || math.IsInf(cost, 0) || cost < 0 {
		return fmt.Errorf("semantic.budget.max-cost-usd: %v must be a finite number, 0 or more", cost)
	}
	if err := c.Output.Formats.checkDestinations(); err != nil {
		return err
	}
	if v := c.Semantic.Votes; v < 1 || v > 9 || v%2 == 0 {
		return fmt.Errorf("semantic.votes: %d must be odd, 1 to 9", v)
	}
	if c.Semantic.Cache.TTL < 0 {
		return errors.New("semantic.cache.ttl: must not be negative")
	}
	if c.Semantic.Concurrency < 1 {
		return errors.New("semantic.concurrency: must be at least 1")
	}
	return nil
}

// IssuesExitCode is the configured findings exit code, 1 by default.
func (c *Config) IssuesExitCode() int {
	if c.Run.IssuesExitCode != nil {
		return *c.Run.IssuesExitCode
	}
	return 1
}

// Tests reports whether test files are analysed (default true, like golangci-lint).
func (c *Config) Tests() bool {
	return c.Run.Tests == nil || *c.Run.Tests
}

// Revalidate checks the config again after command-line flags changed it.
func (c *Config) Revalidate() error { return c.validate() }

// checkDestinations refuses two formats writing to one place, and machine-readable output on stderr,
// which carries the run summary and problems.
func (f Formats) checkDestinations() error {
	seen := map[string]string{}
	add := func(format, path string) error {
		key := path
		switch path {
		case "", "stdout":
			key = "stdout"
		case "stderr":
			if format != "text" {
				return fmt.Errorf("output.formats.%s.path: stderr carries the run summary; use stdout or a file", format)
			}
		default:
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			key = abs
		}
		if other, ok := seen[key]; ok {
			return fmt.Errorf("output.formats.%s.path: %s already gets the %s output", format, path, other)
		}
		seen[key] = format
		return nil
	}
	if f.Text != nil {
		if err := add("text", f.Text.Path); err != nil {
			return err
		}
	}
	if f.JSON != nil {
		if err := add("json", f.JSON.Path); err != nil {
			return err
		}
	}
	return nil
}

// ShowStats reports whether the run summary is printed (default true).
func (c *Config) ShowStats() bool {
	return c.Output.ShowStats == nil || *c.Output.ShowStats
}
