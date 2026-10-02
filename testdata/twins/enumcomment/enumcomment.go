// Package enumcomment holds twins for enum-comment-shift.
//
// Every explanation is a separate comment, a blank line above the block, so it never reaches the
// classifier: in a const block, a comment right above a constant is that constant's comment.
package enumcomment

// Phase is a step of a replication job.
type Phase int

// Defect: a phase was inserted under the comment that named PhaseVerified, so the comment now sits
// on the new constant.

const (
	// PhaseQueued is a replication job waiting for a free worker.
	PhaseQueued Phase = iota
	// PhaseVerified means every block has been copied and checked on the target.
	PhaseCopying // want `comment describes PhaseVerified, not PhaseCopying`
	PhaseVerified
)

// Job is the state of a background job.
type Job int

// Defect: a state was removed and the comments below it moved up by one.

const (
	// waiting in the queue for a free worker
	JobQueued Job = iota
	// the result is stored and the job will not run again
	JobRunning // want `comment describes JobDone, not JobRunning`
	JobDone
)

// Code is the result of a call to a peer.
type Code int

// Defect, explicit values: CodeBusy was added between a comment and the constant it described.

const (
	CodeOK Code = 0
	// the peer did not answer before the deadline
	CodeBusy    Code = 1 // want `comment describes CodeTimeout, not CodeBusy`
	CodeTimeout Code = 2
	// the peer refused the credentials it was given
	CodeDenied Code = 3
)

// Reply is the fixed twin of Code: each comment is on its own constant.
type Reply int

// Fixed twin.

const (
	ReplyOK Reply = 0
	// the peer is handling too many calls to take another
	ReplyBusy Reply = 1
	// no answer came back from the peer before the deadline
	ReplyTimeout Reply = 2
)

// Mode is how a replica is opened.
type Mode int

// Fixed twin: each comment is on its own constant.

const (
	// opens the replica without letting writes through
	ModeReadOnly Mode = iota
	// opens the replica for writes too
	ModeReadWrite
)

// Tier is a storage class.
type Tier int

// Negatives: a section note describes no single constant, and a vague comment abstains.

const (
	// Replica storage tiers, fastest first.
	TierHot  Tier = iota
	TierWarm      // replica tier kept for a while
	TierCold
)

// Negative, near-identical neighbours: what tells the comments apart is a number, which only the
// values match, so they are unsupported rather than asked.

const (
	// the vendor API caps this field at 250 characters.
	maxTitleChars = 250
	// the vendor API caps this field at 4096 characters.
	maxBodyChars = 4096
	// the vendor API caps this field at 512 characters.
	maxLinkChars = 512
)

// Negative, near-identical neighbours in line comments.

const (
	stepEntry  = 1 // block before LoopStmt
	stepAfter  = 2 // block after LoopStmt
	stepBranch = 3 // block after BranchStmt
)

// Negative, a comment over a group that names both constants: taken at its word, not asked.

const (
	// readStallLimit and writeStallLimit cut a connection that stops moving data; the clock restarts
	// on every successful read or write.
	readStallLimit  = 60
	writeStallLimit = 60
)

// Negative, a comment over a group that names neither constant: it describes no single one.

const (
	// Stall limits, in seconds: the clock restarts on every successful read or write, so only a
	// connection that stops moving data is cut.
	readStallSeconds  = 60
	writeStallSeconds = 60
)

// Negative, a comment that names its own constant while its other words fit the neighbour.

const (
	// Turn on extra assertions while developing.
	assertions = false
	// When tracing is set, every assertion that runs is printed too.
	tracing = false
)
