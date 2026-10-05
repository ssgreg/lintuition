package a

import (
	"context"
	stderrors "errors"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/ssgreg/logf"
)

var msg = "events dropped"

func c1() {
	slog.Info("queue full, dropping events") // 1 candidate: info level
}

func c2() {
	slog.Error("queue full, dropping events") // 2 none: error level
}

func c3() {
	slog.Info("dropped") // 3 none: one word
}

func c4() {
	slog.Info(msg) // 4 unsupported: the message is a variable
}

func c5() {
	slog.Debug("upload lost after restart") // 5 candidate: debug level
}

func c6() {
	slog.Warn("queue full, dropping events") // 6 none: warn level
}

func c7() {
	log.Printf("queue full, dropping events") // 7 none: unleveled
}

func c8(n int) {
	log.New(nil, "", 0).Printf("lost %d records", n) // 8 none: unleveled method
}

func c9(ctx context.Context, l *logf.Logger) {
	l.Info(ctx, "session expired, work discarded") // 9 candidate: a typed logger
}

func c10(err error) {
	_ = logf.Error(err) // 10 none: a field constructor is not a log call
}

func c11(n int) {
	slog.Info("%d", n) // 11 none: no words
}

// Branch facts: what the code has established where it logs.

func c12(err error) {
	if stderrors.Is(err, fs.ErrNotExist) {
		slog.Info("no saved state, starting empty") // 12 branch: errors.Is on a sentinel, under a renamed import
	}
}

func c13(err error) {
	if stderrors.Is(err, fs.ErrNotExist) {
		return
	} else {
		slog.Info("state file unreadable, skipped") // 13 no branch: the else of errors.Is names nothing
	}
}

func c14(err error) {
	if !stderrors.Is(err, fs.ErrNotExist) {
		slog.Info("state file unreadable, skipped") // 14 no branch: a negated errors.Is names nothing
	}
}

func c15(err error) error {
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		slog.Debug("snapshot file missing, nothing to load") // 15 branch: the guard before it leaves on every other error
	}
	return nil
}

func c16(err error) {
	if !os.IsNotExist(err) {
		slog.Warn("unexpected error")
	}
	slog.Debug("snapshot file missing, nothing to load") // 16 no branch: the guard before it does not leave
}

func c17(err error) {
	if err == io.EOF {
		slog.Info("stream ended, partial record discarded") // 17 branch: comparison with a sentinel
	}
}

func c18(err error) {
	if err != io.EOF {
		slog.Info("stream broke, partial record discarded") // 18 no branch: the sentinel is ruled out, nothing named
	}
}

func c19(err error) {
	if err != io.EOF {
		return
	} else {
		slog.Info("stream ended, partial record discarded") // 19 branch: the else of != holds the sentinel
	}
}

func c20(ctx context.Context, err error, retry bool) {
	if err != nil && stderrors.Is(ctx.Err(), context.Canceled) && !retry {
		slog.Debug("send aborted, the batch is dropped") // 20 branch: a conjunct of the condition
	}
}

func c21(err error) {
	target := stderrors.New("local")
	if stderrors.Is(err, target) {
		slog.Info("matched a local error value") // 21 no branch: the target is a local variable, not a sentinel
	}
}

func c22(err error) {
	switch {
	case stderrors.Is(err, context.DeadlineExceeded):
		slog.Info("request timed out, reply lost") // 22 branch: a tagless switch case is a condition
	}
}

func c23(err error) {
	switch err {
	case io.ErrUnexpectedEOF:
		slog.Info("upload cut short, chunk lost") // 23 branch: a switch on the error compares with the sentinel
	}
}

func c24(err error) {
	switch err {
	case io.EOF, io.ErrUnexpectedEOF:
		slog.Info("upload cut short, chunk lost") // 24 no branch: one of two values holds, neither is named
	}
}

func c25(ctx context.Context, jobs chan int) {
	select {
	case <-ctx.Done():
		slog.Info("worker exits, jobs keep running") // 25 branch: the Done channel of a context
	case <-jobs:
	}
}

func c26(stop chan struct{}, jobs chan int) {
	select {
	case <-stop:
		slog.Info("worker exits, jobs keep running") // 26 no branch: a plain channel says nothing typed
	case <-jobs:
	}
}

func c27(sigs chan os.Signal, jobs chan int) {
	select {
	case s := <-sigs:
		_ = s
		slog.Info("interrupted, stopping without waiting") // 27 branch: a receive from a channel of os.Signal
	case <-jobs:
	}
}

func bind(s os.Signal, f func(int)) {}

func c28() {
	bind(syscall.SIGINT, func(n int) {
		slog.Info("second interrupt, stopping without waiting") // 28 branch: a function literal passed with a signal
	})
}

func onEvent(name string, f func()) {}

func c29() {
	onEvent("interrupt", func() {
		slog.Info("second interrupt, stopping without waiting") // 29 no branch: no argument is a signal
	})
}

type Signal int

func c30(sigs chan Signal, jobs chan int) {
	select {
	case <-sigs:
		slog.Info("interrupted, stopping without waiting") // 30 no branch: a type named Signal without the methods of os.Signal
	case <-jobs:
	}
}

func c31(ctx context.Context, jobs chan int, err error) {
	select {
	case <-ctx.Done():
		if stderrors.Is(err, context.Canceled) {
			slog.Info("shutdown cut the sync short, changes lost") // 31 branch: both conditions, the nearest first
		}
	case <-jobs:
	}
}

func Is(err, target error) bool { return false }

func c33(err error) {
	if Is(err, fs.ErrNotExist) {
		slog.Info("no saved state, starting empty") // 33 no branch: a function named Is that is not errors.Is
	}
}

func c35(err error) {
	if !os.IsNotExist(err) {
		panic(err)
	}
	slog.Info("no saved state, starting empty") // 35 branch: a guard that panics leaves too
}

func c37(err error) {
	if os.IsNotExist(err) {
		slog.Info("no saved state, starting empty") // 37 branch: an os predicate holds in its body
	}
}

func c38(err error) {
	if stderrors.Is(err, fs.ErrNotExist) {
		slog.Error("state file gone, history lost") // 38 none: error level
	}
}

type lostError struct{}

func (*lostError) Error() string { return "lost" }

var ErrLost = &lostError{}

func c40(err error) {
	if err == ErrLost {
		slog.Info("record vanished before it was read") // 40 branch: a sentinel of a custom error type
	}
}

type doner struct{}

func (doner) Done() <-chan struct{} { return nil }

func c41(d doner, jobs chan int) {
	select {
	case <-d.Done():
		slog.Info("worker exits, jobs keep running") // 41 no branch: a Done method of something that is not a context
	case <-jobs:
	}
}

var errNotReady = stderrors.New("not ready")

func c42(err error) {
	for i := 0; i < 3; i++ {
		if !stderrors.Is(err, errNotReady) {
			continue
		}
		slog.Info("node not ready, request queued") // 42 branch: a guard that continues leaves too
	}
}

func c43(ok bool, err error) {
	if !(ok || !os.IsNotExist(err)) {
		slog.Info("no saved state, starting empty") // 43 branch: a negated disjunction holds each negated part
	}
}

// Control flow into the log: fallthrough and goto.

func c44(err error) {
	switch err {
	case io.EOF:
		fallthrough
	case context.Canceled:
		slog.Info("buffered rows were thrown away") // 44 no branch: the previous case falls through
	}
}

func c45(err error) {
	switch {
	case err == nil:
		fallthrough
	case stderrors.Is(err, context.Canceled):
		slog.Info("buffered rows were thrown away") // 45 no branch: a tagless case reached by fallthrough
	}
}

func c46(err error) {
	if !os.IsNotExist(err) {
		goto report
	}
report:
	slog.Info("buffered rows were thrown away") // 46 no branch: a goto is not a guard that leaves
}

func c47(err error, again bool) {
	if !os.IsNotExist(err) {
		return
	}
	if again {
		goto retry
	}
retry:
	slog.Info("buffered rows were thrown away") // 47 no branch: a label a goto can reach resets the guards
}

// The checked value must still be the one at hand.

func c48(err error) {
	if !os.IsNotExist(err) {
		return
	}
	err = io.ErrUnexpectedEOF
	slog.Info("buffered rows were thrown away", "err", err) // 48 no branch: the checked variable is written after the guard
}

func c49(err error) {
	if stderrors.Is(err, context.Canceled) {
		err = io.ErrUnexpectedEOF
		slog.Info("buffered rows were thrown away", "err", err) // 49 no branch: the checked variable is written in the branch
	}
}

func c50(err error) {
	if stderrors.Is(err, context.Canceled) {
		err := io.ErrUnexpectedEOF
		slog.Info("buffered rows were thrown away", "err", err) // 50 no branch: the name is shadowed at the log
	}
}

func c51(err error) func(error) {
	if stderrors.Is(err, context.Canceled) {
		return func(err error) {
			slog.Info("buffered rows were thrown away", "err", err) // 51 no branch: a function literal runs later, with its own error
		}
	}
	return nil
}

func c52(err error) {
	if stderrors.Is(err, context.Canceled) {
		func() {
			slog.Info("buffered rows were thrown away") // 52 no branch: the walk stops at any function literal
		}()
	}
}

func c53(err, other error) {
	if stderrors.Is(err, context.Canceled) {
		slog.Info("buffered rows were thrown away", "err", other) // 53 no branch: the line logs another error value
	}
}

func c54(ctx context.Context, err error, l *logf.Logger) {
	if stderrors.Is(err, context.Canceled) {
		l.Info(ctx, "buffered rows were thrown away", logf.Error(err)) // 54 branch: the line logs the checked error itself
	}
}

func c60(err error, retry func()) {
	reset := func() { err = nil }
	if stderrors.Is(err, context.Canceled) {
		retry()
		slog.Info("buffered rows were thrown away") // 60 no branch: a function literal in the function writes the variable
	}
	reset()
}

func c61(err error, fill func(*error)) {
	if stderrors.Is(err, context.Canceled) {
		fill(&err)
		slog.Info("buffered rows were thrown away") // 61 no branch: the variable's address is taken
	}
}

var lastErr error

func c62() {
	if stderrors.Is(lastErr, context.Canceled) {
		slog.Info("buffered rows were thrown away") // 62 no branch: a package-level variable can change anywhere
	}
}

func c63(err error) error {
	if stderrors.Is(err, context.Canceled) {
		slog.Info("buffered rows were thrown away") // 63 branch: a write after the log does not matter
		err = nil
	}
	return err
}

func c64(ctx context.Context, other context.Context) {
	if stderrors.Is(ctx.Err(), context.Canceled) {
		ctx = other
		slog.Info("buffered rows were thrown away") // 64 no branch: the context the check read is replaced
	}
}

// Contexts and signals, by what the types say.

type fauxContext struct{ ch chan int }

func (fauxContext) Deadline() int    { return 0 }
func (f fauxContext) Done() chan int { return f.ch }
func (fauxContext) Err() bool        { return false }
func (fauxContext) Value()           {}

func c55(f fauxContext, jobs chan int) {
	select {
	case <-f.Done():
		slog.Info("buffered rows were thrown away") // 55 no branch: methods named like a context's, with other signatures
	case <-jobs:
	}
}

type job struct {
	context.Context
}

func c56(j job, jobs chan int) {
	select {
	case <-j.Done():
		slog.Info("buffered rows were thrown away") // 56 branch: a type embedding a context is one
	case <-jobs:
	}
}

type ctxAlias = context.Context

func c57(ctx ctxAlias, jobs chan int) {
	select {
	case <-ctx.Done():
		slog.Info("buffered rows were thrown away") // 57 branch: an alias of context.Context
	case <-jobs:
	}
}

func callNow(_ os.Signal, f func()) { f() }

func c58() {
	callNow(syscall.SIGUSR1, func() {
		slog.Info("buffered rows were thrown away") // 58 branch: the call's shape is described, not a delivery
	})
}

func c59(sigs chan os.Signal, jobs chan int) {
	select {
	case _, ok := <-sigs:
		if !ok {
			slog.Info("buffered rows were thrown away") // 59 no branch: a two-value receive may be a closed channel
		}
	case <-jobs:
	}
}

// The checked place, not only its variable.

type pair struct{ first, second error }

func c65(p pair) {
	if stderrors.Is(p.first, context.Canceled) {
		slog.Info("buffered rows were thrown away", "err", p.second) // 65 no branch: another field of the same variable
	}
}

func c66(p pair) {
	if stderrors.Is(p.first, context.Canceled) {
		slog.Info("buffered rows were thrown away", "err", p.first) // 66 branch: the same field
	}
}

func c67(p []error) {
	if stderrors.Is(p[0], context.Canceled) {
		slog.Info("buffered rows were thrown away", "err", p[1]) // 67 no branch: an index has no place to compare
	}
}

type source struct{}

func (source) cached() error { return nil }
func (source) read() error   { return nil }

func c68(r source) {
	if stderrors.Is(r.cached(), context.Canceled) {
		slog.Info("buffered rows were thrown away", "err", r.read()) // 68 no branch: a method result is not a place
	}
}

func c69(ctx context.Context) {
	if stderrors.Is(ctx.Err(), context.Canceled) {
		slog.Info("buffered rows were thrown away", "err", ctx.Err()) // 69 branch: ctx.Err() on a context is the same place
	}
}

// Loops and goto.

func c70(err error) {
	if stderrors.Is(err, context.Canceled) {
		for i := 0; i < 2; i++ {
			slog.Info("buffered rows were thrown away", "err", err) // 70 no branch: a write later in the loop reaches the next pass
			err = io.ErrUnexpectedEOF
		}
	}
}

func c71(err error) {
	for i := 0; i < 2; i++ {
		if stderrors.Is(err, context.Canceled) {
			slog.Info("buffered rows were thrown away", "err", err) // 71 branch: the check runs again on every pass
		}
		err = io.ErrUnexpectedEOF
	}
}

func c72(err error, errs []error) {
	if stderrors.Is(err, context.Canceled) {
		for _, err = range errs {
			slog.Info("buffered rows were thrown away", "err", err) // 72 no branch: the range writes the checked variable
		}
	}
}

func c73(err error, again bool) {
	if stderrors.Is(err, context.Canceled) {
		slog.Info("buffered rows were thrown away", "err", err) // 73 no branch: a goto in the function may loop back
	}
	if again {
		goto done
	}
done:
}

// A Value method on a defined empty interface is not context.Context's.

type anything interface{}

type almost struct{ ch <-chan struct{} }

func (almost) Deadline() (time.Time, bool) { return time.Time{}, false }
func (a almost) Done() <-chan struct{}     { return a.ch }
func (almost) Err() error                  { return nil }
func (almost) Value(anything) anything     { return nil }

func c74(a almost, jobs chan int) {
	select {
	case <-a.Done():
		slog.Info("buffered rows were thrown away") // 74 no branch: Value takes and returns a defined type, not any
	case <-jobs:
	}
}

type anyAlias = any

type nearly struct{ ch <-chan struct{} }

func (nearly) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c nearly) Done() <-chan struct{}     { return c.ch }
func (nearly) Err() error                  { return nil }
func (nearly) Value(anyAlias) interface{}  { return nil }

func c75(c nearly, jobs chan int) {
	select {
	case <-c.Done():
		slog.Info("buffered rows were thrown away") // 75 branch: an alias of any and interface{} are any
	case <-jobs:
	}
}
