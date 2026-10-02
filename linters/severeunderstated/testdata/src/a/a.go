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
