package a

import (
	"errors"
	"fmt"

	pkgerrors "github.com/pkg/errors"
)

var ErrNotFound = errors.New("not found") // none: a sentinel declaration is not a return

const gone = "user is gone"

type myErr struct{}

func (myErr) Error() string { return "x" }

func c1() error {
	return errors.New("user not found") // 1 candidate: errors.New
}

func c2(id int) (int, error) {
	return 0, fmt.Errorf("user %d already exists", id) // 2 candidate: second result, formatted
}

func c3(err error) error {
	return fmt.Errorf("load user: %w", err) // 3 none: wraps
}

func c4(err error) error {
	return pkgerrors.Wrap(err, "load user") // 4 none: Wrap
}

func c5(msg string) error {
	return errors.New(msg) // 5 unsupported: the message is built at run time
}

func c6() error {
	return ErrNotFound // 6 none: returns the sentinel
}

func c7() error {
	return myErr{} // 7 none: a typed error
}

func c8() error {
	return pkgerrors.New(gone) // 8 candidate: pkg/errors through a constant
}

func c9() error {
	err := errors.New("permission denied") // 9 none: not returned on the spot
	return err
}

func c10() func() error {
	return func() error {
		return errors.New("timeout waiting for lock") // 10 candidate: inside a closure
	}
}

func c11() error {
	return errors.New("") // 11 unsupported: empty message
}

func c12(err error) error {
	return fmt.Errorf("load user: %v", err) // 12 candidate: %v does not wrap
}

func c13() error {
	return errors.New("100%w done") // 13 candidate: errors.New does not wrap with %w
}

func c14(f string) error {
	return fmt.Errorf(f) // 14 unsupported: the format is built at run time
}

type errorsT struct{}

func (errorsT) New(string) error { return nil }

func c15(e errorsT) error {
	return e.New("user not found") // 15 none: a method named New is not an error constructor
}
