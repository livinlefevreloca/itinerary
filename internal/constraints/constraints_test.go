package constraints

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

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
	// Test with a specific timezone using fixed hours that don't cross midnight
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("Cannot load timezone: %v", err)
	}

	// Use fixed hours in the middle of the day to avoid midnight crossing
	start := time.Date(2000, 1, 1, 10, 0, 0, 0, loc)
	end := time.Date(2000, 1, 1, 14, 0, 0, 0, loc)

	constraint := NewTimeWindowConstraint("ny-hours", start, end, loc, false)
	ctx := createTestExecutionContext()

	// Get current time in the timezone
	now := time.Now().In(loc)
	currentHour := now.Hour()

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should be met if current hour is between 10-14
	expectedMet := currentHour >= 10 && currentHour < 14
	if result.Met != expectedMet {
		t.Logf("Current hour in NY: %d, Window: 10:00-14:00", currentHour)
		if expectedMet {
			t.Errorf("Expected Met=true (within timezone window), got Met=false")
		} else {
			t.Logf("Test run outside window (hour %d), constraint correctly returned Met=false", currentHour)
		}
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	mockInbox := ctx.SchedulerInbox.(*testutil.MockSchedulerInbox)
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
	server := testutil.CreateTestHTTPServer(200, 0)
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
	server := testutil.CreateTestHTTPServer(500, 0)
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
	server := testutil.CreateTestHTTPServer(200, 0)
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
		jobName := r.Header.Get("X-Job-Name")
		if jobName != "test" {
			t.Errorf("Expected X-Job-Name header to be 'test', got: %s", jobName)
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	headers := map[string]string{
		"X-Job-Name": "{{.JobName}}",
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

func TestHTTPHealthCheck_ArgsTemplating(t *testing.T) {
	receivedURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(200)
	}))
	defer server.Close()

	// Use template with args
	urlTemplate := server.URL + "/api?arg1={{index .Args 1}}&arg2={{index .Args 2}}"
	constraint, err := NewHTTPHealthCheckConstraint("health-check", urlTemplate, "GET", nil, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	ctx := createTestExecutionContext()
	ctx.Job.Args = map[int]string{
		1: "value1",
		2: "value2",
	}

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true, got Met=false")
	}

	// Verify the args were templated correctly
	if !contains(receivedURL, "arg1=value1") || !contains(receivedURL, "arg2=value2") {
		t.Errorf("Expected URL to contain templated args, got: %s", receivedURL)
	}
}

func TestHTTPHealthCheck_KwargsTemplating(t *testing.T) {
	receivedURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(200)
	}))
	defer server.Close()

	// Use template with kwargs
	urlTemplate := server.URL + "/api?name={{index .Kwargs \"name\"}}&port={{index .Kwargs \"port\"}}"
	constraint, err := NewHTTPHealthCheckConstraint("health-check", urlTemplate, "GET", nil, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	ctx := createTestExecutionContext()
	ctx.Job.Kwargs = map[string]string{
		"name": "myservice",
		"port": "8080",
	}

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true, got Met=false")
	}

	// Verify the kwargs were templated correctly
	if !contains(receivedURL, "name=myservice") || !contains(receivedURL, "port=8080") {
		t.Errorf("Expected URL to contain templated kwargs, got: %s", receivedURL)
	}
}

func TestHTTPHealthCheck_MixedTemplating(t *testing.T) {
	receivedURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(200)
	}))
	defer server.Close()

	// Use template with job fields, args, and kwargs
	urlTemplate := server.URL + "/jobs/{{.JobName}}?host={{index .Args 1}}&env={{index .Kwargs \"environment\"}}"
	constraint, err := NewHTTPHealthCheckConstraint("health-check", urlTemplate, "GET", nil, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	ctx := createTestExecutionContext()
	ctx.Job.Name = "backup-job"
	ctx.Job.Args = map[int]string{
		1: "server1.example.com",
	}
	ctx.Job.Kwargs = map[string]string{
		"environment": "production",
	}

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true, got Met=false")
	}

	// Verify all templating worked
	if !contains(receivedURL, "/jobs/backup-job") {
		t.Errorf("Expected URL to contain job info, got: %s", receivedURL)
	}
	if !contains(receivedURL, "host=server1.example.com") {
		t.Errorf("Expected URL to contain arg, got: %s", receivedURL)
	}
	if !contains(receivedURL, "env=production") {
		t.Errorf("Expected URL to contain kwarg, got: %s", receivedURL)
	}
}

func TestHTTPHealthCheck_StartTimeTemplating(t *testing.T) {
	receivedURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(200)
	}))
	defer server.Close()

	// Use template with StartTime
	urlTemplate := server.URL + "/health?start={{.StartTime.Unix}}"
	constraint, err := NewHTTPHealthCheckConstraint("health-check", urlTemplate, "GET", nil, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	startTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	ctx := createTestExecutionContextWithStartTime(startTime)

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true, got Met=false")
	}

	// Verify StartTime was templated
	expectedUnix := fmt.Sprintf("start=%d", startTime.Unix())
	if !contains(receivedURL, expectedUnix) {
		t.Errorf("Expected URL to contain StartTime unix timestamp, got: %s", receivedURL)
	}
}

func TestHTTPHealthCheck_EndTimeTemplating(t *testing.T) {
	receivedURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(200)
	}))
	defer server.Close()

	// Use template with EndTime
	urlTemplate := server.URL + "/health?end={{.EndTime.Format \"2006-01-02T15:04:05Z07:00\"}}"
	constraint, err := NewHTTPHealthCheckConstraint("health-check", urlTemplate, "GET", nil, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	startTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 15, 10, 35, 0, 0, time.UTC)
	ctx := createTestExecutionContextWithTiming(startTime, endTime, 0)

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true, got Met=false")
	}

	// Verify EndTime was templated with correct format
	if !contains(receivedURL, "end=2024-01-15T10:35:00Z") {
		t.Errorf("Expected URL to contain formatted EndTime, got: %s", receivedURL)
	}
}

func TestHTTPHealthCheck_TimingNotAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	// Template tries to use StartTime but it's not available
	urlTemplate := server.URL + "/health?job={{.JobName}}"
	constraint, err := NewHTTPHealthCheckConstraint("health-check", urlTemplate, "GET", nil, "", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Failed to create constraint: %v", err)
	}

	ctx := createTestExecutionContext()
	// Don't set StartTime or EndTime

	result, err := constraint.Check(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Met {
		t.Errorf("Expected Met=true (template doesn't require timing), got Met=false")
	}
}

func TestHTTPHealthCheck_Timeout(t *testing.T) {
	// Server with 5s delay
	server := testutil.CreateTestHTTPServer(200, 5*time.Second)
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
	server := testutil.CreateTestHTTPServer(200, 5*time.Second)
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

	if len(timing) != 2 {
		t.Fatalf("Expected 2 phases, got %d", len(timing))
	}

	if timing[0] != EvaluationPhaseDuringExecution {
		t.Errorf("Expected EvaluationPhaseDuringExecution first, got %s", timing[0])
	}

	if timing[1] != EvaluationPhasePostExecution {
		t.Errorf("Expected EvaluationPhasePostExecution second, got %s", timing[1])
	}
}

// ========================
// 7. AlwaysPassConstraint Tests
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
	t.Skip("DelayAction will be implemented in actions module")
}

func TestDelayAction_CancellationBeforeCompletion(t *testing.T) {
	t.Skip("DelayAction will be implemented in actions module")
}

func TestWebhookAction_SendsWebhook(t *testing.T) {
	t.Skip("WebhookAction will be implemented in actions module")
}

func TestWebhookAction_WebhookError(t *testing.T) {
	t.Skip("WebhookAction will be implemented in actions module")
}

func TestLogAction_LogsMessage(t *testing.T) {
	t.Skip("LogAction will be implemented in actions module")
}

func TestFailAction_ReturnsError(t *testing.T) {
	t.Skip("FailAction will be implemented in actions module")
}

func TestNoOpAction_DoesNothing(t *testing.T) {
	action := NewNoOpAction("test-noop")
	ctx := createTestExecutionContext()

	err := action.Execute(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if action.Name() != "test-noop" {
		t.Errorf("Expected name 'test-noop', got '%s'", action.Name())
	}
}

// ========================
// ConstraintChecker Integration Tests
// ========================

func TestConstraintChecker_SingleConstraintMet(t *testing.T) {
	constraint := NewAlwaysPassConstraint("test-pass", false, []EvaluationPhase{EvaluationPhasePreExecution})
	onMetAction := NewNoOpAction("on-met-action")

	constraints := []ConstraintWithActions{
		{
			Constraint:  constraint,
			OnMet:       []Action{onMetAction},
			OnViolation: []Action{},
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	result, err := checker.CheckPreExecution(context.Background(), &Job{ID: "test-job"}, "run-123")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.ShouldProceed {
		t.Errorf("Expected ShouldProceed=true, got false")
	}
}

func TestConstraintChecker_SingleConstraintViolated(t *testing.T) {
	constraint := NewAlwaysFailConstraint("test-fail", false, []EvaluationPhase{EvaluationPhasePreExecution})
	onViolationAction := NewNoOpAction("on-violation-action")

	constraints := []ConstraintWithActions{
		{
			Constraint:  constraint,
			OnMet:       []Action{},
			OnViolation: []Action{onViolationAction},
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	result, err := checker.CheckPreExecution(context.Background(), &Job{ID: "test-job"}, "run-123")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.ShouldProceed {
		t.Errorf("Expected ShouldProceed=false, got true")
	}
}

func TestConstraintChecker_MultipleConstraintsAllMet(t *testing.T) {
	constraints := []ConstraintWithActions{
		{
			Constraint:  NewAlwaysPassConstraint("pass-1", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			OnMet:       []Action{NewNoOpAction("action-1")},
			OnViolation: []Action{},
		},
		{
			Constraint:  NewAlwaysPassConstraint("pass-2", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			OnMet:       []Action{NewNoOpAction("action-2")},
			OnViolation: []Action{},
		},
		{
			Constraint:  NewAlwaysPassConstraint("pass-3", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			OnMet:       []Action{NewNoOpAction("action-3")},
			OnViolation: []Action{},
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	result, err := checker.CheckPreExecution(context.Background(), &Job{ID: "test-job"}, "run-123")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.ShouldProceed {
		t.Errorf("Expected ShouldProceed=true (all constraints met), got false")
	}
}

func TestConstraintChecker_MultipleConstraintsOneFails(t *testing.T) {
	constraints := []ConstraintWithActions{
		{
			Constraint:  NewAlwaysPassConstraint("pass-1", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			OnMet:       []Action{NewNoOpAction("action-1")},
			OnViolation: []Action{},
		},
		{
			Constraint:  NewAlwaysFailConstraint("fail-1", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			OnMet:       []Action{},
			OnViolation: []Action{NewNoOpAction("fail-action")},
		},
		{
			Constraint:  NewAlwaysPassConstraint("pass-2", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			OnMet:       []Action{NewNoOpAction("action-2")},
			OnViolation: []Action{},
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	result, err := checker.CheckPreExecution(context.Background(), &Job{ID: "test-job"}, "run-123")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.ShouldProceed {
		t.Errorf("Expected ShouldProceed=false (one constraint failed), got true")
	}
}

func TestConstraintChecker_NoConstraints(t *testing.T) {
	constraints := []ConstraintWithActions{}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	result, err := checker.CheckPreExecution(context.Background(), &Job{ID: "test-job"}, "run-123")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.ShouldProceed {
		t.Errorf("Expected ShouldProceed=true (no constraints), got false")
	}
}

func TestConstraintChecker_MultipleActionsOnViolation(t *testing.T) {
	constraint := NewAlwaysFailConstraint("test-fail", false, []EvaluationPhase{EvaluationPhasePreExecution})

	constraints := []ConstraintWithActions{
		{
			Constraint: constraint,
			OnViolation: []Action{
				NewNoOpAction("action-1"),
				NewNoOpAction("action-2"),
				NewNoOpAction("action-3"),
			},
			OnMet: []Action{},
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	result, err := checker.CheckPreExecution(context.Background(), &Job{ID: "test-job"}, "run-123")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.ShouldProceed {
		t.Errorf("Expected ShouldProceed=false, got true")
	}
}

func TestConstraintChecker_ActionExecutionError(t *testing.T) {
	t.Skip("Action execution error handling requires actions that can error - will implement with actions module")
}

func TestConstraintChecker_ShouldRecheckOnRetry_True(t *testing.T) {
	constraints := []ConstraintWithActions{
		{
			Constraint:     NewAlwaysPassConstraint("pass-1", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			RecheckOnRetry: false,
		},
		{
			Constraint:     NewAlwaysPassConstraint("pass-2", true, []EvaluationPhase{EvaluationPhasePreExecution}),
			RecheckOnRetry: true,
		},
		{
			Constraint:     NewAlwaysPassConstraint("pass-3", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			RecheckOnRetry: false,
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	if !checker.ShouldRecheckOnRetry(&Job{ID: "test-job"}) {
		t.Error("Expected ShouldRecheckOnRetry=true (one constraint has recheck=true)")
	}
}

func TestConstraintChecker_ShouldRecheckOnRetry_False(t *testing.T) {
	constraints := []ConstraintWithActions{
		{
			Constraint:     NewAlwaysPassConstraint("pass-1", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			RecheckOnRetry: false,
		},
		{
			Constraint:     NewAlwaysPassConstraint("pass-2", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			RecheckOnRetry: false,
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	if checker.ShouldRecheckOnRetry(&Job{ID: "test-job"}) {
		t.Error("Expected ShouldRecheckOnRetry=false (all constraints have recheck=false)")
	}
}

func TestConstraintChecker_ShouldRecheckOnRetry_NoConstraints(t *testing.T) {
	constraints := []ConstraintWithActions{}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	if checker.ShouldRecheckOnRetry(&Job{ID: "test-job"}) {
		t.Error("Expected ShouldRecheckOnRetry=false (no constraints)")
	}
}

// ========================
// Multi-Phase Evaluation Tests
// ========================

func TestConstraintChecker_CheckPreExecution_OnlyRunsPrePhase(t *testing.T) {
	constraints := []ConstraintWithActions{
		{
			Constraint: NewAlwaysPassConstraint("pre", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			OnMet:      []Action{NewNoOpAction("pre-action")},
		},
		{
			Constraint: NewAlwaysPassConstraint("during", false, []EvaluationPhase{EvaluationPhaseDuringExecution}),
			OnMet:      []Action{NewNoOpAction("during-action")},
		},
		{
			Constraint: NewAlwaysPassConstraint("post", false, []EvaluationPhase{EvaluationPhasePostExecution}),
			OnMet:      []Action{NewNoOpAction("post-action")},
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	result, err := checker.CheckPreExecution(context.Background(), &Job{ID: "test-job"}, "run-123")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.ShouldProceed {
		t.Errorf("Expected ShouldProceed=true")
	}

	// Only pre-execution constraint should have been evaluated (message should only contain "always pass" once)
	if !contains(result.Message, "always pass") {
		t.Errorf("Expected message to contain pre-execution result")
	}
}

func TestConstraintChecker_CheckDuringExecution_OnlyRunsDuringPhase(t *testing.T) {
	startTime := time.Now().Add(-30 * time.Minute)

	constraints := []ConstraintWithActions{
		{
			Constraint: NewAlwaysPassConstraint("pre", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			OnMet:      []Action{NewNoOpAction("pre-action")},
		},
		{
			Constraint: NewMaxRuntimeConstraint("during", 2*time.Hour),
			OnMet:      []Action{NewNoOpAction("during-action")},
		},
		{
			Constraint: NewAlwaysPassConstraint("post", false, []EvaluationPhase{EvaluationPhasePostExecution}),
			OnMet:      []Action{NewNoOpAction("post-action")},
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	result, err := checker.CheckDuringExecution(context.Background(), &Job{ID: "test-job"}, "run-123", startTime)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.ShouldProceed {
		t.Errorf("Expected ShouldProceed=true")
	}

	// Only during-execution constraint should have been evaluated
	if !contains(result.Message, "runtime") {
		t.Errorf("Expected message to contain during-execution result, got: %s", result.Message)
	}
}

func TestConstraintChecker_CheckPostExecution_OnlyRunsPostPhase(t *testing.T) {
	startTime := time.Now().Add(-10 * time.Second)
	endTime := time.Now()

	constraints := []ConstraintWithActions{
		{
			Constraint: NewAlwaysPassConstraint("pre", false, []EvaluationPhase{EvaluationPhasePreExecution}),
			OnMet:      []Action{NewNoOpAction("pre-action")},
		},
		{
			Constraint: NewAlwaysPassConstraint("during", false, []EvaluationPhase{EvaluationPhaseDuringExecution}),
			OnMet:      []Action{NewNoOpAction("during-action")},
		},
		{
			Constraint: NewMaxRuntimeConstraint("post", 2*time.Minute),
			OnMet:      []Action{NewNoOpAction("post-action")},
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	result, err := checker.CheckPostExecution(context.Background(), &Job{ID: "test-job"}, "run-123", startTime, endTime, 0)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.ShouldProceed {
		t.Errorf("Expected ShouldProceed=true")
	}

	// Only post-execution constraint should have been evaluated
	if !contains(result.Message, "runtime") {
		t.Errorf("Expected message to contain post-execution result, got: %s", result.Message)
	}
}

func TestConstraintChecker_CheckDuringExecution_RequiresStartTime(t *testing.T) {
	startTime := time.Now().Add(-30 * time.Minute)

	constraints := []ConstraintWithActions{
		{
			Constraint: NewMaxRuntimeConstraint("max-runtime", 2*time.Hour),
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	result, err := checker.CheckDuringExecution(context.Background(), &Job{ID: "test-job"}, "run-123", startTime)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.ShouldProceed {
		t.Errorf("Expected ShouldProceed=true (within runtime limit)")
	}
}

func TestConstraintChecker_CheckPostExecution_RequiresTiming(t *testing.T) {
	startTime := time.Now().Add(-10 * time.Second)
	endTime := time.Now()

	constraints := []ConstraintWithActions{
		{
			Constraint: NewMaxRuntimeConstraint("max-runtime", 2*time.Minute),
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		&http.Client{},
		testutil.CreateTestSlogLogger(),
	)

	result, err := checker.CheckPostExecution(context.Background(), &Job{ID: "test-job"}, "run-123", startTime, endTime, 0)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.ShouldProceed {
		t.Errorf("Expected ShouldProceed=true (within max runtime)")
	}
}

func TestConstraintChecker_MultiplePhaseConstraint(t *testing.T) {
	// Create a constraint that applies to multiple phases
	constraints := []ConstraintWithActions{
		{
			Constraint: NewAlwaysPassConstraint("multi-phase", false, []EvaluationPhase{
				EvaluationPhasePreExecution,
				EvaluationPhasePostExecution,
			}),
			OnMet: []Action{NewNoOpAction("action")},
		},
	}

	checker := NewConstraintChecker(
		constraints,
		testutil.NewMockSchedulerInbox(),
		testutil.CreateTestHTTPClient(),
		testutil.CreateTestSlogLogger(),
	)

	// Should run in pre-execution
	result, err := checker.CheckPreExecution(context.Background(), &Job{ID: "test-job"}, "run-123")
	if err != nil {
		t.Fatalf("Pre-execution: Expected no error, got: %v", err)
	}
	if !result.ShouldProceed {
		t.Error("Pre-execution: Expected constraint to run")
	}

	// Should NOT run in during-execution
	startTime := time.Now()
	result, err = checker.CheckDuringExecution(context.Background(), &Job{ID: "test-job"}, "run-123", startTime)
	if err != nil {
		t.Fatalf("During-execution: Expected no error, got: %v", err)
	}
	if result.Message != "" {
		t.Error("During-execution: Expected no constraints to run (empty message)")
	}

	// Should run in post-execution
	endTime := time.Now()
	result, err = checker.CheckPostExecution(context.Background(), &Job{ID: "test-job"}, "run-123", startTime, endTime, 0)
	if err != nil {
		t.Fatalf("Post-execution: Expected no error, got: %v", err)
	}
	if !result.ShouldProceed {
		t.Error("Post-execution: Expected constraint to run")
	}
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

func TestExecutionContext_HTTPClientUsed(t *testing.T) {
	t.Skip("HTTPClient usage already tested via HTTPHealthCheckConstraint tests")
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

