package a

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"

	pkgerrors "github.com/pkg/errors"
	"github.com/ssgreg/logf"
)

var msg = "delete the cache"

func c1() error {
	return errors.New("index is corrupt; delete the data directory and restart") // 1 candidate: errors.New
}

func c2(err error) error {
	return fmt.Errorf("config invalid: %w; reset it with --reset", err) // 2 candidate: a wrapping format
}

func c3() error {
	return errors.New("file not found") // 3 candidate: every constant text is asked
}

func c4() error {
	return errors.New(msg) // 4 unsupported: the message is a variable
}

func c5() {
	slog.Error("cache is stale, wipe it and restart") // 5 candidate: a log message
}

func c6(err error) {
	_ = logf.Error(err) // 6 none: a field constructor is not a log call
}

func c7(err error) error {
	return pkgerrors.Wrap(err, "run rm -rf /var/lib/app to recover") // 7 candidate: pkg/errors Wrap, rm
}

func c8() {
	slog.Info(msg) // 8 unsupported: the log message is a variable
}

func c9(ctx context.Context, l *logf.Logger) {
	l.Info(ctx, "removed 3 stale sessions") // 9 candidate: a typed logger
}

func c10() {
	log.Printf("%s: %v", "a", 1) // 10 none: no words at all
}

func c11() error {
	return fmt.Errorf("droplet count %d", 3) // 11 candidate: the classifier decides
}

func c12(s fmt.Stringer) string {
	return fmt.Sprintf("delete %s", s) // 12 none: Sprintf makes no error
}

func c13() error {
	return errors.New("format the data volume and restart") // 13 candidate: format
}

func c14() error {
	return errors.New("overwrite the database with an empty file") // 14 candidate: overwrite
}

func c15() error {
	return errors.New("run mkfs.ext4 on the data volume to recover") // 15 candidate: mkfs
}
