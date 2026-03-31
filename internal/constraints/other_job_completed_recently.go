package constraints

import (
	"fmt"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/model"
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

func (o *OtherJobCompletedRecentlyConstraint) Check(ctx *model.ExecutionContext) (model.ConstraintResult, error) {
	request := &model.JobHistoryRequest{JobID: o.otherJobID, Limit: 1}
	resp, err := model.SendAndReceive(ctx.Context, ctx.Inbox, request)
	if err != nil {
		return model.ConstraintResult{}, err
	}

	history := resp.(*model.JobHistoryResponse)

	if len(history.Runs) == 0 {
		return model.ConstraintResult{
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

	return model.ConstraintResult{
		Met: met,
		Message: fmt.Sprintf("job %s last completed %v ago (success=%v)",
			o.otherJobID, timeSinceCompletion.Round(time.Second), lastRun.Success),
	}, nil
}

func (o *OtherJobCompletedRecentlyConstraint) Name() string {
	return o.name
}

func (o *OtherJobCompletedRecentlyConstraint) EvaluationTiming() []model.EvaluationPhase {
	return []model.EvaluationPhase{model.EvaluationPhasePreExecution}
}

func (o *OtherJobCompletedRecentlyConstraint) ShouldRecheckOnRetry() bool {
	return o.recheck
}
