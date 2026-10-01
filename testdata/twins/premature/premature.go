// Package premature holds twins for premature-success.
package premature

import "log/slog"

type Store interface {
	SaveConfig(green bool) error
	Upload(name string) error
}

// Defect: success reported before the call that can fail.
func resetDefect(a Store) error {
	slog.Info("config saved to disk") // want `success logged before SaveConfig has returned`
	if err := a.SaveConfig(true); err != nil {
		return err
	}
	return nil
}

// Fixed twin: the call first, then the report.
func resetFixed(a Store) error {
	if err := a.SaveConfig(true); err != nil {
		return err
	}
	slog.Info("config saved to disk")
	return nil
}

// Negative: a message about starting the operation is not a success claim.
func resetStarting(a Store) error {
	slog.Info("saving config to disk")
	return a.SaveConfig(true)
}

// Negative: the message is about something else; no word in common, so it is not even asked.
func unrelated(a Store) error {
	slog.Info("cache warmed")
	return a.SaveConfig(true)
}

// Negative: progress context, not the outcome of the call.
func progress(a Store) error {
	slog.Info("upload 3 of 5")
	return a.Upload("x")
}
