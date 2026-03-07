package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"text/template"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/db"
)

// Action interface that all action types must implement
type Action interface {
	// Execute performs the action
	Execute(ctx *ExecutionContext) error

	// Name returns the human-readable name of this action
	Name() string
}

// ExecutionContext provides dependencies to actions
type ExecutionContext struct {
	// Job information
	Job   *db.Job
	RunID string

	// Command execution details
	Command string
	Args    []string
	Kwargs  map[string]string

	// Communication
	Inbox          MessageSender
	WebhookHandler WebhookSender

	// Job control
	JobController   JobController
	MetadataUpdater MetadataUpdater
	MetricRecorder  MetricRecorder

	// Logging
	Logger *slog.Logger

	// Cancellation
	Context context.Context
}

// MessageSender interface for sending messages
type MessageSender interface {
	Send(msg interface{}) error
}

// WebhookSender interface for sending webhooks
type WebhookSender interface {
	SendWebhook(url string, payload interface{}) error
}

// JobController interface for job control operations
type JobController interface {
	RetryJob(jobID string) error
	TriggerJob(jobID string, args map[string]interface{}) error
	KillAllInstances(jobID string) error
	KillLatestInstance(jobID string) error
	SkipNextInstance(jobID string) error
}

// MetadataUpdater interface for updating job metadata
type MetadataUpdater interface {
	UpdateMetadata(jobID string, metadata map[string]interface{}) error
}

// MetricRecorder interface for recording metrics
type MetricRecorder interface {
	RecordMetric(name string, value float64, tags map[string]string) error
}

// TemplateData holds data available for template rendering
type TemplateData struct {
	JobID            string
	JobName          string
	RunID            string
	Command          string
	Args             []string
	Kwargs           map[string]string
	Timestamp        time.Time
	ConstraintName   string
	ConstraintStatus string
}

// ActionConfig holds the JSON configuration for creating actions
type ActionConfig struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

// Action implementations

// DelayAction pauses execution for a specified duration
type DelayAction struct {
	name     string
	duration time.Duration
}

func (d *DelayAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info("delaying execution",
		"duration", d.duration,
		"runID", ctx.RunID)

	timer := time.NewTimer(d.duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Context.Done():
		return ctx.Context.Err()
	}
}

func (d *DelayAction) Name() string {
	return d.name
}

// WebhookAction sends an HTTP webhook with template support
type WebhookAction struct {
	name    string
	url     string
	payload interface{}
}

func (w *WebhookAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info("sending webhook",
		"url", w.url,
		"runID", ctx.RunID)

	// Build template data
	templateData := buildTemplateData(ctx)

	// Render URL template
	renderedURL, err := renderTemplate(w.url, templateData)
	if err != nil {
		return fmt.Errorf("failed to render webhook URL: %w", err)
	}

	// Render payload template
	renderedPayload, err := renderPayload(w.payload, templateData)
	if err != nil {
		return fmt.Errorf("failed to render webhook payload: %w", err)
	}

	return ctx.WebhookHandler.SendWebhook(renderedURL, renderedPayload)
}

func (w *WebhookAction) Name() string {
	return w.name
}

// LogAction logs a message
type LogAction struct {
	name    string
	message string
}

func (l *LogAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info(l.message,
		"constraint_action", "log",
		"runID", ctx.RunID)
	return nil
}

func (l *LogAction) Name() string {
	return l.name
}

// FailAction forces the job run to fail immediately
type FailAction struct {
	name   string
	reason string
}

func (f *FailAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Error("failing job due to constraint action",
		"reason", f.reason,
		"runID", ctx.RunID)

	return fmt.Errorf("job failed: %s", f.reason)
}

func (f *FailAction) Name() string {
	return f.name
}

// NoOpAction does nothing - useful for testing
type NoOpAction struct {
	name string
}

func (n *NoOpAction) Execute(ctx *ExecutionContext) error {
	return nil
}

func (n *NoOpAction) Name() string {
	return n.name
}

// RetryAction schedules an immediate retry of the current job
type RetryAction struct {
	name string
}

func (r *RetryAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info("retrying job",
		"jobID", ctx.Job.ID,
		"runID", ctx.RunID)

	return ctx.JobController.RetryJob(ctx.Job.ID)
}

func (r *RetryAction) Name() string {
	return r.name
}

// TriggerJobAction triggers another job with specified arguments
type TriggerJobAction struct {
	name  string
	jobID string
	args  map[string]interface{}
}

func (t *TriggerJobAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info("triggering job",
		"targetJobID", t.jobID,
		"sourceRunID", ctx.RunID)

	return ctx.JobController.TriggerJob(t.jobID, t.args)
}

func (t *TriggerJobAction) Name() string {
	return t.name
}

// SlackAction sends a Slack notification via webhook with template support
type SlackAction struct {
	name       string
	webhookURL string
	channel    string
	username   string
	text       string
	iconEmoji  string
}

func (s *SlackAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info("sending slack notification",
		"channel", s.channel,
		"runID", ctx.RunID)

	// Build template data
	templateData := buildTemplateData(ctx)

	// Render all template fields
	renderedURL, err := renderTemplate(s.webhookURL, templateData)
	if err != nil {
		return fmt.Errorf("failed to render webhook URL: %w", err)
	}

	renderedChannel, err := renderTemplate(s.channel, templateData)
	if err != nil {
		return fmt.Errorf("failed to render channel: %w", err)
	}

	renderedText, err := renderTemplate(s.text, templateData)
	if err != nil {
		return fmt.Errorf("failed to render text: %w", err)
	}

	renderedUsername, err := renderTemplate(s.username, templateData)
	if err != nil {
		return fmt.Errorf("failed to render username: %w", err)
	}

	payload := map[string]interface{}{
		"text":       renderedText,
		"channel":    renderedChannel,
		"username":   renderedUsername,
		"icon_emoji": s.iconEmoji,
	}

	return ctx.WebhookHandler.SendWebhook(renderedURL, payload)
}

func (s *SlackAction) Name() string {
	return s.name
}

// PagerDutyAction creates a PagerDuty incident with template support
type PagerDutyAction struct {
	name          string
	routingKey    string
	severity      string
	summary       string
	source        string
	customDetails map[string]interface{}
}

func (p *PagerDutyAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info("creating pagerduty incident",
		"severity", p.severity,
		"runID", ctx.RunID)

	// Build template data
	templateData := buildTemplateData(ctx)

	// Render template fields
	renderedSummary, err := renderTemplate(p.summary, templateData)
	if err != nil {
		return fmt.Errorf("failed to render summary: %w", err)
	}

	renderedSource, err := renderTemplate(p.source, templateData)
	if err != nil {
		return fmt.Errorf("failed to render source: %w", err)
	}

	// Render custom details
	renderedCustomDetails, err := renderPayload(p.customDetails, templateData)
	if err != nil {
		return fmt.Errorf("failed to render custom details: %w", err)
	}

	payload := map[string]interface{}{
		"routing_key":  p.routingKey,
		"event_action": "trigger",
		"payload": map[string]interface{}{
			"summary":        renderedSummary,
			"severity":       p.severity,
			"source":         renderedSource,
			"custom_details": renderedCustomDetails,
		},
	}

	return ctx.WebhookHandler.SendWebhook("https://events.pagerduty.com/v2/enqueue", payload)
}

func (p *PagerDutyAction) Name() string {
	return p.name
}

// KillAllInstancesAction kills all running instances of a specified job
type KillAllInstancesAction struct {
	name  string
	jobID string
}

func (k *KillAllInstancesAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info("killing all instances",
		"jobID", k.jobID,
		"runID", ctx.RunID)

	return ctx.JobController.KillAllInstances(k.jobID)
}

func (k *KillAllInstancesAction) Name() string {
	return k.name
}

// KillLatestInstanceAction kills the most recent instance of a specified job
type KillLatestInstanceAction struct {
	name  string
	jobID string
}

func (k *KillLatestInstanceAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info("killing latest instance",
		"jobID", k.jobID,
		"runID", ctx.RunID)

	return ctx.JobController.KillLatestInstance(k.jobID)
}

func (k *KillLatestInstanceAction) Name() string {
	return k.name
}

// SkipNextInstanceAction skips the next scheduled run of a specified job
type SkipNextInstanceAction struct {
	name  string
	jobID string
}

func (s *SkipNextInstanceAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info("skipping next instance",
		"jobID", s.jobID,
		"runID", ctx.RunID)

	return ctx.JobController.SkipNextInstance(s.jobID)
}

func (s *SkipNextInstanceAction) Name() string {
	return s.name
}

// UpdateMetadataAction updates job metadata in the database with template support
type UpdateMetadataAction struct {
	name     string
	jobID    string
	metadata map[string]interface{}
}

func (u *UpdateMetadataAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info("updating job metadata",
		"jobID", u.jobID,
		"runID", ctx.RunID)

	// Build template data
	templateData := buildTemplateData(ctx)

	// Render job ID template
	renderedJobID, err := renderTemplate(u.jobID, templateData)
	if err != nil {
		return fmt.Errorf("failed to render job_id: %w", err)
	}

	// Render metadata values
	renderedMetadata, err := renderPayload(u.metadata, templateData)
	if err != nil {
		return fmt.Errorf("failed to render metadata: %w", err)
	}

	// Convert to map[string]interface{}
	metadataMap, ok := renderedMetadata.(map[string]interface{})
	if !ok {
		return fmt.Errorf("rendered metadata is not a map")
	}

	return ctx.MetadataUpdater.UpdateMetadata(renderedJobID, metadataMap)
}

func (u *UpdateMetadataAction) Name() string {
	return u.name
}

// MetricAction records a custom metric with template support
type MetricAction struct {
	name       string
	metricName string
	value      interface{} // can be float64 or string template
	tags       map[string]string
}

func (m *MetricAction) Execute(ctx *ExecutionContext) error {
	ctx.Logger.Info("recording metric",
		"metric", m.metricName,
		"runID", ctx.RunID)

	// Build template data
	templateData := buildTemplateData(ctx)

	// Render metric name
	renderedName, err := renderTemplate(m.metricName, templateData)
	if err != nil {
		return fmt.Errorf("failed to render metric name: %w", err)
	}

	// Render value if it's a string
	var metricValue float64
	switch v := m.value.(type) {
	case float64:
		metricValue = v
	case string:
		renderedValue, err := renderTemplate(v, templateData)
		if err != nil {
			return fmt.Errorf("failed to render metric value: %w", err)
		}
		// Parse rendered string to float64
		parsedValue, err := strconv.ParseFloat(renderedValue, 64)
		if err != nil {
			return fmt.Errorf("failed to parse metric value as float: %w", err)
		}
		metricValue = parsedValue
	default:
		return fmt.Errorf("metric value must be float64 or string, got %T", v)
	}

	// Render tags
	renderedTags := make(map[string]string, len(m.tags))
	for key, value := range m.tags {
		renderedValue, err := renderTemplate(value, templateData)
		if err != nil {
			return fmt.Errorf("failed to render tag %s: %w", key, err)
		}
		renderedTags[key] = renderedValue
	}

	return ctx.MetricRecorder.RecordMetric(renderedName, metricValue, renderedTags)
}

func (m *MetricAction) Name() string {
	return m.name
}

// Template rendering functions

func renderTemplate(tmpl string, data *TemplateData) (string, error) {
	t, err := template.New("action").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func renderPayload(payload interface{}, data *TemplateData) (interface{}, error) {
	// Handle nil payload
	if payload == nil {
		return nil, nil
	}

	return renderPayloadValue(payload, data)
}

func renderPayloadValue(value interface{}, data *TemplateData) (interface{}, error) {
	switch v := value.(type) {
	case string:
		// Render template in string value
		return renderTemplate(v, data)
	case map[string]interface{}:
		// Recursively render map values
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			rendered, err := renderPayloadValue(val, data)
			if err != nil {
				return nil, err
			}
			result[key] = rendered
		}
		return result, nil
	case []interface{}:
		// Recursively render array elements
		result := make([]interface{}, len(v))
		for i, val := range v {
			rendered, err := renderPayloadValue(val, data)
			if err != nil {
				return nil, err
			}
			result[i] = rendered
		}
		return result, nil
	default:
		// Return non-string values as-is (numbers, booleans, etc)
		return value, nil
	}
}

func buildTemplateData(ctx *ExecutionContext) *TemplateData {
	return &TemplateData{
		JobID:     ctx.Job.ID,
		JobName:   ctx.Job.Name,
		RunID:     ctx.RunID,
		Command:   ctx.Command,
		Args:      ctx.Args,
		Kwargs:    ctx.Kwargs,
		Timestamp: time.Now(),
	}
}

// CreateAction factory function
func CreateAction(config ActionConfig) (Action, error) {
	switch config.Type {
	case "delay":
		return parseDelayAction(config)
	case "webhook":
		return parseWebhookAction(config)
	case "log":
		return parseLogAction(config)
	case "fail":
		return parseFailAction(config)
	case "noop":
		return &NoOpAction{name: "noop"}, nil
	case "retry":
		return &RetryAction{name: "retry"}, nil
	case "trigger_job":
		return parseTriggerJobAction(config)
	case "slack":
		return parseSlackAction(config)
	case "pagerduty":
		return parsePagerDutyAction(config)
	case "kill_all_instances":
		return parseKillAllInstancesAction(config)
	case "kill_latest_instance":
		return parseKillLatestInstanceAction(config)
	case "skip_next_instance":
		return parseSkipNextInstanceAction(config)
	case "update_metadata":
		return parseUpdateMetadataAction(config)
	case "metric":
		return parseMetricAction(config)
	default:
		return nil, fmt.Errorf("unknown action type: %s", config.Type)
	}
}

// Parse functions

func parseDelayAction(config ActionConfig) (*DelayAction, error) {
	var cfg struct {
		Duration string `json:"duration"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	duration, err := time.ParseDuration(cfg.Duration)
	if err != nil {
		return nil, fmt.Errorf("invalid duration: %w", err)
	}

	return &DelayAction{
		name:     "delay",
		duration: duration,
	}, nil
}

func parseWebhookAction(config ActionConfig) (*WebhookAction, error) {
	var cfg struct {
		URL     string      `json:"url"`
		Payload interface{} `json:"payload"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	return &WebhookAction{
		name:    "webhook",
		url:     cfg.URL,
		payload: cfg.Payload,
	}, nil
}

func parseLogAction(config ActionConfig) (*LogAction, error) {
	var cfg struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	return &LogAction{
		name:    "log",
		message: cfg.Message,
	}, nil
}

func parseFailAction(config ActionConfig) (*FailAction, error) {
	var cfg struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	return &FailAction{
		name:   "fail",
		reason: cfg.Reason,
	}, nil
}

func parseTriggerJobAction(config ActionConfig) (*TriggerJobAction, error) {
	var cfg struct {
		JobID string                 `json:"job_id"`
		Args  map[string]interface{} `json:"args"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	if cfg.JobID == "" {
		return nil, fmt.Errorf("job_id is required")
	}

	return &TriggerJobAction{
		name:  "trigger_job",
		jobID: cfg.JobID,
		args:  cfg.Args,
	}, nil
}

func parseSlackAction(config ActionConfig) (*SlackAction, error) {
	var cfg struct {
		WebhookURL string `json:"webhook_url"`
		Channel    string `json:"channel"`
		Username   string `json:"username"`
		Text       string `json:"text"`
		IconEmoji  string `json:"icon_emoji"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	if cfg.WebhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required")
	}

	return &SlackAction{
		name:       "slack",
		webhookURL: cfg.WebhookURL,
		channel:    cfg.Channel,
		username:   cfg.Username,
		text:       cfg.Text,
		iconEmoji:  cfg.IconEmoji,
	}, nil
}

func parsePagerDutyAction(config ActionConfig) (*PagerDutyAction, error) {
	var cfg struct {
		RoutingKey    string                 `json:"routing_key"`
		Severity      string                 `json:"severity"`
		Summary       string                 `json:"summary"`
		Source        string                 `json:"source"`
		CustomDetails map[string]interface{} `json:"custom_details"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	if cfg.RoutingKey == "" {
		return nil, fmt.Errorf("routing_key is required")
	}

	return &PagerDutyAction{
		name:          "pagerduty",
		routingKey:    cfg.RoutingKey,
		severity:      cfg.Severity,
		summary:       cfg.Summary,
		source:        cfg.Source,
		customDetails: cfg.CustomDetails,
	}, nil
}

func parseKillAllInstancesAction(config ActionConfig) (*KillAllInstancesAction, error) {
	var cfg struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	if cfg.JobID == "" {
		return nil, fmt.Errorf("job_id is required")
	}

	return &KillAllInstancesAction{
		name:  "kill_all_instances",
		jobID: cfg.JobID,
	}, nil
}

func parseKillLatestInstanceAction(config ActionConfig) (*KillLatestInstanceAction, error) {
	var cfg struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	if cfg.JobID == "" {
		return nil, fmt.Errorf("job_id is required")
	}

	return &KillLatestInstanceAction{
		name:  "kill_latest_instance",
		jobID: cfg.JobID,
	}, nil
}

func parseSkipNextInstanceAction(config ActionConfig) (*SkipNextInstanceAction, error) {
	var cfg struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	if cfg.JobID == "" {
		return nil, fmt.Errorf("job_id is required")
	}

	return &SkipNextInstanceAction{
		name:  "skip_next_instance",
		jobID: cfg.JobID,
	}, nil
}

func parseUpdateMetadataAction(config ActionConfig) (*UpdateMetadataAction, error) {
	var cfg struct {
		JobID    string                 `json:"job_id"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	if cfg.JobID == "" {
		return nil, fmt.Errorf("job_id is required")
	}

	if cfg.Metadata == nil || len(cfg.Metadata) == 0 {
		return nil, fmt.Errorf("metadata is required and must not be empty")
	}

	return &UpdateMetadataAction{
		name:     "update_metadata",
		jobID:    cfg.JobID,
		metadata: cfg.Metadata,
	}, nil
}

func parseMetricAction(config ActionConfig) (*MetricAction, error) {
	var cfg struct {
		Name  string            `json:"name"`
		Value interface{}       `json:"value"`
		Tags  map[string]string `json:"tags"`
	}
	if err := json.Unmarshal(config.Config, &cfg); err != nil {
		return nil, err
	}

	if cfg.Name == "" {
		return nil, fmt.Errorf("metric name is required")
	}

	if cfg.Value == nil {
		return nil, fmt.Errorf("metric value is required")
	}

	// Validate value type
	switch v := cfg.Value.(type) {
	case float64:
		// Valid
	case string:
		// Valid - will be rendered as template
	default:
		return nil, fmt.Errorf("metric value must be number or string, got %T", v)
	}

	if cfg.Tags == nil {
		cfg.Tags = make(map[string]string)
	}

	return &MetricAction{
		name:       "metric",
		metricName: cfg.Name,
		value:      cfg.Value,
		tags:       cfg.Tags,
	}, nil
}
