package a

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

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
	slog.Info(msg) // 8 none: an info message is out of scope
}

func c9(ctx context.Context, l *logf.Logger) {
	l.Info(ctx, "removed 3 stale sessions") // 9 none: info
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

type store struct{}

func (*store) Drop(name string) error { return nil }

type dropper interface{ DropAll() error }

func c16(ctx context.Context, l *logf.Logger, s *store) {
	l.Info(ctx, "drop the scratch volume") // 16 none: info is the program narrating its own step
	_ = s.Drop("scratch")
}

func c17(ctx context.Context, l *logf.Logger) {
	l.Debug(ctx, "wipe the staging area") // 17 none: debug
}

func c18(ctx context.Context, l *logf.Logger, s *store) {
	l.Warn(ctx, "staging copy is stale, it has to be dropped", logf.String("name", "x")) // 18 candidate: warn, next call Drop
	_ = s.Drop("x")
}

func c19(ctx context.Context, l *logf.Logger, err error) error {
	l.Error(ctx, "journal is damaged; wipe it and restart", logf.Error(err)) // 19 candidate: error, next statement calls nothing
	return err
}

func c20(ctx context.Context, l *logf.Logger) {
	l.Warn(ctx, "remove the lock file by hand") // 20 candidate: last statement, no next call
}

func c21(ctx context.Context, l *logf.Logger) {
	l.Warn(ctx, "purge the queue and resubmit") // 21 candidate: the next statement is only a log call
	l.Info(ctx, "done")
}

func c22(ctx context.Context, l *logf.Logger, s *store) {
	l.Warn(ctx, "dropping the orphaned table") // 22 candidate: next call in an if init
	if err := s.Drop("t"); err != nil {
		return
	}
}

func c23(ctx context.Context, l *logf.Logger) {
	l.Warn(ctx, "clearing the temporary directory") // 23 candidate: next call is a package function
	_ = os.RemoveAll("/tmp/x")
}

func c24() {
	log.Printf("reinstall the agent with --clean") // 24 candidate: unleveled
}

func c25(ctx context.Context, l *logf.Logger) {
	l.Info(ctx, msg) // 25 none: an info message is out of scope even when it is not constant
}

func c26(ctx context.Context, l *logf.Logger, s *store, k int) {
	switch k {
	case 1:
		l.Warn(ctx, "erase the old snapshot") // 26 candidate: in a case clause, next call Drop
		_ = s.Drop("snap")
	}
}

func c27(ctx context.Context, l *logf.Logger, s *store) {
	defer l.Warn(ctx, "reset the replica state") // 27 candidate: a deferred call is not a statement of its own
	_ = s.Drop("r")
}

func c28() {
	log.Fatal("format the cache disk and start again") // 28 candidate: fatal
}

func c29(s *store) error {
	err := errors.New("drop the index and rebuild it") // 29 candidate: an error constructor gets no next call
	_ = s.Drop("i")
	return err
}

func c30(ctx context.Context, l *logf.Logger, cleanup func()) {
	l.Warn(ctx, "wipe the workspace") // 30 candidate: a func value has no static callee
	cleanup()
}

func c31(ctx context.Context, l *logf.Logger, d dropper) {
	l.Warn(ctx, "all replicas must be dropped") // 31 candidate: an interface method
	_ = d.DropAll()
}

func c32(ctx context.Context, l *logf.Logger, s *store) {
	l.Warn(ctx, "drop the old shard") // 32 candidate: a call inside a func literal
	go func() { _ = s.Drop("old") }()
}

func c33(ctx context.Context, l *logf.Logger) {
	l.Warn(ctx, msg) // 33 unsupported: a warn message that is not constant
}
