package c

import (
	"errors"
	"testing"
)

var ErrEmpty = errors.New("empty")

// Validate returns an error for an empty string.
func Validate(s string) error {
	if s == "" {
		return ErrEmpty
	}
	return nil
}

// Parse returns a number and an error.
func Parse(s string) (int, error) { return 0, nil }

// ParseFile returns a number and an error.
func ParseFile(s string) (int, error) { return 0, nil }

// Check returns a bool, not an error.
func Check(s string) bool { return s != "" }

type Store struct{}

// Save returns an error when the store is full.
func (s *Store) Save(v string) error { return nil }

// Validate is a method with the same name as the function.
func (s *Store) Validate() error { return nil }

func TestHelperRejectsEmpty(t *testing.T) { // 18 none: not in a _test.go file
	if err := Validate(""); err != nil {
		t.Fatal(err)
	}
}

// Wrap prefixes err; a nil err stays nil.
func Wrap(err error, prefix string) error { return err }

// Join joins errs.
func Join(errs ...error) error { return nil }

// Collect takes a slice of errors, which is not an error argument.
func Collect(errs []error) error { return nil }

// Restore is a method that takes an error.
func (s *Store) Restore(cause error) error { return nil }

// Prefix is a method with a variadic error after a string.
func (s *Store) Prefix(prefix string, errs ...error) error { return nil }

// Join is a method with only a variadic error.
func (s Store) Join(errs ...error) error { return nil }

// Unwrap is generic over its error type.
func Unwrap[T error](err T) error { return err }

// Consume takes a prefix and an error.
func Consume(prefix string, err error) error { return err }

// Pair returns a prefix and an error.
func Pair() (string, error) { return "p", nil }

// Count returns two numbers.
func Count() (string, int) { return "p", 0 }

// Tally takes a prefix and a number.
func Tally(prefix string, n int) error { return nil }

// ErrorAlias is another name for error.
type ErrorAlias = error

// Alias takes an error through an alias.
func Alias(err ErrorAlias) error { return err }

// WrapAVeryLongFunctionNameThatGoesOnAndOnAndOnUntilItIsLongerThanAnyOrdinaryNameWouldBe takes an error.
func WrapAVeryLongFunctionNameThatGoesOnAndOnAndOnUntilItIsLongerThanAnyOrdinaryNameWouldBe(err error) error {
	return err
}
