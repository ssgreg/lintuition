// Package destructiveadvice holds twins for destructive-remediation.
package destructiveadvice

import "errors"

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
