package constraints

import (
	"fmt"
	"time"
)

// MinRuntimeConstraint checks if job ran for at least a specified duration
type MinRuntimeConstraint struct {
	name        string
	minDuration time.Duration
}

// NewMinRuntimeConstraint creates a new MinRuntimeConstraint
func NewMinRuntimeConstraint(name string, minDuration time.Duration) *MinRuntimeConstraint {
	return &MinRuntimeConstraint{
		name:        name,
		minDuration: minDuration,
	}
}

func (m *MinRuntimeConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
	if ctx.StartTime == nil || ctx.EndTime == nil {
		return ConstraintResult{}, fmt.Errorf("start/end time not available")
	}

	runtime := ctx.EndTime.Sub(*ctx.StartTime)
	met := runtime >= m.minDuration

	return ConstraintResult{
		Met: met,
		Message: fmt.Sprintf("runtime %v (minimum %v)",
			runtime.Round(time.Second), m.minDuration),
	}, nil
}

func (m *MinRuntimeConstraint) Name() string {
	return m.name
}

func (m *MinRuntimeConstraint) EvaluationTiming() []EvaluationPhase {
	return []EvaluationPhase{EvaluationPhasePostExecution}
}

func (m *MinRuntimeConstraint) ShouldRecheckOnRetry() bool {
	// Min runtime constraints don't typically recheck on retry
	return false
}
