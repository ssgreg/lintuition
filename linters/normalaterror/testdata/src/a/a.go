package a

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"

	"github.com/rs/zerolog"
	"github.com/sirupsen/logrus"
	"github.com/ssgreg/logf"
	"go.uber.org/zap"
)

var msg = "cache miss"

func c1() {
	slog.Error("cache miss") // 1 candidate: error level
}

func c2() {
	slog.Info("cache miss") // 2 none: info level
}

func c3() {
	slog.Error("failed to save config") // 3 none: the message names a failure
}

func c4() {
	slog.Error(msg) // 4 unsupported: the message is a variable
}

func c5(id string) {
	log.Fatalf("client went away: %s", id) // 5 candidate: fatal level, a format
}

func c6() {
	log.Printf("cache miss") // 6 none: unleveled
}

func c7() {
	slog.Error("unable to connect") // 7 none: unable
}

func c8(err error) {
	_ = logf.Error(err) // 8 none: a field constructor is not a log call
}

func c9(ctx context.Context, l *logf.Logger) {
	l.Error(ctx, "retry scheduled") // 9 candidate: a typed logger
}

func c10() {
	slog.Error("Error reading body") // 10 none: error, capitalised
}

func c11() {
	slog.Error("can't open file") // 11 none: can't
}

func c12() {
	slog.Warn("cache miss") // 12 none: warn level
}

func c13() {
	slog.Error("could not reach peer") // 13 none: could not
}

func c14(err error) {
	slog.Error("request canceled by client", "err", err) // 14 candidate: an error field does not name a failure in the message
}

func c15() {
	slog.Error("planned failover completed successfully") // 15 candidate: a failover is not a failure
}

func c16(l *zap.Logger, err error) {
	if err != nil {
		l.Error("listener close", zap.Error(err)) // 16 candidate: zap.Error field in an err != nil branch
	}
}

func c17(err error) {
	if errors.Is(err, context.Canceled) {
		slog.Error("client went away", "err", err) // 17 candidate: errors.Is with a sentinel
	}
}

func c18(ctx context.Context, l *logf.Logger, err error) {
	l.Error(ctx, "applying config", logf.Error(err)) // 18 candidate: a logf field, no branch
}

func c19(err error) {
	logrus.Errorf("stopping endpoint: %v", err) // 19 candidate: an error after a printf format
}

func c20(err error) {
	logrus.WithError(err).Error("renewing certificates") // 20 candidate: an error on the logger chain
}

func c21(l *zerolog.Logger, err error) {
	l.Error().Err(err).Msg("shutting down exporter") // 21 candidate: zerolog Err on the event chain
}

func c22(l *zap.Logger, err error) {
	if err != nil && !errors.Is(err, net.ErrClosed) {
		l.Error("accept loop", zap.Error(err)) // 22 candidate: && splits, ! negates
	}
}

func c23(l *zap.Logger, err error) {
	if err == nil {
		return
	}
	l.Error("stop hook", zap.Error(err)) // 23 candidate: a guard that returns leaves its negation
}

func c24(l *zap.Logger, err error) {
	if errors.Is(err, io.EOF) {
		return
	}
	if err != nil {
		l.Error("reading frames", zap.Error(err)) // 24 candidate: nearest first, then the guard
	}
}

func c25(l *zap.Logger, err error) {
	if err == nil {
		slog.Info("done")
	} else {
		l.Error("flushing buffer", zap.Error(err)) // 25 candidate: the else of err == nil
	}
}

func c26(l *zap.Logger, err error) {
	if err == io.EOF {
		l.Error("peer closed the stream", zap.Error(err)) // 26 candidate: == with a sentinel
	}
}

func c27(l *zap.Logger, err error) {
	if os.IsNotExist(err) {
		l.Error("no state file yet", zap.Error(err)) // 27 candidate: an os predicate
	}
}

func c28(l *zap.Logger, err error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		l.Error("request deadline passed", zap.Error(err)) // 28 candidate: a tagless switch case
	}
}

func c29(l *zap.Logger, err error) {
	switch err {
	case io.EOF:
		l.Error("end of input", zap.Error(err)) // 29 candidate: a switch on the error
	}
}

func c30(l *zap.Logger, err error, retry bool) {
	switch {
	case retry:
		fallthrough
	case errors.Is(err, io.EOF):
		l.Error("stream ended", zap.Error(err)) // 30 candidate: reached by fallthrough, no check
	}
}

func c31(l *zap.Logger, err error) {
	if err != nil {
		err = fmt.Errorf("wrapped: %w", err)
		l.Error("closing socket", zap.Error(err)) // 31 candidate: reassigned in the branch, no check
	}
}

func c32(l *zap.Logger, err error, next func() error) {
	if err == nil {
		return
	}
	err = next()
	l.Error("second attempt", zap.Error(err)) // 32 candidate: reassigned after the guard, no check
}

func c33(l *zap.Logger, err error, fill func(*error)) {
	fill(&err)
	if err != nil {
		l.Error("filling result", zap.Error(err)) // 33 candidate: its address is taken, no check
	}
}

func c34(l *zap.Logger, err error) {
	reset := func() { err = nil }
	if err != nil {
		reset()
		l.Error("reset path", zap.Error(err)) // 34 candidate: a closure assigns it, no check
	}
}

func c35(l *zap.Logger, err error) {
	if err != nil {
		go func() {
			l.Error("async report", zap.Error(err)) // 35 candidate: inside a literal, the outer check does not count
		}()
	}
}

func c36(l *zap.Logger, stop func() error) {
	go func() {
		err := stop()
		if err != nil {
			l.Error("stopping admin endpoint", zap.Error(err)) // 36 candidate: declared and checked in the literal
		}
	}()
}

func c37(ctx context.Context, l *zap.Logger) {
	<-ctx.Done()
	l.Error("waiting for shutdowns", zap.Error(ctx.Err())) // 37 candidate: a call result, no check
}

func c38(l *zap.Logger, a, b error) {
	if a != nil {
		l.Error("closing both ends", zap.Error(a), zap.Error(b)) // 38 candidate: two error values, no check
	}
}

func c39(reason string) {
	slog.Error("request dropped", "err", reason) // 39 candidate: a string under the key err is not an error
}

type myErr struct{}

func (*myErr) Error() string { return "" }

func c40(e *myErr) {
	if e != nil {
		slog.Error("custom type path", "err", e) // 40 candidate: a concrete error type, no check
	}
}

func c41() {
	slog.Error("nil attached", "err", nil) // 41 candidate: an untyped nil is not an error
}

func c42(l *zap.Logger, err error, force bool) {
	if err != nil || force {
		l.Error("forced close", zap.Error(err)) // 42 candidate: || establishes nothing
	}
}

func c43(l *zap.Logger, err error) {
	if err == nil {
		goto done
	}
	l.Error("goto path", zap.Error(err)) // 43 candidate: a goto in the function, no check
done:
}

func c44(l *zap.Logger, err error) {
	if err == nil {
		slog.Info("nothing to report")
	}
	l.Error("guard that stays", zap.Error(err)) // 44 candidate: a guard that does not leave, no check
}

func c45(l *zap.Logger, err, target error) {
	if errors.Is(err, target) {
		l.Error("local target", zap.Error(err)) // 45 candidate: errors.Is with a local target, no check
	}
}

func c46(l *zap.Logger, err, other error) {
	if other != nil {
		l.Error("other variable", zap.Error(err)) // 46 candidate: the check is on another variable
	}
}

func c47(err error) {
	if !errors.Is(err, context.Canceled) {
		slog.Error("handler returned", "err", err) // 47 candidate: a negated errors.Is
	}
}

func c48(err error) {
	if err != nil {
		slog.Error("failed to close listener", "err", err) // 48 none: the message names a failure
	}
}

func c49(err error) {
	if err != nil {
		slog.Info("listener close", "err", err) // 49 none: info level
	}
}

func c50(s *zap.SugaredLogger, err error) {
	s.Errorw("listener close", "err", err) // 50 candidate: sugared key-value
}

func c51(l *zap.Logger, err error, next func() error) {
	for i := 0; i < 3; i++ {
		if err != nil {
			l.Error("retry scheduled", zap.Error(err)) // 51 candidate: reassigned later in the branch, no check
			err = next()
		}
	}
}

func c52(l *zap.Logger, err error, ch chan int) {
	select {
	case <-ch:
		if err == nil {
			return
		}
		l.Error("draining channel", zap.Error(err)) // 52 candidate: a guard in a select case
	}
}

func c53(l *zap.Logger, err error) {
	if err != nil {
		l.Fatal("binding port", zap.Error(err)) // 53 candidate: fatal level
	}
}

func c54(l *zap.Logger, err error) {
	if err := io.EOF; err != nil {
		l.Error("shadowed in init", zap.Error(err)) // 54 candidate: the if declares its own err
	}
	_ = err
}

func c55(l *zap.Logger, err error) {
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			l.Error("listener gone", zap.Error(err)) // 55 candidate: nested checks, nearest first
		}
	}
}

func c56(l *zap.Logger, err error, next func() error) {
	if err != nil {
		err = next()
		if errors.Is(err, io.EOF) {
			l.Error("after reassign", zap.Error(err)) // 56 candidate: the inner check holds, the outer one does not
		}
	}
}

func c57(l *zap.Logger, err error, read func() (int, error)) {
	if err != nil {
		n, err := read()
		_ = n
		l.Error("short redeclare", zap.Error(err)) // 57 candidate: := declares a new err in the branch, the outer check is not its
	}
}

func c58(l *zap.Logger, err error, read func() (int, error)) {
	var n int
	if err != nil {
		n, err = read()
		l.Error("plain reassign", zap.Error(err)) // 58 candidate: a multi-value assignment, no check
	}
	_ = n
}

type wrapper struct{ cause error }

func (w *wrapper) Error() string { return "wrapped" }
func (w *wrapper) Unwrap() error { return w.cause }

func c61(l *zap.Logger) {
	w := &wrapper{cause: context.Canceled}
	var err error = w
	if errors.Is(err, context.Canceled) {
		w.cause = io.ErrUnexpectedEOF
		l.Error("streaming rows", zap.Error(err)) // 61 candidate: the wrapped cause is rewritten, errors.Is is not kept
	}
}

func c62(l *zap.Logger) {
	var err error = &wrapper{cause: context.Canceled}
	if errors.Is(err, context.Canceled) {
		l.Error("streaming columns", zap.Error(err)) // 62 candidate: nothing is rewritten, errors.Is is kept
	}
}

var errStopped = errors.New("stopped")

func c63(l *zap.Logger, err error) {
	if err == errStopped {
		errStopped = io.EOF
		l.Error("handling stop", zap.Error(err)) // 63 candidate: the sentinel is rewritten, == is not kept
	}
}

type stats struct{ seen int }

func c64(l *zap.Logger, s *stats, err error) {
	if err != nil {
		s.seen++
		l.Error("counting frames", zap.Error(err)) // 64 candidate: a field write keeps not nil, which depends on err alone
	}
}

func c65(l *zap.Logger, err error) {
	n := 1
	if errors.Is(err, io.EOF) {
		n = 0
		l.Error("draining input", zap.Error(err)) // 65 candidate: a local write keeps errors.Is
	}
	_ = n
}

func c66() {
	slog.Error("closing the listener", "err", context.Canceled) // 66 candidate: a package-level error logged by name
}

func c67() {
	slog.Error("closing the listener", "err", errors.New("x")) // 67 candidate: a new error, neither named nor checked
}

func c68(err error) {
	if err == nil {
		slog.Error("config reload", "err", err) // 68 candidate: known nil
	}
}

func c69(err error) {
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) &&
		!errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.ErrClosedPipe) && !errors.Is(err, io.ErrShortBuffer) &&
		!errors.Is(err, io.ErrShortWrite) {
		slog.Error("reading stream", "err", err) // 69 candidate: a long chain is cut to six and still passes the policy
	}
}

var frames int

func c70(l *zap.Logger, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, io.EOF) {
		return
	}
	frames++
	l.Error("decoding frame", zap.Error(err)) // 70 candidate: a package write after the guards drops is not, keeps not nil
}

func c71(l *zap.Logger, w *wrapper) {
	var err error = w
	if errors.Is(err, io.EOF) {
		defer func() { w.cause = nil }()
		l.Error("deferred rewrite", zap.Error(err)) // 71 candidate: a write in a literal in the region counts too
	}
}

func c72(err error) {
	if err == context.Canceled {
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, io.EOF) &&
			!errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.ErrClosedPipe) && !errors.Is(err, io.ErrShortBuffer) {
			slog.Error("closing the listener", "err", err) // 72 candidate: the identity beyond the cut is kept and still decides
		}
	}
}

func c73(err error) {
	if err == context.Canceled {
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, io.EOF) &&
			!errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.ErrClosedPipe) {
			slog.Error("closing the listener", "err", err) // 73 candidate: six checks, nothing cut
		}
	}
}

func c74(err error) {
	if err == nil {
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) &&
			!errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.ErrClosedPipe) && !errors.Is(err, io.ErrShortBuffer) {
			slog.Error("config reload", "err", err) // 74 candidate: nil beyond the cut is kept and still decides
		}
	}
}
