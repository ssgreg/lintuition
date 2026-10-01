package a

import (
	"context"
	"log"
	"log/slog"
	"time"

	"github.com/ssgreg/logf"
)

type Config struct {
	Password string
	User     string
}

func Redact(s string) string    { return "" }
func HashToken(s string) string { return "" }

var msg = "dynamic"

func c1(cfg Config) {
	slog.Info("user logged in", "password", cfg.Password) // 1 candidate: a field from a selector
}

func c2() {
	slog.Info("started") // 2 none: no fields
}

func c3(tok string) {
	slog.Info("login", "token", Redact(tok)) // 3 none: the value is redacted
}

func c4(ctx context.Context, l *logf.Logger, tok string) {
	l.Info(ctx, "login", logf.String("jwt", tok)) // 4 candidate: a field constructor
}

func c5() {
	slog.Info("login", "password", "hunter2") // 5 none: a literal is in the source already
}

func c6(args []any) {
	slog.Info("login", args...) // 6 unsupported: fields passed on, not readable
}

func c7(ok bool, d time.Duration, at time.Time) {
	slog.Info("login", "has_password", ok, "took", d, "at", at) // 7 none: bool, duration, time
}

func c8(u, tok string) {
	slog.Info("login", "user", u, "token_hash", HashToken(tok), "secret", "hunter2") // 8 candidate: only u remains
}

func c9(tok string) {
	slog.Info("login", "token", tok[:4]) // 9 unsupported: an expression the analyzer cannot name
}

func c10(pw string) {
	log.Println("password", pw) // 10 none: Println's arguments are not fields
}

func c11(pw string) {
	slog.Info("login", "pass:word", pw) // 11 unsupported: the key cannot be a fact
}

func c12(s string) {
	slog.Info(msg, "secret", s) // 12 candidate: the message is not known, the fields are
}

func c13(ctx context.Context, l *logf.Logger, err error) {
	_ = logf.Error(err) // 13 none: a field constructor is not a log call
}

func c14(key []byte, p *Config) {
	slog.Info("key loaded", "key", key, "cfg", p) // 14 candidate: types written as facts
}

func c15(u string, attrs []slog.Attr) {
	slog.Info("login", "user", u, attrs) // 15 candidate: readable fields beside unreadable ones
}

func c16(requestID, key, password string) {
	slog.Info("login", "request_id", requestID, slog.String(key, password)) // 16 candidate: a readable field beside a dynamic key, and the unread part
}
