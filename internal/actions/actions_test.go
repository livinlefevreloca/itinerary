package actions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

// TestDelayAction_Success tests DelayAction with a short duration
func TestDelayAction_Success(t *testing.T) {
	action := &DelayAction{
		name:     "delay",
		duration: 50 * time.Millisecond,
	}

	ctx := NewExecutionContextBuilder().Build()
	start := time.Now()
	err := action.Execute(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if elapsed < 40*time.Millisecond || elapsed > 150*time.Millisecond {
		t.Errorf("Expected delay around 50ms, got %v", elapsed)
	}
}

// TestDelayAction_ContextCancelled tests that DelayAction respects context cancellation
func TestDelayAction_ContextCancelled(t *testing.T) {
	action := &DelayAction{
		name:     "delay",
		duration: 5 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	execCtx := NewExecutionContextBuilder().
		WithContext(ctx).
		Build()

	// Cancel context after 50ms
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := action.Execute(execCtx)
	elapsed := time.Since(start)

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}

	// Should complete quickly, not after 5 seconds
	if elapsed > 500*time.Millisecond {
		t.Errorf("Delay took too long after cancellation: %v", elapsed)
	}
}

// TestDelayAction_Name tests the Name method
func TestDelayAction_Name(t *testing.T) {
	action := &DelayAction{name: "delay"}
	if action.Name() != "delay" {
		t.Errorf("Expected name 'delay', got '%s'", action.Name())
	}
}

// TestDelayAction_ZeroDuration tests DelayAction with zero duration
func TestDelayAction_ZeroDuration(t *testing.T) {
	action := &DelayAction{
		name:     "delay",
		duration: 0,
	}

	ctx := NewExecutionContextBuilder().Build()
	start := time.Now()
	err := action.Execute(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Should complete almost immediately
	if elapsed > 10*time.Millisecond {
		t.Errorf("Zero duration delay took too long: %v", elapsed)
	}
}

// TestWebhookAction_Success tests successful webhook sending
func TestWebhookAction_Success(t *testing.T) {
	mockHandler := testutil.NewMockWebhookHandler()
	action := &WebhookAction{
		name:    "webhook",
		url:     "https://example.com/webhook",
		payload: map[string]interface{}{"status": "success"},
	}

	ctx := NewExecutionContextBuilder().
		WithWebhookHandler(mockHandler).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	calls := mockHandler.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 webhook call, got %d", len(calls))
	}

	if calls[0].URL != "https://example.com/webhook" {
		t.Errorf("Expected URL 'https://example.com/webhook', got '%s'", calls[0].URL)
	}
}

// TestWebhookAction_Name tests the Name method
func TestWebhookAction_Name(t *testing.T) {
	action := &WebhookAction{name: "webhook"}
	if action.Name() != "webhook" {
		t.Errorf("Expected name 'webhook', got '%s'", action.Name())
	}
}

// TestLogAction_Success tests successful logging
func TestLogAction_Success(t *testing.T) {
	action := &LogAction{
		name:    "log",
		message: "Test log message",
	}

	ctx := NewExecutionContextBuilder().Build()
	err := action.Execute(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestLogAction_Name tests the Name method
func TestLogAction_Name(t *testing.T) {
	action := &LogAction{name: "log"}
	if action.Name() != "log" {
		t.Errorf("Expected name 'log', got '%s'", action.Name())
	}
}

// TestFailAction_Success tests that FailAction returns an error
func TestFailAction_Success(t *testing.T) {
	action := &FailAction{
		name:   "fail",
		reason: "test failure",
	}

	ctx := NewExecutionContextBuilder().Build()
	err := action.Execute(ctx)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if err.Error() != "job failed: test failure" {
		t.Errorf("Expected error 'job failed: test failure', got '%s'", err.Error())
	}
}

// TestFailAction_Name tests the Name method
func TestFailAction_Name(t *testing.T) {
	action := &FailAction{name: "fail"}
	if action.Name() != "fail" {
		t.Errorf("Expected name 'fail', got '%s'", action.Name())
	}
}

// TestNoOpAction_Success tests that NoOpAction does nothing
func TestNoOpAction_Success(t *testing.T) {
	action := &NoOpAction{name: "noop"}
	ctx := NewExecutionContextBuilder().Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestNoOpAction_Name tests the Name method
func TestNoOpAction_Name(t *testing.T) {
	action := &NoOpAction{name: "noop"}
	if action.Name() != "noop" {
		t.Errorf("Expected name 'noop', got '%s'", action.Name())
	}
}

// TestRetryAction_Success tests successful retry
func TestRetryAction_Success(t *testing.T) {
	mockController := testutil.NewMockJobController()
	action := &RetryAction{name: "retry"}

	ctx := NewExecutionContextBuilder().
		WithJobController(mockController).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	calls := mockController.GetRetryCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 retry call, got %d", len(calls))
	}

	if calls[0] != "test-job-id" {
		t.Errorf("Expected job ID 'test-job-id', got '%s'", calls[0])
	}
}

// TestRetryAction_ControllerError tests error handling
func TestRetryAction_ControllerError(t *testing.T) {
	mockController := testutil.NewMockJobController()
	mockController.SetError(errors.New("controller error"))

	action := &RetryAction{name: "retry"}
	ctx := NewExecutionContextBuilder().
		WithJobController(mockController).
		Build()

	err := action.Execute(ctx)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// TestRetryAction_Name tests the Name method
func TestRetryAction_Name(t *testing.T) {
	action := &RetryAction{name: "retry"}
	if action.Name() != "retry" {
		t.Errorf("Expected name 'retry', got '%s'", action.Name())
	}
}

// TestTriggerJobAction_Success tests successful job triggering
func TestTriggerJobAction_Success(t *testing.T) {
	mockController := testutil.NewMockJobController()
	action := &TriggerJobAction{
		name:  "trigger_job",
		jobID: "target-job",
		args:  map[string]interface{}{"key": "value"},
	}

	ctx := NewExecutionContextBuilder().
		WithJobController(mockController).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	calls := mockController.GetTriggerCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 trigger call, got %d", len(calls))
	}

	call := calls[0]
	if call.JobID != "target-job" {
		t.Errorf("Expected job ID 'target-job', got '%s'", call.JobID)
	}
}

// TestTriggerJobAction_Name tests the Name method
func TestTriggerJobAction_Name(t *testing.T) {
	action := &TriggerJobAction{name: "trigger_job"}
	if action.Name() != "trigger_job" {
		t.Errorf("Expected name 'trigger_job', got '%s'", action.Name())
	}
}

// TestSlackAction_Success tests successful Slack notification
func TestSlackAction_Success(t *testing.T) {
	mockHandler := testutil.NewMockWebhookHandler()
	action := &SlackAction{
		name:       "slack",
		webhookURL: "https://hooks.slack.com/test",
		channel:    "#test",
		username:   "bot",
		text:       "test message",
		iconEmoji:  ":robot:",
	}

	ctx := NewExecutionContextBuilder().
		WithWebhookHandler(mockHandler).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	calls := mockHandler.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 webhook call, got %d", len(calls))
	}
}

// TestSlackAction_Name tests the Name method
func TestSlackAction_Name(t *testing.T) {
	action := &SlackAction{name: "slack"}
	if action.Name() != "slack" {
		t.Errorf("Expected name 'slack', got '%s'", action.Name())
	}
}

// TestPagerDutyAction_Success tests successful PagerDuty incident creation
func TestPagerDutyAction_Success(t *testing.T) {
	mockHandler := testutil.NewMockWebhookHandler()
	action := &PagerDutyAction{
		name:       "pagerduty",
		routingKey: "test-key",
		severity:   "error",
		summary:    "test incident",
		source:     "test",
	}

	ctx := NewExecutionContextBuilder().
		WithWebhookHandler(mockHandler).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	calls := mockHandler.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 webhook call, got %d", len(calls))
	}
}

// TestPagerDutyAction_Name tests the Name method
func TestPagerDutyAction_Name(t *testing.T) {
	action := &PagerDutyAction{name: "pagerduty"}
	if action.Name() != "pagerduty" {
		t.Errorf("Expected name 'pagerduty', got '%s'", action.Name())
	}
}

// TestKillAllInstancesAction_Success tests successful kill all instances
func TestKillAllInstancesAction_Success(t *testing.T) {
	mockController := testutil.NewMockJobController()
	action := &KillAllInstancesAction{
		name:  "kill_all_instances",
		jobID: "target-job",
	}

	ctx := NewExecutionContextBuilder().
		WithJobController(mockController).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	calls := mockController.GetKillAllCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 kill all call, got %d", len(calls))
	}

	if calls[0] != "target-job" {
		t.Errorf("Expected job ID 'target-job', got '%s'", calls[0])
	}
}

// TestKillAllInstancesAction_Name tests the Name method
func TestKillAllInstancesAction_Name(t *testing.T) {
	action := &KillAllInstancesAction{name: "kill_all_instances"}
	if action.Name() != "kill_all_instances" {
		t.Errorf("Expected name 'kill_all_instances', got '%s'", action.Name())
	}
}

// TestKillLatestInstanceAction_Success tests successful kill latest instance
func TestKillLatestInstanceAction_Success(t *testing.T) {
	mockController := testutil.NewMockJobController()
	action := &KillLatestInstanceAction{
		name:  "kill_latest_instance",
		jobID: "target-job",
	}

	ctx := NewExecutionContextBuilder().
		WithJobController(mockController).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	calls := mockController.GetKillLatestCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 kill latest call, got %d", len(calls))
	}

	if calls[0] != "target-job" {
		t.Errorf("Expected job ID 'target-job', got '%s'", calls[0])
	}
}

// TestKillLatestInstanceAction_Name tests the Name method
func TestKillLatestInstanceAction_Name(t *testing.T) {
	action := &KillLatestInstanceAction{name: "kill_latest_instance"}
	if action.Name() != "kill_latest_instance" {
		t.Errorf("Expected name 'kill_latest_instance', got '%s'", action.Name())
	}
}

// TestSkipNextInstanceAction_Success tests successful skip next instance
func TestSkipNextInstanceAction_Success(t *testing.T) {
	mockController := testutil.NewMockJobController()
	action := &SkipNextInstanceAction{
		name:  "skip_next_instance",
		jobID: "target-job",
	}

	ctx := NewExecutionContextBuilder().
		WithJobController(mockController).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	calls := mockController.GetSkipNextCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 skip next call, got %d", len(calls))
	}

	if calls[0] != "target-job" {
		t.Errorf("Expected job ID 'target-job', got '%s'", calls[0])
	}
}

// TestSkipNextInstanceAction_Name tests the Name method
func TestSkipNextInstanceAction_Name(t *testing.T) {
	action := &SkipNextInstanceAction{name: "skip_next_instance"}
	if action.Name() != "skip_next_instance" {
		t.Errorf("Expected name 'skip_next_instance', got '%s'", action.Name())
	}
}

// TestUpdateMetadataAction_Success tests successful metadata update
func TestUpdateMetadataAction_Success(t *testing.T) {
	mockUpdater := testutil.NewMockMetadataUpdater()
	action := &UpdateMetadataAction{
		name:  "update_metadata",
		jobID: "test-job",
		metadata: map[string]interface{}{
			"key": "value",
		},
	}

	ctx := NewExecutionContextBuilder().
		WithMetadataUpdater(mockUpdater).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	calls := mockUpdater.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 update call, got %d", len(calls))
	}

	if calls[0].JobID != "test-job" {
		t.Errorf("Expected job ID 'test-job', got '%s'", calls[0].JobID)
	}
}

// TestUpdateMetadataAction_Name tests the Name method
func TestUpdateMetadataAction_Name(t *testing.T) {
	action := &UpdateMetadataAction{name: "update_metadata"}
	if action.Name() != "update_metadata" {
		t.Errorf("Expected name 'update_metadata', got '%s'", action.Name())
	}
}

// TestMetricAction_Success tests successful metric recording
func TestMetricAction_Success(t *testing.T) {
	mockRecorder := testutil.NewMockMetricRecorder()
	action := &MetricAction{
		name:       "metric",
		metricName: "test.metric",
		value:      123.45,
		tags:       map[string]string{"env": "test"},
	}

	ctx := NewExecutionContextBuilder().
		WithMetricRecorder(mockRecorder).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	calls := mockRecorder.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 record call, got %d", len(calls))
	}

	if calls[0].Name != "test.metric" {
		t.Errorf("Expected metric name 'test.metric', got '%s'", calls[0].Name)
	}

	if calls[0].Value != 123.45 {
		t.Errorf("Expected value 123.45, got %f", calls[0].Value)
	}
}

// TestMetricAction_Name tests the Name method
func TestMetricAction_Name(t *testing.T) {
	action := &MetricAction{name: "metric"}
	if action.Name() != "metric" {
		t.Errorf("Expected name 'metric', got '%s'", action.Name())
	}
}
