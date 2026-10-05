// Package suppress holds twins for suppression-rationale.
//
// Every explanation is a separate comment, a blank line above the code it explains, so it never
// reaches the classifier: only a directive's reason is sent.
package suppress

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Defect: errcheck reports the unchecked Close; the reason talks about the file's size.

// Head reads the first bytes of a file.
func Head(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close() //nolint:errcheck // the file is small, so reading it whole is fine // want `nolint rationale is about something other than what errcheck reports`
	buf := make([]byte, 64)
	n, _ := f.Read(buf)
	return buf[:n]
}

// Fixed twin: the reason says why the lost close error does not matter.

// HeadFixed reads the first bytes of a file.
func HeadFixed(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close() //nolint:errcheck // a close error on a file opened read-only loses no data
	buf := make([]byte, 64)
	n, _ := f.Read(buf)
	return buf[:n]
}

// Defect: funlen reports the function's length; the reason is about who calls it.

// Opcode names an opcode.
//
//nolint:funlen // only the tests call this function // want `nolint rationale is about something other than what funlen reports`
func Opcode(op byte) string {
	switch op {
	case 0:
		return "nop"
	case 1:
		return "load"
	}
	return "?"
}

// Fixed twin: the reason is about the function's length.

// OpcodeFixed names an opcode.
//
//nolint:funlen // one flat switch over every opcode; splitting it would hide the table
func OpcodeFixed(op byte) string {
	switch op {
	case 0:
		return "nop"
	case 1:
		return "load"
	}
	return "?"
}

// Negative: an lll directive whose reason is about the long line.

const docURL = "https://example.com/a/very/long/path/that/goes/on/and/on/so/that/the/line/is/longer/than/the/limit" //nolint:lll // a URL in a string cannot be wrapped

// Negative: two linters are unsupported, never asked.

var _ = os.Remove //nolint:errcheck,gosec // both fine here

// Negative: a reason whose second clause, after another //, answers errcheck is read whole.

// Cleanup closes f.
func Cleanup(f *os.File) {
	f.Close() //nolint:errcheck // the buffer is small // errors from this best-effort cleanup are deliberately ignored
}

// Defect: the reason names gosec's G304 (file path from taint input) and then talks about the
// file's size.

func loadManifest(path string) []byte {
	b, _ := os.ReadFile(path) //nolint:gosec // G304: the file is only a few bytes long // want `nolint rationale is about something other than gosec G304, the rule it names`
	return b
}

// Fixed twin: the reason says where the path comes from.

func loadManifestFixed(dir string) []byte {
	b, _ := os.ReadFile(filepath.Join(dir, "manifest.json")) //nolint:gosec // G304: built from the build directory and a constant file name
	return b
}

// Defect: G204 (command execution) answered with speed.

func runHook(name string) *exec.Cmd {
	return exec.Command(name) //nolint:gosec // G204: this runs on every request and has to stay fast // want `nolint rationale is about something other than gosec G204, the rule it names`
}

// Negative: the same rule answered without naming it, through the general gosec description.

func runFormatter() *exec.Cmd {
	return exec.Command("gofmt", "-l", ".") //nolint:gosec // the command line is a fixed list of literals
}

// Defect: G306 (file permissions) answered with the code's age.

func writeStamp(path string, b []byte) {
	_ = os.WriteFile(path, b, 0o644) //nolint:gosec // G306: legacy code from the first release // want `nolint rationale is about something other than gosec G306, the rule it names`
}

// Fixed twin: the reason is about who may read the file.

func writeStampFixed(path string, b []byte) {
	_ = os.WriteFile(path, b, 0o644) //nolint:gosec // G306: the stamp is public build metadata, readable by anyone on purpose
}

// Defect: no rule named, and the reason only says the code is old.

func readSpool(path string) []byte {
	b, _ := os.ReadFile(path) //nolint:gosec // inherited from the old service, a rewrite is planned // want `nolint rationale is about something other than what gosec reports`
	return b
}

// Defect: revive's exported rule answered with speed.

//nolint:revive // exported: the loop over the cache is cheap // want `nolint rationale is about something other than revive exported, the rule it names`
type CacheStats struct{ Hits int }

// Negative: revive's var-naming rule, answered.

var userIdKey = "user_id" //nolint:revive // var-naming: the name matches the field of the wire protocol

// Negative: staticcheck's SA1019 (a deprecated API), answered.

func seekStart(f *os.File) {
	_, _ = f.Seek(0, os.SEEK_SET) //nolint:staticcheck // SA1019: the replacement constant is missing in the oldest Go release we build with
}

// Negative: a reason that names two gosec rules is unsupported, never asked.

func readTwo(path string) []byte {
	b, _ := os.ReadFile(path) //nolint:gosec // G304/G703: the file sits next to the binary
	return b
}

// Negative: a rule the table does not describe is unsupported, never asked.

func readNew(path string) []byte {
	b, _ := os.ReadFile(path) //nolint:gosec // G999: a check from a newer release
	return b
}

// Negative: an ID inside a file name is not a citation, so the reason is read against gosec as a
// whole, and it answers the file mode. Known false positive: with no rule ID the classifier sees
// only gosec's general description, does not connect "readable by every test" with file
// permissions, and Jev reported this reason in 3 of 3 live runs. It stays unmarked, because no
// finding is the right answer; the scripted answer below is that answer, so the fake suite passes
// and a live eval shows the miss as an unaccounted finding.

func writeGolden(path string, b []byte) {
	_ = os.WriteFile(path, b, 0o644) //nolint:gosec // the G304.golden fixture is meant to be readable by every test
}

// Negative, a control for the case above: the same file mode, a reason that names the
// permissions.

func writeGoldenExplicit(path string, b []byte) {
	_ = os.WriteFile(path, b, 0o644) //nolint:gosec // the G304.golden fixture gets world-readable file permissions so every test can read it
}
