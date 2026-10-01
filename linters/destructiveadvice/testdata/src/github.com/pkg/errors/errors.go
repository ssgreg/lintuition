// Package errors is a stand-in for github.com/pkg/errors.
package errors

func New(message string) error             { return nil }
func Wrap(err error, message string) error { return nil }
