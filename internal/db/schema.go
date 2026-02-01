package db

import "time"

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

// Job represents a scheduled job definition
type Job struct {
	ID        string
	Name      string
	Schedule  string
	PodSpec   string    // JSON - Kubernetes pod specification
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Constraint represents a specific constraint configuration for a job
type Constraint struct {
	ID               string
	JobID            string
	ConstraintTypeID int
	Config           *string // JSON - constraint-specific configuration
	CreatedAt        time.Time
}

// Action represents an action that can be triggered when a constraint is met or violated
type Action struct {
	ID           string
	ConstraintID string
	ActionTypeID int
	Trigger      string  // 'on_met', 'on_violated'
	Config       *string // JSON - action-specific configuration
	CreatedAt    time.Time
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
	Details      *string // JSON - run-specific details
}

// ActionRun represents an action execution
type ActionRun struct {
	ID               string
	RunID            string
	ConstraintRunID  *string // FK: references the constraint run that triggered this action
	ActionID         string
	ExecutedAt       time.Time
	Success          bool
	Error            *string
	Details          *string // JSON - action-specific details (webhook response, retry count, etc.)
}

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
	StatsPeriodID          string
	StartTime              time.Time
	EndTime                time.Time
	MessagesReceived       int
	MessagesProcessed      int
	SchedulerMessages      int
	OrchestratorMessages   int
	SyncerMessages         int
	WebhookMessages        int
	PeriodsCompleted       int
	DatabaseFlushes        int
	FlushErrors            int
	AvgInboxLength         *float64
	MaxInboxLength         *int
	MinInboxLength         *int
	AvgProcessingTime      *float64 // microseconds
	MaxProcessingTime      *int     // microseconds
	MinProcessingTime      *int     // microseconds
}
