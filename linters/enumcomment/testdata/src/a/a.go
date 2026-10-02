package a

type State int

// 1 comments on constants of a block are candidates, sent as written; the block's own doc is not
const (
	// a job nobody has picked up yet
	StateIdle    State = iota
	StateRunning       // the job finished and its result is stored
	StateDone
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

// 6 a constant named like the answer option "none" makes every comment of the block unsupported
const (
	none  = 1 // named like an option
	other = 2 // other is fine
)

// 7 a constant named like the answer option "unclear" is unsupported too
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

// 9 a comment that opens with its own name is sent with only that opening replaced
const (
	// ModeFast skips the checksum; ModeFast is the default.
	ModeFast = iota
	// ModeSafe: verifies the checksum of every block.
	ModeSafe
)

// 10 a stale description under the right name is asked the same way
const (
	// PhaseCopying means every block was copied and checked.
	PhaseCopying = iota
	PhaseChecked
)

// 11 the own name further on is not an opening: the comment is sent as written
const (
	// Enable extra checks while developing.
	debug = false
	// If trace is set, debugging output is printed.
	trace = false
)

// 12 a comment that opens with another constant's name is sent as written
const (
	// StepVerified means every block was checked.
	StepCopying = iota
	StepVerified
)

// 13 it is asked although it mentions its own constant further on
const (
	// ReadWrite permits reads and writes, and everything ReadOnly permits.
	ReadOnly = iota
	ReadWrite
)

// 14 an opening that names several constants is a group note: unsupported
const (
	// readIdle and writeIdle cut a connection that stalls.
	readIdle = 60
	// readIdle, writeIdle: both restart on every byte.
	writeIdle = 60
	// writeIdle or readIdle, whichever runs out first, wins.
	anyIdle = 60
)

// 15 a reference to a neighbour after the own opening stays in the text
const (
	// LevelLow is quiet.
	LevelLow = iota
	// LevelHigh is like LevelLow, but louder.
	LevelHigh
)

// 16 a one-letter name never counts: neither as an opening nor further on
const (
	// A placeholder until the value is known.
	P = iota
	// P is a letter here.
	Q
)

// 17 the own name inside a longer word does not count
const (
	// Running jobs are counted here.
	Run = iota
	Stop
)

// 18 names are Go identifiers: letters beyond ASCII belong to them
const (
	// ÉtatPrêt means the worker can take a job.
	ÉtatPrêt = iota
	// StatoΩ is the idle state.
	StatoΩ
	// PréReady describes a queued job.
	Ready
	PréReady
)

// 19 comments of different constants that differ only in a number are unsupported
const (
	// the vendor caps this at 250 characters.
	limitTitle = 250
	// the vendor caps this at 1024 characters.
	limitBody = 1024
	// the vendor caps this at 512 characters.
	limitLink = 512
)

// 20 identical comments of different constants are unsupported, doc against line comment too
const (
	// reserved
	codeOne = 1
	codeTwo = 2 // reserved
	codeSix = 6 // a value of its own
)

// 21 a doc and a line comment that say the same on one constant are one candidate
const (
	// the operation completed successfully
	WorkRunning = iota // the operation completed successfully
	WorkDone
)

// 22 a copied comment that opens with the other constant's name is asked; the original too
const (
	// CopyDone means the copy completed successfully.
	CopyRunning = iota
	// CopyDone means the copy completed successfully.
	CopyDone
)

// 23 comments that differ in a word naming a different constant each are asked
const (
	// the operation permits reads
	AccessWrite = iota
	// the operation permits writes
	AccessRead
)

// 24 a differing word that names no constant leaves a template: unsupported
const (
	KindForDone = 1 // block after ForStmt
	KindIfDone  = 2 // block after IfStmt
	KindForBody = 3 // body of ForStmt
)

// 25 a differing word that fits several constants' names does not single one out
const (
	readFast  = 1 // budget for reads
	readSlow  = 2
	writeFast = 3 // budget for writes
)

// 26 two-word comments that differ in one word are asked
const (
	inTimeout  = 1 // read timeout
	outTimeout = 2 // write timeout
)

// 27 three words that differ in two are asked
const (
	kindHead = 1 // head of loop
	kindTail = 2 // block after loop
	kindBody = 3 // body of switch
)

// 28 comments that open with their own names are compared without them
const (
	// SlotL is the sensor at the left wheel.
	SlotL = 0
	// SlotR is the sensor at the right wheel.
	SlotR = 1
)

// 29 the template ignores case, punctuation and which constant of the block is named
const (
	ruleBase  = 0
	ruleRead  = 1 // Same rule as ruleBase, up to 5.
	ruleWrite = 2 // same rule as (ruleBase) up to 9
)

// 30 the own name in one comment of a constant does not change its other comment
const (
	// FlagOn is set by default.
	FlagOn  = true // turned off by the operator
	FlagOff = false
)

// 31 a blank constant's comment is not a candidate
const (
	First = iota
	// a gap in the numbering
	_
	Third
)

// 32 a word equal to a name's word beats one it only extends
const (
	KindPrint  = 1 // behaves like fmt.Print
	KindPrintf = 2 // behaves like fmt.Printf
)
