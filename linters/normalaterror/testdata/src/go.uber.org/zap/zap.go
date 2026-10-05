// Package zap is a stand-in for go.uber.org/zap with the shapes the analyzer reads.
package zap

type Field struct{}

// Error is a field constructor whose key is its name.
func Error(err error) Field { return Field{} }

// String is a field constructor.
func String(k, v string) Field { return Field{} }

type Logger struct{}

func (l *Logger) Error(msg string, fs ...Field) {}
func (l *Logger) Fatal(msg string, fs ...Field) {}
func (l *Logger) Sugar() *SugaredLogger         { return nil }

type SugaredLogger struct{}

func (s *SugaredLogger) Errorw(msg string, kv ...any)      {}
func (s *SugaredLogger) Errorf(format string, args ...any) {}
