package testname

import (
	"errors"
	"testing"
)

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

type fakeStore struct{ err error }

func (f fakeStore) Put(string, string) error { return f.err }

type fakeCache struct{ err error }

func (f fakeCache) Write(string, string) error { return f.err }

var errDiskFull = errors.New("disk full")

// Defect: the store is set up to fail and the name says Save fails, but the check requires success.

func TestSaveFailsWhenTheStoreRefuses(t *testing.T) { // want `test name expects an error from Save, but the test fails when Save returns one`
	s := &Saver{Store: fakeStore{err: errDiskFull}, Cache: fakeCache{}}
	if err := s.Save("k", "v"); err != nil {
		t.Fatal(err)
	}
}

// Fixed twin.

func TestSaveFailsWhenTheStoreRefusesFixed(t *testing.T) {
	s := &Saver{Store: fakeStore{err: errDiskFull}, Cache: fakeCache{}}
	if err := s.Save("k", "v"); err == nil {
		t.Fatal("no error")
	}
}

// Defect, the reverse: the name and the doc say Save gets past a cache failure, but the check
// demands an error.

// TestSaveSurvivesACacheFailure checks that a value is saved even when the cache cannot keep a copy.
func TestSaveSurvivesACacheFailure(t *testing.T) { // want `test name expects Save to succeed, but the test fails when Save returns no error`
	s := &Saver{Store: fakeStore{}, Cache: fakeCache{err: errDiskFull}}
	if err := s.Save("k", "v"); err == nil {
		t.Fatal("no error")
	}
}

// Fixed twin.

// TestSaveSurvivesACacheFailureFixed checks that a value is saved even when the cache cannot keep a
// copy.
func TestSaveSurvivesACacheFailureFixed(t *testing.T) {
	s := &Saver{Store: fakeStore{}, Cache: fakeCache{err: errDiskFull}}
	if err := s.Save("k", "v"); err != nil {
		t.Fatal(err)
	}
}

// Negative: the name describes the failure the test arranges in the cache, and the doc says it is
// harmless; Save must succeed.

// TestSaveCacheWriteFailure covers a cache that cannot be written. The cache only speeds up reads,
// so this is not an error for the caller.
func TestSaveCacheWriteFailure(t *testing.T) {
	s := &Saver{Store: fakeStore{}, Cache: fakeCache{err: errDiskFull}}
	if err := s.Save("k", "v"); err != nil {
		t.Fatal(err)
	}
}

// Negative: the error in the name is the input; Wrap takes an error and a nil one stays nil.

func TestWrapNilError(t *testing.T) {
	var err error
	if got := Wrap(err, "load"); got != nil {
		t.Fatal(got)
	}
}
