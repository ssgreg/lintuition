// Package enumcomment holds twins for enum-comment-shift.
package enumcomment

// Phase is a step of a replication job.
type Phase int

// Defect: a phase was inserted and the comments shifted by one.
const (
	// PhaseQueued is a replication job waiting for a free worker.
	PhaseQueued Phase = iota
	// PhaseCopying means every block has been copied and verified on the target.
	PhaseCopying // want `comment describes PhaseVerified, not PhaseCopying`
	PhaseVerified
)

// Mode is how a replica is opened.
type Mode int

// Fixed twin: each comment is on its own constant.
const (
	// ModeReadOnly opens the replica without letting writes through.
	ModeReadOnly Mode = iota
	// ModeReadWrite opens the replica for writes too.
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
