package sdk

import (
	"fmt"
	"math"
)

// Threshold returns the configured probability threshold, or def when it is not set. It must be a
// finite number in (0, 1]: NaN compares false with everything, so it would pass every check.
func Threshold(v *float64, def float64) (float64, error) {
	if v == nil {
		return def, nil
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) || *v <= 0 || *v > 1 {
		return 0, fmt.Errorf("threshold %v must be a number in (0, 1]", *v)
	}
	return *v, nil
}
