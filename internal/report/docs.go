package report

import (
	"sort"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// builtins are the linters package builtin registered. Each has a section in docs/linters.md
// under a `### <name>` heading, so the name is the anchor. A linter registered by anyone else, a
// plugin that took a built-in's name in a binary without builtin included, gets no link.
var builtins = map[string]bool{}

// MarkBuiltin records that package builtin registered the linter; only builtin calls it, from init.
// With builtin imported a plugin cannot take the name, since registering it twice panics.
func MarkBuiltin(linter string) {
	builtins[linter] = true
}

// Builtins returns the marked linters sorted by name.
func Builtins() []string {
	out := make([]string, 0, len(builtins))
	for name := range builtins {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// DocURL is the address of a built-in linter's section in docs/linters.md, at the ref that matches
// the build version; for any other linter it is empty.
func DocURL(version, linter string) string {
	if !builtins[linter] {
		return ""
	}
	return "https://github.com/ssgreg/lintuition/blob/" + docRef(version) + "/docs/linters.md#" + linter
}

// docRef is the git ref whose docs describe this build: a release version is its own tag. Anything
// else is main: no version or "(devel)", a pseudo-version, a +dirty build and a GoReleaser snapshot,
// none of which is a tag.
func docRef(version string) string {
	if !semver.IsValid(version) || semver.Canonical(version) != version || module.IsPseudoVersion(version) ||
		strings.Contains(semver.Prerelease(version), "SNAPSHOT") {
		return "main"
	}
	return version
}

// docURL is the DocURL of the run's linter by that name.
func (r Run) docURL(linter string) string {
	for _, l := range r.Linters {
		if l.Name == linter {
			return l.DocURL
		}
	}
	return ""
}
