package config

import (
	"fmt"
	"sort"

	"github.com/ssgreg/lintuition/sdk"
)

// Enabled is a linter selected for a run, with its rule built from settings.
type Enabled struct {
	Linter sdk.Linter
	Rule   sdk.Rule
}

// Select resolves linters.default/enable/disable against the registry and builds each rule from its
// settings. Unknown names in enable, disable or settings are errors.
func (c *Config) Select(registry []sdk.Linter) ([]Enabled, error) {
	byName := map[string]sdk.Linter{}
	for _, l := range registry {
		byName[l.Name] = l
	}
	for _, list := range [][]string{c.Linters.Enable, c.Linters.Disable} {
		for _, n := range list {
			if _, ok := byName[n]; !ok {
				return nil, fmt.Errorf("unknown linter %q (see `lintuition linters`)", n)
			}
		}
	}
	for n := range c.Linters.Settings {
		if _, ok := byName[n]; !ok {
			return nil, fmt.Errorf("linters.settings: unknown linter %q", n)
		}
	}
	on := map[string]bool{}
	for _, l := range registry {
		switch c.Linters.Default {
		case "all":
			on[l.Name] = true
		case "standard":
			on[l.Name] = l.Standard
		case "fast":
			// Every linter so far asks a classifier; fast means the ones that do not.
			on[l.Name] = false
		}
	}
	for _, n := range c.Linters.Enable {
		on[n] = true
	}
	for _, n := range c.Linters.Disable {
		if contains(c.Linters.Enable, n) {
			return nil, fmt.Errorf("linter %q is both enabled and disabled", n)
		}
		on[n] = false
	}
	var names []string
	for n, v := range on {
		if v {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	out := make([]Enabled, 0, len(names))
	for _, n := range names {
		l := byName[n]
		var settings any
		if l.NewSettings != nil {
			settings = l.NewSettings()
			if node, ok := c.Linters.Settings[n]; ok {
				if err := DecodeNode(node, settings); err != nil {
					return nil, fmt.Errorf("linters.settings.%s: %w", n, err)
				}
			}
		} else if _, ok := c.Linters.Settings[n]; ok {
			return nil, fmt.Errorf("linters.settings.%s: the linter takes no settings", n)
		}
		r, err := l.New(settings)
		if err != nil {
			return nil, fmt.Errorf("linter %s: %w", n, err)
		}
		out = append(out, Enabled{Linter: l, Rule: r})
	}
	return out, nil
}

// Classifier builds the configured classifier backend. It returns nil, nil when none is configured.
func (c *Config) Classifier(registry []sdk.ClassifierFactory) (sdk.Classifier, string, error) {
	// Validate every configured backend, not only the selected one, so a typo does not wait for the
	// day someone switches to it.
	for n, node := range c.Semantic.Classifiers {
		f, ok := factory(registry, n)
		if !ok {
			return nil, "", fmt.Errorf("semantic.classifiers: unknown classifier %q (see `lintuition classifiers`)", n)
		}
		if f.NewSettings == nil {
			return nil, "", fmt.Errorf("semantic.classifiers.%s: the classifier takes no settings", n)
		}
		if err := DecodeNode(node, f.NewSettings()); err != nil {
			return nil, "", fmt.Errorf("semantic.classifiers.%s: %w", n, err)
		}
	}
	name := c.Semantic.Classifier
	if name == "" {
		return nil, "", nil
	}
	for _, f := range registry {
		if f.Name != name {
			continue
		}
		var settings any
		if f.NewSettings != nil {
			settings = f.NewSettings()
			if node, ok := c.Semantic.Classifiers[name]; ok {
				if err := DecodeNode(node, settings); err != nil {
					return nil, "", fmt.Errorf("semantic.classifiers.%s: %w", name, err)
				}
			}
		} else if _, ok := c.Semantic.Classifiers[name]; ok {
			return nil, "", fmt.Errorf("semantic.classifiers.%s: the classifier takes no settings", name)
		}
		cl, err := f.New(settings)
		if err != nil {
			return nil, "", fmt.Errorf("classifier %s: %w", name, err)
		}
		if c.Semantic.LocalOnly && !cl.Capabilities().Local {
			return nil, "", fmt.Errorf("semantic.local-only is set, but classifier %s sends requests off the machine", name)
		}
		return cl, name, nil
	}
	return nil, "", fmt.Errorf("semantic.classifier: unknown classifier %q (see `lintuition classifiers`)", name)
}

func factory(r []sdk.ClassifierFactory, name string) (sdk.ClassifierFactory, bool) {
	for _, f := range r {
		if f.Name == name {
			return f, true
		}
	}
	return sdk.ClassifierFactory{}, false
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
