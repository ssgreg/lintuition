// Package assert is a stand-in for testify's assert with the shapes the analyzer reads.
package assert

type TestingT interface {
	Errorf(format string, args ...any)
}

func NoError(t TestingT, err error, msgs ...any) bool          { return true }
func Error(t TestingT, err error, msgs ...any) bool            { return true }
func Equal(t TestingT, expected, actual any, msgs ...any) bool { return true }

type Assertions struct{}

func New(t TestingT) *Assertions { return &Assertions{} }

func (a *Assertions) NoError(err error, msgs ...any) bool { return true }
func (a *Assertions) Error(err error, msgs ...any) bool   { return true }
