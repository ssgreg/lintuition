package a

import (
	"context"
	"errors"
)

type Buffer struct{ n int }

// Sync writes the buffer to disk and returns the number of bytes written.
func (b *Buffer) Sync() {} // 1 result: no results, a return word

// Valid reports whether the buffer is non-empty.
func Valid(b *Buffer) {} // 2 result: "reports whether" is a return word

// Run blocks and returns when ctx is done.
func Run(ctx context.Context) {} // 3 result: asked; returning is not a result, the classifier says so

// Close closes the buffer.
func Close(b *Buffer) {} // 4 none: no return word

func Bare() {} // 5 none: no doc

//go:noinline
func Directive() {} // 6 none: the doc is only a directive

// GetSize returns the size; see GetSizeLimit.
func GetSize() {} // 7 result: own name masked, a longer name kept

// Validate returns an error if the name is empty.
func Validate(name string) bool { return name != "" } // 8 error: results, none an error

// Find returns ErrNotFound when the key is missing.
func Find(key string) *Buffer { return nil } // 9 error: an ErrX word

// Len is the number of bytes.
func Len(b *Buffer) int { return b.n } // 10 none: results, no error word

// Load returns an error if the file is missing.
func Load(p string) (int, error) { return 0, nil } // 11 none: already has an error result

type MyErr struct{}

func (*MyErr) Error() string { return "x" }

// Check returns an error when the input is bad.
func Check() *MyErr { return nil } // 12 none: *MyErr implements error

type Coded interface {
	error
	Code() int
}

// Parse returns a coded error on bad input.
func Parse() Coded { return nil } // 13 none: an interface embedding error

// Watch returns a channel that receives an error when the watch fails.
func Watch() chan error { return nil } // 14 unsupported: chan error

// Lazy returns a function that returns an error.
func Lazy() func() error { return nil } // 15 unsupported: func() error

type Result struct {
	N   int
	Err error
}

// Do runs the job; the error is in Result.Err.
func Do() Result { return Result{} } // 16 unsupported: a struct with an error field

// First returns the first element or an error value of type T.
func First[T any](xs []T) T { var z T; return z } // 17 unsupported: a type parameter

// All returns every error collected.
func All() []error { return nil } // 18 unsupported: []error

// Value returns an error description.
func Value() MyErr { return MyErr{} } // 19 error: the value MyErr does not implement error, *MyErr does

type Sizer interface{ Size() int }

type Store interface {
	// Flush writes pending data and returns the count.
	Flush() // 20 result: an interface method with no results

	// Get returns the value or an error.
	Get(k string) string // 21 error: an interface method with results, none an error

	// Put returns an error on a full store.
	Put(k, v string) error // 22 none: has an error result

	Sizer // 23 none: embedded
}

// Open opens the file.
func Open() (n int, err error) { return 0, nil } // 24 none: named results with an error

type E = error

// Alias returns an error.
func Alias() E { return nil } // 25 none: an alias of error

// Count logs an error when the counter overflows.
func Count() int { return 0 } // 26 error: asked; logging is not returning, the classifier says so

// Group uses errgroup to fan out.
func Group() int { return 0 } // 27 none: errgroup is not an error word

// Errorf builds the message.
func Errorf() string { return "" } // 28 none: Errorf is not an error word, no match on its own

type FlagError struct{ flag string }

// Error implements error.
func (e *FlagError) Error() string { return e.flag } // 34 none: the Error method of an error type

// Flag returns the flag for which the error occurred.
func (e FlagError) Flag() string { return e.flag } // 35 none: a method of an error type, value receiver

// Dynamic returns an error on failure, otherwise the decoded value.
func Dynamic(fail bool) any { // 40 unsupported: an any result may hold an error
	if fail {
		return errors.New("bad")
	}
	return 12
}

type AnyAlias = any

// DynamicAlias returns an error on failure, otherwise the decoded value.
func DynamicAlias(fail bool) AnyAlias { return Dynamic(fail) } // 41 unsupported: an alias of any

type Reader interface{ Read() int }

// Open2 returns a reader, or an error if the file is missing.
func Open2() Reader { return nil } // 42 unsupported: a value behind Reader may also implement error

// Deep returns nested slices of error values.
func Deep() [][][][][]error { return nil } // 43 unsupported: no depth limit

type Leaf struct{ Err error }
type Inner struct{ Long *Leaf }
type LongFirst struct {
	Deep  *Inner
	Short *Leaf
}
type ShortFirst struct {
	Short *Leaf
	Deep  *Inner
}

// OrderedLong returns a holder for an error.
func OrderedLong() LongFirst { return LongFirst{} } // 44 unsupported: the long path first

// OrderedShort returns a holder for an error.
func OrderedShort() ShortFirst { return ShortFirst{} } // 45 unsupported: the short path first

type Holder[T any] = struct{ Value T }

// AliasGeneric returns a holder for an error.
func AliasGeneric() Holder[error] { return Holder[error]{} } // 46 unsupported: a generic alias holding error

// Constrained returns an error.
func Constrained[T error]() T { var v T; return v } // 47 unsupported: a type parameter constrained by error

type Generic[T any] interface {
	// Read returns an error.
	Read() T // 48 unsupported: an interface method returning a type parameter
}

type Paren (interface {
	// FlushParen returns the number of bytes written.
	FlushParen() // 49 result: a parenthesised interface
})

type ParenAlias = (interface {
	// FlushAlias returns the number of bytes written.
	FlushAlias() // 50 result: a parenthesised interface alias
})

type ErrorView interface {
	error
	// Code returns the code identifying the error.
	Code() int // 51 none: a method of an error interface
}

type ConcreteError struct{}

func (ConcreteError) Error() string { return "error" }

// Code returns the code identifying the error.
func (ConcreteError) Code() int { return 1 } // 52 none: the concrete twin of 51

func LocalTypes() {
	type Local interface {
		// FlushLocal returns the number of bytes written.
		FlushLocal() // 53 result: a local interface
	}
	var _ Local
}

type Cycle struct {
	Next *Cycle
	Err  error
}

// Cyclic returns a list containing an error.
func Cyclic() *Cycle { return nil } // 54 unsupported: a recursive type with an error field

// ErrorMap returns a mapping of error values to counts.
func ErrorMap() map[error]int { return nil } // 55 unsupported: an error map key

// ErrorArray returns an array of error values.
func ErrorArray() [2]error { return [2]error{} } // 56 unsupported: an array of errors

type ConcreteAlias = *ConcreteError

// ConcreteAliasResult returns an error.
func ConcreteAliasResult() ConcreteAlias { return nil } // 57 none: an alias of an error pointer

// CallbackInput returns an error.
func CallbackInput() func(error) { return nil } // 58 error: a callback that takes an error returns none

type NoErr struct{ N int }

type Cyc2 struct{ Next *Cyc2 }

// Chain returns the chain or an error.
func Chain() *Cyc2 { return nil } // 59 error: a recursive type without an error is walked to the end

// Empty2 returns an error on failure.
func Empty2() interface{} { return nil } // 64 unsupported: interface{} is any

// Readers returns the readers, or an error.
func Readers() []Reader { return nil } // 65 unsupported: a slice of such an interface

type Coder interface{ Code() int }

type codedError struct{}

func (codedError) Code() int     { return 1 }
func (codedError) Error() string { return "coded" }

// Classify returns an error carrying a code when the input is bad.
func Classify(bad bool) Coder { // 66 unsupported: Codex's probe, the doc is true
	if bad {
		return codedError{}
	}
	return nil
}

type Odd interface{ Error() int }

// Weird returns an error value.
func Weird() Odd { return nil } // 67 error: Error() int excludes every error

// Nested returns the errors found.
func Nested[T error]() []T { return nil } // 68 unsupported: a type parameter inside a slice

type Box2[T any] struct{ V T }

// InBox returns an error in a box.
func InBox[T any]() Box2[T] { return Box2[T]{} } // 69 unsupported: a type parameter inside a struct

type GenericHolder[T Coder] struct{ Value T }

type HolderAlias[T Coder] = GenericHolder[T]

// Held returns a holder for an error.
func Held[T Coder]() GenericHolder[T] { return GenericHolder[T]{} } // 70 unsupported: a field of a constrained type parameter

// HeldAlias returns a holder for an error.
func HeldAlias[T Coder]() HolderAlias[T] { return HolderAlias[T]{} } // 71 unsupported: through a generic alias
