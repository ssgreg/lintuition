// Package logsensitive holds twins for log-sensitive-field.
package logsensitive

import "log/slog"

type Request struct {
	User     string
	Password string
	TokenID  string
}

func Redact(s string) string { return "***" }

// Defect: the password itself is logged.
func signInDefect(req Request) {
	slog.Info("user signed in", "password", req.Password) // want `log field value looks like a secret: password`
}

// Fixed twin: the user is logged, not the password.
func signInFixed(req Request) {
	slog.Info("user signed in", "user", req.User)
}

// Negative: a key named after a secret whose value is only its id is metadata.
func tokenRotated(req Request) {
	slog.Info("token rotated", "token_id", req.TokenID)
}

// Negative: a redacted value is not the secret; it is not even asked about.
func signInRedacted(req Request) {
	slog.Info("user signed in", "password", Redact(req.Password))
}
