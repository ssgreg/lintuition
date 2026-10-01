// Package sentinelname holds twins for sentinel-name-vs-text.
package sentinelname

import (
	"errors"
	"fmt"
)

// Defect: a copied declaration whose message was not edited to match the name.
var ErrQuotaExceeded = errors.New("tenant record not found") // want `sentinel error name and message describe different conditions: ErrQuotaExceeded`

// Fixed twin: the message says what the name says.
var ErrTenantQuotaExceeded = errors.New("tenant storage quota exceeded")

// Negative: too vague to tell is not a finding.
var ErrBackend = errors.New("backend hiccup")

// Negative: weak support for different abstains.
var ErrStale = errors.New("cached lease is outdated")

// Negative: a wrapping sentinel is not asked; its message is not the whole condition.
var ErrTenantGone = fmt.Errorf("%w: tenant", ErrQuotaExceeded)
