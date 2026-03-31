package constraints

import "github.com/livinlefevreloca/itinerary/internal/model"

// AlwaysFailConstraint always returns Met=false, useful for testing
type AlwaysFailConstraint struct {
	name    string
	recheck bool
	phases  []model.EvaluationPhase
}

// NewAlwaysFailConstraint creates a new AlwaysFailConstraint
func NewAlwaysFailConstraint(name string, recheck bool, phases []model.EvaluationPhase) *AlwaysFailConstraint {
	return &AlwaysFailConstraint{
		name:    name,
		recheck: recheck,
		phases:  phases,
	}
}

func (a *AlwaysFailConstraint) Check(ctx *model.ExecutionContext) (model.ConstraintResult, error) {
	return model.ConstraintResult{Met: false, Message: "always fail"}, nil
}

func (a *AlwaysFailConstraint) Name() string {
	return a.name
}

func (a *AlwaysFailConstraint) EvaluationTiming() []model.EvaluationPhase {
	return a.phases
}

func (a *AlwaysFailConstraint) ShouldRecheckOnRetry() bool {
	return a.recheck
}
