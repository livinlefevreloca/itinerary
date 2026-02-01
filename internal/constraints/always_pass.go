package constraints

// AlwaysPassConstraint always returns Met=true, useful for testing
type AlwaysPassConstraint struct {
	name    string
	recheck bool
	phases  []EvaluationPhase
}

// NewAlwaysPassConstraint creates a new AlwaysPassConstraint
func NewAlwaysPassConstraint(name string, recheck bool, phases []EvaluationPhase) *AlwaysPassConstraint {
	return &AlwaysPassConstraint{
		name:    name,
		recheck: recheck,
		phases:  phases,
	}
}

func (a *AlwaysPassConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
	return ConstraintResult{Met: true, Message: "always pass"}, nil
}

func (a *AlwaysPassConstraint) Name() string {
	return a.name
}

func (a *AlwaysPassConstraint) EvaluationTiming() []EvaluationPhase {
	return a.phases
}

func (a *AlwaysPassConstraint) ShouldRecheckOnRetry() bool {
	return a.recheck
}
