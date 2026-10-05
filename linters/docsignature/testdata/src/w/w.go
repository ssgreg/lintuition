package w

import (
	"bytes"
	"io"
	"net/http"
	"os"
)

type handlers struct{}

// Version returns the build version.
func (h *handlers) Version(w http.ResponseWriter, r *http.Request) {} // 1 writer: parameter w, an http.ResponseWriter

// Dump returns the state as text.
func Dump(out io.Writer) {} // 2 writer: parameter out, an io.Writer

// Print returns the report.
func Print(f *os.File) {} // 3 writer: a concrete type implementing io.Writer

type sink struct{ n int }

func (s *sink) Write(p []byte) (int, error) { s.n += len(p); return len(p), nil }

// Flush returns the number of bytes written.
func (s *sink) Flush() {} // 4 writer: the receiver, a pointer receiver implementing io.Writer

// Emit returns the line.
func Emit(s sink) {} // 5 writer: only *sink implements io.Writer, the parameter is addressable

type reply struct {
	w    http.ResponseWriter
	code int
}

// Send returns the body.
func (r *reply) Send() {} // 6 writer: a receiver field

type ginLike struct {
	Writer  ginWriter
	Request *http.Request
}

type ginWriter interface {
	http.ResponseWriter
	Status() int
}

// Show returns the item.
func Show(c *ginLike) {} // 7 writer: an exported field of a framework context

type outer struct{ in inner }

type inner struct{ resp reply }

// Deep returns the page.
func Deep(o *outer) {} // 8 writer: three hops down nested fields

type echoLike interface {
	Response() *echoResponse
	Param(name string) string
}

type echoResponse struct {
	Writer http.ResponseWriter
	Status int
}

func (r *echoResponse) Header() http.Header         { return r.Writer.Header() }
func (r *echoResponse) Write(b []byte) (int, error) { return r.Writer.Write(b) }
func (r *echoResponse) WriteHeader(code int)        { r.Status = code }

// Get returns the user.
func Get(c echoLike) {} // 9 writer: a method without arguments returns a response writer

type API interface {
	// Status returns the health of the service.
	Status(http.ResponseWriter, *http.Request) // 10 writer: an unnamed interface method parameter
}

type withArgs struct{}

func (withArgs) Out(name string) io.Writer { return nil }

// Named returns the value.
func Named(x withArgs) {} // 11 result: a method with arguments is not followed

var global io.Writer = os.Stdout

// Global returns the value.
func Global() { _, _ = global.Write(nil) } // 12 result: a package variable is not seen

// Closure returns a handler.
func Closure(w io.Writer) func() { return func() {} } // 13 none: has a result, so no result claim

// Captured returns the value.
func Captured() { // 14 result: the writer is local, captured by a closure
	var w io.Writer = os.Stdout
	_ = func() { _, _ = w.Write(nil) }
}

// Hidden returns the value.
func Hidden(v any) {} // 15 result: a writer behind any is not seen

type loop struct {
	Next *loop
	Prev *loop
}

// Cycle returns the value.
func Cycle(l *loop) {} // 16 result: a type cycle terminates with nothing found

type far struct{ A *far1 }
type far1 struct{ B *far2 }
type far2 struct{ C *far3 }
type far3 struct{ W io.Writer }

// TooFar returns the value.
func TooFar(f *far) {} // 17 result: four hops is past the bound

// Blank returns the value.
func Blank(_ io.Writer) {} // 18 result: a blank parameter cannot be written to

// Variadic returns the value.
func Variadic(ws ...io.Writer) {} // 19 result: a slice of writers is not one output

type mixed struct {
	buf bytes.Buffer
}

// Mixed returns the value.
func Mixed(m *mixed) {} // 20 writer: an unexported field of the same package, a value whose pointer writes

// Order returns the value.
func Order(a *outer, w http.ResponseWriter) {} // 21 writer: the first parameter in order wins, by its own path

type short struct {
	Long  *outer
	Short http.ResponseWriter
}

// Shortest returns the value.
func Shortest(s *short) {} // 22 writer: the shortest path, not the first field

type stream interface {
	io.Writer
	// Sync returns once the data is durable.
	Sync() // 23 writer: the interface method's receiver is a writer
}

// Remote returns the request body.
func Remote(r *http.Request) {} // 24 result: a request has no writer reachable

type écran struct{ Sortie http.ResponseWriter }

// Unicode returns the value.
func Unicode(é *écran) {} // 25 result: a non-ASCII identifier is not sent as a fact

type averyveryveryveryveryverylongnameforafieldholder struct {
	Averyveryveryveryveryverylongnameforafield struct {
		Averyveryveryveryveryverylongnameforafieldagain http.ResponseWriter
	}
}

// Long returns the value.
func Long(averyveryveryveryveryverylongparametername averyveryveryveryveryverylongnameforafieldholder) { // 26 result: a fact over the length limit is dropped
	_ = averyveryveryveryveryverylongparametername
}
