package constraints

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// DefaultConstraintChecker implements the ConstraintChecker interface
type DefaultConstraintChecker struct {
	constraints []ConstraintWithActions
	logger      *slog.Logger
	// Dependencies for creating execution contexts
	schedulerInbox MessageSender
	httpClient     *http.Client
}

// NewConstraintChecker creates a new DefaultConstraintChecker
func NewConstraintChecker(
	constraints []ConstraintWithActions,
	schedulerInbox MessageSender,
	httpClient *http.Client,
	logger *slog.Logger,
) *DefaultConstraintChecker {
	return &DefaultConstraintChecker{
		constraints:    constraints,
		schedulerInbox: schedulerInbox,
		httpClient:     httpClient,
		logger:         logger,
	}
}

func (c *DefaultConstraintChecker) CheckPreExecution(ctx context.Context, job *Job, runID string) (ConstraintCheckResult, error) {
	return c.checkConstraints(ctx, job, runID, EvaluationPhasePreExecution, nil, nil, nil)
}

func (c *DefaultConstraintChecker) CheckDuringExecution(ctx context.Context, job *Job, runID string, startTime time.Time) (ConstraintCheckResult, error) {
	return c.checkConstraints(ctx, job, runID, EvaluationPhaseDuringExecution, &startTime, nil, nil)
}

func (c *DefaultConstraintChecker) CheckPostExecution(ctx context.Context, job *Job, runID string, startTime, endTime time.Time, exitCode int) (ConstraintCheckResult, error) {
	return c.checkConstraints(ctx, job, runID, EvaluationPhasePostExecution, &startTime, &endTime, &exitCode)
}

func (c *DefaultConstraintChecker) ShouldRecheckOnRetry(job *Job) bool {
	for _, cwa := range c.constraints {
		if cwa.RecheckOnRetry {
			return true
		}
	}
	return false
}

func (c *DefaultConstraintChecker) checkConstraints(
	ctx context.Context,
	job *Job,
	runID string,
	phase EvaluationPhase,
	startTime *time.Time,
	endTime *time.Time,
	exitCode *int,
) (ConstraintCheckResult, error) {
	execCtx := c.buildExecutionContext(ctx, job, runID, startTime, endTime, exitCode)

	allMet := true
	messages := []string{}

	for _, cwa := range c.constraints {
		// Skip if constraint doesn't apply to this phase
		if !c.appliesToPhase(cwa.Constraint, phase) {
			continue
		}

		result, err := cwa.Constraint.Check(execCtx)
		if err != nil {
			return ConstraintCheckResult{}, err
		}

		messages = append(messages, result.Message)

		if result.Met {
			// Execute onMet actions
			for _, action := range cwa.OnMet {
				if err := action.Execute(execCtx); err != nil {
					c.logger.Error("failed to execute onMet action",
						"constraint", cwa.Constraint.Name(),
						"action", action.Name(),
						"error", err)
				}
			}
		} else {
			allMet = false

			// Execute onViolation actions
			for _, action := range cwa.OnViolation {
				if err := action.Execute(execCtx); err != nil {
					c.logger.Error("failed to execute onViolation action",
						"constraint", cwa.Constraint.Name(),
						"action", action.Name(),
						"error", err)
				}
			}
		}
	}

	return ConstraintCheckResult{
		ShouldProceed: allMet,
		Message:       strings.Join(messages, "; "),
	}, nil
}

func (c *DefaultConstraintChecker) appliesToPhase(constraint Constraint, phase EvaluationPhase) bool {
	phases := constraint.EvaluationTiming()
	for _, p := range phases {
		if p == phase {
			return true
		}
	}
	return false
}

func (c *DefaultConstraintChecker) buildExecutionContext(
	ctx context.Context,
	job *Job,
	runID string,
	startTime *time.Time,
	endTime *time.Time,
	exitCode *int,
) *ExecutionContext {
	return &ExecutionContext{
		Job:            job,
		RunID:          runID,
		StartTime:      startTime,
		EndTime:        endTime,
		ExitCode:       exitCode,
		SchedulerInbox: c.schedulerInbox,
		HTTPClient:     c.httpClient,
		Logger:         c.logger,
		Context:        ctx,
	}
}
