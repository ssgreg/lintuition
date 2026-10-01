// Package logf is a stand-in for github.com/ssgreg/logf with the shapes the analyzer must tell apart.
package logf

import "context"

type Field struct{}

// Error is a field constructor, not a log call.
func Error(err error) Field { return Field{} }

// String is a field constructor.
func String(k, v string) Field { return Field{} }

type Logger struct{}

func (l *Logger) Info(ctx context.Context, text string, fs ...Field)  {}
func (l *Logger) Error(ctx context.Context, text string, fs ...Field) {}
