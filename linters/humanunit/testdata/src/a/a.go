package a

import (
	"fmt"
	"log"
	"math"
	"time"

	pkgerrors "github.com/pkg/errors"
)

type Logger struct{}

func (Logger) Infof(format string, args ...any) {}

// Notf ends in f but its last parameter is not ...any.
func Notf(format string, args ...string) {}

// Duration is not time.Duration.
type Duration int64

func (Duration) Seconds() float64 { return 0 }

var format = "took %d ms"

// Millis is a numeric named type that prints its own number.
type Millis float64

func (m Millis) String() string { return fmt.Sprintf("%.0f", float64(m)*1000) }

func cases(d time.Duration, l Logger, err error, n int, own Duration, args []any) {
	log.Printf("request took %.0f ms", d.Seconds())                                          // 1 candidate: seconds
	fmt.Printf("retry %d of %d in %d ms\n", n, n, d.Milliseconds())                          // 2 candidate: third verb
	l.Infof("waited %d s", int(d.Minutes()))                                                 // 3 candidate: a method of a logger, under a conversion
	_ = pkgerrors.Wrapf(err, "timed out after %.1f min", math.Round(d.Hours()))              // 4 candidate: Wrapf, under math.Round
	_ = fmt.Sprintf("%*d us", n, d.Microseconds())                                           // 5 candidate: a * width consumes an argument
	_ = fmt.Sprintf("100%% done in %d ns", d.Nanoseconds())                                  // 6 candidate: %% is not a verb
	fmt.Printf("waited %v", d)                                                               // 7 none: a Duration prints its own unit
	fmt.Printf("waited %s", d.String())                                                      // 8 none: not a unit method
	fmt.Printf(format, d.Milliseconds())                                                     // 9 unsupported: the format is a variable
	fmt.Printf("took %[1]d ms", d.Milliseconds())                                            // 10 unsupported: explicit argument index
	fmt.Printf("took ms", d.Milliseconds())                                                  // 11 unsupported: no verb for the argument
	fmt.Printf("took %d ms", args...)                                                        // 12 none: a spread slice cannot be bound
	fmt.Println("took", d.Seconds(), "s")                                                    // 13 none: not printf-style
	Notf("took %d s", "x")                                                                   // 14 none: not ...any
	fmt.Printf("took %.1f s", own.Seconds())                                                 // 15 none: not time.Duration
	fmt.Printf("took %.1f s and %d ms", d.Seconds(), d.Milliseconds())                       // 16 candidate: two verbs, two candidates
	_ = fmt.Sprintf("%5% seconds; one %f seconds; two %d ms", d.Seconds(), d.Milliseconds()) // 17 candidate: a decorated percent consumes no argument
	_ = fmt.Sprintf("elapsed %v ms", Millis(d.Seconds()))                                    // 18 none: a named type prints its own number
	_ = fmt.Sprintf("elapsed %s ms", string(rune(d.Milliseconds())))                         // 19 none: a string conversion is not the number
	_ = fmt.Sprintf("elapsed %d s", int64(float32(d.Seconds())))                             // 20 candidate: nested basic numeric conversions
}
