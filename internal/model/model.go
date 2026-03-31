package model

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// =============================================================================
// Core Domain Types
// =============================================================================

// Job represents a scheduled job definition
type Job struct {
	ID        string
	Name      string
	Schedule  string
	PodSpec   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// JobRun represents a single execution of a job
type JobRun struct {
	JobID       string
	RunID       string
	ScheduledAt time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Status      string
	Success     *bool
	Error       *string
	Trigger     string // 'scheduled', 'manual', 'retry', 'action'
}

// =============================================================================
// Constraint & Action Configuration (DB rows)
// =============================================================================

// ConstraintType represents a dimension table entry for constraint types
type ConstraintType struct {
	ID   int
	Name string
}

// ActionType represents a dimension table entry for action types
type ActionType struct {
	ID   int
	Name string
}

// ConstraintConfig represents a constraint configuration record from the database
type ConstraintConfig struct {
	ID               string
	JobID            string
	ConstraintTypeID int
	Config           *string // JSON
	CreatedAt        time.Time
}

// ActionConfig represents an action configuration record from the database
type ActionConfig struct {
	ID           string
	ConstraintID string
	ActionTypeID int
	Trigger      string  // 'on_met', 'on_violated'
	Config       *string // JSON
	CreatedAt    time.Time
}

// =============================================================================
// Execution Records
// =============================================================================

// ConstraintRun represents a single execution of a constraint check
type ConstraintRun struct {
	ID           string
	RunID        string
	ConstraintID string
	ExecutedAt   time.Time
	Success      bool
	Violated     bool
	InError      bool
	Error        *string
	Details      *string // JSON
}

// ActionRun represents an action execution record
type ActionRun struct {
	ID              string
	RunID           string
	ConstraintRunID *string
	ActionID        string
	ExecutedAt      time.Time
	Success         bool
	Error           *string
	Details         *string // JSON
}

// =============================================================================
// Constraint Evaluation
// =============================================================================

// EvaluationPhase defines when a constraint should be evaluated
type EvaluationPhase string

const (
	EvaluationPhasePreExecution    EvaluationPhase = "pre"
	EvaluationPhaseDuringExecution EvaluationPhase = "during"
	EvaluationPhasePostExecution   EvaluationPhase = "post"
)

// ConstraintResult represents the result of evaluating a single constraint
type ConstraintResult struct {
	Met     bool
	Message string
}

// ConstraintCheckResult represents the aggregate result of evaluating all constraints
type ConstraintCheckResult struct {
	ShouldProceed bool
	Message       string
}

// =============================================================================
// Scheduler Query Types
// =============================================================================

// JobStateRequest queries the scheduler for job state
type JobStateRequest struct {
	JobID      string
	ResponseTo chan interface{}
}

// SetResponseChan implements RequestMessage
func (r *JobStateRequest) SetResponseChan(ch chan interface{}) {
	r.ResponseTo = ch
}

// JobStateResponse contains job state information
type JobStateResponse struct {
	JobID     string
	IsRunning bool
	LastRun   *JobRunSummary
	NextRun   *time.Time
}

// JobRunSummary contains summary information about a job run
type JobRunSummary struct {
	RunID       string
	StartedAt   time.Time
	CompletedAt time.Time
	Success     bool
}

// JobHistoryRequest queries the scheduler for job history
type JobHistoryRequest struct {
	JobID      string
	Limit      int
	ResponseTo chan interface{}
}

// SetResponseChan implements RequestMessage
func (r *JobHistoryRequest) SetResponseChan(ch chan interface{}) {
	r.ResponseTo = ch
}

// JobHistoryResponse contains job history
type JobHistoryResponse struct {
	JobID string
	Runs  []JobRunSummary
}

// RequestMessage is implemented by messages that expect a response
type RequestMessage interface {
	SetResponseChan(ch chan interface{})
}

// SendAndReceive sends a request to the scheduler inbox and waits for the response.
// The context cancellation is respected.
func SendAndReceive(ctx context.Context, inbox MessageSender, req RequestMessage) (interface{}, error) {
	ch := make(chan interface{}, 1)
	req.SetResponseChan(ch)
	if err := inbox.Send(req); err != nil {
		return nil, err
	}
	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// =============================================================================
// Execution Context
// =============================================================================

// ExecutionContext provides runtime dependencies to constraints and actions.
// Passed as a pointer — not every consumer uses every field.
type ExecutionContext struct {
	// Job information
	Job   *Job
	RunID string

	// Execution timing (used by during/post execution constraints)
	StartTime *time.Time
	EndTime   *time.Time
	ExitCode  *int

	// Command execution details (used by actions)
	Command string
	Args    []string
	Kwargs  map[string]string

	// Communication
	Inbox          MessageSender
	WebhookHandler WebhookSender
	HTTPClient     *http.Client

	// Job control (used by actions)
	JobController   JobController
	MetadataUpdater MetadataUpdater
	MetricRecorder  MetricRecorder

	// Logging
	Logger *slog.Logger

	// Cancellation
	Context context.Context
}

// =============================================================================
// Dependency Interfaces
// =============================================================================

// MessageSender sends messages to the scheduler inbox
type MessageSender interface {
	Send(msg interface{}) error
}

// WebhookSender sends HTTP webhooks
type WebhookSender interface {
	SendWebhook(url string, payload interface{}) error
}

// JobController provides job control operations
type JobController interface {
	RetryJob(jobID string) error
	TriggerJob(jobID string, args map[string]interface{}) error
	KillAllInstances(jobID string) error
	KillLatestInstance(jobID string) error
	SkipNextInstance(jobID string) error
}

// MetadataUpdater updates job metadata
type MetadataUpdater interface {
	UpdateMetadata(jobID string, metadata map[string]interface{}) error
}

// MetricRecorder records custom metrics
type MetricRecorder interface {
	RecordMetric(name string, value float64, tags map[string]string) error
}

// =============================================================================
// Action Parsing
// =============================================================================

// ActionParseConfig holds the JSON configuration for creating actions
type ActionParseConfig struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

// TemplateData holds data available for template rendering in actions
type TemplateData struct {
	JobID            string
	JobName          string
	RunID            string
	Command          string
	Args             []string
	Kwargs           map[string]string
	Timestamp        time.Time
	ConstraintName   string
	ConstraintStatus string
}

// =============================================================================
// Stats Types (DB row representations)
// =============================================================================

// SchedulerStats represents scheduler performance metrics
type SchedulerStats struct {
	StatsPeriodID         string
	StartTime             time.Time
	EndTime               time.Time
	Iterations            int
	RunJobs               int
	LateJobs              int
	TimePassedRunTime     int
	MissedJobs            int
	TimePassedGracePeriod int
	JobsCancelled         int
	MinInboxLength        *int
	MaxInboxLength        *int
	AvgInboxLength        *float64
	EmptyInboxTime        *int
	AvgTimeInInbox        *float64
	MinTimeInInbox        *int
	MaxTimeInInbox        *int
}

// OrchestratorStats represents orchestrator performance metrics
type OrchestratorStats struct {
	RunID              string
	StatsPeriodID      string
	Runtime            int
	ConstraintsChecked int
	ActionsTaken       int
}

// SyncerStats represents syncer performance metrics
type SyncerStats struct {
	StatsPeriodID         string
	StartTime             time.Time
	EndTime               time.Time
	TotalWrites           int
	WritesSucceeded       int
	WritesFailed          int
	AvgWritesInFlight     *float64
	MaxWritesInFlight     *int
	MinWritesInFlight     *int
	AvgQueuedWrites       *float64
	MaxQueuedWrites       *int
	MinQueuedWrites       *int
	AvgInboxLength        *float64
	MaxInboxLength        *int
	MinInboxLength        *int
	AvgTimeInWriteQueue   *float64
	MaxTimeInWriteQueue   *int
	MinTimeInWriteQueue   *int
	AvgTimeInInbox        *float64
	MaxTimeInInbox        *int
	MinTimeInInbox        *int
}

// StatsCollectorStats represents stats collector performance metrics
type StatsCollectorStats struct {
	StatsPeriodID        string
	StartTime            time.Time
	EndTime              time.Time
	MessagesReceived     int
	MessagesProcessed    int
	SchedulerMessages    int
	OrchestratorMessages int
	SyncerMessages       int
	WebhookMessages      int
	PeriodsCompleted     int
	DatabaseFlushes      int
	FlushErrors          int
	AvgInboxLength       *float64
	MaxInboxLength       *int
	MinInboxLength       *int
	AvgProcessingTime    *float64
	MaxProcessingTime    *int
	MinProcessingTime    *int
}
