package a

type Queue struct {
	head  int
	items []int
	hits  int
}

var calls int

// Peek returns the next item without altering the queue.
func (q *Queue) Peek() int { q.head++; return q.items[q.head-1] } // 1 candidate: a receiver write

// Len returns the length without modifying q.
func (q *Queue) Len() int { return len(q.items) } // 2 none: no write

// Sum adds the values and does not modify xs.
func (q *Queue) Sum(xs []int) int { q.hits++; return 0 } // 3 candidate: writes the receiver, promise about xs

// Scale doubles every value, leaving xs unchanged.
func Scale(xs []int) { xs[0] *= 2 } // 4 candidate: a parameter write

// Count is pure.
func Count() int { calls++; return calls } // 5 candidate: a package-level write

// Snapshot returns a copy without modifying the queue.
func (q Queue) Snapshot() Queue { q.head = 0; return q } // 6 none: a value receiver's own copy

// Later returns a function; it does not modify q now.
func (q *Queue) Later() func() { return func() { q.head = 0 } } // 7 unsupported: the write is in a closure

// Push appends v to the queue.
func (q *Queue) Push(v int) { q.items = append(q.items, v) } // 8 candidate: every documented writer is asked

func (q *Queue) Bare() { q.head = 0 } // 9 none: no doc

// Reset is read-only.
func (q *Queue) Reset(_ int, n int) { q.head = n } // 10 candidate: a blank parameter is not an option

// Peek2 returns the head; it never changes the queue, but records a hit.
func (q *Queue) Peek2() int { q.hits++; return q.head } // 11 candidate: mixed doc, the classifier decides

// Fill writes into dst, leaving src unchanged.
func Fill(dst, src []int) { copy(dst, src) } // 12 candidate: dst is written, src is promised

type Stack []int

// Both rewinds q and clears s, without modifying anything else.
func (q *Queue) Both(s Stack) { q.head = 0; s[0] = 0; q.head = 1 } // 13 candidate: two roots, two candidates; a root once

//go:noinline
func (q *Queue) Directive() { q.head = 0 } // 14 none: the doc is only a directive

// Advance moves the head; m itself is left as it was.
func (q *Queue) Advance() { q.head++ } // 15 candidate: a promise without any no-change keyword

// Mixed rewinds q and schedules a later write to p, leaving both unchanged now.
func (q *Queue) Mixed(p *Queue) func() { q.head = 0; return func() { p.head = 1 } } // 16 candidate for q, unsupported for p

// Fresh leaves p unchanged.
func Fresh(p *int) { p = new(int); *p = 42 } // 17 unsupported: p holds fresh storage when written

// Both2 sets *p, then replaces p, leaving p unchanged.
func Both2(p *int) { *p = 1; p = new(int); *p = 2 } // 18 candidate: the write before the rebind is certain

// Restore leaves q unchanged at return.
func (q *Queue) Restore() { old := q.head; q.head = 0; q.head = old } // 19 none: a proven save and restore
