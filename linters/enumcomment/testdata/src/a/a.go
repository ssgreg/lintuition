package a

type State int

// 1 comments on constants of a block are candidates; the block's own doc comment is not
const (
	// a job nobody has picked up yet
	StateIdle    State = iota
	StateRunning       // the job finished and its result is stored
	StateDone
	_ // skipped values are not options
)

// 2 a single constant in a block is not a candidate
const (
	One = 1 // the only one
)

// 3 not in a parenthesised block
const Lone = 2 // a lone constant

// 4 a comment on a line of several constants is unsupported
const (
	A, B = 1, 2 // two at once
	C    = 3
)

// 5 a directive alone is not a comment
const (
	//nolint:revive // a directive only is not a comment
	X = 1
	Y = 2
)

// 6 a constant named like the answer option "none" is unsupported
const (
	none  = 1 // named like an option
	other = 2
)

// 7 a constant named like the answer option "unclear" is unsupported
const (
	unclear = 1 // named like an option
	plenty  = 2
)

// 8 a block inside a function is read the same way
func f() {
	const (
		// inside a function
		LocalA = iota
		LocalB
	)
	_, _ = LocalA, LocalB
}

// 9 a comment that opens with its own constant's name is taken at its word
const (
	// ModeFast skips the checksum.
	ModeFast = iota
	// ModeSafe verifies the checksum of every block that is checked.
	ModeSafe
)

// 10 the own name anywhere in the comment counts, also where the rest reads like a neighbour
const (
	// Enable extra checks while developing.
	debug = false
	// If trace is set, debugging output is printed.
	trace = false
)

// 11 a comment that names its own constant and a neighbour is taken at its word too
const (
	// readIdle and writeIdle cut a connection that stalls.
	readIdle  = 60
	writeIdle = 60
)

// 12 a comment that names another constant of the block but not its own is asked, as written
const (
	// PhaseVerified means every block was checked.
	PhaseCopying = iota
	PhaseVerified
)

// 13 a one-letter name never counts as named: it would match the article
const (
	// A placeholder until the value is known.
	P = iota
	Q
)

// 14 the own name inside a longer word does not count
const (
	// Running jobs are counted here.
	Run = iota
	Stop
)

// 15 comments that differ in one word (here a number) are unsupported, every one of them
const (
	// the vendor caps this at 250 characters.
	limitTitle = 250
	// the vendor caps this at 1024 characters.
	limitBody = 1024
	// the vendor caps this at 512 characters.
	limitLink = 512
)

// 16 identical comments are unsupported, doc against line comment too
const (
	// reserved
	codeOne = 1
	codeTwo = 2 // reserved
	codeSix = 6 // a value of its own
)

// 17 two-word comments that differ in one word are asked
const (
	inTimeout  = 1 // read timeout
	outTimeout = 2 // write timeout
)

// 18 three words that differ in two are asked
const (
	kindHead = 1 // head of loop
	kindTail = 2 // block after loop
	kindBody = 3 // body of switch
)

// 19 the template ignores case, surrounding punctuation and which constant of the block is named
const (
	ruleBase  = 0
	ruleRead  = 1 // Same rule as ruleBase, for reads.
	ruleWrite = 2 // same rule as (ruleBase) for writes
)

// 20 the own name in one comment of a constant does not anchor its other comment
const (
	// FlagOn is set by default.
	FlagOn  = true // turned off by the operator
	FlagOff = false
)

// 21 a blank constant's comment is not a candidate
const (
	First = iota
	// a gap in the numbering
	_
	Third
)
