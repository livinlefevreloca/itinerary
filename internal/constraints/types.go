package constraints

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// EvaluationPhase defines when a constraint should be evaluated
type EvaluationPhase string

const (
	EvaluationPhasePreExecution    EvaluationPhase = "pre"
	EvaluationPhaseDuringExecution EvaluationPhase = "during"
	EvaluationPhasePostExecution   EvaluationPhase = "post"
)

// ConstraintResult represents the result of evaluating a constraint
type ConstraintResult struct {
	Met     bool   // true if constraint is satisfied
	Message string // Description of constraint evaluation
}

// Constraint is implemented by all constraint types
type Constraint interface {
	// Check evaluates the constraint and returns whether it is met
	Check(ctx *ExecutionContext) (ConstraintResult, error)

	// Name returns the human-readable name of this constraint
	Name() string

	// EvaluationTiming returns when this constraint should be evaluated
	EvaluationTiming() []EvaluationPhase

	// ShouldRecheckOnRetry returns whether this constraint should be re-evaluated on retry
	ShouldRecheckOnRetry() bool
}

// Action is implemented by all action types
type Action interface {
	// Execute performs the action
	Execute(ctx *ExecutionContext) error

	// Name returns the human-readable name of this action
	Name() string
}

// ExecutionContext provides dependencies to constraints and actions
type ExecutionContext struct {
	// Job information
	Job   *Job
	RunID string

	// Execution timing (for during/post execution checks)
	StartTime *time.Time
	EndTime   *time.Time
	ExitCode  *int

	// Message-based communication with scheduler for job state queries
	SchedulerInbox MessageSender

	// HTTP client for making external requests
	HTTPClient *http.Client

	// Logging
	Logger *slog.Logger

	// Cancellation
	Context context.Context
}

// MessageSender sends messages to the scheduler
type MessageSender interface {
	Send(msg interface{}) error
}

// Job represents a job definition (simplified for constraints module)
type Job struct {
	ID   string
	Name string
}

// JobStateRequest queries the scheduler for job state
type JobStateRequest struct {
	JobID      string
	ResponseTo chan interface{}
}

// JobStateResponse contains job state information
type JobStateResponse struct {
	JobID     string
	IsRunning bool
	LastRun   *JobRunSummary
	NextRun   *time.Time
}

// JobRunSummary contains information about a job run
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

// JobHistoryResponse contains job history
type JobHistoryResponse struct {
	JobID string
	Runs  []JobRunSummary
}

// ConstraintCheckResult is returned by ConstraintChecker
type ConstraintCheckResult struct {
	ShouldProceed bool   // false if constraints prevent execution/continuation
	Message       string // Summary of constraint evaluation and actions taken
}

// ConstraintChecker evaluates constraints at various phases
type ConstraintChecker interface {
	// CheckPreExecution evaluates constraints before job starts
	CheckPreExecution(ctx context.Context, job *Job, runID string) (ConstraintCheckResult, error)

	// CheckDuringExecution evaluates constraints while job is running
	CheckDuringExecution(ctx context.Context, job *Job, runID string, startTime time.Time) (ConstraintCheckResult, error)

	// CheckPostExecution evaluates constraints after job completes
	CheckPostExecution(ctx context.Context, job *Job, runID string, startTime, endTime time.Time, exitCode int) (ConstraintCheckResult, error)

	// ShouldRecheckOnRetry returns whether constraints should be re-evaluated on retry
	ShouldRecheckOnRetry(job *Job) bool
}

// ConstraintWithActions pairs a constraint with its associated actions
type ConstraintWithActions struct {
	Constraint     Constraint
	OnViolation    []Action
	OnMet          []Action
	RecheckOnRetry bool
}
