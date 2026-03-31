package constraints

import (
	"fmt"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/model"
)

// TimeWindowConstraint checks if current time is within a specified window
type TimeWindowConstraint struct {
	name      string
	startTime time.Time // Daily start time (hour/minute matter, date doesn't)
	endTime   time.Time // Daily end time (hour/minute matter, date doesn't)
	timezone  *time.Location
	recheck   bool
}

// NewTimeWindowConstraint creates a new TimeWindowConstraint
func NewTimeWindowConstraint(name string, startTime, endTime time.Time, timezone *time.Location, recheck bool) *TimeWindowConstraint {
	return &TimeWindowConstraint{
		name:      name,
		startTime: startTime,
		endTime:   endTime,
		timezone:  timezone,
		recheck:   recheck,
	}
}

func (t *TimeWindowConstraint) Check(ctx *model.ExecutionContext) (model.ConstraintResult, error) {
	now := time.Now().In(t.timezone)

	// Convert now to today's window times
	start := time.Date(now.Year(), now.Month(), now.Day(),
		t.startTime.Hour(), t.startTime.Minute(), 0, 0, t.timezone)
	end := time.Date(now.Year(), now.Month(), now.Day(),
		t.endTime.Hour(), t.endTime.Minute(), 0, 0, t.timezone)

	met := now.After(start) && now.Before(end)

	return model.ConstraintResult{
		Met: met,
		Message: fmt.Sprintf("time window check [%s-%s]: %v",
			start.Format("15:04"), end.Format("15:04"), met),
	}, nil
}

func (t *TimeWindowConstraint) Name() string {
	return t.name
}

func (t *TimeWindowConstraint) EvaluationTiming() []model.EvaluationPhase {
	return []model.EvaluationPhase{model.EvaluationPhasePreExecution}
}

func (t *TimeWindowConstraint) ShouldRecheckOnRetry() bool {
	return t.recheck
}
