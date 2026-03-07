package constraints

import (
	"fmt"
	"time"
)

// MaxRuntimeConstraint checks if job has been running for less than a specified duration
type MaxRuntimeConstraint struct {
	name        string
	maxDuration time.Duration
}

// NewMaxRuntimeConstraint creates a new MaxRuntimeConstraint
func NewMaxRuntimeConstraint(name string, maxDuration time.Duration) *MaxRuntimeConstraint {
	return &MaxRuntimeConstraint{
		name:        name,
		maxDuration: maxDuration,
	}
}

func (m *MaxRuntimeConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
	if ctx.StartTime == nil {
		return ConstraintResult{}, fmt.Errorf("start time not available")
	}

	elapsed := time.Since(*ctx.StartTime)
	met := elapsed <= m.maxDuration

	return ConstraintResult{
		Met: met,
		Message: fmt.Sprintf("runtime %v / %v",
			elapsed.Round(time.Second), m.maxDuration),
	}, nil
}

func (m *MaxRuntimeConstraint) Name() string {
	return m.name
}

func (m *MaxRuntimeConstraint) EvaluationTiming() []EvaluationPhase {
	return []EvaluationPhase{EvaluationPhaseDuringExecution, EvaluationPhasePostExecution}
}

func (m *MaxRuntimeConstraint) ShouldRecheckOnRetry() bool {
	// Max runtime constraints don't typically recheck on retry
	return false
}
