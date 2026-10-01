// Package normalaterror holds twins for normal-event-at-error.
package normalaterror

import (
	"context"
	"errors"
	"log/slog"
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

// Negative: a degradation is a fair thing to report at error level.
func quota(err error) {
	if err != nil {
		slog.Error("disk quota exceeded, uploads are paused", "err", err)
	}
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
