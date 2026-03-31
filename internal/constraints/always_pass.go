package constraints

import "github.com/livinlefevreloca/itinerary/internal/model"

// AlwaysPassConstraint always returns Met=true, useful for testing
type AlwaysPassConstraint struct {
	name    string
	recheck bool
	phases  []model.EvaluationPhase
}

// NewAlwaysPassConstraint creates a new AlwaysPassConstraint
func NewAlwaysPassConstraint(name string, recheck bool, phases []model.EvaluationPhase) *AlwaysPassConstraint {
	return &AlwaysPassConstraint{
		name:    name,
		recheck: recheck,
		phases:  phases,
	}
}

func (a *AlwaysPassConstraint) Check(ctx *model.ExecutionContext) (model.ConstraintResult, error) {
	return model.ConstraintResult{Met: true, Message: "always pass"}, nil
}

func (a *AlwaysPassConstraint) Name() string {
	return a.name
}

func (a *AlwaysPassConstraint) EvaluationTiming() []model.EvaluationPhase {
	return a.phases
}

func (a *AlwaysPassConstraint) ShouldRecheckOnRetry() bool {
	return a.recheck
}
