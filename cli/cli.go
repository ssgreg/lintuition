// Package cli is the lintuition command line. It is public so a custom binary, built with extra
// plugin linters and classifiers, runs the same commands.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/ssgreg/lintuition/internal/classify"
	"github.com/ssgreg/lintuition/internal/config"
	"github.com/ssgreg/lintuition/internal/custom"
	"github.com/ssgreg/lintuition/internal/engine"
	"github.com/ssgreg/lintuition/internal/report"
	"github.com/ssgreg/lintuition/internal/twins"
	"github.com/ssgreg/lintuition/sdk"
)

// Exit codes. ExitIncomplete is reserved: a run that could not analyse everything it was asked to
// never exits as clean or as a plain findings exit.
const (
	ExitClean      = 0
	ExitIncomplete = 2
)

// Version is set at build time with -ldflags "-X github.com/ssgreg/lintuition/cli.Version=v0.1.0".
var Version = ""

// Plugins lists the plugin modules compiled into a custom binary, "module version" each; the main
// that `lintuition custom` generates fills it in.
var Plugins []string

// Main runs the CLI and returns the exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	code := ExitClean
	root := &cobra.Command{
		Use:           "lintuition",
		Short:         "System One for your Go code.",
		Long:          "lintuition checks whether comments, logs and test descriptions match what your code does.\nA classifier reads the words; Go code checks the logic.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(runCmd(&code), lintersCmd(), classifiersCmd(), configCmd(), cacheCmd(), customCmd(), evalCmd(), versionCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return ExitIncomplete
	}
	return code
}

type configFlags struct {
	path     string
	noConfig bool
}

func (f *configFlags) add(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.path, "config", "c", "", "read config from file path `PATH`")
	cmd.Flags().BoolVar(&f.noConfig, "no-config", false, "don't read a config file")
}

func (f *configFlags) load() (*config.Config, error) {
	if f.noConfig && f.path != "" {
		return nil, errors.New("--config and --no-config are mutually exclusive")
	}
	path := f.path
	if path == "" && !f.noConfig {
		var err error
		if path, err = config.Find("."); err != nil {
			return nil, err
		}
	}
	return config.Load(path)
}

func runCmd(code *int) *cobra.Command {
	var (
		cf          configFlags
		enable      []string
		disable     []string
		def         string
		timeout     time.Duration
		tests       bool
		buildTags   []string
		exitCode    int
		dryRun      bool
		preview     string
		textPath    string
		jsonPath    string
		otherPaths  = map[string]*string{}
		payload     string
		classifier  string
		maxRequests int
	)
	cmd := &cobra.Command{
		Use:   "run [packages]",
		Short: "Run the linters",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cf.load()
			if err != nil {
				return err
			}
			fl := cmd.Flags()
			// Flags override the config, as in golangci-lint.
			if fl.Changed("default") {
				c.Linters.Default = def
			}
			c.Linters.Enable = append(c.Linters.Enable, enable...)
			c.Linters.Disable = append(c.Linters.Disable, disable...)
			if fl.Changed("timeout") {
				c.Run.Timeout = config.Duration(timeout)
			}
			if fl.Changed("tests") {
				c.Run.Tests = &tests
			}
			if fl.Changed("build-tags") {
				c.Run.BuildTags = buildTags
			}
			if fl.Changed("issues-exit-code") {
				if exitCode == ExitIncomplete {
					return errors.New("--issues-exit-code 2 is reserved for an incomplete run")
				}
				c.Run.IssuesExitCode = &exitCode
			}
			changed := fl.Changed("output.text.path") || fl.Changed("output.json.path")
			for name := range otherPaths {
				changed = changed || fl.Changed("output."+name+".path")
			}
			if changed {
				// Output flags replace the configured formats, as in golangci-lint.
				f := config.Formats{}
				if textPath != "" {
					f.Text = &config.TextFormat{Path: textPath}
				}
				pf := func(p string) *config.PathFormat {
					if p == "" {
						return nil
					}
					return &config.PathFormat{Path: p}
				}
				f.JSON = pf(jsonPath)
				f.SARIF = pf(*otherPaths["sarif"])
				f.Checkstyle = pf(*otherPaths["checkstyle"])
				f.CodeClimate = pf(*otherPaths["code-climate"])
				f.JUnitXML = pf(*otherPaths["junit-xml"])
				f.GitHubActions = pf(*otherPaths["github-actions"])
				c.Output.Formats = f
			}
			if fl.Changed("payload") {
				c.Semantic.Payload = payload
			}
			if fl.Changed("classifier") {
				c.Semantic.Classifier = classifier
			}
			if fl.Changed("max-requests") {
				c.Semantic.Budget.MaxRequests = maxRequests
			}
			if err := c.Revalidate(); err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			opts := engine.Options{Config: c, Patterns: args, DryRun: dryRun, Log: cmd.ErrOrStderr()}
			var previewFile *os.File
			if preview != "" {
				if !dryRun {
					return errors.New("--preview needs --dry-run")
				}
				if err := c.Output.Formats.CheckExtra("--preview", preview); err != nil {
					return err
				}
				if previewFile, err = os.OpenFile(preview, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600); err != nil {
					return err
				}
				defer previewFile.Close()
				opts.Preview = previewFile
			}
			res, err := engine.Run(ctx, opts)
			if previewFile != nil {
				if cerr := previewFile.Close(); cerr != nil && err == nil {
					err = fmt.Errorf("preview: %w", cerr)
				}
			}
			if err != nil {
				return err
			}
			if err := report.Write(c, res.Issues, res.Run, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return err
			}
			summary(cmd.ErrOrStderr(), res, dryRun, c.ShowStats())
			switch {
			case res.Run.Incomplete:
				*code = ExitIncomplete
			case len(res.Issues) > 0:
				*code = c.IssuesExitCode()
			}
			return nil
		},
	}
	cf.add(cmd)
	f := cmd.Flags()
	f.StringSliceVarP(&enable, "enable", "E", nil, "enable linters by name")
	f.StringSliceVarP(&disable, "disable", "D", nil, "disable linters by name")
	f.StringVar(&def, "default", "", "default set of linters: standard, all, none, fast")
	f.DurationVar(&timeout, "timeout", 0, "timeout for the whole run")
	f.BoolVar(&tests, "tests", true, "analyze tests (*_test.go)")
	f.StringSliceVar(&buildTags, "build-tags", nil, "build tags")
	f.IntVar(&exitCode, "issues-exit-code", 1, "exit code when issues are found")
	f.BoolVar(&dryRun, "dry-run", false, "extract and plan requests, send none")
	f.StringVar(&preview, "preview", "", "with --dry-run: write every planned request, as it would be sent, to this file (JSON lines)")
	f.StringVar(&textPath, "output.text.path", "", "text output: stdout, stderr or a file path")
	f.StringVar(&jsonPath, "output.json.path", "", "JSON output: stdout or a file path")
	for _, name := range []string{"sarif", "checkstyle", "code-climate", "junit-xml", "github-actions"} {
		p := new(string)
		otherPaths[name] = p
		f.StringVar(p, "output."+name+".path", "", name+" output: stdout or a file path")
	}
	f.StringVar(&payload, "payload", "", "what may leave the machine: facts, prose, source")
	f.StringVar(&classifier, "classifier", "", "classifier backend to use")
	f.IntVar(&maxRequests, "max-requests", 0, "cap on classifier requests; reaching it makes the run incomplete")
	return cmd
}

func summary(w io.Writer, res *engine.Result, dryRun, stats bool) {
	var cand, asked, abst, skip, fail, planned, unsup, notAsked int
	for _, l := range res.Run.Linters {
		cand += l.Candidates
		asked += l.Asked
		abst += l.Abstained
		skip += l.Skipped
		fail += l.Failed
		planned += l.Planned
		unsup += l.Unsupported
		notAsked += l.NotAsked
	}
	switch {
	case !stats && !dryRun:
	case dryRun:
		fmt.Fprintf(w, "dry run: %d packages, %d candidates: %d to ask (%d requests at %d vote(s) each), %d cached, %d skipped, %d unsupported, %d failed",
			res.Run.Stats.Packages, cand, planned, res.Run.Stats.Requests, res.Run.Stats.Votes, res.Run.Stats.CacheHits, skip, unsup, fail)
		fmt.Fprintln(w)
	default:
		fmt.Fprintf(w, "%d issue(s). %d packages, %d candidates: %d asked, %d abstained, %d skipped, %d unsupported, %d failed", len(res.Issues), res.Run.Stats.Packages, cand, asked, abst, skip, unsup, fail)
		if notAsked > 0 {
			fmt.Fprintf(w, ", %d not asked (budget)", notAsked)
		}
		fmt.Fprintf(w, ". %d requests", res.Run.Stats.Requests)
		if res.Run.Stats.CacheHits > 0 {
			fmt.Fprintf(w, ", %d from cache", res.Run.Stats.CacheHits)
		}
		if res.Run.Stats.CostUSD > 0 {
			fmt.Fprintf(w, ", ~$%.6f at the backend's price assumption", res.Run.Stats.CostUSD)
		}
		fmt.Fprintln(w)
	}
	if res.Run.Incomplete {
		fmt.Fprintln(w, "INCOMPLETE run:")
		for _, p := range res.Run.Problems {
			fmt.Fprintf(w, "  %s\n", p)
		}
	}
}

func lintersCmd() *cobra.Command {
	var cf configFlags
	cmd := &cobra.Command{
		Use:   "linters",
		Short: "List the linters and whether the config enables them",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := cf.load()
			if err != nil {
				return err
			}
			enabled, err := c.Select(sdk.Linters())
			if err != nil {
				return err
			}
			on := map[string]bool{}
			for _, e := range enabled {
				on[e.Linter.Name] = true
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			for _, group := range []bool{true, false} {
				if group {
					fmt.Fprintln(tw, "Enabled by your configuration linters:")
				} else {
					fmt.Fprintln(tw, "\nDisabled by your configuration linters:")
				}
				for _, l := range sdk.Linters() {
					if on[l.Name] == group {
						std := ""
						if l.Standard {
							std = " [standard]"
						}
						fmt.Fprintf(tw, "%s:\t%s%s\n", l.Name, l.Doc, std)
					}
				}
			}
			return tw.Flush()
		},
	}
	cf.add(cmd)
	return cmd
}

func classifiersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "classifiers",
		Short: "List the classifier backends",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			for _, f := range sdk.Classifiers() {
				fmt.Fprintf(tw, "%s:\t%s\n", f.Name, f.Doc)
			}
			return tw.Flush()
		},
	}
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Inspect the configuration"}
	var cf configFlags
	verify := &cobra.Command{
		Use:   "verify",
		Short: "Check the config: syntax, unknown keys, unknown linters and classifiers, settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := cf.load()
			if err != nil {
				return err
			}
			if _, err := c.Select(sdk.Linters()); err != nil {
				return err
			}
			if _, _, err := c.Classifier(sdk.Classifiers()); err != nil {
				return err
			}
			if _, err := report.NewProcessor(c); err != nil {
				return err
			}
			where := c.Path
			if where == "" {
				where = "defaults (no config file)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: ok\n", where)
			return nil
		},
	}
	cf.add(verify)
	path := &cobra.Command{
		Use:   "path",
		Short: "Print the config file that would be used",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := config.Find(".")
			if err != nil {
				return err
			}
			if p == "" {
				return errors.New("no config file found")
			}
			fmt.Fprintln(cmd.OutOrStdout(), p)
			return nil
		},
	}
	cmd.AddCommand(verify, path)
	return cmd
}

func versionCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version, toolchain and registered linters and classifiers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v := struct {
				Version     string   `json:"version"`
				Commit      string   `json:"commit,omitempty"`
				Go          string   `json:"go"`
				Linters     []string `json:"linters"`
				Classifiers []string `json:"classifiers"`
				Plugins     []string `json:"plugins,omitempty"`
			}{Version: Version, Go: runtime.Version(), Plugins: Plugins}
			if bi, ok := debug.ReadBuildInfo(); ok {
				if v.Version == "" {
					v.Version = bi.Main.Version
				}
				for _, s := range bi.Settings {
					if s.Key == "vcs.revision" {
						v.Commit = s.Value
					}
				}
			}
			for _, l := range sdk.Linters() {
				v.Linters = append(v.Linters, l.Name)
			}
			for _, c := range sdk.Classifiers() {
				v.Classifiers = append(v.Classifiers, c.Name)
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(v)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "lintuition %s built with %s\nlinters: %s\nclassifiers: %s\n",
				v.Version, v.Go, strings.Join(v.Linters, ", "), strings.Join(v.Classifiers, ", "))
			if len(v.Plugins) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "plugins: %s\n", strings.Join(v.Plugins, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print as JSON")
	return cmd
}

func cacheCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "cache", Short: "Inspect or clean the answer cache"}
	var cf configFlags
	dir := func() (string, error) {
		c, err := cf.load()
		if err != nil {
			return "", err
		}
		if c.Semantic.Cache.Dir != "" {
			return c.Semantic.Cache.Dir, nil
		}
		return classify.DefaultCacheDir()
	}
	path := &cobra.Command{
		Use:   "path",
		Short: "Print the cache directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := dir()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), d)
			return nil
		},
	}
	var expired bool
	clean := &cobra.Command{
		Use:   "clean",
		Short: "Remove cached answer records (and nothing else in the directory)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := dir()
			if err != nil {
				return err
			}
			c, err := cf.load()
			if err != nil {
				return err
			}
			n, err := (&classify.Cache{Dir: d, TTL: time.Duration(c.Semantic.Cache.TTL)}).Clean(expired)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %d cached answer record(s) from %s\n", n, d)
			return nil
		},
	}
	clean.Flags().BoolVar(&expired, "expired", false, "remove only records older than semantic.cache.ttl")
	cf.add(path)
	cf.add(clean)
	cmd.AddCommand(path, clean)
	return cmd
}

func customCmd() *cobra.Command {
	var manifest string
	cmd := &cobra.Command{
		Use:   "custom",
		Short: "Build a lintuition binary with the plugin linters and classifiers in " + custom.ManifestName,
		Long: "Build a lintuition binary with plugins compiled in, like golangci-lint custom.\n" +
			"Plugins are trusted code: they run in-process with your rights and see every candidate.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, err := custom.Load(manifest)
			if err != nil {
				return err
			}
			out, err := m.Build(cmd.Context(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&manifest, "manifest", "m", custom.ManifestName, "the build manifest")
	return cmd
}

func evalCmd() *cobra.Command {
	var (
		cfgPath  string
		runs     int
		jsonPath string
		maxCost  float64
	)
	cmd := &cobra.Command{
		Use:   "eval DIR [packages]",
		Short: "Run the linters over marked twins several times and score them",
		Long: "Run the linters over defect / fixed twins marked with // want comments, with the configured\n" +
			"classifier, several fresh times (the answer cache is off), and report per marked case how often\n" +
			"it was caught, and every finding no mark accounts for. Cases and runs are counted apart.\n" +
			"This spends classifier requests; set semantic.budget.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runs < 1 || runs > 20 {
				return errors.New("--runs must be 1 to 20")
			}
			dir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			patterns := args[1:]
			if len(patterns) == 0 {
				patterns = []string{"./..."}
			}
			type caseStat struct {
				Key, Pattern string
				Caught       int
			}
			type alarm struct {
				Key, Linter, Text string
				Runs              int
			}
			var (
				cases    []*caseStat
				alarms   = map[string]*alarm{}
				problems []string
				requests int
				cost     float64
				abstain  int
			)
			for r := 0; r < runs; r++ {
				c, err := twins.Load(dir, cfgPath)
				if err != nil {
					return err
				}
				// Fresh samples every run: a replay is not a new observation.
				c.Semantic.Cache.Disabled = true
				if maxCost > 0 {
					left := maxCost - cost
					if left <= 0 {
						problems = append(problems, fmt.Sprintf("--max-cost-usd %.4f spent after %d of %d runs", maxCost, r, runs))
						break
					}
					// The whole evaluation shares one cap: each run gets what the earlier ones left.
					if c.Semantic.Budget.MaxCostUSD == 0 || c.Semantic.Budget.MaxCostUSD > left {
						c.Semantic.Budget.MaxCostUSD = left
					}
				}
				out, err := twins.Run(cmd.Context(), c, dir, patterns)
				if err != nil {
					return err
				}
				if out.Result.Run.Incomplete {
					for _, p := range out.Result.Run.Problems {
						problems = append(problems, fmt.Sprintf("run %d: %s", r+1, p))
					}
				}
				requests += out.Result.Run.Stats.Requests
				cost += out.Result.Run.Stats.CostUSD
				for _, l := range out.Result.Run.Linters {
					abstain += l.Abstained
				}
				if r == 0 {
					for _, w := range out.Wants {
						cases = append(cases, &caseStat{Key: w.Key, Pattern: w.Pattern.String()})
					}
				}
				if len(out.Wants) != len(cases) {
					return fmt.Errorf("run %d saw %d marked cases, the first run %d", r+1, len(out.Wants), len(cases))
				}
				for i := range out.Wants {
					if out.Caught[i] {
						cases[i].Caught++
					}
				}
				for _, is := range out.Unexpected {
					k := fmt.Sprintf("%s:%d %s", is.Pos.Filename, is.Pos.Line, is.FromLinter)
					if alarms[k] == nil {
						alarms[k] = &alarm{Key: fmt.Sprintf("%s:%d", is.Pos.Filename, is.Pos.Line), Linter: is.FromLinter, Text: is.Text}
					}
					alarms[k].Runs++
				}
			}
			w := cmd.OutOrStdout()
			tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "case\tcaught\twant\n")
			always, never := 0, 0
			for _, c := range cases {
				fmt.Fprintf(tw, "%s\t%d/%d\t%s\n", c.Key, c.Caught, runs, c.Pattern)
				switch c.Caught {
				case runs:
					always++
				case 0:
					never++
				}
			}
			tw.Flush()
			keys := make([]string, 0, len(alarms))
			for k := range alarms {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				fmt.Fprintln(w, "\nfindings no mark accounts for:")
				for _, k := range keys {
					a := alarms[k]
					fmt.Fprintf(w, "  %s %d/%d: %s (%s)\n", a.Key, a.Runs, runs, a.Text, a.Linter)
				}
			}
			fmt.Fprintf(w, "\n%d marked cases x %d runs: caught in every run %d, in some %d, in none %d; %d unaccounted finding(s); %d abstentions; %d requests, ~$%.6f\n",
				len(cases), runs, always, len(cases)-always-never, never, len(keys), abstain, requests, cost)
			if jsonPath != "" {
				b, _ := json.MarshalIndent(map[string]any{"runs": runs, "cases": cases, "unaccounted": alarms, "requests": requests, "cost_usd": cost, "abstentions": abstain, "problems": problems}, "", "  ")
				if err := os.WriteFile(jsonPath, b, 0o600); err != nil {
					return err
				}
			}
			if len(problems) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "INCOMPLETE eval:\n  %s\n", strings.Join(problems, "\n  "))
				return errIncomplete
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&cfgPath, "config", "c", "", "config file (default: the one found from DIR upwards)")
	cmd.Flags().IntVar(&runs, "runs", 3, "fresh runs")
	cmd.Flags().StringVar(&jsonPath, "json", "", "also write the scores as JSON to this file")
	cmd.Flags().Float64Var(&maxCost, "max-cost-usd", 0, "cap on the cost of all runs together; reaching it stops the evaluation, which then counts as incomplete")
	return cmd
}

var errIncomplete = errors.New("the evaluation is incomplete")
