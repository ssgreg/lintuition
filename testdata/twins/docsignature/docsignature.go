// Package docsignature holds twins for doc-vs-signature.
package docsignature

import "context"

type Buffer struct{ data []byte }

// Defect: the count was dropped from the signature, the doc still promises it.
// Sync flushes the buffer and returns the number of bytes written.
func (b *Buffer) Sync() { b.data = b.data[:0] } // want `doc of Sync says it returns a result, but Sync returns nothing`

// Fixed twin: the count is back.
// SyncN flushes the buffer and returns the number of bytes written.
func (b *Buffer) SyncN() int { n := len(b.data); b.data = b.data[:0]; return n }

// Defect: the function stopped returning an error and became a predicate; the doc did not follow.
// Validate returns an error if the name is empty.
func Validate(name string) bool { return name != "" } // want `doc of Validate says it returns an error, but Validate has no error result`

// Fixed twin: the doc says what the predicate reports.
// ValidName reports whether the name is non-empty.
func ValidName(name string) bool { return name != "" }

// Negative: returning when ctx is done is about timing, not a result.
// Run blocks and returns when ctx is done.
func Run(ctx context.Context) { <-ctx.Done() }

// Negative: logging an error is not returning one.
// Count logs an error when the counter overflows.
func Count(n int) int { return n + 1 }

// Negative: weak support abstains.
// Drain returns once the queue is empty.
func Drain(q chan int) {
	for range q {
	}
}

// Negative: the error is inside the result; not asked.
// Watch returns a channel that receives an error when the watch fails.
func Watch() chan error { return nil }

type Queue struct{ items []int }

// Defect: a predicate turned into a command; the doc still describes a bool.
// Empty reports whether the queue has no items.
func (q *Queue) Empty() { q.items = nil } // want `doc of Empty says it returns a result, but Empty returns nothing`

type Box struct{ v int }

// Defect: the old value is no longer returned.
// Swap stores v and returns the old value.
func (b *Box) Swap(v int) { b.v = v } // want `doc of Swap says it returns a result, but Swap returns nothing`

type Config struct{ Name string }

// Defect: the error result was dropped when the parser learned to default.
// Parse parses s and returns an error on malformed input.
func Parse(s string) Config { return Config{Name: s} } // want `doc of Parse says it returns an error, but Parse has no error result`

type Conn struct{}

type Pool struct{ closed bool }

// Defect: a sentinel the function can no longer return.
// Get takes a connection from the pool and returns ErrClosed if the pool is closed.
func (p *Pool) Get() *Conn { return &Conn{} } // want `doc of Get says it returns an error, but Get has no error result`

// Negative: returning into a pool is not a result.
// Put returns c into the pool.
func (p *Pool) Put(c *Conn) {}

// Negative: a string that describes an error is not an error result.
// Describe formats the error for display.
func Describe(err error) string { return err.Error() }

type Field struct{ Value any }

// Negative: a value that carries an error is not an error result.
// ErrorField returns a Field that carries an error under the key "error".
func ErrorField(err error) Field { return Field{Value: err} }
