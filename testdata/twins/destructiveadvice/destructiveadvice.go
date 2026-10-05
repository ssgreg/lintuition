// Package destructiveadvice holds twins for destructive-remediation.
//
// Every explanation is a separate comment, a blank line above the function or the call, so it
// never reaches the classifier: only the message text is sent.
package destructiveadvice

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
)

// Defect: the advice destroys the stored backups and does not say so.

func openDefect() error {
	return errors.New("backup index mismatch; delete the repository directory and run init again") // want `advises a destructive step without saying what is lost`
}

// Fixed twin: the same step, with what it costs and how to keep it.

func openFixed() error {
	return errors.New("backup index mismatch; deleting the repository directory erases every stored backup, so copy it elsewhere first, then run init again")
}

// Negative: a non-destructive instruction that only shares a word with a destructive one.

func applySafe() error {
	return errors.New("nothing was changed; remove the --dry-run flag to apply the plan")
}

// Negative: the text reports a removal, it does not advise one.

func cleaned() error {
	return errors.New("lock file was removed by another process")
}

// Defect: a destructive step in words no keyword list holds.

func mountDefect() error {
	return errors.New("data volume is unreadable; format it and restart the node") // want `advises a destructive step without saying what is lost`
}

// Defect: a warning that tells the operator to throw away the index.

func indexDefect() {
	slog.Warn("search index checksum mismatch; drop the index folder and start the service again") // want `advises a destructive step without saying what is lost`
}

// Defect: an error log that tells the operator to wipe the ledger.

func ledgerDefect() {
	slog.Error("ledger file cannot be parsed; wipe the ledger and resync from scratch") // want `advises a destructive step without saying what is lost`
}

// Fixed twin: the same kind of advice, saying what goes and that it comes back.

func thumbsFixed() {
	slog.Warn("thumbnail cache is corrupt; removing it loses only cached thumbnails, which are rebuilt on the next start, so delete it and restart")
}

type replicas struct{}

func (replicas) Drop(name string) error { return nil }

// Negative: the program announces its own deletion at info and then does it.

func expire(dir string) error {
	slog.Info("drop expired archive")
	return os.RemoveAll(dir)
}

// Negative: the program announces its own deletion at debug.

func purge() {
	slog.Debug("purge the upload queue")
}

// Negative: a warning that names the program's own decision, followed by the deletion it names.

func evict(r replicas) error {
	slog.Warn("replica failed its consistency check, the replica has to be dropped")
	return r.Drop("standby")
}

// Negative: an error prefix names the operation that failed, the Go way.

func purgeBucket(err error) error {
	return fmt.Errorf("purge staging bucket: %w", err)
}

// Negative: a routine warning, no advice at all.

func usage() {
	slog.Warn("disk usage is above 80 percent")
}
