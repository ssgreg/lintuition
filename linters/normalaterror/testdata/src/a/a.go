package a

import (
	"context"
	"log"
	"log/slog"

	"github.com/ssgreg/logf"
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
