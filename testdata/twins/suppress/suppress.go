// Package suppress holds twins for suppression-rationale.
package suppress

import "os"

// Head reads the first bytes of a file.
func Head(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	// Defect: errcheck reports the unchecked Close; the reason talks about the file's size.
	defer f.Close() //nolint:errcheck // the file is small, so reading it whole is fine // want `nolint rationale is about something other than what errcheck reports`
	buf := make([]byte, 64)
	n, _ := f.Read(buf)
	return buf[:n]
}

// HeadFixed is the fixed twin.
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

// OpcodeFixed is the fixed twin.
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
func Cleanup(f *os.File) {
	f.Close() //nolint:errcheck // the buffer is small // errors from this best-effort cleanup are deliberately ignored
}
