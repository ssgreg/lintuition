// Package zerolog is a stand-in for github.com/rs/zerolog with the shapes the analyzer reads.
package zerolog

type Logger struct{}

func (l *Logger) Error() *Event { return nil }

type Event struct{}

func (e *Event) Err(err error) *Event { return e }
func (e *Event) Msg(msg string)       {}
