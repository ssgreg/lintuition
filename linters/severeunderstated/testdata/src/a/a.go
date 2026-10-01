package a

import (
	"context"
	"log"
	"log/slog"

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
