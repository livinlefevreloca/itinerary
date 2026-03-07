package constraints

import (
	"context"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/db"
	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

// NoOpAction is a test action that does nothing
type NoOpAction struct {
	name string
}

func NewNoOpAction(name string) *NoOpAction {
	return &NoOpAction{name: name}
}

func (n *NoOpAction) Execute(ctx *ExecutionContext) error {
	return nil
}

func (n *NoOpAction) Name() string {
	return n.name
}

// Helper to create ExecutionContext for tests
func createTestExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		Job:            &db.Job{ID: "test-job", Name: "test"},
		RunID:          "test-run-id",
		SchedulerInbox: testutil.NewMockSchedulerInbox(),
		HTTPClient:     testutil.CreateTestHTTPClient(),
		Logger:         testutil.CreateTestSlogLogger(),
		Context:        context.Background(),
	}
}

func createTestExecutionContextWithStartTime(startTime time.Time) *ExecutionContext {
	ctx := createTestExecutionContext()
	ctx.StartTime = &startTime
	return ctx
}

func createTestExecutionContextWithTiming(startTime, endTime time.Time, exitCode int) *ExecutionContext {
	ctx := createTestExecutionContext()
	ctx.StartTime = &startTime
	ctx.EndTime = &endTime
	ctx.ExitCode = &exitCode
	return ctx
}
