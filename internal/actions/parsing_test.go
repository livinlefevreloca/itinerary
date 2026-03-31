package actions

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/model"
)

// TestCreateAction_DelayAction tests creating a DelayAction
func TestCreateAction_DelayAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "delay",
		Config: json.RawMessage(`{"duration":"1h"}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	delayAction, ok := action.(*DelayAction)
	if !ok {
		t.Fatal("Expected *DelayAction")
	}

	if delayAction.Name() != "delay" {
		t.Errorf("Expected name 'delay', got '%s'", delayAction.Name())
	}
}

// TestCreateAction_DelayAction_InvalidDuration tests error handling for invalid duration
func TestCreateAction_DelayAction_InvalidDuration(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "delay",
		Config: json.RawMessage(`{"duration":"invalid"}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for invalid duration, got nil")
	}

	if !strings.Contains(err.Error(), "duration") {
		t.Errorf("Expected error to mention duration, got '%s'", err.Error())
	}
}

// TestCreateAction_WebhookAction tests creating a WebhookAction
func TestCreateAction_WebhookAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "webhook",
		Config: json.RawMessage(`{"url":"http://example.com","payload":{"key":"value"}}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	webhookAction, ok := action.(*WebhookAction)
	if !ok {
		t.Fatal("Expected *WebhookAction")
	}

	if webhookAction.Name() != "webhook" {
		t.Errorf("Expected name 'webhook', got '%s'", webhookAction.Name())
	}
}

// TestCreateAction_LogAction tests creating a LogAction
func TestCreateAction_LogAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "log",
		Config: json.RawMessage(`{"message":"test"}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	logAction, ok := action.(*LogAction)
	if !ok {
		t.Fatal("Expected *LogAction")
	}

	if logAction.Name() != "log" {
		t.Errorf("Expected name 'log', got '%s'", logAction.Name())
	}
}

// TestCreateAction_FailAction tests creating a FailAction
func TestCreateAction_FailAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "fail",
		Config: json.RawMessage(`{"reason":"test failure"}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	failAction, ok := action.(*FailAction)
	if !ok {
		t.Fatal("Expected *FailAction")
	}

	if failAction.Name() != "fail" {
		t.Errorf("Expected name 'fail', got '%s'", failAction.Name())
	}
}

// TestCreateAction_NoOpAction tests creating a NoOpAction
func TestCreateAction_NoOpAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "noop",
		Config: json.RawMessage(`{}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	noopAction, ok := action.(*NoOpAction)
	if !ok {
		t.Fatal("Expected *NoOpAction")
	}

	if noopAction.Name() != "noop" {
		t.Errorf("Expected name 'noop', got '%s'", noopAction.Name())
	}
}

// TestCreateAction_RetryAction tests creating a RetryAction
func TestCreateAction_RetryAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "retry",
		Config: json.RawMessage(`{}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	retryAction, ok := action.(*RetryAction)
	if !ok {
		t.Fatal("Expected *RetryAction")
	}

	if retryAction.Name() != "retry" {
		t.Errorf("Expected name 'retry', got '%s'", retryAction.Name())
	}
}

// TestCreateAction_TriggerJobAction tests creating a TriggerJobAction
func TestCreateAction_TriggerJobAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "trigger_job",
		Config: json.RawMessage(`{"job_id":"job-123","args":{"key":"value"}}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	triggerAction, ok := action.(*TriggerJobAction)
	if !ok {
		t.Fatal("Expected *TriggerJobAction")
	}

	if triggerAction.jobID != "job-123" {
		t.Errorf("Expected jobID 'job-123', got '%s'", triggerAction.jobID)
	}
}

// TestCreateAction_TriggerJobAction_MissingJobID tests error for missing job_id
func TestCreateAction_TriggerJobAction_MissingJobID(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "trigger_job",
		Config: json.RawMessage(`{"args":{}}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for missing job_id, got nil")
	}

	if !strings.Contains(err.Error(), "job_id") {
		t.Errorf("Expected error to mention job_id, got '%s'", err.Error())
	}
}

// TestCreateAction_SlackAction tests creating a SlackAction
func TestCreateAction_SlackAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "slack",
		Config: json.RawMessage(`{"webhook_url":"https://hooks.slack.com/test","text":"alert"}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	slackAction, ok := action.(*SlackAction)
	if !ok {
		t.Fatal("Expected *SlackAction")
	}

	if slackAction.Name() != "slack" {
		t.Errorf("Expected name 'slack', got '%s'", slackAction.Name())
	}
}

// TestCreateAction_SlackAction_MissingWebhookURL tests error for missing webhook_url
func TestCreateAction_SlackAction_MissingWebhookURL(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "slack",
		Config: json.RawMessage(`{"text":"alert"}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for missing webhook_url, got nil")
	}

	if !strings.Contains(err.Error(), "webhook_url") {
		t.Errorf("Expected error to mention webhook_url, got '%s'", err.Error())
	}
}

// TestCreateAction_PagerDutyAction tests creating a PagerDutyAction
func TestCreateAction_PagerDutyAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "pagerduty",
		Config: json.RawMessage(`{"routing_key":"key","severity":"error","summary":"alert"}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	pdAction, ok := action.(*PagerDutyAction)
	if !ok {
		t.Fatal("Expected *PagerDutyAction")
	}

	if pdAction.Name() != "pagerduty" {
		t.Errorf("Expected name 'pagerduty', got '%s'", pdAction.Name())
	}
}

// TestCreateAction_PagerDutyAction_MissingRoutingKey tests error for missing routing_key
func TestCreateAction_PagerDutyAction_MissingRoutingKey(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "pagerduty",
		Config: json.RawMessage(`{"severity":"error"}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for missing routing_key, got nil")
	}

	if !strings.Contains(err.Error(), "routing_key") {
		t.Errorf("Expected error to mention routing_key, got '%s'", err.Error())
	}
}

// TestCreateAction_KillAllInstancesAction tests creating a KillAllInstancesAction
func TestCreateAction_KillAllInstancesAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "kill_all_instances",
		Config: json.RawMessage(`{"job_id":"job-123"}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	killAction, ok := action.(*KillAllInstancesAction)
	if !ok {
		t.Fatal("Expected *KillAllInstancesAction")
	}

	if killAction.jobID != "job-123" {
		t.Errorf("Expected jobID 'job-123', got '%s'", killAction.jobID)
	}
}

// TestCreateAction_KillAllInstancesAction_MissingJobID tests error for missing job_id
func TestCreateAction_KillAllInstancesAction_MissingJobID(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "kill_all_instances",
		Config: json.RawMessage(`{}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for missing job_id, got nil")
	}

	if !strings.Contains(err.Error(), "job_id") {
		t.Errorf("Expected error to mention job_id, got '%s'", err.Error())
	}
}

// TestCreateAction_UpdateMetadataAction tests creating an UpdateMetadataAction
func TestCreateAction_UpdateMetadataAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "update_metadata",
		Config: json.RawMessage(`{"job_id":"job-123","metadata":{"key":"value"}}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	updateAction, ok := action.(*UpdateMetadataAction)
	if !ok {
		t.Fatal("Expected *UpdateMetadataAction")
	}

	if updateAction.jobID != "job-123" {
		t.Errorf("Expected jobID 'job-123', got '%s'", updateAction.jobID)
	}
}

// TestCreateAction_UpdateMetadataAction_MissingJobID tests error for missing job_id
func TestCreateAction_UpdateMetadataAction_MissingJobID(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "update_metadata",
		Config: json.RawMessage(`{"metadata":{}}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for missing job_id, got nil")
	}

	if !strings.Contains(err.Error(), "job_id") {
		t.Errorf("Expected error to mention job_id, got '%s'", err.Error())
	}
}

// TestCreateAction_UpdateMetadataAction_MissingMetadata tests error for missing metadata
func TestCreateAction_UpdateMetadataAction_MissingMetadata(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "update_metadata",
		Config: json.RawMessage(`{"job_id":"job-123"}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for missing metadata, got nil")
	}

	if !strings.Contains(err.Error(), "metadata") {
		t.Errorf("Expected error to mention metadata, got '%s'", err.Error())
	}
}

// TestCreateAction_UpdateMetadataAction_EmptyMetadata tests error for empty metadata
func TestCreateAction_UpdateMetadataAction_EmptyMetadata(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "update_metadata",
		Config: json.RawMessage(`{"job_id":"job-123","metadata":{}}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for empty metadata, got nil")
	}

	if !strings.Contains(err.Error(), "metadata") {
		t.Errorf("Expected error to mention metadata, got '%s'", err.Error())
	}
}

// TestCreateAction_MetricAction tests creating a MetricAction
func TestCreateAction_MetricAction(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "metric",
		Config: json.RawMessage(`{"name":"metric.name","value":123.45,"tags":{"env":"prod"}}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	metricAction, ok := action.(*MetricAction)
	if !ok {
		t.Fatal("Expected *MetricAction")
	}

	if metricAction.metricName != "metric.name" {
		t.Errorf("Expected metricName 'metric.name', got '%s'", metricAction.metricName)
	}
}

// TestCreateAction_MetricAction_StringValue tests creating a MetricAction with string value
func TestCreateAction_MetricAction_StringValue(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "metric",
		Config: json.RawMessage(`{"name":"metric.name","value":"100"}`),
	}

	action, err := CreateAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	metricAction, ok := action.(*MetricAction)
	if !ok {
		t.Fatal("Expected *MetricAction")
	}

	if metricAction.value.(string) != "100" {
		t.Errorf("Expected value '100', got '%v'", metricAction.value)
	}
}

// TestCreateAction_MetricAction_MissingName tests error for missing name
func TestCreateAction_MetricAction_MissingName(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "metric",
		Config: json.RawMessage(`{"value":100}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for missing name, got nil")
	}

	if !strings.Contains(err.Error(), "name") {
		t.Errorf("Expected error to mention name, got '%s'", err.Error())
	}
}

// TestCreateAction_MetricAction_MissingValue tests error for missing value
func TestCreateAction_MetricAction_MissingValue(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "metric",
		Config: json.RawMessage(`{"name":"metric.name"}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for missing value, got nil")
	}

	if !strings.Contains(err.Error(), "value") {
		t.Errorf("Expected error to mention value, got '%s'", err.Error())
	}
}

// TestCreateAction_MetricAction_InvalidValueType tests error for invalid value type
func TestCreateAction_MetricAction_InvalidValueType(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "metric",
		Config: json.RawMessage(`{"name":"metric.name","value":true}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for invalid value type, got nil")
	}
}

// TestCreateAction_UnknownType tests error for unknown action type
func TestCreateAction_UnknownType(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "unknown",
		Config: json.RawMessage(`{}`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for unknown type, got nil")
	}

	expectedError := "unknown action type: unknown"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestCreateAction_InvalidJSON tests error for invalid JSON
func TestCreateAction_InvalidJSON(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "delay",
		Config: json.RawMessage(`{invalid json`),
	}

	_, err := CreateAction(config)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// TestParseDelayAction_ValidDurations tests parsing various valid durations
func TestParseDelayAction_ValidDurations(t *testing.T) {
	tests := []struct {
		duration string
		expected time.Duration
	}{
		{"1s", 1 * time.Second},
		{"5m", 5 * time.Minute},
		{"2h", 2 * time.Hour},
		{"1h30m", 90 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.duration, func(t *testing.T) {
			config := model.ActionParseConfig{
				Type:   "delay",
				Config: json.RawMessage(`{"duration":"` + tt.duration + `"}`),
			}

			action, err := parseDelayAction(config)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if action.duration != tt.expected {
				t.Errorf("Expected duration %v, got %v", tt.expected, action.duration)
			}
		})
	}
}

// TestParseDelayAction_InvalidDuration tests parsing invalid durations
func TestParseDelayAction_InvalidDuration(t *testing.T) {
	invalidDurations := []string{"invalid", "1x", ""}

	for _, duration := range invalidDurations {
		t.Run(duration, func(t *testing.T) {
			config := model.ActionParseConfig{
				Type:   "delay",
				Config: json.RawMessage(`{"duration":"` + duration + `"}`),
			}

			_, err := parseDelayAction(config)
			if err == nil {
				t.Error("Expected error for invalid duration, got nil")
			}
		})
	}
}

// TestParseWebhookAction_ComplexPayload tests parsing webhook with complex payload
func TestParseWebhookAction_ComplexPayload(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "webhook",
		Config: json.RawMessage(`{"url":"http://example.com","payload":{"nested":{"key":"value"},"array":[1,2,3]}}`),
	}

	action, err := parseWebhookAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	payload, ok := action.payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be a map")
	}

	nested, ok := payload["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected nested to be a map")
	}

	if nested["key"] != "value" {
		t.Errorf("Expected nested key to be 'value', got '%v'", nested["key"])
	}
}

// TestParseMetricAction_ZeroValue tests that zero value is valid
func TestParseMetricAction_ZeroValue(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "metric",
		Config: json.RawMessage(`{"name":"metric.name","value":0}`),
	}

	action, err := parseMetricAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if action.value.(float64) != 0.0 {
		t.Errorf("Expected value 0.0, got %v", action.value)
	}
}

// TestParseMetricAction_NegativeValue tests that negative value is valid
func TestParseMetricAction_NegativeValue(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "metric",
		Config: json.RawMessage(`{"name":"metric.name","value":-10.5}`),
	}

	action, err := parseMetricAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if action.value.(float64) != -10.5 {
		t.Errorf("Expected value -10.5, got %v", action.value)
	}
}

// TestParseMetricAction_TemplateInValue tests that template strings are stored as-is
func TestParseMetricAction_TemplateInValue(t *testing.T) {
	config := model.ActionParseConfig{
		Type:   "metric",
		Config: json.RawMessage(`{"name":"metric.name","value":"{{.Variable}}"}`),
	}

	action, err := parseMetricAction(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if action.value.(string) != "{{.Variable}}" {
		t.Errorf("Expected value '{{.Variable}}', got '%v'", action.value)
	}
}
