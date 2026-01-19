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
	ID          string
	Name        string
	Schedule    string
	PodSpec     string    // JSON - Kubernetes pod specification
	Constraints *string   // JSON - map of constraint_type_id → constraint config
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// JobAction represents an action configured for a job
type JobAction struct {
	ID                 string
	JobID              string
	ActionTypeID       int
	Trigger            string // 'on_failure', 'on_violation', 'on_success', 'on_timeout'
	ConstraintTypeID   *int   // FK: Only used when Trigger is 'on_violation'
	Config             *string // JSON - action-specific configuration
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
}

// ConstraintViolation represents a constraint violation
type ConstraintViolation struct {
	ID               string
	RunID            string
	ConstraintTypeID int
	ViolationTime    time.Time
	Details          *string // JSON - violation-specific details
}

// ActionRun represents an action execution
type ActionRun struct {
	ID                      string
	RunID                   string
	ActionTypeID            int
	Trigger                 string  // 'on_failure', 'on_violation', 'on_success', 'manual'
	ConstraintViolationID   *string // FK: references the violation that triggered this action
	ExecutedAt              time.Time
	Success                 bool
	Error                   *string
	Details                 *string // JSON - action-specific details (webhook response, retry count, etc.)
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

// WebhookDelivery represents a webhook delivery attempt (future)
type WebhookDelivery struct {
	ID              string
	RunID           string
	WebhookType     string // 'slack', 'newrelic', 'pagerduty', 'custom'
	Trigger         string // 'on_failure', 'on_violation', 'on_success', 'manual'
	URL             string
	AttemptCount    int
	StatusCode      *int
	Success         bool
	Error           *string
	RequestDuration *int       // milliseconds
	CreatedAt       time.Time
	DeliveredAt     *time.Time
}

// WebhookHandlerStats represents webhook handler performance metrics (future)
type WebhookHandlerStats struct {
	StatsPeriodID      string
	StartTime          time.Time
	EndTime            time.Time
	WebhooksSent       int
	WebhooksSucceeded  int
	WebhooksFailed     int
	TotalRetries       int
	AvgDeliveryTime    *float64 // milliseconds
	MaxDeliveryTime    *int     // milliseconds
	MinDeliveryTime    *int     // milliseconds
	AvgInboxLength     *float64
	MaxInboxLength     *int
	MinInboxLength     *int
}
