// Package custom builds a lintuition binary with plugin linters and classifiers compiled in, the way
// golangci-lint's module plugin system does: a manifest lists Go modules, a generated main imports
// them next to the built-ins, and `go build` makes the binary. Nothing is loaded at run time.
//
// Plugins are trusted code: they run with the user's rights, in-process, and see every candidate.
package custom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/module"
	"gopkg.in/yaml.v3"
)

// ManifestName is the default manifest file.
const ManifestName = ".custom-lintuition.yml"

// Manifest is .custom-lintuition.yml.
type Manifest struct {
	// Version is the lintuition module version to build on (v0.1.0). Path, instead, builds on a
	// local lintuition checkout.
	Version string `yaml:"version"`
	Path    string `yaml:"path"`
	// Name of the binary (default custom-lintuition) and the directory it goes to (default .).
	Name        string   `yaml:"name"`
	Destination string   `yaml:"destination"`
	Plugins     []Plugin `yaml:"plugins"`

	dir string // the manifest's directory, for relative paths
}

// Plugin is one module to compile in.
type Plugin struct {
	Module string `yaml:"module"`
	// Import is the package to import when it is not the module root.
	Import string `yaml:"import"`
	// Version from the module proxy, or Path to a local checkout; exactly one.
	Version string `yaml:"version"`
	Path    string `yaml:"path"`
}

var (
	modRE  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._~/-]*$`)
	verRE  = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.+-]+)?$`)
	nameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
)

// Load reads and validates a manifest strictly.
func Load(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	m := &Manifest{}
	if err := dec.Decode(m); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: only one YAML document is allowed", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	m.dir = filepath.Dir(abs)
	if err := m.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

func (m *Manifest) validate() error {
	if (m.Version == "") == (m.Path == "") {
		return errors.New("set exactly one of version and path")
	}
	if m.Version != "" && !verRE.MatchString(m.Version) {
		return fmt.Errorf("version %q: want a semantic version such as v0.1.0", m.Version)
	}
	if m.Name == "" {
		m.Name = "custom-lintuition"
	}
	if !nameRE.MatchString(m.Name) {
		return fmt.Errorf("name %q: letters, digits, dot, dash and underscore only", m.Name)
	}
	if m.Destination == "" {
		m.Destination = "."
	}
	if len(m.Plugins) == 0 {
		return errors.New("plugins: list at least one")
	}
	seen := map[string]bool{}
	for i, p := range m.Plugins {
		if !modRE.MatchString(p.Module) || strings.Contains(p.Module, "..") {
			return fmt.Errorf("plugins[%d].module %q is not a module path", i, p.Module)
		}
		if seen[p.Module] {
			return fmt.Errorf("plugins[%d]: module %s is listed twice", i, p.Module)
		}
		seen[p.Module] = true
		if (p.Version == "") == (p.Path == "") {
			return fmt.Errorf("plugins[%d] (%s): set exactly one of version and path", i, p.Module)
		}
		if p.Version != "" && !verRE.MatchString(p.Version) {
			return fmt.Errorf("plugins[%d].version %q: want a semantic version", i, p.Version)
		}
		if err := module.CheckPath(p.Module); err != nil {
			return fmt.Errorf("plugins[%d].module: %v", i, err)
		}
		if p.Version != "" {
			if err := module.Check(p.Module, p.Version); err != nil {
				return fmt.Errorf("plugins[%d].version: %v", i, err)
			}
		}
		if p.Import != "" && p.Import != p.Module && !strings.HasPrefix(p.Import, p.Module+"/") {
			return fmt.Errorf("plugins[%d].import %q is not inside module %s", i, p.Import, p.Module)
		}
	}
	return nil
}

func (m *Manifest) abs(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(m.dir, p)
}

const lintuitionModule = "github.com/ssgreg/lintuition"

// Generate writes the build module into dir: go.mod and main.go.
func (m *Manifest) Generate(dir string) error {
	var mod strings.Builder
	mod.WriteString("module custom-lintuition\n\ngo 1.26\n\nrequire (\n")
	ver := m.Version
	if ver == "" {
		ver = localVersion(lintuitionModule)
	}
	fmt.Fprintf(&mod, "\t%s %s\n", lintuitionModule, ver)
	for _, p := range m.Plugins {
		v := p.Version
		if v == "" {
			v = localVersion(p.Module)
		}
		fmt.Fprintf(&mod, "\t%s %s\n", p.Module, v)
	}
	mod.WriteString(")\n")
	if m.Path != "" {
		fmt.Fprintf(&mod, "\nreplace %s => %s\n", lintuitionModule, strconv.Quote(m.abs(m.Path)))
	}
	for _, p := range m.Plugins {
		if p.Path != "" {
			fmt.Fprintf(&mod, "replace %s => %s\n", p.Module, strconv.Quote(m.abs(p.Path)))
		}
	}
	var imports []string
	for _, p := range m.Plugins {
		imp := p.Import
		if imp == "" {
			imp = p.Module
		}
		imports = append(imports, imp)
	}
	sort.Strings(imports)
	var main strings.Builder
	main.WriteString("// Code generated by lintuition custom. DO NOT EDIT.\n\npackage main\n\nimport (\n\t\"os\"\n\n")
	main.WriteString("\t_ \"github.com/ssgreg/lintuition/builtin\"\n\t\"github.com/ssgreg/lintuition/cli\"\n\n")
	for _, imp := range imports {
		fmt.Fprintf(&main, "\t_ %s\n", strconv.Quote(imp))
	}
	main.WriteString(")\n\nfunc main() {\n")
	main.WriteString("\tcli.Plugins = plugins\n")
	main.WriteString("\tos.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))\n}\n")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "main.go"), []byte(main.String()), 0o644)
}

// localVersion is the placeholder version required for a module replaced by a local path: a
// pseudo-version of the module's major version, so a /v2 module gets a v2 placeholder.
func localVersion(path string) string {
	_, pathMajor, _ := module.SplitPathVersion(path)
	major := module.PathMajorPrefix(pathMajor)
	if major == "" {
		major = "v0"
	}
	return module.PseudoVersion(major, "", time.Time{}, "000000000000")
}

// pluginsFile writes the plugins the binary reports, from the versions Go actually selected.
func pluginsFile(dir string, lines []string) error {
	var b strings.Builder
	b.WriteString("// Code generated by lintuition custom. DO NOT EDIT.\n\npackage main\n\nvar plugins = []string{\n")
	for _, l := range lines {
		fmt.Fprintf(&b, "\t%s,\n", strconv.Quote(l))
	}
	b.WriteString("}\n")
	return os.WriteFile(filepath.Join(dir, "plugins.go"), []byte(b.String()), 0o644)
}

type listedModule struct {
	Path    string
	Version string
	Replace *struct {
		Path    string
		Version string
	}
}

// resolved checks the selected module versions against the manifest: a pinned version must be the
// one built (minimal version selection may raise it when another module needs more), and a local
// path must be what replaces the module. It returns the lines the binary reports.
func (m *Manifest) resolved(mods map[string]listedModule) ([]string, error) {
	check := func(mod, version, path string) (string, error) {
		got, ok := mods[mod]
		if !ok {
			return "", fmt.Errorf("%s is not in the build", mod)
		}
		if path != "" {
			if got.Replace == nil || filepath.Clean(got.Replace.Path) != filepath.Clean(m.abs(path)) {
				return "", fmt.Errorf("%s is not built from %s", mod, m.abs(path))
			}
			return mod + " local " + m.abs(path), nil
		}
		if got.Replace != nil {
			return "", fmt.Errorf("%s is replaced; pin it with path instead", mod)
		}
		if got.Version != version {
			return "", fmt.Errorf("%s: the build selects %s, not the pinned %s (another module requires it); pin %s or change the plugin set", mod, got.Version, version, got.Version)
		}
		return mod + " " + version, nil
	}
	if _, err := check(lintuitionModule, m.Version, m.Path); err != nil {
		return nil, err
	}
	var lines []string
	for _, p := range m.Plugins {
		l, err := check(p.Module, p.Version, p.Path)
		if err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}
	return lines, nil
}

// Build generates the module in a temporary directory, resolves it and builds the binary. It returns
// the binary's path. log gets the go command's output.
func (m *Manifest) Build(ctx context.Context, log io.Writer) (string, error) {
	dir, err := os.MkdirTemp("", "lintuition-custom-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	if err := m.Generate(dir); err != nil {
		return "", err
	}
	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		cmd.Stdout, cmd.Stderr = log, log
		cmd.Env = append(os.Environ(), "GOWORK=off")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
		}
		return nil
	}
	out := filepath.Join(m.abs(m.Destination), m.Name)
	if st, err := os.Stat(out); err == nil && st.IsDir() {
		return "", fmt.Errorf("%s is a directory; choose another name or destination", out)
	}
	if err := pluginsFile(dir, nil); err != nil {
		return "", err
	}
	if err := run("mod", "tidy"); err != nil {
		return "", err
	}
	var listed bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", "all")
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, &listed, log
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go list -m: %w", err)
	}
	mods := map[string]listedModule{}
	dec := json.NewDecoder(&listed)
	for dec.More() {
		var lm listedModule
		if err := dec.Decode(&lm); err != nil {
			return "", fmt.Errorf("go list -m: %w", err)
		}
		mods[lm.Path] = lm
	}
	lines, err := m.resolved(mods)
	if err != nil {
		return "", err
	}
	if err := pluginsFile(dir, lines); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	// Build next to the target and move it in only after it starts: a duplicate linter or classifier
	// name panics at registration, which compiling cannot show.
	staged := out + ".new"
	if err := run("build", "-trimpath", "-o", staged, "."); err != nil {
		return "", err
	}
	if st, err := os.Stat(staged); err != nil || !st.Mode().IsRegular() {
		os.RemoveAll(staged)
		return "", fmt.Errorf("go build did not produce %s", staged)
	}
	var startErr bytes.Buffer
	probe := exec.CommandContext(ctx, staged, "version")
	probe.Stdout, probe.Stderr = io.Discard, &startErr
	if err := probe.Run(); err != nil {
		os.Remove(staged)
		return "", fmt.Errorf("the built binary does not start (%v): %s", err, strings.TrimSpace(firstLine(startErr.String())))
	}
	if err := os.Rename(staged, out); err != nil {
		os.Remove(staged)
		return "", err
	}
	return out, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
