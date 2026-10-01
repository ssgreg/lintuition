// Package require is a stand-in for testify's require with the shapes the analyzer reads.
package require

type TestingT interface {
	Errorf(format string, args ...any)
	FailNow()
}

func NoError(t TestingT, err error, msgs ...any)         {}
func Error(t TestingT, err error, msgs ...any)           {}
func ErrorIs(t TestingT, err, target error, msgs ...any) {}
