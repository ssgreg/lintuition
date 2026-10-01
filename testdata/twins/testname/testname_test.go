package testname

import "testing"

// Defect: the test was copied from the accepting one and its check not flipped.
func TestUnquoteRejectsUnbalancedQuote(t *testing.T) { // want `test name expects an error from Unquote, but the test fails when Unquote returns one`
	if _, err := Unquote(`"abc`); err != nil {
		t.Fatal(err)
	}
}

// Fixed twin.
func TestUnquoteRejectsUnbalancedQuoteFixed(t *testing.T) {
	if _, err := Unquote(`"abc`); err == nil {
		t.Fatal("no error")
	}
}

// Defect: the name says it succeeds, the test demands an error.
func TestUnquoteAcceptsQuotedWord(t *testing.T) { // want `test name expects Unquote to succeed, but the test fails when Unquote returns no error`
	_, err := Unquote(`"abc"`)
	if err == nil {
		t.Fatal("no error")
	}
}

// Fixed twin.
func TestUnquoteAcceptsQuotedWordFixed(t *testing.T) {
	_, err := Unquote(`"abc"`)
	if err != nil {
		t.Fatal(err)
	}
}

// Negative: the name only describes the input.
func TestUnquoteEmptyString(t *testing.T) {
	if _, err := Unquote(""); err == nil {
		t.Fatal("no error")
	}
}
