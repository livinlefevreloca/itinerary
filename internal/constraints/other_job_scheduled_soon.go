package constraints

import (
	"fmt"
	"time"
)

// OtherJobScheduledSoonConstraint checks if another job is scheduled to run within a specified time window
type OtherJobScheduledSoonConstraint struct {
	name       string
	otherJobID string
	within     time.Duration
	recheck    bool
}

// NewOtherJobScheduledSoonConstraint creates a new OtherJobScheduledSoonConstraint
func NewOtherJobScheduledSoonConstraint(name string, otherJobID string, within time.Duration, recheck bool) *OtherJobScheduledSoonConstraint {
	return &OtherJobScheduledSoonConstraint{
		name:       name,
		otherJobID: otherJobID,
		within:     within,
		recheck:    recheck,
	}
}

func (o *OtherJobScheduledSoonConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
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

		if state.NextRun == nil {
			return ConstraintResult{
				Met:     false,
				Message: fmt.Sprintf("job %s has no scheduled runs", o.otherJobID),
			}, nil
		}

		timeUntilRun := time.Until(*state.NextRun)
		met := timeUntilRun > 0 && timeUntilRun <= o.within

		return ConstraintResult{
			Met: met,
			Message: fmt.Sprintf("job %s scheduled in %v",
				o.otherJobID, timeUntilRun.Round(time.Second)),
		}, nil
	case <-ctx.Context.Done():
		return ConstraintResult{}, ctx.Context.Err()
	}
}

func (o *OtherJobScheduledSoonConstraint) Name() string {
	return o.name
}

func (o *OtherJobScheduledSoonConstraint) EvaluationTiming() []EvaluationPhase {
	return []EvaluationPhase{EvaluationPhasePreExecution}
}

func (o *OtherJobScheduledSoonConstraint) ShouldRecheckOnRetry() bool {
	return o.recheck
}
