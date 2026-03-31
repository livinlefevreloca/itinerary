package orchestrator

import "time"

// OrchestratorStatus represents the current state of an orchestrator
type OrchestratorStatus int

const (
	// Pre-execution states
	OrchestratorPreRun           OrchestratorStatus = iota // Created, waiting for start time
	OrchestratorPending                                    // Initial pre-execution phase
	OrchestratorConditionRunning                           // Checking pre-execution requirements and running associated actions

	// Execution states
	OrchestratorContainerCreating // Creating Kubernetes pod/container
	OrchestratorRunning           // Job executing
	OrchestratorTerminating       // Job finishing/cleanup

	// Terminal states
	OrchestratorCompleted // Completed successfully
	OrchestratorFailed    // Failed
	OrchestratorCancelled // Cancelled
	OrchestratorOrphaned  // No heartbeats, assumed dead
)

// String returns a human-readable representation of the orchestrator status
func (s OrchestratorStatus) String() string {
	switch s {
	case OrchestratorPreRun:
		return "prerun"
	case OrchestratorPending:
		return "pending"
	case OrchestratorConditionRunning:
		return "condition_running"
	case OrchestratorContainerCreating:
		return "container_creating"
	case OrchestratorRunning:
		return "running"
	case OrchestratorTerminating:
		return "terminating"
	case OrchestratorCompleted:
		return "completed"
	case OrchestratorFailed:
		return "failed"
	case OrchestratorCancelled:
		return "cancelled"
	case OrchestratorOrphaned:
		return "orphaned"
	default:
		return "unknown"
	}
}

// OrchestratorHeartbeatMsg is sent periodically to prove liveness
type OrchestratorHeartbeatMsg struct {
	RunID     string
	Timestamp time.Time
}

// OrchestratorStateChangeMsg notifies of state transitions
type OrchestratorStateChangeMsg struct {
	RunID     string
	NewStatus OrchestratorStatus
	Timestamp time.Time
}

// OrchestratorCompleteMsg notifies of orchestrator completion
type OrchestratorCompleteMsg struct {
	RunID       string
	Success     bool
	CompletedAt time.Time
	Error       error
}
