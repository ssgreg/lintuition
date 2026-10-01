package a

import (
	"context"
	"log"
	"log/slog"

	"github.com/ssgreg/logf"
)

type state struct {
	elapsed int
	inner   struct{ total int }
}

const maxLimit = 10

func count() int   { return 0 }
func handle()      {}
func get() *state  { return nil }
func items() []int { return nil }

func c1(elapsedAttempts int) {
	slog.Info("retrying", "remaining_attempts", elapsedAttempts) // 1 candidate: a variable
}

func c2(userID string) {
	slog.Info("login", "user_id", userID) // 2 none: the key's words are the value's words
}

func c3() {
	slog.Info("retrying", "attempts", 3) // 3 none: a literal
}

func c4() {
	slog.Info("retrying", "attempts", count()) // 4 none: a call result
}

func c5(s state) {
	slog.Info("retrying", "remaining", s.elapsed, "sum", s.inner.total) // 5 candidate: selectors
}

func c6(args []any) {
	slog.Info("retrying", args...) // 6 unsupported: fields passed on, not readable
}

func c7(remaining int) {
	log.Println("elapsed", remaining) // 7 none: Println's arguments are not fields
}

func c8(x int) {
	slog.Info("retrying", "a:b", x) // 8 unsupported: the key cannot be a fact
}

func c9(userID string, started int) {
	slog.Info("login", "user_id", userID, "deadline", started) // 9 candidate: only deadline is asked
}

func c10() {
	slog.Info("limits", "limit", maxLimit) // 10 none: a constant
}

func c11() {
	slog.Info("serving", "handler", handle) // 11 none: a function value
}

func c12(ctx context.Context, l *logf.Logger, err error, name string) {
	l.Info(ctx, "failed", logf.Error(err), logf.String("path", name)) // 12 candidate: an error is skipped, a constructor field is read
}

func c13() {
	slog.Info("retrying", "remaining", get().elapsed) // 13 none: a call on the way
}

func c14() {
	slog.Info("retrying", "first", items()[0]) // 14 none: an index is not a variable
}
