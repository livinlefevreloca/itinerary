package orchestrator

import (
	"context"
	"encoding/json"
	"time"
)

// OrchestratorStatus represents the current state of an orchestrator
type OrchestratorStatus int

const (
	// Pre-execution states
	OrchestratorPreRun           OrchestratorStatus = iota // Created, waiting for start time
	OrchestratorPending                                    // Initial pre-execution phase
	OrchestratorConditionPending                           // About to check pre-execution requirements
	OrchestratorConditionRunning                           // Checking pre-execution requirements
	OrchestratorActionPending                              // Requirement failed, about to take action
	OrchestratorActionRunning                              // Taking action based on requirement outcome

	// Execution states
	OrchestratorContainerCreating // Creating Kubernetes pod/container
	OrchestratorRunning           // Job executing
	OrchestratorTerminating       // Job finishing/cleanup

	// Retry state
	OrchestratorRetrying // After failure, before retry

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
	case OrchestratorConditionPending:
		return "condition_pending"
	case OrchestratorConditionRunning:
		return "condition_running"
	case OrchestratorActionPending:
		return "action_pending"
	case OrchestratorActionRunning:
		return "action_running"
	case OrchestratorContainerCreating:
		return "container_creating"
	case OrchestratorRunning:
		return "running"
	case OrchestratorTerminating:
		return "terminating"
	case OrchestratorRetrying:
		return "retrying"
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

// Job represents a job definition
type Job struct {
	ID       string
	Name     string
	Schedule string
	PodSpec  string

	// Opaque to orchestrator - interpreted by constraint/action modules
	ConstraintConfig json.RawMessage
	ActionConfig     json.RawMessage

	// Orchestrator needs to understand this for retry coordination
	RetryConfig *RetryConfig
}

// RetryConfig defines retry behavior for a job
type RetryConfig struct {
	MaxRetries        int
	InitialDelay      time.Duration
	BackoffMultiplier float64
	MaxDelay          time.Duration
}

// ConstraintCheckResult is returned by constraint checker
type ConstraintCheckResult struct {
	ShouldProceed bool   // false if constraints prevent execution
	Message       string // Summary of constraint evaluation and actions taken
}

// ConstraintChecker evaluates pre and post execution constraints
type ConstraintChecker interface {
	// CheckPreExecution evaluates all pre-execution constraints
	CheckPreExecution(ctx context.Context, job *Job, runID string) (ConstraintCheckResult, error)

	// CheckPostExecution evaluates all post-execution constraints
	CheckPostExecution(ctx context.Context, job *Job, runID string, startTime, endTime time.Time, exitCode int) (ConstraintCheckResult, error)

	// ShouldRecheckOnRetry returns true if any constraints need to be re-evaluated on retry
	ShouldRecheckOnRetry(job *Job) bool
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
