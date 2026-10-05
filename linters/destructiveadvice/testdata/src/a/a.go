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
	l.Warn(ctx, "drop the old shard") // 32 candidate: a go statement runs its call later, no next call
	go func() { _ = s.Drop("old") }()
}

func c33(ctx context.Context, l *logf.Logger) {
	l.Warn(ctx, msg) // 33 unsupported: a warn message that is not constant
}

func mustStore() *store           { return nil }
func mustName() string            { return "" }
func removeAll()                  {}
func confirmed() bool             { return true }
func (*store) Load() (int, error) { return 0, nil }

type holder struct{ s *store }

func c34(ctx context.Context, l *logf.Logger) {
	l.Warn(ctx, "erase the build cache") // 34 candidate: a stored func literal is not called
	cleanup := func() { removeAll() }
	_ = cleanup
}

func c35(ctx context.Context, l *logf.Logger, k bool) {
	l.Warn(ctx, "erase the build cache") // 35 candidate: a call in a branch may not run
	if k {
		removeAll()
	}
}

func c36(ctx context.Context, l *logf.Logger, k bool) {
	l.Warn(ctx, "erase the build cache") // 36 candidate: a short-circuited operand may not run
	_ = k && confirmed()
}

func c37(ctx context.Context, l *logf.Logger) {
	l.Warn(ctx, "erase the build cache") // 37 candidate: a deferred call runs later
	defer removeAll()
}

func c38() {
	log.Fatal("erase the build cache and start over") // 38 candidate: fatal exits before the next statement
	removeAll()
}

func c39() {
	log.Panic("erase the build cache and start over") // 39 candidate: panic leaves before the next statement
	removeAll()
}

func c40(ctx context.Context, l *logf.Logger, s *store) {
	l.Warn(ctx, "erase the build cache") // 40 candidate: an argument is called first
	_ = s.Drop(mustName())
}

func c41(ctx context.Context, l *logf.Logger) {
	l.Warn(ctx, "erase the build cache") // 41 candidate: the receiver is made by a call first
	_ = mustStore().Drop("x")
}

func c42(ctx context.Context, l *logf.Logger, s *store) {
	l.Warn(ctx, "erase the build cache") // 42 candidate: a func literal argument
	_ = s.Drop(func() string { return "x" }())
}

func c43(ctx context.Context, l *logf.Logger, s *store, b []byte) {
	l.Warn(ctx, "erase the build cache") // 43 candidate: a conversion in an argument is a call expression too
	_ = s.Drop(string(b))
}

func c44(ctx context.Context, l *logf.Logger, h *holder, err error) {
	l.Warn(ctx, "erase the build cache") // 44 candidate: assigning to a field evaluates its operand first
	h.s, err = nil, h.s.Drop("x")
	_ = err
}

func c45(ctx context.Context, l *logf.Logger, s *store) (int, error) {
	l.Warn(ctx, "the cache entries are stale and get dropped") // 45 candidate: x, err := call
	n, err := s.Load()
	return n, err
}

func c46(ctx context.Context, l *logf.Logger, h *holder) error {
	l.Warn(ctx, "the cache entries are stale and get dropped") // 46 candidate: a receiver through a pointer field, the call site is named
	return (h.s.Drop("x"))
}

func c47(ctx context.Context, l *logf.Logger, s *store) {
	l.Warn(ctx, "erase the build cache") // 47 candidate: a call in an if condition, no init
	if s.Drop("x") != nil {
		return
	}
}

func c48(ctx context.Context, l *logf.Logger, ch <-chan bool) {
	select {
	case <-ch:
		l.Warn(ctx, "the cache entries are stale and get dropped") // 48 candidate: in a select clause
		removeAll()
	default:
	}
}

func c49(ctx context.Context, l *logf.Logger, k bool) {
	if k {
		l.Warn(ctx, "erase the build cache") // 49 candidate: last in its block, the outer call is not borrowed
	}
	removeAll()
}

func c50(ctx context.Context, l *logf.Logger) {
	l.Info(ctx, "operator: erase the build cache by hand and restart") // 50 none: explicit advice at info is out of scope by design
}

func c51(ctx context.Context, l *logf.Logger, s *store) {
	l.Warn(ctx, "erase the build cache")             // 51 candidate: the next statement starts with another log call
	l.Error(ctx, "and again", logf.String("k", "v")) // 51 candidate: the log after it does get the call
	_ = s.Drop("x")
}

func dropPath(p string) error    { return nil }
func dropPtr(p *string) error    { return nil }
func dropN(n int) error          { return nil }
func (v valueStore) Drop() error { return nil }

type valueStore struct{}

type box struct {
	s    *store
	v    valueStore
	name string
}

var defaultPath = "/tmp/x"

const prefix = "/tmp/"

func c52(ctx context.Context, l *logf.Logger, paths []string, i int) {
	l.Warn(ctx, "erase the build cache") // 52 candidate: an index argument, the call site is named even though the index may panic
	_ = dropPath(paths[i])
}

func c53(ctx context.Context, l *logf.Logger, p *string) {
	l.Warn(ctx, "erase the build cache") // 53 candidate: a dereferenced argument, the call site is named
	_ = dropPath(*p)
}

func c54(ctx context.Context, l *logf.Logger, ch <-chan string) {
	l.Warn(ctx, "erase the build cache") // 54 candidate: a receive argument, the call site is named
	_ = dropPath(<-ch)
}

func c55(ctx context.Context, l *logf.Logger, b *box) {
	l.Warn(ctx, "erase the build cache") // 55 candidate: a receiver through a pointer that may be nil, the call site is named
	_ = b.s.Drop("x")
}

func c56(ctx context.Context, l *logf.Logger, b *box) {
	l.Warn(ctx, "erase the build cache") // 56 candidate: an argument field through a pointer, the call site is named
	_ = dropPath(b.name)
}

func c57(ctx context.Context, l *logf.Logger, v *valueStore) {
	l.Warn(ctx, "erase the build cache") // 57 candidate: a value method through a pointer, the call site is named
	_ = v.Drop()
}

func c58(ctx context.Context, l *logf.Logger, a, b int) {
	l.Warn(ctx, "erase the build cache") // 58 candidate: arithmetic in an argument, the call site is named
	_ = dropN(a / b)
}

func c59(ctx context.Context, l *logf.Logger, s *store) {
	l.Warn(ctx, "erase the build cache") // 59 candidate: a method expression names its method
	_ = (*store).Drop(s, "x")
}

func c60(ctx context.Context, l *logf.Logger) {
	l.Warn(ctx, "the cache entries are stale and get dropped") // 60 candidate: a constant expression argument
	_ = dropPath(prefix + "cache")
}

func c61(ctx context.Context, l *logf.Logger, p string) {
	l.Warn(ctx, "the cache entries are stale and get dropped") // 61 candidate: &name
	_ = dropPtr(&p)
}

func c62(ctx context.Context, l *logf.Logger) {
	l.Warn(ctx, "the cache entries are stale and get dropped") // 62 candidate: a package variable argument
	_ = dropPath(defaultPath)
}

func c63(ctx context.Context, l *logf.Logger, b box) {
	l.Warn(ctx, "the cache entries are stale and get dropped") // 63 candidate: fields of a value, no pointer on the way
	_ = b.v.Drop()
	_ = dropPath(b.name)
}

func c64(ctx context.Context, l *logf.Logger, b box) {
	l.Warn(ctx, "the cache entries are stale and get dropped") // 64 candidate: a pointer field used as the receiver of a pointer method
	_ = b.s.Drop(b.name)
}
