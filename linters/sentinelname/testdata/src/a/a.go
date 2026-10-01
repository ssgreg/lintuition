package a

import (
	"errors"
	"fmt"

	pkgerrors "github.com/pkg/errors"
)

const notFound = "not found"

var msg = "built at run time"

type myErr struct{}

func (myErr) Error() string { return "x" }

var ErrNotFound = errors.New("permission denied") // 1 candidate: errors.New

var errClosed = fmt.Errorf("connection closed") // 2 candidate: unexported, fmt.Errorf without %w

var ErrHTTPTimeout = pkgerrors.New("request timed out") // 3 candidate: pkg/errors, words split

var ErrMissing = errors.New(notFound) // 4 candidate: through a constant

var ErrDynamic = errors.New(msg) // 5 unsupported: the message is built at run time

var ErrWrapped = fmt.Errorf("%w: user", ErrNotFound) // 6 unsupported: wraps another error

var ErrPercent = errors.New("100%w done") // 7 candidate: errors.New does not wrap with %w

var Err = errors.New("anything") // 8 none: the name says nothing beyond Err

var Errata = errors.New("typo list") // 9 none: not the Err convention

var ErrCustom error = myErr{} // 10 none: not an error constructor

var ErrAlias = ErrNotFound // 11 none: not a constructor

var NotAnErr = errors.New("not found") // 12 none: not named as a sentinel

var ErrCount = 3 // 13 none: an int named Err is not an error

var (
	ErrA, ErrB = errors.New("a failed"), errors.New("b failed") // 14 candidate: two in one spec, bound by position
)

var ErrEmpty = errors.New("") // 15 unsupported: empty message

var ErrPkgWrapped = pkgerrors.Wrap(ErrNotFound, "lookup") // 16 unsupported: Wrap

func f() {
	var ErrLocal = errors.New("local") // 17 none: not package-level
	_ = ErrLocal
}
