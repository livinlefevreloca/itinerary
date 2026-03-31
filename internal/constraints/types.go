package constraints

import (
	"context"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/model"
)

// Constraint is implemented by all constraint types
type Constraint interface {
	// Check evaluates the constraint and returns whether it is met
	Check(ctx *model.ExecutionContext) (model.ConstraintResult, error)

	// Name returns the human-readable name of this constraint
	Name() string

	// EvaluationTiming returns when this constraint should be evaluated
	EvaluationTiming() []model.EvaluationPhase

	// ShouldRecheckOnRetry returns whether this constraint should be re-evaluated on retry
	ShouldRecheckOnRetry() bool
}

// Action is implemented by action types that run in response to constraint outcomes
type Action interface {
	// Execute performs the action
	Execute(ctx *model.ExecutionContext) error

	// Name returns the human-readable name of this action
	Name() string
}

// ConstraintChecker evaluates constraints at various phases
type ConstraintChecker interface {
	// CheckPreExecution evaluates constraints before job starts
	CheckPreExecution(ctx context.Context, job *model.Job, runID string) (model.ConstraintCheckResult, error)

	// CheckDuringExecution evaluates constraints while job is running
	CheckDuringExecution(ctx context.Context, job *model.Job, runID string, startTime time.Time) (model.ConstraintCheckResult, error)

	// CheckPostExecution evaluates constraints after job completes
	CheckPostExecution(ctx context.Context, job *model.Job, runID string, startTime, endTime time.Time, exitCode int) (model.ConstraintCheckResult, error)

	// ShouldRecheckOnRetry returns whether constraints should be re-evaluated on retry
	ShouldRecheckOnRetry(job *model.Job) bool
}

// ConstraintWithActions pairs a constraint with its associated actions
type ConstraintWithActions struct {
	Constraint     Constraint
	OnViolation    []Action
	OnMet          []Action
	RecheckOnRetry bool
}
