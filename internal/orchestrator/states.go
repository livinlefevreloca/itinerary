package orchestrator

import "time"

// State is the interface that all orchestrator states must implement
type State interface {
	Name() string
}

// Recorder is an optional hook for recording state transitions, used in tests.
type Recorder interface {
	Record(name string)
}

// Phase timing boundaries (stored separately from states)
type PhaseTiming struct {
	CreatedAt              time.Time
	ConstraintCheckStarted time.Time
	ExecutionStartedAt     time.Time
	CompletedAt            time.Time
}
