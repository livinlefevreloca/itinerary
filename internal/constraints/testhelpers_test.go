package constraints

import (
	"time"

	"github.com/livinlefevreloca/itinerary/internal/model"
	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

// createTestExecutionContext delegates to testutil for use in internal constraint tests
func createTestExecutionContext() *model.ExecutionContext {
	return testutil.NewTestExecutionContext()
}

func createTestExecutionContextWithStartTime(startTime time.Time) *model.ExecutionContext {
	return testutil.NewTestExecutionContextWithStartTime(startTime)
}

func createTestExecutionContextWithTiming(startTime, endTime time.Time, exitCode int) *model.ExecutionContext {
	return testutil.NewTestExecutionContextWithTiming(startTime, endTime, exitCode)
}

// NewNoOpAction wraps testutil.NewNoOpAction for use in constraint tests
func NewNoOpAction(name string) *testutil.NoOpAction {
	return testutil.NewNoOpAction(name)
}
