// Package readonlypromise holds twins for read-only-promise.
//
// Every explanation is a separate comment, a blank line above the doc, so it never reaches the
// classifier.
package readonlypromise

type Queue struct {
	head  int
	items []int
	hits  int
}

// Defect: a peek that advances the queue.

// Peek returns the next item without altering the queue.
func (q *Queue) Peek() int { q.head++; return q.items[q.head-1] } // want `doc of Peek promises to leave q unchanged, but Peek writes q.head`

// Fixed twin: the peek only reads.

// Front returns the next item without altering the queue.
func (q *Queue) Front() int { return q.items[q.head] }

// Defect: a "pure" function that counts its calls in a package variable.

var calls int

// Score is pure: the same input gives the same result.
func Score(n int) int { calls++; return n * 2 } // want `doc of Score promises it changes nothing, but Score writes calls`

// Fixed twin.

// Double is pure: the same input gives the same result.
func Double(n int) int { return n * 2 }

// Defect: the parameter is promised and sorted in place.

// Median returns the median of xs, leaving xs unchanged.
func Median(xs []int) int { // want `doc of Median promises to leave xs unchanged, but Median writes xs\[j\]`
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
	return xs[len(xs)/2]
}

// Fixed twin: it sorts a copy.

// MedianOf returns the median of xs, leaving xs unchanged.
func MedianOf(xs []int) int {
	c := append([]int(nil), xs...)
	for i := 1; i < len(c); i++ {
		for j := i; j > 0 && c[j] < c[j-1]; j-- {
			c[j], c[j-1] = c[j-1], c[j]
		}
	}
	return c[len(c)/2]
}

type Cache struct {
	m map[string]string
}

// Defect: a value receiver, but the map is shared.

// Lookup returns the value for k and does not modify the cache.
func (c Cache) Lookup(k string) string { delete(c.m, ""); return c.m[k] } // want `doc of Lookup promises to leave c unchanged, but Lookup writes c.m`

// Negative: the promise is about the argument, the write is to the receiver.

// Total adds up xs without modifying xs.
func (q *Queue) Total(xs []int) int { q.hits++; return len(xs) }

// Negative: a promise about something else.

// Flush writes pending items to disk; it does not change the file's permissions.
func (q *Queue) Flush() { q.items = nil }

// Negative: the doc says what changes, the no-change words are about another thing.

// Reset empties the queue; the capacity stays unchanged.
func (q *Queue) Reset() { q.items = q.items[:0]; q.head = 0 }

// Negative: a value receiver's own copy.

// Rewound returns a copy rewound to the start, without modifying q.
func (q Queue) Rewound() Queue { q.head = 0; return q }

// Negative: weak support abstains.

// Touch records an access; the contents stay as they are, so callers see no change.
func (q *Queue) Touch() { q.hits++ }

// Negative: the write is in a returned closure; not asked.

// Later returns a function that rewinds q; calling Later does not change q.
func (q *Queue) Later() func() { return func() { q.head = 0 } }

// Negative: the promise holds only on failure.

// TryPop removes the head and returns true; on an empty queue it returns false and leaves the queue unchanged.
func (q *Queue) TryPop() bool {
	if len(q.items) == 0 {
		return false
	}
	q.items = q.items[1:]
	return true
}
