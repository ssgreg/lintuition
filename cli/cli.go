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
	"runtime"
	"runtime/debug"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/ssgreg/lintuition/internal/config"
	"github.com/ssgreg/lintuition/internal/engine"
	"github.com/ssgreg/lintuition/internal/report"
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
	root.AddCommand(runCmd(&code), lintersCmd(), classifiersCmd(), configCmd(), versionCmd())
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
		textPath    string
		jsonPath    string
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
			if fl.Changed("output.text.path") || fl.Changed("output.json.path") {
				c.Output.Formats = config.Formats{}
				if textPath != "" {
					c.Output.Formats.Text = &config.TextFormat{Path: textPath}
				}
				if jsonPath != "" {
					c.Output.Formats.JSON = &config.PathFormat{Path: jsonPath}
				}
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
			res, err := engine.Run(ctx, engine.Options{Config: c, Patterns: args, DryRun: dryRun, Log: cmd.ErrOrStderr()})
			if err != nil {
				return err
			}
			if err := report.Write(c, res.Issues, res.Run, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return err
			}
			summary(cmd.ErrOrStderr(), res, dryRun)
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
	f.StringVar(&textPath, "output.text.path", "", "text output: stdout, stderr or a file path")
	f.StringVar(&jsonPath, "output.json.path", "", "JSON output: stdout, stderr or a file path")
	f.StringVar(&payload, "payload", "", "what may leave the machine: facts, prose, source")
	f.StringVar(&classifier, "classifier", "", "classifier backend to use")
	f.IntVar(&maxRequests, "max-requests", 0, "cap on classifier requests; reaching it makes the run incomplete")
	return cmd
}

func summary(w io.Writer, res *engine.Result, dryRun bool) {
	var cand, asked, abst, skip, fail, planned int
	for _, l := range res.Run.Linters {
		cand += l.Candidates
		asked += l.Asked
		abst += l.Abstained
		skip += l.Skipped
		fail += l.Failed
		planned += l.Planned
	}
	if dryRun {
		fmt.Fprintf(w, "dry run: %d packages, %d candidates: %d requests planned, %d skipped, %d failed",
			res.Run.Stats.Packages, cand, planned, skip, fail)
	} else {
		fmt.Fprintf(w, "%d issue(s). %d packages, %d candidates: %d asked, %d abstained, %d skipped, %d failed. %d requests",
			len(res.Issues), res.Run.Stats.Packages, cand, asked, abst, skip, fail, res.Run.Stats.Requests)
	}
	if res.Run.Stats.CostUSD > 0 {
		fmt.Fprintf(w, ", $%.4f", res.Run.Stats.CostUSD)
	}
	fmt.Fprintln(w)
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
			}{Version: Version, Go: runtime.Version()}
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
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print as JSON")
	return cmd
}
