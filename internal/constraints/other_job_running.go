package constraints

import "fmt"

// OtherJobRunningConstraint checks if another job is currently running
type OtherJobRunningConstraint struct {
	name            string
	otherJobID      string
	shouldBeRunning bool // true = met when running, false = met when NOT running
	recheck         bool
}

// NewOtherJobRunningConstraint creates a new OtherJobRunningConstraint
func NewOtherJobRunningConstraint(name string, otherJobID string, shouldBeRunning bool, recheck bool) *OtherJobRunningConstraint {
	return &OtherJobRunningConstraint{
		name:            name,
		otherJobID:      otherJobID,
		shouldBeRunning: shouldBeRunning,
		recheck:         recheck,
	}
}

func (o *OtherJobRunningConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
	responseChan := make(chan interface{}, 1)
	request := &JobStateRequest{
		JobID:      o.otherJobID,
		ResponseTo: responseChan,
	}

	if err := ctx.SchedulerInbox.Send(request); err != nil {
		return ConstraintResult{}, err
	}

	select {
	case resp := <-responseChan:
		state := resp.(*JobStateResponse)
		met := state.IsRunning == o.shouldBeRunning

		return ConstraintResult{
			Met: met,
			Message: fmt.Sprintf("job %s running=%v (expected=%v)",
				o.otherJobID, state.IsRunning, o.shouldBeRunning),
		}, nil
	case <-ctx.Context.Done():
		return ConstraintResult{}, ctx.Context.Err()
	}
}

func (o *OtherJobRunningConstraint) Name() string {
	return o.name
}

func (o *OtherJobRunningConstraint) EvaluationTiming() []EvaluationPhase {
	return []EvaluationPhase{EvaluationPhasePreExecution}
}

func (o *OtherJobRunningConstraint) ShouldRecheckOnRetry() bool {
	return o.recheck
}
