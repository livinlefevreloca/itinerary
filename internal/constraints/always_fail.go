package constraints

// AlwaysFailConstraint always returns Met=false, useful for testing
type AlwaysFailConstraint struct {
	name    string
	recheck bool
	phases  []EvaluationPhase
}

// NewAlwaysFailConstraint creates a new AlwaysFailConstraint
func NewAlwaysFailConstraint(name string, recheck bool, phases []EvaluationPhase) *AlwaysFailConstraint {
	return &AlwaysFailConstraint{
		name:    name,
		recheck: recheck,
		phases:  phases,
	}
}

func (a *AlwaysFailConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
	return ConstraintResult{Met: false, Message: "always fail"}, nil
}

func (a *AlwaysFailConstraint) Name() string {
	return a.name
}

func (a *AlwaysFailConstraint) EvaluationTiming() []EvaluationPhase {
	return a.phases
}

func (a *AlwaysFailConstraint) ShouldRecheckOnRetry() bool {
	return a.recheck
}
