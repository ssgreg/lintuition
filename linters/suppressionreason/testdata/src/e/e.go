// Package e holds extraction cases. Each case is a function cN; its explanation is the function's
// doc comment, which is not a directive and never becomes part of a reason.
package e

import (
	"os"
	"os/exec"
)

var f *os.File

// A reason that answers errcheck.

func c1() {
	defer f.Close() //nolint:errcheck // a close error on a read-only file loses nothing
}

// A gosec directive on its own line with a reason that names no rule: the general description.

func c2(path string) {
	//nolint:gosec // the path comes from the operator's own config
	g, _ := os.Open(path)
	_ = g
}

// No reason: not a candidate.

func c3() {
	_ = f.Close() //nolint:errcheck
}

// A bare directive suppresses nothing.

func c4() {
	_ = f.Close() //nolint // a bare directive suppresses nothing
}

// nolint:all suppresses every risk; there is no one risk to compare with.

func c5() {
	_ = f.Close() //nolint:all // nothing here matters
}

// Several linters.

func c6() {
	_ = f.Close() //nolint:errcheck,gosec // both are fine here
}

// A linter with no description.

func c7() {
	_ = f.Close() //nolint:mylinter // it is fine
}

// A lintuition linter: its Doc is the description.

func c8() {
	_ = f.Close() //nolint:suppression-rationale // checked by hand
}

// A plugin linter whose Doc cannot be sent as a fact.

func c9() {
	_ = f.Close() //nolint:quoted-doc // fine
}

// A second // is part of the reason.

func c10() {
	_ = f.Close() //nolint:lll // a URL cannot be wrapped // see the style guide
}

// Not a directive: the text before nolint.

func c11() {
	_ = f.Close() // see nolint:errcheck // not a directive
}

// Not a directive: a block comment.

func c12() {
	_ = f.Close() /*nolint:errcheck // a block comment*/
}

// A space after // still makes a directive.

func c13() {
	_ = f.Close() // nolint:errcheck // spaced directive
}

// A reason in two clauses is read whole.

func c14() {
	_ = f.Close() //nolint:errcheck // the buffer is small // errors from this best-effort cleanup are deliberately ignored
}

// A gosec rule named first: the rule's own description.

func c15(path string) {
	b, _ := os.ReadFile(path) //nolint:gosec // G304: a constant file name under the build directory
	_ = b
}

// A gosec rule named anywhere in the reason.

func c16(name string) {
	_ = exec.Command(name) //nolint:gosec // the binary name is a constant (G204)
}

// The same rule named twice is one rule.

func c17(path string) {
	b, _ := os.ReadFile(path) //nolint:gosec // G304 and again G304: a file name this program generated
	_ = b
}

// Two rules: which one the reason answers is not established.

func c18(path string) {
	b, _ := os.ReadFile(path) //nolint:gosec // G304/G703: the file sits next to the binary
	_ = b
}

// A rule the table does not have.

func c19(path string) {
	b, _ := os.ReadFile(path) //nolint:gosec // G999: a rule from a newer release
	_ = b
}

// A reassigned rule ID: the table leaves it out, since the reason may mean the old check.

func c20(path string) {
	g, _ := os.Create(path) //nolint:gosec // G307: the file holds no secrets
	_ = g
}

// Not rule IDs: a lower-case g, four digits, an ID glued into a word.

func c21(path string) {
	b, _ := os.ReadFile(path) //nolint:gosec // g304 does not apply, G3040 is no rule, xG304 is a word, GOFILE is an environment variable
	_ = b
}

// A staticcheck SA check.

func c22() {
	_ = f.Close() //nolint:staticcheck // SA1019: the replacement needs a newer Go than we support
}

// A staticcheck S, ST and QF check, one per directive.

func c23() {
	_ = f.Close() //nolint:staticcheck // S1000: the select is kept for the timeout case added next
	_ = f.Close() //nolint:staticcheck // ST1003: the name follows the wire format
	_ = f.Close() //nolint:staticcheck // QF1001: the condition reads as the spec states it
}

// A gosec ID under staticcheck is not a staticcheck rule: the general description.

func c24() {
	_ = f.Close() //nolint:staticcheck // G304 is not ours to fix here
}

// Five digits are no staticcheck ID.

func c25() {
	_ = f.Close() //nolint:staticcheck // SA10190 is a ticket number
}

// A revive rule before a colon, as revive prints it.

func c26() {
	_ = f.Close() //nolint:revive // var-naming: the name mirrors the protocol field
}

// A revive rule after revive's own disable syntax, with and without the revive: prefix.

func c27() {
	_ = f.Close() //nolint:revive // disable-line:unused-receiver
	_ = f.Close() //nolint:revive // revive:disable-next-line:unused-parameter the hook signature is fixed
}

// A leading word with a colon that names no revive rule: the general description.

func c28() {
	_ = f.Close() //nolint:revive // note: the exported name is part of the public API
}

// A revive rule name that is not first is an ordinary word.

func c29() {
	_ = f.Close() //nolint:revive // the range loop needs the index
}

// revive's disable syntax naming a rule the table does not have.

func c30() {
	_ = f.Close() //nolint:revive // disable-line:no-such-rule
}

// A linter without a rule table: an ID in its reason is only text.

func c31() {
	_ = f.Close() //nolint:errcheck // G104 already covers it; a close error on a read-only file loses nothing
}
