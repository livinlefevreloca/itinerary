package constraints

import (
	"fmt"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/model"
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

func (o *OtherJobScheduledSoonConstraint) Check(ctx *model.ExecutionContext) (model.ConstraintResult, error) {
	request := &model.JobStateRequest{JobID: o.otherJobID}
	resp, err := model.SendAndReceive(ctx.Context, ctx.Inbox, request)
	if err != nil {
		return model.ConstraintResult{}, err
	}

	state := resp.(*model.JobStateResponse)

	if state.NextRun == nil {
		return model.ConstraintResult{
			Met:     false,
			Message: fmt.Sprintf("job %s has no scheduled runs", o.otherJobID),
		}, nil
	}

	timeUntilRun := time.Until(*state.NextRun)
	met := timeUntilRun > 0 && timeUntilRun <= o.within

	return model.ConstraintResult{
		Met: met,
		Message: fmt.Sprintf("job %s scheduled in %v",
			o.otherJobID, timeUntilRun.Round(time.Second)),
	}, nil
}

func (o *OtherJobScheduledSoonConstraint) Name() string {
	return o.name
}

func (o *OtherJobScheduledSoonConstraint) EvaluationTiming() []model.EvaluationPhase {
	return []model.EvaluationPhase{model.EvaluationPhasePreExecution}
}

func (o *OtherJobScheduledSoonConstraint) ShouldRecheckOnRetry() bool {
	return o.recheck
}
