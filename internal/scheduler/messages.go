package scheduler

import (
	"time"

	"github.com/livinlefevreloca/itinerary/internal/inbox"
)

// InboxMessage is the container for all messages sent to the scheduler
type InboxMessage struct {
	Type         MessageType
	Data         interface{}
	ResponseChan chan<- interface{} // Optional, for request/response pattern
}

// MessageType identifies the type of message being sent to the scheduler
type MessageType int

const (
	// From orchestrators
	MsgOrchestratorStateChange MessageType = iota // Generic state change
	MsgOrchestratorComplete                       // Orchestrator completed
	MsgOrchestratorFailed                         // Orchestrator failed
	MsgOrchestratorHeartbeat                      // Periodic heartbeat from orchestrator

	// From watcher
	MsgCancelRun       // Cancel a running job
	MsgUpdateRunConfig // Update config while in PreRun

	// State queries
	MsgGetOrchestratorState // Request state of specific orchestrator
	MsgGetAllActiveRuns     // Request all active orchestrators
	MsgGetStats             // Request scheduler statistics

	// Control
	MsgShutdown // Shutdown the scheduler
)

// String returns a human-readable representation of the message type
func (m MessageType) String() string {
	switch m {
	case MsgOrchestratorStateChange:
		return "orchestrator_state_change"
	case MsgOrchestratorComplete:
		return "orchestrator_complete"
	case MsgOrchestratorFailed:
		return "orchestrator_failed"
	case MsgOrchestratorHeartbeat:
		return "orchestrator_heartbeat"
	case MsgCancelRun:
		return "cancel_run"
	case MsgUpdateRunConfig:
		return "update_run_config"
	case MsgGetOrchestratorState:
		return "get_orchestrator_state"
	case MsgGetAllActiveRuns:
		return "get_all_active_runs"
	case MsgGetStats:
		return "get_stats"
	case MsgShutdown:
		return "shutdown"
	default:
		return "unknown"
	}
}

// OrchestratorStateChangeMsg is sent when an orchestrator changes state
type OrchestratorStateChangeMsg struct {
	RunID     string
	NewStatus OrchestratorStatus
	Timestamp time.Time
}

// OrchestratorCompleteMsg is sent when an orchestrator completes
type OrchestratorCompleteMsg struct {
	RunID       string
	Success     bool
	CompletedAt time.Time
	Error       error
}

// OrchestratorHeartbeatMsg is sent periodically by orchestrators to indicate they're alive
type OrchestratorHeartbeatMsg struct {
	RunID     string
	Timestamp time.Time
}

// CancelRunMsg requests cancellation of a running job
type CancelRunMsg struct {
	RunID string
}

// UpdateRunConfigMsg updates the configuration for a job while in PreRun state
type UpdateRunConfigMsg struct {
	RunID     string
	NewConfig *Job
}

// GetOrchestratorStateMsg requests the state of a specific orchestrator
type GetOrchestratorStateMsg struct {
	RunID string
}

// OrchestratorStateResponse is the response to GetOrchestratorStateMsg
type OrchestratorStateResponse struct {
	State *OrchestratorState
	Found bool
}

// GetAllActiveRunsMsg requests all active orchestrators (empty payload)
type GetAllActiveRunsMsg struct{}

// AllActiveRunsResponse is the response to GetAllActiveRunsMsg
type AllActiveRunsResponse struct {
	Runs []*OrchestratorState
}

// GetStatsMsg requests scheduler statistics (empty payload)
type GetStatsMsg struct{}

// StatsResponse is the response to GetStatsMsg
type StatsResponse struct {
	SchedulerStats SchedulerStats
	InboxStats     inbox.Stats
	SyncerStats    SyncerStats
}

// JobRunUpdate represents a database update for a job run
type JobRunUpdate struct {
	UpdateID    string // UUID for idempotent database writes
	RunID       string // Deterministic format: "jobID:unixTimestamp"
	JobID       string
	ScheduledAt time.Time
	CompletedAt time.Time
	Status      string
	Success     bool
	Error       error
}

// SchedulerIterationStats captures metrics from a single scheduler iteration
type SchedulerIterationStats struct {
	Timestamp               time.Time
	IterationDuration       time.Duration
	ActiveOrchestratorCount int
	IndexSize               int
	InboxDepth              int
	MessagesProcessed       int
	IndexBuildCount         int           // Total index builds performed
	LastIndexBuildDuration  time.Duration // Duration of most recent build
	MissedHeartbeatCount    int           // Total missed heartbeats detected
}

// SchedulerStats provides high-level scheduler statistics
type SchedulerStats struct {
	ActiveOrchestratorCount int
	IndexSize               int
	IndexBuildCount         int
	LastIndexBuildDuration  time.Duration
	MissedHeartbeatCount    int
}

// StatsUpdate is sent to the syncer for persistence
type StatsUpdate struct {
	Stats []SchedulerIterationStats
}
