// Package docsignature holds twins for doc-vs-signature.
//
// Every explanation is a separate comment, a blank line above the doc, so it never reaches the
// classifier: only the doc itself is the function's doc comment.
package docsignature

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

type Buffer struct{ data []byte }

// Defect: the count was dropped from the signature.

// Sync flushes the buffer and returns the number of bytes written.
func (b *Buffer) Sync() { b.data = b.data[:0] } // want `doc of Sync says it returns a result, but Sync returns nothing`

// Fixed twin, signature: the count is back.

// SyncN flushes the buffer and returns the number of bytes written.
func (b *Buffer) SyncN() int { n := len(b.data); b.data = b.data[:0]; return n }

// Defect: the function became a predicate.

// Validate returns an error if the name is empty.
func Validate(name string) bool { return name != "" } // want `doc of Validate says it returns an error, but Validate has no error result`

// Fixed twin, doc.

// ValidName reports whether the name is non-empty.
func ValidName(name string) bool { return name != "" }

type Queue struct{ items []int }

// Defect: a predicate turned into a command.

// Empty reports whether the queue has no items.
func (q *Queue) Empty() { q.items = nil } // want `doc of Empty says it returns a result, but Empty returns nothing`

// Fixed twin, doc.

// Clear removes every item from the queue.
func (q *Queue) Clear() { q.items = nil }

type Box struct{ v int }

// Defect: the old value is no longer returned.

// Swap stores v and returns the old value.
func (b *Box) Swap(v int) { b.v = v } // want `doc of Swap says it returns a result, but Swap returns nothing`

// Fixed twin, signature.

// SwapOld stores v and returns the old value.
func (b *Box) SwapOld(v int) int { old := b.v; b.v = v; return old }

type Config struct{ Name string }

// Defect: the error result was dropped when the parser learned to default.

// Parse parses s and returns an error on malformed input.
func Parse(s string) Config { return Config{Name: s} } // want `doc of Parse says it returns an error, but Parse has no error result`

// Fixed twin, signature.

// ParseStrict parses s and returns an error on malformed input.
func ParseStrict(s string) (Config, error) { return Config{Name: s}, nil }

type Conn struct{}

type Pool struct{ closed bool }

// Defect: a sentinel the function can no longer return.

// Get takes a connection from the pool and returns ErrClosed if the pool is closed.
func (p *Pool) Get() *Conn { return &Conn{} } // want `doc of Get says it returns an error, but Get has no error result`

// Fixed twin, doc.

// Take takes a connection from the pool; on a closed pool it returns a new connection.
func (p *Pool) Take() *Conn { return &Conn{} }

// Defect: a mixed paragraph; one sentence promises a result, another returns into a pool.

// Write writes the entry, returns any errors, and returns the entry to a pool for re-use.
func (b *Buffer) Write(p []byte) { b.data = append(b.data, p...) } // want `doc of Write says it returns a result, but Write returns nothing`

type Desc struct{}

type Collector struct{}

// Defect, wording: the values go out on the channel, not as a result; the verb is wrong.

// Describe returns all descriptions of the collector.
func (c *Collector) Describe(ch chan<- *Desc) { ch <- &Desc{} } // want `doc of Describe says it returns a result, but Describe returns nothing`

// Fixed twin, doc: the delivery is stated.

// Report sends all descriptions of the collector on ch.
func (c *Collector) Report(ch chan<- *Desc) { ch <- &Desc{} }

// Negative: an explicit delivery through a channel is not a result.

// Publish returns all observations through ch; it has no return values.
func Publish(ch chan<- int) { ch <- 1 }

// Negative: writing to an output is not a result.

// Dump writes the state as JSON to w and returns once it is flushed.
func Dump(w io.Writer) { fmt.Fprintln(w, "{}") }

// Negative: yielding to a callback is not a result.

// Each yields every item to yield, stopping when yield returns false.
func Each(yield func(int) bool) { yield(1) }

// Negative: timing.

// Run blocks and returns when ctx is done.
func Run(ctx context.Context) { <-ctx.Done() }

// Negative: ownership.

// Put returns c into the pool.
func (p *Pool) Put(c *Conn) {}

// Negative: another function's result.

// Watchdog restarts the worker whenever check returns false.
func Watchdog(check func() bool) { check() }

// Negative: an explicit negation.

// Reset clears the box; it returns nothing.
func (b *Box) Reset() { b.v = 0 }

// Negative: logging an error is not returning one.

// Count logs an error when the counter overflows.
func Count(n int) int { return n + 1 }

// Negative: recording an error is not returning one.

// Track records the last error on the box and returns the attempt count.
func (b *Box) Track(err error) int { return b.v }

// Negative: panicking with an error is not returning one.

// MustLen returns the length of s and panics with an error if s is empty.
func MustLen(s string) int { return len(s) }

// Negative: passing an error to a callback is not returning one.

// Visit calls fn for each item, passes it an error for a broken one, and returns the item count.
func Visit(fn func(error)) int { return 0 }

// Negative: a string that describes an error is not an error result.

// Explain formats the error for display.
func Explain(err error) string { return err.Error() }

type Field struct{ Value string }

// Negative: a value that wraps an error's message is not an error result.

// Wrap returns a Field that wraps the message of an error.
func Wrap(err error) Field { return Field{Value: err.Error()} }

// Negative: another function's error.

// Retries reports how many times Do returned an error.
func Retries() int { return 0 }

// Negative: weak support abstains.

// Drain returns once the queue is empty.
func Drain(q chan int) {
	for range q {
	}
}

// Negative: the error may be inside the result; not asked.

// Watch returns a channel that receives an error when the watch fails.
func Watch() chan error { return nil }

type Gateway struct{ client *http.Client }

// Defect: a handler whose doc still states the Go contract it had before it wrote the response
// itself; an error never reaches an HTTP client.

// Forward relays the request to target, returns the upstream status code and an error.
func (g *Gateway) Forward(w http.ResponseWriter, r *http.Request, target string) { // want `doc of Forward says it returns (an error|a result), but Forward returns nothing`
	w.WriteHeader(http.StatusBadGateway)
}

type Store struct{ limit int64 }

// Defect: a handler doc promising an error to its caller.

// Upload saves the posted file and returns an error if the body exceeds the limit.
func (s *Store) Upload(w http.ResponseWriter, r *http.Request) { // want `doc of Upload says it returns (an error|a result), but Upload returns nothing`
	w.WriteHeader(http.StatusCreated)
}

type Journal struct{ pending []byte }

func (j *Journal) Write(p []byte) (int, error) {
	j.pending = append(j.pending, p...)
	return len(p), nil
}

// Defect: the receiver is a writer, but the count is promised to the caller.

// Commit appends the pending entries to the log file and returns how many entries were committed.
func (j *Journal) Commit() { j.pending = nil } // want `doc of Commit says it returns a result, but Commit returns nothing`

// Negative: a handler's doc says what goes to the client.

// Uptime returns how long the service has been running.
func (g *Gateway) Uptime(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "1h") }

// Negative: the same, with the subject of the response named.

// Probe returns the most recent probe outcome of the node in the path.
func (g *Gateway) Probe(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }

type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request
}

type API struct{}

// Negative: a framework context holding the response writer in a field.

// Orders returns the orders of the current account.
func (a *API) Orders(c *Context) { fmt.Fprint(c.Writer, "[]") }

type Reply struct{ W http.ResponseWriter }

func (r *Reply) Header() http.Header         { return r.W.Header() }
func (r *Reply) Write(b []byte) (int, error) { return r.W.Write(b) }
func (r *Reply) WriteHeader(code int)        { r.W.WriteHeader(code) }

type Ctx interface {
	Reply() *Reply
	Param(name string) string
}

// Negative: a framework context whose method gives the response writer.

// Profile returns the profile of the signed-in member.
func (a *API) Profile(c Ctx) { fmt.Fprint(c.Reply(), "{}") }

type Keys interface {
	// Negative: an interface method answering a request.

	// KeyState returns whether the stored key is still accepted by the vault.
	KeyState(w http.ResponseWriter, r *http.Request)
}
