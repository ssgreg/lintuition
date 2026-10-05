package w

import (
	"bytes"
	"flag"
	"io"
	"net/http"
	"os"
	"testing"
)

type handlers struct{}

// Version returns the build version.
func (h *handlers) Version(w http.ResponseWriter, r *http.Request) {} // 1 writer: parameter w, an http.ResponseWriter

// Dump returns the state as text.
func Dump(out io.Writer) {} // 2 result: a plain io.Writer gives no fact

// Print returns the report.
func Print(f *os.File) {} // 3 result: a concrete io.Writer gives no fact

type sink struct{ n int }

func (s *sink) Write(p []byte) (int, error) { s.n += len(p); return len(p), nil }

// Flush returns the number of bytes written.
func (s *sink) Flush() {} // 4 result: a pointer receiver implementing only io.Writer

// Emit returns the line.
func Emit(s sink) {} // 5 result: only *sink implements io.Writer, still no response writer

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
func Mixed(m *mixed) {} // 20 result: a bytes.Buffer field is an io.Writer only

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
	Sync() // 23 result: an interface receiver embedding io.Writer only
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

// Lookup returns the cached record.
func Lookup(t *testing.T) { t.Helper() } // 27 result: t.Output() is an io.Writer only

type analyzerLike struct{ Flags flag.FlagSet }

type passLike struct{ Analyzer *analyzerLike }

// Resolve returns the cached record.
func Resolve(pass *passLike) { _ = pass } // 28 result: a flag set's Output() is an io.Writer only

// Incidental returns the cached record.
func Incidental(debug io.Writer) { _ = debug } // 29 result: an unused debug io.Writer

type panicky struct{}

func (panicky) Output() io.Writer { panic("never") }

// Panics returns the cached record.
func Panics(p panicky) {} // 30 result: an Output() io.Writer method

type ptrRW struct{ h http.Header }

func (p *ptrRW) Header() http.Header         { return p.h }
func (p *ptrRW) Write(b []byte) (int, error) { return len(b), nil }
func (p *ptrRW) WriteHeader(code int)        {}

type copier struct{}

func (copier) Snapshot() ptrRW { return ptrRW{} }

// Copy returns the cached record.
func Copy(c copier) {} // 31 result: a method result is not addressable, only *ptrRW is a response writer

type blankOnly struct{ _ http.ResponseWriter }

// BlankField returns the cached record.
func BlankField(b blankOnly) {} // 32 result: a blank field cannot be named

type blankThenNamed struct {
	_ http.ResponseWriter
	w http.ResponseWriter
}

// BlankNamed returns the cached record.
func BlankNamed(b blankThenNamed) {} // 33 writer: the named field after a blank one

// UnicodeFirst returns the value.
func UnicodeFirst(é io.Writer, w http.ResponseWriter) {} // 34 writer: an unsendable root passes to the next

type tee struct{ rw http.ResponseWriter }

func (t *tee) Write(p []byte) (int, error) { return t.rw.Write(p) }

// Tee returns the page.
func Tee(t *tee) {} // 35 writer: the walk goes on past a plain io.Writer to a response writer in it

type deep struct{ D struct{ Out io.Writer } }

// Trace returns the cached record.
func Trace(x deep, r *http.Request) {} // 36 result: a nested io.Writer and a request give no fact

// Owned returns the page.
func Owned(p ptrRW) {} // 37 writer: a parameter is addressable, so *ptrRW's methods count
