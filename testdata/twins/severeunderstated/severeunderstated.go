// Package severeunderstated holds twins for severe-event-understated.
//
// Every explanation is a separate comment, a blank line above the function or the call, so it
// never reaches the classifier: only the message, its level and the branch facts are sent.
package severeunderstated

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
)

type Event struct{ ID string }

// Defect: events are lost for good, and the line says so at info level.

func enqueueDefect(q chan Event, e Event) {
	select {
	case q <- e:
	default:
		slog.Info("queue full, event dropped and not retried") // want `unintended loss logged at info level`
	}
}

// Fixed twin: the same loss at error level is not this linter's business.

func enqueueFixed(q chan Event, e Event) {
	select {
	case q <- e:
	default:
		slog.Error("queue full, event dropped and not retried")
	}
}

// Defect: a failed write throws the batch away, and only a debug line says so.

func flushDefect(write func([]Event) error, batch []Event) {
	if err := write(batch); err != nil {
		slog.Debug("batch write rejected by the store, the readings in it are gone", "err", err) // want `unintended loss logged at debug level`
	}
}

// Defect: the upload gives up and the file is never stored; info level hides it.

func uploadDefect(attempts int) {
	if attempts > 5 {
		slog.Info("upload abandoned after the last attempt, the file was never stored") // want `unintended loss logged at info level`
	}
}

// Defect: the code checked for a missing file, but what is missing held unsent work. The branch
// fact does not make the loss expected.

func spoolDefect(path string) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		slog.Info("spool file vanished while sending, the queued messages in it are lost for good") // want `unintended loss logged at info level`
	}
}

// Defect: the cancellation was checked, but the error is replaced before the log: the final flush
// failed and its loss sits in the cancelled branch. No branch fact is sent for it.

func drainDefect(err error, flush func() error) {
	if errors.Is(err, context.Canceled) {
		err = flush()
		if err != nil {
			slog.Info("final flush on cancel failed, the buffered entries are gone", "err", err) // want `unintended loss logged at info level`
		}
	}
}

// Negative: a requested deletion is routine progress.

func purge(ids []string) {
	slog.Info("deleted expired sessions as requested")
}

// Negative: a loss the program chose on purpose (sampling) is not understated.

func sample(e Event) {
	slog.Debug("debug event dropped by the sampler as configured")
}

// Negative: an optional file that is missing on the first start; the branch on fs.ErrNotExist
// says the code expected it.

func loadState(path string) []byte {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		slog.Info("no saved state file, starting with an empty state")
		return nil
	}
	return b
}

// Negative: the same, in the guard form: only a not-exist error reaches the log.

func loadIndex(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		slog.Debug("index snapshot not present, it will be built from scratch")
		return nil, nil
	}
	return f, nil
}

// Negative: a failure only because the context was cancelled on shutdown or reload; any other
// failure is logged at error level.

func notify(ctx context.Context, send func(context.Context) error) {
	if err := send(ctx); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			slog.Debug("delivery of the pending notice failed")
			return
		}
		slog.Error("delivery of the pending notice failed", "err", err)
	}
}

// Negative: the operator pressed interrupt a second time and asked not to wait.

func trap(sigs chan os.Signal, done chan struct{}, soft, hard func()) {
	signal.Notify(sigs, os.Interrupt)
	<-sigs
	soft()
	select {
	case <-sigs:
		slog.Info("second interrupt, exiting without waiting for running jobs")
		hard()
	case <-done:
	}
}

// Negative: the worker stops when its context is done, and leaving the jobs running is the design.

func serve(ctx context.Context, tick chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("worker exits, the jobs it started keep running on their own")
			return
		case <-tick:
		}
	}
}

// Negative: the program recovers on its own: a job whose host went away is resumed.

func resume(job Event, start func(Event)) {
	slog.Info("resuming the job whose host went away")
	start(job)
}

// Negative: a peer that failed long ago expires and is dropped from the table, as designed.

func expire(peers map[string]int, name string) {
	slog.Debug("unreachable peer expired, removed from the member table")
	delete(peers, name)
}
