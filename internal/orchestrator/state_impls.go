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

func (s *ConditionPendingState) Name() string { return "condition_pending" }
func (s *ConditionPendingState) ToConditionRunning() *ConditionRunningState {
	return &ConditionRunningState{}
}
func (s *ConditionPendingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// ConditionRunningState - checking pre-execution requirements
type ConditionRunningState struct{}

func (s *ConditionRunningState) Name() string { return "condition_running" }
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

func (s *ActionPendingState) Name() string { return "action_pending" }
func (s *ActionPendingState) ToActionRunning() *ActionRunningState {
	return &ActionRunningState{}
}
func (s *ActionPendingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// ActionRunningState - taking action based on requirement outcome
type ActionRunningState struct{}

func (s *ActionRunningState) Name() string { return "action_running" }
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

func (s *ContainerCreatingState) Name() string { return "container_creating" }
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
func (s *TerminatingState) ToRetrying() *RetryingState {
	return &RetryingState{}
}
func (s *TerminatingState) ToCancelled() *CancelledState {
	return &CancelledState{}
}

// RetryingState - after failure, before retry
type RetryingState struct{}

func (s *RetryingState) Name() string { return "retrying" }
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

func (s *CompletedState) Name() string { return "completed" }

// FailedState - failed
type FailedState struct{}

func (s *FailedState) Name() string { return "failed" }
func (s *FailedState) ToRetrying() *RetryingState {
	return &RetryingState{}
}

// CancelledState - cancelled
type CancelledState struct{}

func (s *CancelledState) Name() string { return "cancelled" }

// OrphanedState - no heartbeats, assumed dead
type OrphanedState struct{}

func (s *OrphanedState) Name() string { return "orphaned" }
