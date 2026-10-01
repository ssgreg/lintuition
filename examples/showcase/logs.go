package showcase

import (
	"errors"
	"log"
	"log/slog"
)

type Config struct{ Password string }

type Store interface{ SaveConfig(cfg Config) error }

// premature-success: "saved" is logged before the save, which can still fail.
func saveConfig(store Store, cfg Config) error {
	log.Print("config saved to disk") // want `success logged before SaveConfig has returned`
	if err := store.SaveConfig(cfg); err != nil {
		return err
	}
	return nil
}

// log-sensitive-field: the password itself goes into the log.
func connect(logger *slog.Logger, cfg Config) {
	logger.Info("connecting", slog.String("password", cfg.Password)) // want `log field value looks like a secret`
}

// log-key-value-role: the key says remaining, the value is the elapsed count.
func retry(elapsedAttempts int) {
	slog.Info("retry", "remaining_attempts", elapsedAttempts) // want `log key names a different quantity than its value`
}

// normal-event-at-error: a routine cache miss at error level.
func load(key string) {
	slog.Error("cache miss, loading from the database", "key", key) // want `routine event logged at error level`
}

// severe-event-understated: lost data at info level.
func flush() {
	slog.Info("events for the last hour are lost, the write to disk was refused") // want `unintended loss logged at info level`
}

// destructive-remediation: delete advice without saying what is lost.
func openState() error {
	return errors.New("state is corrupted, delete the data directory and restart") // want `advises a destructive step without saying what is lost`
}
