package scheduler

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// OrchestratorState tracks the state of a running orchestrator
type OrchestratorState struct {
	RunID            string
	JobID            string
	JobConfig        *Job // Current job configuration
	ScheduledAt      time.Time
	ActualStart      time.Time
	Status           OrchestratorStatus
	CancelChan       chan struct{}
	ConfigUpdate     chan *Job // For updating config while in PreRun
	CompletedAt      time.Time
	LastHeartbeat    time.Time // Last time heartbeat was received
	MissedHeartbeats int       // Consecutive missed heartbeats
	RetryCount       int       // Number of retries attempted
}

// Job represents a job definition (placeholder for future implementation)
type Job struct {
	ID       string
	Schedule string
}

// generateRunID creates a deterministic run ID from job ID and scheduled time
func generateRunID(jobID string, scheduledAt time.Time) string {
	// Deterministic runID: jobID:unixTimestamp
	// Same job at same time always produces same runID
	return fmt.Sprintf("%s:%d", jobID, scheduledAt.Unix())
}

// generateUpdateID creates a unique ID for database update deduplication
func generateUpdateID() string {
	// Use UUID for update deduplication in database
	return uuid.New().String()
}

// runOrchestrator is the main orchestrator goroutine stub implementation
func (s *Scheduler) runOrchestrator(jobID string, scheduledAt time.Time, runID string,
	cancelChan chan struct{}, configUpdate chan *Job) {

	// Helper to send heartbeat
	sendHeartbeat := func() {
		s.inbox.Send(InboxMessage{
			Type: MsgOrchestratorHeartbeat,
			Data: OrchestratorHeartbeatMsg{
				RunID:     runID,
				Timestamp: time.Now(),
			},
		})
	}

	// PreRun state: wait for scheduled time or config updates
	// Send heartbeats while waiting to prove orchestrator is alive
	waitDuration := time.Until(scheduledAt)
	if waitDuration > 0 {
		heartbeatTicker := time.NewTicker(s.config.OrchestratorHeartbeatInterval)
		defer heartbeatTicker.Stop()

		deadline := time.After(waitDuration)
		for {
			select {
			case <-deadline:
				// Time to start
				goto START

			case <-heartbeatTicker.C:
				// Send heartbeat to prove we're alive
				sendHeartbeat()

			case newConfig := <-configUpdate:
				// Config updated while waiting
				s.logger.Info("received config update in PreRun",
					"run_id", runID,
					"job_id", newConfig.ID)
				// Continue waiting with updated config

			case <-cancelChan:
				// Cancelled before start
				s.inbox.Send(InboxMessage{
					Type: MsgOrchestratorComplete,
					Data: OrchestratorCompleteMsg{
						RunID:       runID,
						Success:     false,
						CompletedAt: time.Now(),
						Error:       errors.New("cancelled in PreRun"),
					},
				})
				return
			}
		}
	}

START:
	// Notify state change to Pending
	s.inbox.Send(InboxMessage{
		Type: MsgOrchestratorStateChange,
		Data: OrchestratorStateChangeMsg{
			RunID:     runID,
			NewStatus: OrchestratorPending,
			Timestamp: time.Now(),
		},
	})

	// TODO: Actual orchestrator implementation with retry logic
	// For now, simulate execution with heartbeats during execution
	// In real implementation, this would:
	// 1. Detect if job has retry configuration
	// 2. Track retry attempts (stored in orchestrator state)
	// 3. Send MsgOrchestratorStateChange with OrchestratorRetrying status
	// 4. This prevents race where retry pod starts before grace period expires

	executionDuration := 250 * time.Millisecond
	executionDeadline := time.After(executionDuration)
	heartbeatTicker := time.NewTicker(s.config.OrchestratorHeartbeatInterval)
	defer heartbeatTicker.Stop()

	// Execution loop - send heartbeats to prove orchestrator is alive
	for {
		select {
		case <-executionDeadline:
			// Execution complete
			goto COMPLETE

		case <-heartbeatTicker.C:
			// Send heartbeat during execution
			sendHeartbeat()

		case <-cancelChan:
			// Cancelled during execution
			s.inbox.Send(InboxMessage{
				Type: MsgOrchestratorComplete,
				Data: OrchestratorCompleteMsg{
					RunID:       runID,
					Success:     false,
					CompletedAt: time.Now(),
					Error:       errors.New("cancelled during execution"),
				},
			})
			return
		}
	}

COMPLETE:
	s.inbox.Send(InboxMessage{
		Type: MsgOrchestratorComplete,
		Data: OrchestratorCompleteMsg{
			RunID:       runID,
			Success:     true,
			CompletedAt: time.Now(),
			Error:       nil,
		},
	})
}
