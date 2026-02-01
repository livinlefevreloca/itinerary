package constraints

import (
	"fmt"
	"time"
)

// OtherJobCompletedRecentlyConstraint checks if another job completed within a specified time window
type OtherJobCompletedRecentlyConstraint struct {
	name        string
	otherJobID  string
	within      time.Duration
	mustSucceed bool
	recheck     bool
}

// NewOtherJobCompletedRecentlyConstraint creates a new OtherJobCompletedRecentlyConstraint
func NewOtherJobCompletedRecentlyConstraint(name string, otherJobID string, within time.Duration, mustSucceed bool, recheck bool) *OtherJobCompletedRecentlyConstraint {
	return &OtherJobCompletedRecentlyConstraint{
		name:        name,
		otherJobID:  otherJobID,
		within:      within,
		mustSucceed: mustSucceed,
		recheck:     recheck,
	}
}

func (o *OtherJobCompletedRecentlyConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
	responseChan := make(chan interface{}, 1)
	request := &JobHistoryRequest{
		JobID:      o.otherJobID,
		Limit:      1,
		ResponseTo: responseChan,
	}

	if err := ctx.SchedulerInbox.Send(request); err != nil {
		return ConstraintResult{}, err
	}

	select {
	case resp := <-responseChan:
		history := resp.(*JobHistoryResponse)

		if len(history.Runs) == 0 {
			return ConstraintResult{
				Met:     false,
				Message: fmt.Sprintf("job %s has no recent runs", o.otherJobID),
			}, nil
		}

		lastRun := history.Runs[0]
		timeSinceCompletion := time.Since(lastRun.CompletedAt)

		met := timeSinceCompletion <= o.within
		if o.mustSucceed {
			met = met && lastRun.Success
		}

		return ConstraintResult{
			Met: met,
			Message: fmt.Sprintf("job %s last completed %v ago (success=%v)",
				o.otherJobID, timeSinceCompletion.Round(time.Second), lastRun.Success),
		}, nil
	case <-ctx.Context.Done():
		return ConstraintResult{}, ctx.Context.Err()
	}
}

func (o *OtherJobCompletedRecentlyConstraint) Name() string {
	return o.name
}

func (o *OtherJobCompletedRecentlyConstraint) EvaluationTiming() []EvaluationPhase {
	return []EvaluationPhase{EvaluationPhasePreExecution}
}

func (o *OtherJobCompletedRecentlyConstraint) ShouldRecheckOnRetry() bool {
	return o.recheck
}
