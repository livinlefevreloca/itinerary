package constraints

import (
	"fmt"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/model"
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

func (m *MaxRuntimeConstraint) Check(ctx *model.ExecutionContext) (model.ConstraintResult, error) {
	if ctx.StartTime == nil {
		return model.ConstraintResult{}, fmt.Errorf("start time not available")
	}

	elapsed := time.Since(*ctx.StartTime)
	met := elapsed <= m.maxDuration

	return model.ConstraintResult{
		Met: met,
		Message: fmt.Sprintf("runtime %v / %v",
			elapsed.Round(time.Second), m.maxDuration),
	}, nil
}

func (m *MaxRuntimeConstraint) Name() string {
	return m.name
}

func (m *MaxRuntimeConstraint) EvaluationTiming() []model.EvaluationPhase {
	return []model.EvaluationPhase{model.EvaluationPhaseDuringExecution, model.EvaluationPhasePostExecution}
}

func (m *MaxRuntimeConstraint) ShouldRecheckOnRetry() bool {
	return false
}
