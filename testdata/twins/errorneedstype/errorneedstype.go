// Package errorneedstype holds twins for error-needs-type.
package errorneedstype

import (
	"errors"
	"fmt"
)

type Invoice struct{ Paid bool }

var invoices = map[string]*Invoice{}

// Defect: a caller can only match "invoice not found" by its text.
func findDefect(id string) (*Invoice, error) {
	inv, ok := invoices[id]
	if !ok {
		return nil, errors.New("invoice not found") // want `branchable condition returned as a plain string error`
	}
	return inv, nil
}

// ErrInvoiceNotFound is what findFixed returns for an unknown id.
var ErrInvoiceNotFound = errors.New("invoice does not exist")

// Fixed twin: the condition is a sentinel the caller can errors.Is.
func findFixed(id string) (*Invoice, error) {
	inv, ok := invoices[id]
	if !ok {
		return nil, ErrInvoiceNotFound
	}
	return inv, nil
}

// Negative: an internal invariant is not something a caller branches on.
func settle(inv *Invoice) error {
	if inv.Paid {
		return errors.New("invoice ledger invariant broken: settled twice")
	}
	inv.Paid = true
	return nil
}

// Negative: a wrapped error keeps the cause the caller can branch on; it is not asked.
func load(id string) (*Invoice, error) {
	if _, err := findFixed(id); err != nil {
		return nil, fmt.Errorf("load invoice %s: %w", id, err)
	}
	return invoices[id], nil
}

// Negative: an ambiguous answer abstains.
func validate(inv *Invoice) error {
	if inv == nil {
		return errors.New("invoice payload rejected")
	}
	return nil
}
