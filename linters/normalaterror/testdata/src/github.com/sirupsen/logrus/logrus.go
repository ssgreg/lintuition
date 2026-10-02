// Package logrus is a stand-in for github.com/sirupsen/logrus with the shapes the analyzer reads.
package logrus

type Entry struct{}

func WithError(err error) *Entry { return nil }

func (e *Entry) Error(args ...any)                 {}
func (e *Entry) Errorf(format string, args ...any) {}

func Errorf(format string, args ...any) {}
