package orchestrator

import "time"

// State is the interface that all orchestrator states must implement
type State interface {
	Name() string
}

// StateRecorder tracks state transitions for testing
type StateRecorder struct {
	path []string
}

// NewStateRecorder creates a new StateRecorder
func NewStateRecorder() *StateRecorder {
	return &StateRecorder{path: make([]string, 0)}
}

// Record records a state transition
func (r *StateRecorder) Record(state State) {
	r.path = append(r.path, state.Name())
}

// Path returns the recorded state transition path
func (r *StateRecorder) Path() []string {
	return r.path
}

// Phase timing boundaries (stored separately from states)
type PhaseTiming struct {
	CreatedAt              time.Time
	ConstraintCheckStarted time.Time
	ExecutionStartedAt     time.Time
	CompletedAt            time.Time
}
