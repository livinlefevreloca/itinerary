package actions

import (
	"testing"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/model"
	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

// TestRenderTemplate_SimpleVariable tests rendering a simple variable
func TestRenderTemplate_SimpleVariable(t *testing.T) {
	data := &model.TemplateData{
		JobID: "job-123",
	}

	result, err := renderTemplate("Job {{.JobID}}", data)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "Job job-123"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// TestRenderTemplate_MultipleVariables tests rendering multiple variables
func TestRenderTemplate_MultipleVariables(t *testing.T) {
	data := &model.TemplateData{
		JobID: "job-123",
		RunID: "run-456",
	}

	result, err := renderTemplate("Job {{.JobID}} Run {{.RunID}}", data)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "Job job-123 Run run-456"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// TestRenderTemplate_Command tests rendering command
func TestRenderTemplate_Command(t *testing.T) {
	data := &model.TemplateData{
		Command: "deploy.sh",
	}

	result, err := renderTemplate("Command: {{.Command}}", data)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "Command: deploy.sh"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// TestRenderTemplate_KwargsIndex tests accessing kwargs
func TestRenderTemplate_KwargsIndex(t *testing.T) {
	data := &model.TemplateData{
		Kwargs: map[string]string{"env": "prod"},
	}

	result, err := renderTemplate("Env: {{index .Kwargs \"env\"}}", data)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "Env: prod"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// TestRenderTemplate_ArgsIndex tests accessing args
func TestRenderTemplate_ArgsIndex(t *testing.T) {
	data := &model.TemplateData{
		Args: []string{"arg1", "arg2"},
	}

	result, err := renderTemplate("First: {{index .Args 0}}", data)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "First: arg1"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// TestRenderTemplate_InvalidSyntax tests handling of invalid template syntax
func TestRenderTemplate_InvalidSyntax(t *testing.T) {
	data := &model.TemplateData{}

	_, err := renderTemplate("{{.JobID", data)
	if err == nil {
		t.Error("Expected error for invalid syntax, got nil")
	}
}

// TestRenderTemplate_NoTemplateVariables tests plain text with no variables
func TestRenderTemplate_NoTemplateVariables(t *testing.T) {
	data := &model.TemplateData{}

	result, err := renderTemplate("Plain text with no variables", data)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "Plain text with no variables"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// TestRenderTemplate_URLEncoding tests rendering in URLs
func TestRenderTemplate_URLEncoding(t *testing.T) {
	data := &model.TemplateData{
		JobID: "job-123",
		RunID: "run-456",
	}

	result, err := renderTemplate("https://api.com?job={{.JobID}}&run={{.RunID}}", data)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "https://api.com?job=job-123&run=run-456"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// TestRenderPayload_SimpleObject tests rendering a simple object
func TestRenderPayload_SimpleObject(t *testing.T) {
	data := &model.TemplateData{
		JobID: "job-123",
	}

	payload := map[string]interface{}{
		"job": "{{.JobID}}",
	}

	result, err := renderPayload(payload, data)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["job"] != "job-123" {
		t.Errorf("Expected job field to be 'job-123', got '%v'", resultMap["job"])
	}
}

// TestRenderPayload_NestedObject tests rendering nested objects
func TestRenderPayload_NestedObject(t *testing.T) {
	data := &model.TemplateData{
		JobID:   "job-123",
		JobName: "test-job",
	}

	payload := map[string]interface{}{
		"job": map[string]interface{}{
			"id":   "{{.JobID}}",
			"name": "{{.JobName}}",
		},
	}

	result, err := renderPayload(payload, data)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	jobMap, ok := resultMap["job"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected job field to be a map")
	}

	if jobMap["id"] != "job-123" {
		t.Errorf("Expected id to be 'job-123', got '%v'", jobMap["id"])
	}

	if jobMap["name"] != "test-job" {
		t.Errorf("Expected name to be 'test-job', got '%v'", jobMap["name"])
	}
}

// TestRenderPayload_EmptyObject tests rendering an empty object
func TestRenderPayload_EmptyObject(t *testing.T) {
	data := &model.TemplateData{}
	payload := map[string]interface{}{}

	result, err := renderPayload(payload, data)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if len(resultMap) != 0 {
		t.Errorf("Expected empty map, got %v", resultMap)
	}
}

// TestBuildTemplateData_AllFields tests building template data with all fields
func TestBuildTemplateData_AllFields(t *testing.T) {
	ctx := NewExecutionContextBuilder().
		WithJob(&model.Job{ID: "job-123", Name: "test-job"}).
		WithRunID("run-456").
		WithCommand("deploy.sh").
		WithArgs([]string{"arg1", "arg2"}).
		WithKwargs(map[string]string{"env": "prod"}).
		Build()

	data := buildTemplateData(ctx)

	if data.JobID != "job-123" {
		t.Errorf("Expected JobID 'job-123', got '%s'", data.JobID)
	}

	if data.JobName != "test-job" {
		t.Errorf("Expected JobName 'test-job', got '%s'", data.JobName)
	}

	if data.RunID != "run-456" {
		t.Errorf("Expected RunID 'run-456', got '%s'", data.RunID)
	}

	if data.Command != "deploy.sh" {
		t.Errorf("Expected Command 'deploy.sh', got '%s'", data.Command)
	}

	if len(data.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(data.Args))
	}

	if data.Kwargs["env"] != "prod" {
		t.Errorf("Expected env kwarg 'prod', got '%s'", data.Kwargs["env"])
	}
}

// TestBuildTemplateData_EmptyKwargs tests building template data with empty kwargs
func TestBuildTemplateData_EmptyKwargs(t *testing.T) {
	ctx := NewExecutionContextBuilder().
		WithKwargs(make(map[string]string)).
		Build()

	data := buildTemplateData(ctx)

	if data.Kwargs == nil {
		t.Error("Expected Kwargs to be non-nil")
	}

	if len(data.Kwargs) != 0 {
		t.Errorf("Expected empty Kwargs, got %v", data.Kwargs)
	}
}

// TestBuildTemplateData_EmptyArgs tests building template data with empty args
func TestBuildTemplateData_EmptyArgs(t *testing.T) {
	ctx := NewExecutionContextBuilder().
		WithArgs([]string{}).
		Build()

	data := buildTemplateData(ctx)

	if data.Args == nil {
		t.Error("Expected Args to be non-nil")
	}

	if len(data.Args) != 0 {
		t.Errorf("Expected empty Args, got %v", data.Args)
	}
}

// TestBuildTemplateData_Timestamp tests that timestamp is set
func TestBuildTemplateData_Timestamp(t *testing.T) {
	ctx := NewExecutionContextBuilder().Build()

	before := time.Now()
	data := buildTemplateData(ctx)
	after := time.Now()

	if data.Timestamp.Before(before) || data.Timestamp.After(after) {
		t.Errorf("Expected timestamp between %v and %v, got %v", before, after, data.Timestamp)
	}
}

// TestWebhookAction_TemplateInURL tests template rendering in URL
func TestWebhookAction_TemplateInURL(t *testing.T) {
	mockHandler := testutil.NewMockWebhookHandler()
	action := &WebhookAction{
		name:    "webhook",
		url:     "https://api.com/jobs/{{.JobID}}",
		payload: map[string]interface{}{},
	}

	ctx := NewExecutionContextBuilder().
		WithJob(&model.Job{ID: "job-123"}).
		WithWebhookHandler(mockHandler).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	calls := mockHandler.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	expectedURL := "https://api.com/jobs/job-123"
	if calls[0].URL != expectedURL {
		t.Errorf("Expected URL '%s', got '%s'", expectedURL, calls[0].URL)
	}
}

// TestWebhookAction_TemplateInPayload tests template rendering in payload
func TestWebhookAction_TemplateInPayload(t *testing.T) {
	mockHandler := testutil.NewMockWebhookHandler()
	action := &WebhookAction{
		name: "webhook",
		url:  "https://api.com/webhook",
		payload: map[string]interface{}{
			"job": "{{.JobID}}",
			"run": "{{.RunID}}",
		},
	}

	ctx := NewExecutionContextBuilder().
		WithJob(&model.Job{ID: "job-123"}).
		WithRunID("run-456").
		WithWebhookHandler(mockHandler).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	calls := mockHandler.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	payload, ok := calls[0].Payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be a map")
	}

	if payload["job"] != "job-123" {
		t.Errorf("Expected job 'job-123', got '%v'", payload["job"])
	}

	if payload["run"] != "run-456" {
		t.Errorf("Expected run 'run-456', got '%v'", payload["run"])
	}
}

// TestSlackAction_TemplateInText tests template rendering in Slack text
func TestSlackAction_TemplateInText(t *testing.T) {
	mockHandler := testutil.NewMockWebhookHandler()
	action := &SlackAction{
		name:       "slack",
		webhookURL: "https://hooks.slack.com/test",
		text:       "Job {{.JobName}} completed",
	}

	ctx := NewExecutionContextBuilder().
		WithJob(&model.Job{Name: "deploy-prod"}).
		WithWebhookHandler(mockHandler).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	calls := mockHandler.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	payload, ok := calls[0].Payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be a map")
	}

	if payload["text"] != "Job deploy-prod completed" {
		t.Errorf("Expected text 'Job deploy-prod completed', got '%v'", payload["text"])
	}
}

// TestUpdateMetadataAction_TemplateInMetadata tests template rendering in metadata
func TestUpdateMetadataAction_TemplateInMetadata(t *testing.T) {
	mockUpdater := testutil.NewMockMetadataUpdater()
	action := &UpdateMetadataAction{
		name:  "update_metadata",
		jobID: "job-123",
		metadata: map[string]interface{}{
			"last_run": "{{.RunID}}",
			"env":      "{{index .Kwargs \"env\"}}",
		},
	}

	ctx := NewExecutionContextBuilder().
		WithRunID("run-123").
		WithKwargs(map[string]string{"env": "prod"}).
		WithMetadataUpdater(mockUpdater).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	calls := mockUpdater.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	if calls[0].Metadata["last_run"] != "run-123" {
		t.Errorf("Expected last_run 'run-123', got '%v'", calls[0].Metadata["last_run"])
	}

	if calls[0].Metadata["env"] != "prod" {
		t.Errorf("Expected env 'prod', got '%v'", calls[0].Metadata["env"])
	}
}

// TestMetricAction_TemplateInName tests template rendering in metric name
func TestMetricAction_TemplateInName(t *testing.T) {
	mockRecorder := testutil.NewMockMetricRecorder()
	action := &MetricAction{
		name:       "metric",
		metricName: "job.{{.JobName}}.duration",
		value:      123.45,
		tags:       map[string]string{},
	}

	ctx := NewExecutionContextBuilder().
		WithJob(&model.Job{Name: "deploy"}).
		WithMetricRecorder(mockRecorder).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	calls := mockRecorder.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	expectedName := "job.deploy.duration"
	if calls[0].Name != expectedName {
		t.Errorf("Expected metric name '%s', got '%s'", expectedName, calls[0].Name)
	}
}

// TestMetricAction_TemplateInStringValue tests template rendering in string value
func TestMetricAction_TemplateInStringValue(t *testing.T) {
	mockRecorder := testutil.NewMockMetricRecorder()
	action := &MetricAction{
		name:       "metric",
		metricName: "test.metric",
		value:      "{{index .Kwargs \"duration\"}}",
		tags:       map[string]string{},
	}

	ctx := NewExecutionContextBuilder().
		WithKwargs(map[string]string{"duration": "123.45"}).
		WithMetricRecorder(mockRecorder).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	calls := mockRecorder.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	if calls[0].Value != 123.45 {
		t.Errorf("Expected value 123.45, got %f", calls[0].Value)
	}
}

// TestMetricAction_TemplateInTags tests template rendering in tags
func TestMetricAction_TemplateInTags(t *testing.T) {
	mockRecorder := testutil.NewMockMetricRecorder()
	action := &MetricAction{
		name:       "metric",
		metricName: "test.metric",
		value:      100.0,
		tags: map[string]string{
			"job": "{{.JobID}}",
			"run": "{{.RunID}}",
		},
	}

	ctx := NewExecutionContextBuilder().
		WithJob(&model.Job{ID: "job-123"}).
		WithRunID("run-456").
		WithMetricRecorder(mockRecorder).
		Build()

	err := action.Execute(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	calls := mockRecorder.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	if calls[0].Tags["job"] != "job-123" {
		t.Errorf("Expected job tag 'job-123', got '%s'", calls[0].Tags["job"])
	}

	if calls[0].Tags["run"] != "run-456" {
		t.Errorf("Expected run tag 'run-456', got '%s'", calls[0].Tags["run"])
	}
}
