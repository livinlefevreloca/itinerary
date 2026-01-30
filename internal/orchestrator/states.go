package orchestrator

import "time"

// State is a marker interface for all orchestrator states
type State interface {
	stateName() string
}

// PreRunState - waiting for scheduled time
type PreRunState struct{}

func (s *PreRunState) stateName() string { return "prerun" }
func (s *PreRunState) ToPending() *PendingState {
	return &PendingState{}
}
func (s *PreRunState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// PendingState - initial pre-execution phase
type PendingState struct{}

func (s *PendingState) stateName() string { return "pending" }
func (s *PendingState) ToConditionPending() *ConditionPendingState {
	return &ConditionPendingState{}
}
func (s *PendingState) ToContainerCreating() *ContainerCreatingState {
	return &ContainerCreatingState{}
}
func (s *PendingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// ConditionPendingState - about to check pre-execution requirements
type ConditionPendingState struct{}

func (s *ConditionPendingState) stateName() string { return "condition_pending" }
func (s *ConditionPendingState) ToConditionRunning() *ConditionRunningState {
	return &ConditionRunningState{}
}
func (s *ConditionPendingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// ConditionRunningState - checking pre-execution requirements
type ConditionRunningState struct{}

func (s *ConditionRunningState) stateName() string { return "condition_running" }
func (s *ConditionRunningState) ToActionPending() *ActionPendingState {
	return &ActionPendingState{}
}
func (s *ConditionRunningState) ToContainerCreating() *ContainerCreatingState {
	return &ContainerCreatingState{}
}
func (s *ConditionRunningState) ToFailed() *FailedState {
	return &FailedState{}
}
func (s *ConditionRunningState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// ActionPendingState - requirement failed, about to take action
type ActionPendingState struct{}

func (s *ActionPendingState) stateName() string { return "action_pending" }
func (s *ActionPendingState) ToActionRunning() *ActionRunningState {
	return &ActionRunningState{}
}
func (s *ActionPendingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// ActionRunningState - taking action based on requirement outcome
type ActionRunningState struct{}

func (s *ActionRunningState) stateName() string { return "action_running" }
func (s *ActionRunningState) ToContainerCreating() *ContainerCreatingState {
	return &ContainerCreatingState{}
}
func (s *ActionRunningState) ToCompleted() *CompletedState {
	return &CompletedState{}
}
func (s *ActionRunningState) ToFailed() *FailedState {
	return &FailedState{}
}
func (s *ActionRunningState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// ContainerCreatingState - creating Kubernetes pod/container
type ContainerCreatingState struct{}

func (s *ContainerCreatingState) stateName() string { return "container_creating" }
func (s *ContainerCreatingState) ToRunning() *RunningState {
	return &RunningState{}
}
func (s *ContainerCreatingState) ToFailed() *FailedState {
	return &FailedState{}
}
func (s *ContainerCreatingState) ToRetrying() *RetryingState {
	return &RetryingState{}
}
func (s *ContainerCreatingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// RunningState - job executing
type RunningState struct{}

func (s *RunningState) stateName() string { return "running" }
func (s *RunningState) ToTerminating() *TerminatingState {
	return &TerminatingState{}
}
func (s *RunningState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// TerminatingState - job finishing/cleanup
type TerminatingState struct{}

func (s *TerminatingState) stateName() string { return "terminating" }
func (s *TerminatingState) ToCompleted() *CompletedState {
	return &CompletedState{}
}
func (s *TerminatingState) ToFailed() *FailedState {
	return &FailedState{}
}
func (s *TerminatingState) ToRetrying() *RetryingState {
	return &RetryingState{}
}
func (s *TerminatingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// RetryingState - after failure, before retry
type RetryingState struct{}

func (s *RetryingState) stateName() string { return "retrying" }
func (s *RetryingState) ToConditionPending() *ConditionPendingState {
	return &ConditionPendingState{}
}
func (s *RetryingState) ToPending() *PendingState {
	return &PendingState{}
}
func (s *RetryingState) ToFailed() *FailedState {
	return &FailedState{}
}
func (s *RetryingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// Terminal States

// CompletedState - completed successfully
type CompletedState struct{}

func (s *CompletedState) stateName() string { return "completed" }

// FailedState - failed
type FailedState struct{}

func (s *FailedState) stateName() string { return "failed" }
func (s *FailedState) ToRetrying() *RetryingState {
	return &RetryingState{}
}

// CancelledState - cancelled
type CancelledState struct{}

func (s *CancelledState) stateName() string { return "cancelled" }

// OrphanedState - no heartbeats, assumed dead
type OrphanedState struct{}

func (s *OrphanedState) stateName() string { return "orphaned" }

// Helper to track state transitions for testing
type StateRecorder struct {
	path []string
}

func NewStateRecorder() *StateRecorder {
	return &StateRecorder{path: make([]string, 0)}
}

func (r *StateRecorder) Record(state State) {
	r.path = append(r.path, state.stateName())
}

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
