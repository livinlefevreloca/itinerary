package orchestrator

// PreRunState - waiting for scheduled time
type PreRunState struct{}

func (s *PreRunState) Name() string { return "prerun" }
func (s *PreRunState) ToPending() *PendingState {
	return &PendingState{}
}
func (s *PreRunState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// PendingState - initial pre-execution phase
type PendingState struct{}

func (s *PendingState) Name() string { return "pending" }
func (s *PendingState) ToConditionRunning() *ConditionRunningState {
	return &ConditionRunningState{}
}
func (s *PendingState) ToContainerCreating() *ContainerCreatingState {
	return &ContainerCreatingState{}
}
func (s *PendingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// ConditionRunningState - checking pre-execution requirements and running associated actions
type ConditionRunningState struct{}

func (s *ConditionRunningState) Name() string { return "condition_running" }
func (s *ConditionRunningState) ToContainerCreating() *ContainerCreatingState {
	return &ContainerCreatingState{}
}
func (s *ConditionRunningState) ToFailed() *FailedState {
	return &FailedState{}
}
func (s *ConditionRunningState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// ContainerCreatingState - creating Kubernetes pod/container
type ContainerCreatingState struct{}

func (s *ContainerCreatingState) Name() string { return "container_creating" }
func (s *ContainerCreatingState) ToRunning() *RunningState {
	return &RunningState{}
}
func (s *ContainerCreatingState) ToFailed() *FailedState {
	return &FailedState{}
}
func (s *ContainerCreatingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// RunningState - job executing
type RunningState struct{}

func (s *RunningState) Name() string { return "running" }
func (s *RunningState) ToTerminating() *TerminatingState {
	return &TerminatingState{}
}
func (s *RunningState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// TerminatingState - job finishing/cleanup
type TerminatingState struct{}

func (s *TerminatingState) Name() string { return "terminating" }
func (s *TerminatingState) ToCompleted() *CompletedState {
	return &CompletedState{}
}
func (s *TerminatingState) ToFailed() *FailedState {
	return &FailedState{}
}
func (s *TerminatingState) ToPending() *PendingState {
	return &PendingState{}
}
func (s *TerminatingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// Terminal States

// CompletedState - completed successfully
type CompletedState struct{}

func (s *CompletedState) Name() string { return "completed" }

// FailedState - failed
type FailedState struct{}

func (s *FailedState) Name() string { return "failed" }
func (s *FailedState) ToPending() *PendingState {
	return &PendingState{}
}

// CancelledState - cancelled
type CancelledState struct{}

func (s *CancelledState) Name() string { return "cancelled" }

// OrphanedState - no heartbeats, assumed dead
type OrphanedState struct{}

func (s *OrphanedState) Name() string { return "orphaned" }
