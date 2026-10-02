// Package normalaterror holds twins for normal-event-at-error.
//
// Every explanation is a separate comment, a blank line above the function or the call, so it
// never reaches the classifier: only the message and the facts about the error it carries are
// sent.
package normalaterror

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
)

type Cache interface {
	Get(key string) (string, bool)
}

// Defect: a cache miss is the cache working as designed, not an error.

func lookupDefect(c Cache, key string) string {
	v, ok := c.Get(key)
	if !ok {
		slog.Error("cache miss, loading from the database", "key", key) // want `routine event logged at error level: "cache miss, loading from the database"`
	}
	return v
}

// Fixed twin: the same event at debug level.

func lookupFixed(c Cache, key string) string {
	v, ok := c.Get(key)
	if !ok {
		slog.Debug("cache miss, loading from the database", "key", key)
	}
	return v
}

// Defect: the client went away, the code checked that this is the cancellation, and still logs
// it as an error.

func respondDefect(write func() error) {
	if err := write(); errors.Is(err, context.Canceled) {
		slog.Error("client disconnected before the response was written", "err", err) // want `routine event logged at error level`
	}
}

// Fixed twin: the same event at debug level.

func respondFixed(write func() error) {
	if err := write(); errors.Is(err, context.Canceled) {
		slog.Debug("client disconnected before the response was written", "err", err)
	}
}

// Defect: the message names the operation, and the error it carries is the one the code checked
// for: the caller of the request cancelled it while results were streaming. Nothing failed.

func searchDefect(ctx context.Context, stream func(context.Context) error) {
	if err := stream(ctx); errors.Is(err, context.Canceled) {
		slog.Error("streaming search results to the caller", "err", err) // want `routine event logged at error level`
	}
}

// Defect: the accept loop ends because shutdown closed the listener; the code checked for exactly
// that error.

func serveDefect(accept func() error) {
	for {
		err := accept()
		if errors.Is(err, net.ErrClosed) {
			slog.Error("accept loop finished, the listener was closed for shutdown", "err", err) // want `routine event logged at error level`
			return
		}
	}
}

// Defect: a retry that is already scheduled is the program handling a hiccup, not an error, even
// with the error of the attempt attached.

func pollDefect(fetch func() error) {
	if err := fetch(); err != nil {
		slog.Error("feed poll retry scheduled in 30 seconds", "err", err) // want `routine event logged at error level`
	}
}

// Negative: the message names the operation and carries the error that stopped it; that is how
// a failure reads in a structured log.

func closeListener(ln net.Listener) {
	if err := ln.Close(); err != nil {
		slog.Error("closing the metrics listener", "err", err)
	}
}

// Negative: the same, with the expected error ruled out by the code.

func reload(apply func() error) {
	if err := apply(); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("applying the new configuration", "err", err)
	}
}

// Negative: a replay that stops before the end, with io.EOF ruled out by a guard.

func replay(next func() error) {
	for {
		err := next()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			slog.Error("replaying the journal", "err", err)
			return
		}
	}
}

// Negative: a degradation is a fair thing to report at error level.

func quota(err error) {
	if err != nil {
		slog.Error("disk quota exceeded, uploads are paused", "err", err)
	}
}

// Negative: a failure said in words that are not on the failure list.

func journal() {
	slog.Error("the journal is unreadable, recovery stopped halfway")
}

// Negative: the message names the failure itself; it is not asked about.

func save(err error) {
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("failed to save the upload", "err", err)
	}
}

// Defect: a planned failover that worked is routine, though its name starts with "fail".

func failoverDefect() {
	slog.Error("planned failover to the standby completed") // want `routine event logged at error level`
}
