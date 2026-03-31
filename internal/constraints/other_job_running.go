package constraints

import (
	"fmt"

	"github.com/livinlefevreloca/itinerary/internal/model"
)

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

func (o *OtherJobRunningConstraint) Check(ctx *model.ExecutionContext) (model.ConstraintResult, error) {
	request := &model.JobStateRequest{JobID: o.otherJobID}
	resp, err := model.SendAndReceive(ctx.Context, ctx.Inbox, request)
	if err != nil {
		return model.ConstraintResult{}, err
	}

	state := resp.(*model.JobStateResponse)
	met := state.IsRunning == o.shouldBeRunning

	return model.ConstraintResult{
		Met: met,
		Message: fmt.Sprintf("job %s running=%v (expected=%v)",
			o.otherJobID, state.IsRunning, o.shouldBeRunning),
	}, nil
}

func (o *OtherJobRunningConstraint) Name() string {
	return o.name
}

func (o *OtherJobRunningConstraint) EvaluationTiming() []model.EvaluationPhase {
	return []model.EvaluationPhase{model.EvaluationPhasePreExecution}
}

func (o *OtherJobRunningConstraint) ShouldRecheckOnRetry() bool {
	return o.recheck
}
