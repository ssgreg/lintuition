package a

import "context"

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

type Store interface {
	// Flush writes pending data and returns the count.
	Flush() // 20 result: an interface method with no results

	// Get returns the value or an error.
	Get(k string) string // 21 error: an interface method with results, none an error

	// Put returns an error on a full store.
	Put(k, v string) error // 22 none: has an error result

	Coded // 23 none: embedded
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
