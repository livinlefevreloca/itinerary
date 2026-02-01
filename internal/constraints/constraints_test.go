package constraints

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

// ========================
// Test Helpers and Mocks
// ========================

// MockSchedulerInbox for testing message sending to scheduler
type MockSchedulerInbox struct {
	messages       []interface{}
	mu             sync.Mutex
	responseFunc   func(msg interface{})
	shouldError    bool
	errorToReturn  error
}

func NewMockSchedulerInbox() *MockSchedulerInbox {
	return &MockSchedulerInbox{
		messages: []interface{}{},
	}
}

func (m *MockSchedulerInbox) Send(msg interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldError {
		return m.errorToReturn
	}

	m.messages = append(m.messages, msg)

	// Auto-respond if response function is set
	if m.responseFunc != nil {
		m.responseFunc(msg)
	}

	return nil
}

func (m *MockSchedulerInbox) GetMessages() []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]interface{}{}, m.messages...)
}

func (m *MockSchedulerInbox) SetResponseFunc(f func(msg interface{})) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseFunc = f
}

func (m *MockSchedulerInbox) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldError = true
	m.errorToReturn = err
}

// MockWebhookHandler for testing webhooks
type MockWebhookHandler struct {
	calls []WebhookCall
	err   error
	mu    sync.Mutex
}

type WebhookCall struct {
	URL     string
	Payload interface{}
}

func (m *MockWebhookHandler) SendWebhook(url string, payload interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, WebhookCall{URL: url, Payload: payload})
	return m.err
}

func (m *MockWebhookHandler) GetCalls() []WebhookCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]WebhookCall{}, m.calls...)
}

// TestLogger for capturing log output
func createTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// Helper to create ExecutionContext for tests
func createTestExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		Job:             &Job{ID: "test-job", Name: "test"},
		RunID:           "test-run-id",
		SchedulerInbox:  NewMockSchedulerInbox(),
		WebhookHandler:  &MockWebhookHandler{},
		HTTPClient:      &http.Client{Timeout: 5 * time.Second},
		Logger:          createTestLogger(),
		Context:         context.Background(),
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

// ========================
// 1. TimeWindowConstraint Tests
// ========================

func TestTimeWindowConstraint_WithinWindow(t *testing.T) {
	loc := time.UTC
	// Create window 09:00-17:00
	startTime := time.Date(2000, 1, 1, 9, 0, 0, 0, loc)
	endTime := time.Date(2000, 1, 1, 17, 0, 0, 0, loc)

	constraint := NewTimeWindowConstraint("business-hours", startTime, endTime, loc, false)
	ctx := createTestExecutionContext()

	// Mock current time to 12:00 by creating the constraint with times that will match
	// Since we can't easily mock time.Now(), we'll create times relative to now
	now := time.Now().In(loc)
	startHour := now.Hour() - 1 // 1 hour ago
	endHour := now.Hour() + 1    // 1 hour from now

	start := time.Date(2000, 1, 1, startHour, 0, 0, 0, loc)
	end := time.Date(2000, 1, 1, endHour, 0, 0, 0, loc)

	constraint = NewTimeWindowConstraint("test", start, end, loc, false)

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true (within window), got Met=false")
	}

	if result.Message == "" {
		t.Error("Expected non-empty message")
	}
}

func TestTimeWindowConstraint_BeforeWindow(t *testing.T) {
	loc := time.UTC
	now := time.Now().In(loc)

	// Create window that starts 2 hours from now
	startHour := now.Hour() + 2
	endHour := now.Hour() + 4

	start := time.Date(2000, 1, 1, startHour, 0, 0, 0, loc)
	end := time.Date(2000, 1, 1, endHour, 0, 0, 0, loc)

	constraint := NewTimeWindowConstraint("future-window", start, end, loc, false)
	ctx := createTestExecutionContext()

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Met {
		t.Errorf("Expected Met=false (before window), got Met=true")
	}
}

func TestTimeWindowConstraint_AfterWindow(t *testing.T) {
	loc := time.UTC
	now := time.Now().In(loc)

	// Create window that ended 2 hours ago
	startHour := now.Hour() - 4
	endHour := now.Hour() - 2

	start := time.Date(2000, 1, 1, startHour, 0, 0, 0, loc)
	end := time.Date(2000, 1, 1, endHour, 0, 0, 0, loc)

	constraint := NewTimeWindowConstraint("past-window", start, end, loc, false)
	ctx := createTestExecutionContext()

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Met {
		t.Errorf("Expected Met=false (after window), got Met=true")
	}
}

func TestTimeWindowConstraint_Timezone(t *testing.T) {
	// Test with a specific timezone
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("Cannot load timezone: %v", err)
	}

	now := time.Now().In(loc)
	startHour := now.Hour() - 1
	endHour := now.Hour() + 1

	start := time.Date(2000, 1, 1, startHour, 0, 0, 0, loc)
	end := time.Date(2000, 1, 1, endHour, 0, 0, 0, loc)

	constraint := NewTimeWindowConstraint("ny-hours", start, end, loc, false)
	ctx := createTestExecutionContext()

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should be within window
	if !result.Met {
		t.Errorf("Expected Met=true (within timezone window), got Met=false")
	}
}

func TestTimeWindowConstraint_ShouldRecheckOnRetry(t *testing.T) {
	loc := time.UTC
	start := time.Date(2000, 1, 1, 9, 0, 0, 0, loc)
	end := time.Date(2000, 1, 1, 17, 0, 0, 0, loc)

	tests := []struct {
		name    string
		recheck bool
	}{
		{"recheck true", true},
		{"recheck false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint := NewTimeWindowConstraint("test", start, end, loc, tt.recheck)
			if constraint.ShouldRecheckOnRetry() != tt.recheck {
				t.Errorf("Expected ShouldRecheckOnRetry()=%v, got %v", tt.recheck, constraint.ShouldRecheckOnRetry())
			}
		})
	}
}

func TestTimeWindowConstraint_EvaluationTiming(t *testing.T) {
	loc := time.UTC
	start := time.Date(2000, 1, 1, 9, 0, 0, 0, loc)
	end := time.Date(2000, 1, 1, 17, 0, 0, 0, loc)

	constraint := NewTimeWindowConstraint("test", start, end, loc, false)
	timing := constraint.EvaluationTiming()

	if len(timing) != 1 {
		t.Fatalf("Expected 1 phase, got %d", len(timing))
	}

	if timing[0] != EvaluationPhasePreExecution {
		t.Errorf("Expected EvaluationPhasePreExecution, got %s", timing[0])
	}
}

// ========================
// 2. OtherJobRunningConstraint Tests
// ========================

func TestOtherJobRunningConstraint_JobIsRunning(t *testing.T) {
	constraint := NewOtherJobRunningConstraint("check-etl", "etl-job", false, false)
	ctx := createTestExecutionContext()

	// Setup mock to respond with job running
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobStateRequest); ok {
			req.ResponseTo <- &JobStateResponse{
				JobID:     req.JobID,
				IsRunning: true,
				LastRun:   nil,
				NextRun:   nil,
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// shouldBeRunning=false, but job is running, so Met should be false
	if result.Met {
		t.Errorf("Expected Met=false (job is running but shouldn't be), got Met=true")
	}

	// Verify scheduler was queried
	messages := mockInbox.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message sent to scheduler, got %d", len(messages))
	}

	if req, ok := messages[0].(*JobStateRequest); ok {
		if req.JobID != "etl-job" {
			t.Errorf("Expected JobID='etl-job', got '%s'", req.JobID)
		}
	} else {
		t.Errorf("Expected JobStateRequest, got %T", messages[0])
	}
}

func TestOtherJobRunningConstraint_JobIsNotRunning(t *testing.T) {
	constraint := NewOtherJobRunningConstraint("check-etl", "etl-job", false, false)
	ctx := createTestExecutionContext()

	// Setup mock to respond with job not running
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobStateRequest); ok {
			req.ResponseTo <- &JobStateResponse{
				JobID:     req.JobID,
				IsRunning: false,
				LastRun:   nil,
				NextRun:   nil,
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// shouldBeRunning=false, and job is not running, so Met should be true
	if !result.Met {
		t.Errorf("Expected Met=true (job is not running as expected), got Met=false")
	}
}

func TestOtherJobRunningConstraint_ExpectRunning(t *testing.T) {
	constraint := NewOtherJobRunningConstraint("check-monitor", "monitor-job", true, false)
	ctx := createTestExecutionContext()

	// Setup mock to respond with job running
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobStateRequest); ok {
			req.ResponseTo <- &JobStateResponse{
				JobID:     req.JobID,
				IsRunning: true,
				LastRun:   nil,
				NextRun:   nil,
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// shouldBeRunning=true, and job is running, so Met should be true
	if !result.Met {
		t.Errorf("Expected Met=true (job is running as expected), got Met=false")
	}
}

func TestOtherJobRunningConstraint_SchedulerError(t *testing.T) {
	constraint := NewOtherJobRunningConstraint("check-job", "some-job", false, false)
	ctx := createTestExecutionContext()

	// Setup mock to return error
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetError(fmt.Errorf("scheduler unavailable"))

	_, err := constraint.Check(ctx)
	if err == nil {
		t.Fatal("Expected error from scheduler, got nil")
	}

	if err.Error() != "scheduler unavailable" {
		t.Errorf("Expected 'scheduler unavailable', got '%s'", err.Error())
	}
}

func TestOtherJobRunningConstraint_ContextCancelled(t *testing.T) {
	constraint := NewOtherJobRunningConstraint("check-job", "some-job", false, false)
	ctx := createTestExecutionContext()

	// Cancel context immediately
	cancelCtx, cancel := context.WithCancel(ctx.Context)
	cancel()
	ctx.Context = cancelCtx

	// Setup mock to not respond (simulating slow response)
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		// Don't send response - let context cancellation happen
	})

	_, err := constraint.Check(ctx)
	if err == nil {
		t.Fatal("Expected context.Canceled error, got nil")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
}

func TestOtherJobRunningConstraint_EvaluationTiming(t *testing.T) {
	constraint := NewOtherJobRunningConstraint("test", "job-id", false, false)
	timing := constraint.EvaluationTiming()

	if len(timing) != 1 {
		t.Fatalf("Expected 1 phase, got %d", len(timing))
	}

	if timing[0] != EvaluationPhasePreExecution {
		t.Errorf("Expected EvaluationPhasePreExecution, got %s", timing[0])
	}
}

// ========================
// 3. OtherJobCompletedRecentlyConstraint Tests
// ========================

func TestOtherJobCompletedRecently_WithinWindow(t *testing.T) {
	constraint := NewOtherJobCompletedRecentlyConstraint("check-upstream", "upstream-job", 30*time.Minute, false, false)
	ctx := createTestExecutionContext()

	// Setup mock to respond with run completed 15m ago
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobHistoryRequest); ok {
			req.ResponseTo <- &JobHistoryResponse{
				JobID: req.JobID,
				Runs: []JobRunSummary{
					{
						RunID:       "run-123",
						StartedAt:   time.Now().Add(-20 * time.Minute),
						CompletedAt: time.Now().Add(-15 * time.Minute),
						Success:     true,
					},
				},
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true (completed within 30m window), got Met=false")
	}
}

func TestOtherJobCompletedRecently_OutsideWindow(t *testing.T) {
	constraint := NewOtherJobCompletedRecentlyConstraint("check-upstream", "upstream-job", 30*time.Minute, false, false)
	ctx := createTestExecutionContext()

	// Setup mock to respond with run completed 45m ago
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobHistoryRequest); ok {
			req.ResponseTo <- &JobHistoryResponse{
				JobID: req.JobID,
				Runs: []JobRunSummary{
					{
						RunID:       "run-123",
						StartedAt:   time.Now().Add(-50 * time.Minute),
						CompletedAt: time.Now().Add(-45 * time.Minute),
						Success:     true,
					},
				},
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Met {
		t.Errorf("Expected Met=false (completed outside 30m window), got Met=true")
	}
}

func TestOtherJobCompletedRecently_MustSucceed_Success(t *testing.T) {
	constraint := NewOtherJobCompletedRecentlyConstraint("check-upstream", "upstream-job", 30*time.Minute, true, false)
	ctx := createTestExecutionContext()

	// Setup mock with successful run
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobHistoryRequest); ok {
			req.ResponseTo <- &JobHistoryResponse{
				JobID: req.JobID,
				Runs: []JobRunSummary{
					{
						RunID:       "run-123",
						StartedAt:   time.Now().Add(-20 * time.Minute),
						CompletedAt: time.Now().Add(-15 * time.Minute),
						Success:     true,
					},
				},
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true (completed within window and succeeded), got Met=false")
	}
}

func TestOtherJobCompletedRecently_MustSucceed_Failed(t *testing.T) {
	constraint := NewOtherJobCompletedRecentlyConstraint("check-upstream", "upstream-job", 30*time.Minute, true, false)
	ctx := createTestExecutionContext()

	// Setup mock with failed run
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobHistoryRequest); ok {
			req.ResponseTo <- &JobHistoryResponse{
				JobID: req.JobID,
				Runs: []JobRunSummary{
					{
						RunID:       "run-123",
						StartedAt:   time.Now().Add(-20 * time.Minute),
						CompletedAt: time.Now().Add(-15 * time.Minute),
						Success:     false,
					},
				},
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Met {
		t.Errorf("Expected Met=false (run failed but mustSucceed=true), got Met=true")
	}
}

func TestOtherJobCompletedRecently_NoRuns(t *testing.T) {
	constraint := NewOtherJobCompletedRecentlyConstraint("check-upstream", "upstream-job", 30*time.Minute, false, false)
	ctx := createTestExecutionContext()

	// Setup mock with no runs
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobHistoryRequest); ok {
			req.ResponseTo <- &JobHistoryResponse{
				JobID: req.JobID,
				Runs:  []JobRunSummary{},
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Met {
		t.Errorf("Expected Met=false (no runs), got Met=true")
	}

	if result.Message == "" || result.Message != "job upstream-job has no recent runs" {
		t.Errorf("Expected message about no runs, got: %s", result.Message)
	}
}

func TestOtherJobCompletedRecently_EvaluationTiming(t *testing.T) {
	constraint := NewOtherJobCompletedRecentlyConstraint("test", "job-id", 30*time.Minute, false, false)
	timing := constraint.EvaluationTiming()

	if len(timing) != 1 {
		t.Fatalf("Expected 1 phase, got %d", len(timing))
	}

	if timing[0] != EvaluationPhasePreExecution {
		t.Errorf("Expected EvaluationPhasePreExecution, got %s", timing[0])
	}
}

// ========================
// 4. OtherJobScheduledSoonConstraint Tests
// ========================

func TestOtherJobScheduledSoon_ScheduledWithinWindow(t *testing.T) {
	constraint := NewOtherJobScheduledSoonConstraint("check-scheduled", "high-priority-job", 10*time.Minute, false)
	ctx := createTestExecutionContext()

	// Setup mock to respond with job scheduled in 5 minutes
	nextRun := time.Now().Add(5 * time.Minute)
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobStateRequest); ok {
			req.ResponseTo <- &JobStateResponse{
				JobID:     req.JobID,
				IsRunning: false,
				NextRun:   &nextRun,
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true (scheduled within 10m window), got Met=false")
	}
}

func TestOtherJobScheduledSoon_ScheduledOutsideWindow(t *testing.T) {
	constraint := NewOtherJobScheduledSoonConstraint("check-scheduled", "low-priority-job", 10*time.Minute, false)
	ctx := createTestExecutionContext()

	// Setup mock to respond with job scheduled in 30 minutes
	nextRun := time.Now().Add(30 * time.Minute)
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobStateRequest); ok {
			req.ResponseTo <- &JobStateResponse{
				JobID:     req.JobID,
				IsRunning: false,
				NextRun:   &nextRun,
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Met {
		t.Errorf("Expected Met=false (scheduled outside 10m window), got Met=true")
	}
}

func TestOtherJobScheduledSoon_NoScheduledRun(t *testing.T) {
	constraint := NewOtherJobScheduledSoonConstraint("check-scheduled", "some-job", 10*time.Minute, false)
	ctx := createTestExecutionContext()

	// Setup mock to respond with no scheduled run
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobStateRequest); ok {
			req.ResponseTo <- &JobStateResponse{
				JobID:     req.JobID,
				IsRunning: false,
				NextRun:   nil,
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Met {
		t.Errorf("Expected Met=false (no scheduled run), got Met=true")
	}
}

func TestOtherJobScheduledSoon_ScheduledInPast(t *testing.T) {
	constraint := NewOtherJobScheduledSoonConstraint("check-scheduled", "late-job", 10*time.Minute, false)
	ctx := createTestExecutionContext()

	// Setup mock to respond with job scheduled in the past
	nextRun := time.Now().Add(-5 * time.Minute)
	mockInbox := ctx.SchedulerInbox.(*MockSchedulerInbox)
	mockInbox.SetResponseFunc(func(msg interface{}) {
		if req, ok := msg.(*JobStateRequest); ok {
			req.ResponseTo <- &JobStateResponse{
				JobID:     req.JobID,
				IsRunning: false,
				NextRun:   &nextRun,
			}
		}
	})

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Met {
		t.Errorf("Expected Met=false (scheduled in past), got Met=true")
	}
}

func TestOtherJobScheduledSoon_EvaluationTiming(t *testing.T) {
	constraint := NewOtherJobScheduledSoonConstraint("test", "job-id", 10*time.Minute, false)
	timing := constraint.EvaluationTiming()

	if len(timing) != 1 {
		t.Fatalf("Expected 1 phase, got %d", len(timing))
	}

	if timing[0] != EvaluationPhasePreExecution {
		t.Errorf("Expected EvaluationPhasePreExecution, got %s", timing[0])
	}
}

// ========================
// 5. HTTPHealthCheckConstraint Tests
// ========================

func TestHTTPHealthCheck_Success(t *testing.T) {
	server := createTestHTTPServer(200, 0)
	defer server.Close()

	constraint, err := NewHTTPHealthCheckConstraint("health-check", server.URL, "GET", nil, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	ctx := createTestExecutionContext()
	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true (200 response), got Met=false")
	}
}

func TestHTTPHealthCheck_Failure(t *testing.T) {
	server := createTestHTTPServer(500, 0)
	defer server.Close()

	constraint, err := NewHTTPHealthCheckConstraint("health-check", server.URL, "GET", nil, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	ctx := createTestExecutionContext()
	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Met {
		t.Errorf("Expected Met=false (500 response), got Met=true")
	}
}

func TestHTTPHealthCheck_URLTemplating(t *testing.T) {
	server := createTestHTTPServer(200, 0)
	defer server.Close()

	// Use template in URL
	urlTemplate := server.URL + "/health?job={{.JobName}}"
	constraint, err := NewHTTPHealthCheckConstraint("health-check", urlTemplate, "GET", nil, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	ctx := createTestExecutionContext()
	ctx.Job.Name = "my-test-job"

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true, got Met=false")
	}

	// Check that the message contains the rendered URL
	if !contains(result.Message, "my-test-job") {
		t.Errorf("Expected message to contain rendered job name, got: %s", result.Message)
	}
}

func TestHTTPHealthCheck_HeaderTemplating(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that header was set correctly
		runID := r.Header.Get("X-Run-ID")
		if runID != "test-run-id" {
			t.Errorf("Expected X-Run-ID header to be 'test-run-id', got: %s", runID)
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	headers := map[string]string{
		"X-Run-ID": "{{.RunID}}",
	}

	constraint, err := NewHTTPHealthCheckConstraint("health-check", server.URL, "GET", headers, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	ctx := createTestExecutionContext()
	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true, got Met=false")
	}
}

func TestHTTPHealthCheck_Timeout(t *testing.T) {
	// Server with 5s delay
	server := createTestHTTPServer(200, 5*time.Second)
	defer server.Close()

	// Constraint with 100ms timeout
	constraint, err := NewHTTPHealthCheckConstraint("health-check", server.URL, "GET", nil, "", 100*time.Millisecond, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	ctx := createTestExecutionContext()
	start := time.Now()
	_, err = constraint.Check(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Should complete in roughly 100ms, not 5s
	if elapsed > 1*time.Second {
		t.Errorf("Expected timeout to trigger quickly (~100ms), took %v", elapsed)
	}
}

func TestHTTPHealthCheck_ContextCancellation(t *testing.T) {
	server := createTestHTTPServer(200, 5*time.Second)
	defer server.Close()

	constraint, err := NewHTTPHealthCheckConstraint("health-check", server.URL, "GET", nil, "", 10*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	ctx := createTestExecutionContext()
	cancelCtx, cancel := context.WithCancel(ctx.Context)
	ctx.Context = cancelCtx

	// Cancel immediately
	cancel()

	_, err = constraint.Check(ctx)
	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}
}

func TestHTTPHealthCheck_POSTMethod(t *testing.T) {
	methodReceived := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodReceived = r.Method
		w.WriteHeader(200)
	}))
	defer server.Close()

	constraint, err := NewHTTPHealthCheckConstraint("health-check", server.URL, "POST", nil, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	ctx := createTestExecutionContext()
	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true, got Met=false")
	}

	if methodReceived != "POST" {
		t.Errorf("Expected POST method, got: %s", methodReceived)
	}
}

func TestHTTPHealthCheck_EvaluationTiming(t *testing.T) {
	constraint, err := NewHTTPHealthCheckConstraint("test", "http://example.com", "GET", nil, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	timing := constraint.EvaluationTiming()

	if len(timing) != 1 {
		t.Fatalf("Expected 1 phase, got %d", len(timing))
	}

	if timing[0] != EvaluationPhasePreExecution {
		t.Errorf("Expected EvaluationPhasePreExecution, got %s", timing[0])
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ========================
// 6. MaxRuntimeConstraint Tests
// ========================

func TestMaxRuntime_WithinLimit(t *testing.T) {
	constraint := NewMaxRuntimeConstraint("max-2h", 2*time.Hour)

	// Job started 1 hour ago
	startTime := time.Now().Add(-1 * time.Hour)
	ctx := createTestExecutionContextWithStartTime(startTime)

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true (1h < 2h limit), got Met=false")
	}
}

func TestMaxRuntime_ExceedsLimit(t *testing.T) {
	constraint := NewMaxRuntimeConstraint("max-2h", 2*time.Hour)

	// Job started 3 hours ago
	startTime := time.Now().Add(-3 * time.Hour)
	ctx := createTestExecutionContextWithStartTime(startTime)

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Met {
		t.Errorf("Expected Met=false (3h > 2h limit), got Met=true")
	}
}

func TestMaxRuntime_AtLimit(t *testing.T) {
	constraint := NewMaxRuntimeConstraint("max-1h", 1*time.Hour)

	// Job started just under 1 hour ago (to account for execution time)
	startTime := time.Now().Add(-1*time.Hour + 100*time.Millisecond)
	ctx := createTestExecutionContextWithStartTime(startTime)

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true (at/near limit should pass), got Met=false")
	}
}

func TestMaxRuntime_NoStartTime(t *testing.T) {
	constraint := NewMaxRuntimeConstraint("max-1h", 1*time.Hour)

	ctx := createTestExecutionContext()
	// Don't set StartTime

	_, err := constraint.Check(ctx)
	if err == nil {
		t.Fatal("Expected error about missing start time, got nil")
	}

	if err.Error() != "start time not available" {
		t.Errorf("Expected 'start time not available', got: %s", err.Error())
	}
}

func TestMaxRuntime_EvaluationPhase(t *testing.T) {
	constraint := NewMaxRuntimeConstraint("max-1h", 1*time.Hour)
	timing := constraint.EvaluationTiming()

	if len(timing) != 1 {
		t.Fatalf("Expected 1 phase, got %d", len(timing))
	}

	if timing[0] != EvaluationPhaseDuringExecution {
		t.Errorf("Expected EvaluationPhaseDuringExecution, got %s", timing[0])
	}
}

// ========================
// 7. MinRuntimeConstraint Tests
// ========================

func TestMinRuntime_MeetsMinimum(t *testing.T) {
	constraint := NewMinRuntimeConstraint("min-30s", 30*time.Second)

	// Job ran for 45 seconds
	startTime := time.Now().Add(-45 * time.Second)
	endTime := time.Now()
	ctx := createTestExecutionContextWithTiming(startTime, endTime, 0)

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true (45s >= 30s minimum), got Met=false")
	}
}

func TestMinRuntime_BelowMinimum(t *testing.T) {
	constraint := NewMinRuntimeConstraint("min-30s", 30*time.Second)

	// Job ran for 10 seconds
	startTime := time.Now().Add(-10 * time.Second)
	endTime := time.Now()
	ctx := createTestExecutionContextWithTiming(startTime, endTime, 0)

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Met {
		t.Errorf("Expected Met=false (10s < 30s minimum), got Met=true")
	}
}

func TestMinRuntime_AtMinimum(t *testing.T) {
	constraint := NewMinRuntimeConstraint("min-30s", 30*time.Second)

	// Job ran for exactly 30 seconds
	startTime := time.Now().Add(-30 * time.Second)
	endTime := time.Now()
	ctx := createTestExecutionContextWithTiming(startTime, endTime, 0)

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true (at minimum should pass), got Met=false")
	}
}

func TestMinRuntime_NoTiming(t *testing.T) {
	constraint := NewMinRuntimeConstraint("min-30s", 30*time.Second)

	ctx := createTestExecutionContext()
	// Don't set StartTime or EndTime

	_, err := constraint.Check(ctx)
	if err == nil {
		t.Fatal("Expected error about missing timing, got nil")
	}

	if err.Error() != "start/end time not available" {
		t.Errorf("Expected 'start/end time not available', got: %s", err.Error())
	}
}

func TestMinRuntime_EvaluationPhase(t *testing.T) {
	constraint := NewMinRuntimeConstraint("min-30s", 30*time.Second)
	timing := constraint.EvaluationTiming()

	if len(timing) != 1 {
		t.Fatalf("Expected 1 phase, got %d", len(timing))
	}

	if timing[0] != EvaluationPhasePostExecution {
		t.Errorf("Expected EvaluationPhasePostExecution, got %s", timing[0])
	}
}

// ========================
// 8. AlwaysPassConstraint Tests
// ========================

func TestAlwaysPassConstraint_AlwaysReturnsTrue(t *testing.T) {
	constraint := NewAlwaysPassConstraint("test-pass", false, []EvaluationPhase{EvaluationPhasePreExecution})
	ctx := createTestExecutionContext()

	// Call Check() multiple times
	for i := 0; i < 5; i++ {
		result, err := constraint.Check(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if !result.Met {
			t.Errorf("Call %d: Expected Met=true, got Met=false", i)
		}
		if result.Message != "always pass" {
			t.Errorf("Call %d: Expected message 'always pass', got: %s", i, result.Message)
		}
	}
}

func TestAlwaysPassConstraint_EvaluationTiming(t *testing.T) {
	tests := []struct {
		name   string
		phases []EvaluationPhase
	}{
		{
			name:   "single phase",
			phases: []EvaluationPhase{EvaluationPhasePreExecution},
		},
		{
			name:   "multiple phases",
			phases: []EvaluationPhase{EvaluationPhasePreExecution, EvaluationPhasePostExecution},
		},
		{
			name:   "all phases",
			phases: []EvaluationPhase{EvaluationPhasePreExecution, EvaluationPhaseDuringExecution, EvaluationPhasePostExecution},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint := NewAlwaysPassConstraint("test", false, tt.phases)
			timing := constraint.EvaluationTiming()

			if len(timing) != len(tt.phases) {
				t.Fatalf("Expected %d phases, got %d", len(tt.phases), len(timing))
			}

			for i, expected := range tt.phases {
				if timing[i] != expected {
					t.Errorf("Phase %d: expected %s, got %s", i, expected, timing[i])
				}
			}
		})
	}
}

func TestAlwaysPassConstraint_ShouldRecheckOnRetry(t *testing.T) {
	tests := []struct {
		name    string
		recheck bool
	}{
		{"recheck true", true},
		{"recheck false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint := NewAlwaysPassConstraint("test", tt.recheck, []EvaluationPhase{EvaluationPhasePreExecution})
			if constraint.ShouldRecheckOnRetry() != tt.recheck {
				t.Errorf("Expected ShouldRecheckOnRetry()=%v, got %v", tt.recheck, constraint.ShouldRecheckOnRetry())
			}
		})
	}
}

func TestAlwaysPassConstraint_Name(t *testing.T) {
	constraint := NewAlwaysPassConstraint("my-constraint", false, []EvaluationPhase{EvaluationPhasePreExecution})
	if constraint.Name() != "my-constraint" {
		t.Errorf("Expected name 'my-constraint', got: %s", constraint.Name())
	}
}

// ========================
// 9. AlwaysFailConstraint Tests
// ========================

func TestAlwaysFailConstraint_AlwaysReturnsFalse(t *testing.T) {
	constraint := NewAlwaysFailConstraint("test-fail", false, []EvaluationPhase{EvaluationPhasePreExecution})
	ctx := createTestExecutionContext()

	// Call Check() multiple times
	for i := 0; i < 5; i++ {
		result, err := constraint.Check(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if result.Met {
			t.Errorf("Call %d: Expected Met=false, got Met=true", i)
		}
		if result.Message != "always fail" {
			t.Errorf("Call %d: Expected message 'always fail', got: %s", i, result.Message)
		}
	}
}

func TestAlwaysFailConstraint_EvaluationTiming(t *testing.T) {
	tests := []struct {
		name   string
		phases []EvaluationPhase
	}{
		{
			name:   "single phase",
			phases: []EvaluationPhase{EvaluationPhasePreExecution},
		},
		{
			name:   "multiple phases",
			phases: []EvaluationPhase{EvaluationPhasePreExecution, EvaluationPhasePostExecution},
		},
		{
			name:   "all phases",
			phases: []EvaluationPhase{EvaluationPhasePreExecution, EvaluationPhaseDuringExecution, EvaluationPhasePostExecution},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint := NewAlwaysFailConstraint("test", false, tt.phases)
			timing := constraint.EvaluationTiming()

			if len(timing) != len(tt.phases) {
				t.Fatalf("Expected %d phases, got %d", len(tt.phases), len(timing))
			}

			for i, expected := range tt.phases {
				if timing[i] != expected {
					t.Errorf("Phase %d: expected %s, got %s", i, expected, timing[i])
				}
			}
		})
	}
}

func TestAlwaysFailConstraint_ShouldRecheckOnRetry(t *testing.T) {
	tests := []struct {
		name    string
		recheck bool
	}{
		{"recheck true", true},
		{"recheck false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint := NewAlwaysFailConstraint("test", tt.recheck, []EvaluationPhase{EvaluationPhasePreExecution})
			if constraint.ShouldRecheckOnRetry() != tt.recheck {
				t.Errorf("Expected ShouldRecheckOnRetry()=%v, got %v", tt.recheck, constraint.ShouldRecheckOnRetry())
			}
		})
	}
}

func TestAlwaysFailConstraint_Name(t *testing.T) {
	constraint := NewAlwaysFailConstraint("my-failing-constraint", false, []EvaluationPhase{EvaluationPhasePreExecution})
	if constraint.Name() != "my-failing-constraint" {
		t.Errorf("Expected name 'my-failing-constraint', got: %s", constraint.Name())
	}
}

// ========================
// Action Integration Tests
// ========================

func TestDelayAction_CompletesAfterDuration(t *testing.T) {
	t.Skip("DelayAction not yet implemented")
}

func TestDelayAction_CancellationBeforeCompletion(t *testing.T) {
	t.Skip("DelayAction not yet implemented")
}

func TestWebhookAction_SendsWebhook(t *testing.T) {
	t.Skip("WebhookAction not yet implemented")
}

func TestWebhookAction_WebhookError(t *testing.T) {
	t.Skip("WebhookAction not yet implemented")
}

func TestLogAction_LogsMessage(t *testing.T) {
	t.Skip("LogAction not yet implemented")
}

func TestFailAction_ReturnsError(t *testing.T) {
	t.Skip("FailAction not yet implemented")
}

func TestNoOpAction_DoesNothing(t *testing.T) {
	t.Skip("NoOpAction not yet implemented")
}

// ========================
// ConstraintChecker Integration Tests
// ========================

func TestConstraintChecker_SingleConstraintMet(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_SingleConstraintViolated(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_MultipleConstraintsAllMet(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_MultipleConstraintsOneFails(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_NoConstraints(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_MultipleActionsOnViolation(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_ActionExecutionError(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_ShouldRecheckOnRetry_True(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_ShouldRecheckOnRetry_False(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_ShouldRecheckOnRetry_NoConstraints(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

// ========================
// Multi-Phase Evaluation Tests
// ========================

func TestConstraintChecker_CheckPreExecution_OnlyRunsPrePhase(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_CheckDuringExecution_OnlyRunsDuringPhase(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_CheckPostExecution_OnlyRunsPostPhase(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_CheckDuringExecution_RequiresStartTime(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_CheckPostExecution_RequiresTiming(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_MultiplePhaseConstraint(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

// ========================
// Scheduler Communication Tests
// ========================

func TestSchedulerCommunication_JobStateRequest(t *testing.T) {
	t.Skip("Scheduler communication not yet implemented")
}

func TestSchedulerCommunication_JobStateResponse(t *testing.T) {
	t.Skip("Scheduler communication not yet implemented")
}

func TestSchedulerCommunication_JobHistoryRequest(t *testing.T) {
	t.Skip("Scheduler communication not yet implemented")
}

func TestSchedulerCommunication_JobHistoryResponse(t *testing.T) {
	t.Skip("Scheduler communication not yet implemented")
}

func TestSchedulerCommunication_ContextCancellation(t *testing.T) {
	t.Skip("Scheduler communication not yet implemented")
}

func TestSchedulerCommunication_SchedulerSendError(t *testing.T) {
	t.Skip("Scheduler communication not yet implemented")
}

func TestSchedulerCommunication_ConcurrentRequests(t *testing.T) {
	t.Skip("Scheduler communication not yet implemented")
}

// ========================
// ExecutionContext Tests
// ========================

func TestExecutionContext_CreatedWithDependencies(t *testing.T) {
	ctx := createTestExecutionContext()

	if ctx.Job == nil {
		t.Error("Expected Job to be set")
	}
	if ctx.RunID == "" {
		t.Error("Expected RunID to be set")
	}
	if ctx.Logger == nil {
		t.Error("Expected Logger to be set")
	}
	if ctx.SchedulerInbox == nil {
		t.Error("Expected SchedulerInbox to be set")
	}
	if ctx.WebhookHandler == nil {
		t.Error("Expected WebhookHandler to be set")
	}
	if ctx.HTTPClient == nil {
		t.Error("Expected HTTPClient to be set")
	}
	if ctx.Context == nil {
		t.Error("Expected Context to be set")
	}
}

func TestExecutionContext_SchedulerInboxUsed(t *testing.T) {
	t.Skip("Constraint using scheduler inbox not yet implemented")
}

func TestExecutionContext_WebhookHandlerUsed(t *testing.T) {
	t.Skip("WebhookAction not yet implemented")
}

func TestExecutionContext_LoggerUsed(t *testing.T) {
	t.Skip("LogAction not yet implemented")
}

// ========================
// Error Handling Tests
// ========================

func TestConstraintChecker_ConstraintCheckError(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_ActionExecutionError_Logged(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_ActionExecutionError_SubsequentActionsRun(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_ContextCancelled_DuringCheck(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_ContextCancelled_DuringAction(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

// ========================
// Thread Safety Tests
// ========================

func TestConstraintChecker_ConcurrentChecks(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

func TestConstraintChecker_ConcurrentWithDifferentJobs(t *testing.T) {
	t.Skip("ConstraintChecker not yet implemented")
}

// ========================
// Configuration Parsing Tests (placeholder for when we implement parsing)
// ========================

func TestParseConstraints_ValidSingleConstraint(t *testing.T) {
	t.Skip("Configuration parsing not yet implemented")
}

func TestParseConstraints_ValidMultipleConstraints(t *testing.T) {
	t.Skip("Configuration parsing not yet implemented")
}

func TestParseConstraints_InvalidJSON(t *testing.T) {
	t.Skip("Configuration parsing not yet implemented")
}

func TestParseConstraints_UnknownConstraintType(t *testing.T) {
	t.Skip("Configuration parsing not yet implemented")
}

// HTTP Test Server Helper
func createTestHTTPServer(statusCode int, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		w.WriteHeader(statusCode)
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
}
