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
