package scheduler

import (
	"fmt"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/model"
)

// OrchestratorState tracks the state of an orchestrator in the scheduler
type OrchestratorState struct {
	RunID            string
	JobID            string
	JobConfig        *model.Job // Current job configuration
	ScheduledAt      time.Time
	ActualStart      time.Time
	Status           OrchestratorStatus
	CancelChan       chan struct{}
	ConfigUpdate     chan *model.Job // For updating config while in PreRun
	CompletedAt      time.Time
	LastHeartbeat    time.Time // Last time heartbeat was received
	MissedHeartbeats int       // Consecutive missed heartbeats
}

// OrchestratorStatus represents the current state of an orchestrator
type OrchestratorStatus int

const (
	// Pre-execution states
	OrchestratorPreRun           OrchestratorStatus = iota // Created, waiting for start time
	OrchestratorPending                                    // Initial pre-execution phase
	OrchestratorConditionRunning                           // Checking pre-execution constraints and running associated actions

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

// generateRunID creates a deterministic run ID from job ID and scheduled time
func generateRunID(jobID string, scheduledAt time.Time) string {
	return fmt.Sprintf("%s:%d", jobID, scheduledAt.Unix())
}

// generateUpdateID creates a unique ID for database updates
// TODO: Replace with proper UUID generation
func generateUpdateID() string {
	return fmt.Sprintf("update_%d", time.Now().UnixNano())
}
