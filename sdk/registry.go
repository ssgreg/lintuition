package sdk

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu          sync.Mutex
	linters     = map[string]Linter{}
	classifiers = map[string]ClassifierFactory{}
)

// RegisterLinter adds a linter to the registry. Built-in and plugin linters both register from an init
// function; a duplicate or malformed definition panics, so a bad build fails at start.
func RegisterLinter(l Linter) {
	if err := l.Validate(); err != nil {
		panic(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := linters[l.Name]; ok {
		panic(fmt.Sprintf("lintuition: linter %q registered twice", l.Name))
	}
	linters[l.Name] = l
}

// RegisterClassifier adds a classifier backend to the registry; a duplicate name panics.
func RegisterClassifier(f ClassifierFactory) {
	if !nameRE.MatchString(f.Name) || f.New == nil {
		panic(fmt.Sprintf("lintuition: malformed classifier %q", f.Name))
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := classifiers[f.Name]; ok {
		panic(fmt.Sprintf("lintuition: classifier %q registered twice", f.Name))
	}
	classifiers[f.Name] = f
}

// Linters returns the registered linters sorted by name.
func Linters() []Linter {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Linter, 0, len(linters))
	for _, l := range linters {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Classifiers returns the registered classifier factories sorted by name.
func Classifiers() []ClassifierFactory {
	mu.Lock()
	defer mu.Unlock()
	out := make([]ClassifierFactory, 0, len(classifiers))
	for _, c := range classifiers {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
