package report

import (
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// BuiltinLinters are the linters with a section in docs/linters.md, under a `### <name>` heading, so
// the name is the anchor. A plugin linter is not here and its findings carry no link. A test in
// package builtin keeps the list equal to what builtin registers and to the headings.
var BuiltinLinters = []string{
	"destructive-remediation",
	"doc-vs-signature",
	"doc-vs-table",
	"enum-comment-shift",
	"error-needs-type",
	"human-unit-contradiction",
	"log-key-value-role",
	"log-sensitive-field",
	"metric-type-vs-help",
	"normal-event-at-error",
	"premature-success",
	"read-only-promise",
	"sentinel-name-vs-text",
	"severe-event-understated",
	"suppression-rationale",
	"table-case-vs-expectation",
	"test-name-vs-assertion",
}

// DocURL is the address of a built-in linter's section in docs/linters.md, at the ref that matches
// the build version; for any other linter it is empty.
func DocURL(version, linter string) string {
	for _, name := range BuiltinLinters {
		if name == linter {
			return "https://github.com/ssgreg/lintuition/blob/" + docRef(version) + "/docs/linters.md#" + linter
		}
	}
	return ""
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
