package a

type State int

// Block doc comments are not candidates.
const (
	// StateIdle is a job nobody has picked up yet.
	StateIdle    State = iota
	StateRunning       // the job finished and its result is stored
	StateDone
	_ // skipped values are not options
)

const (
	One = 1 // a single constant in a block is not a candidate
)

const Lone = 2 // not in a parenthesised block

const (
	A, B = 1, 2 // on a line of several constants: unsupported
	C    = 3
)

const (
	//nolint:revive // a directive only is not a comment
	X = 1
	Y = 2
)

const (
	none  = 1 // named like an option: unsupported
	other = 2
)

func f() {
	const (
		// LocalA is inside a function.
		LocalA = iota
		LocalB
	)
	_, _ = LocalA, LocalB
}
