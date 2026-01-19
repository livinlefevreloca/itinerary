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

// ConstraintViolation represents a constraint violation and action taken
type ConstraintViolation struct {
	ID               string
	RunID            string
	ConstraintTypeID int
	ViolationTime    time.Time
	ActionTaken      *string
	Details          string
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
