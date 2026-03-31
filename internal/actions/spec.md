# Action Module Specification

## Overview

The action module provides a pluggable system for executing actions in response to constraint evaluation results. Actions are triggered when constraints are met or violated, and run in complete isolation from each other.

## Design Principles

1. **Interface-Based** - All actions implement the `Action` interface, allowing easy extension with new types.

2. **Isolated Execution** - Each action runs independently. If one action fails, it doesn't prevent other actions from running.

3. **Type-Safe** - Each action type is its own struct with compile-time guarantees.

4. **Context-Based Communication** - Actions receive an execution context with dependencies (inbox, webhook handler, logger).

5. **Trigger-Driven** - Actions specify when they should trigger: `on_met` (constraint satisfied) or `on_violated` (constraint violated).

## Core Interface

### Action Interface

Each action type implements this interface:

```go
type Action interface {
    // Execute performs the action
    Execute(ctx *ExecutionContext) error

    // Name returns the human-readable name of this action
    Name() string
}
```

### ExecutionContext

Provides dependencies to actions:

```go
type ExecutionContext struct {
    // Job information
    Job   *Job
    RunID string

    // Command execution details
    Command     string
    Args        []string
    Kwargs      map[string]string

    // Communication
    Inbox          MessageSender
    WebhookHandler WebhookSender

    // Job control
    JobController  JobController
    MetadataUpdater MetadataUpdater
    MetricRecorder  MetricRecorder

    // Logging
    Logger *slog.Logger

    // Cancellation
    Context context.Context
}

type MessageSender interface {
    Send(msg interface{}) error
}

type WebhookSender interface {
    SendWebhook(url string, payload interface{}) error
}

type JobController interface {
    // RetryJob schedules an immediate retry of the specified job
    RetryJob(jobID string) error

    // TriggerJob triggers another job with the given arguments
    TriggerJob(jobID string, args map[string]interface{}) error

    // KillAllInstances kills all running instances of the specified job
    KillAllInstances(jobID string) error

    // KillLatestInstance kills the most recent instance of the specified job
    KillLatestInstance(jobID string) error

    // SkipNextInstance skips the next scheduled run of the specified job
    SkipNextInstance(jobID string) error
}

type MetadataUpdater interface {
    // UpdateMetadata updates job metadata fields
    UpdateMetadata(jobID string, metadata map[string]interface{}) error
}

type MetricRecorder interface {
    // RecordMetric records a custom metric with optional tags
    RecordMetric(name string, value float64, tags map[string]string) error
}
```

## Configuration Format

Actions are configured in the database `actions` table, with JSON config specific to each action type:

```json
{
  "type": "webhook",
  "config": {
    "url": "https://example.com/notify",
    "payload": {"status": "delayed"}
  }
}
```

## Template Variables

Webhook-based actions (WebhookAction, SlackAction, PagerDutyAction) support templating in URLs and payloads. Templates use Go's text/template syntax with double curly braces `{{.Variable}}`.

### Available Template Variables

```go
type TemplateData struct {
    // Job information
    JobID   string
    JobName string
    RunID   string

    // Command execution
    Command string
    Args    []string
    Kwargs  map[string]string

    // Timestamp
    Timestamp time.Time

    // Constraint information (if triggered by constraint)
    ConstraintName   string
    ConstraintStatus string // "met" or "violated"
}
```

### Template Examples

**URL templating:**
```
https://api.example.com/jobs/{{.JobID}}/notify?run={{.RunID}}
```

**Payload templating:**
```json
{
  "job_id": "{{.JobID}}",
  "run_id": "{{.RunID}}",
  "command": "{{.Command}}",
  "status": "{{.ConstraintStatus}}",
  "timestamp": "{{.Timestamp}}"
}
```

**Accessing kwargs:**
```json
{
  "environment": "{{index .Kwargs "env"}}",
  "region": "{{index .Kwargs "region"}}"
}
```

**Accessing args:**
```json
{
  "first_arg": "{{index .Args 0}}",
  "second_arg": "{{index .Args 1}}"
}
```

### Template Rendering

```go
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
    // Serialize payload to JSON
    jsonBytes, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }

    // Render templates in JSON string
    rendered, err := renderTemplate(string(jsonBytes), data)
    if err != nil {
        return nil, err
    }

    // Unmarshal back to interface{}
    var result interface{}
    if err := json.Unmarshal([]byte(rendered), &result); err != nil {
        return nil, err
    }

    return result, nil
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
```

## Action Types

### 1. DelayAction

Pauses execution for a specified duration.

**Config:**
```json
{
  "duration": "1h"
}
```

**Implementation:**
```go
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
```

### 2. WebhookAction

Sends an HTTP webhook with template support for URLs and payloads.

**Config:**
```json
{
  "url": "https://example.com/notify?job={{.JobID}}&run={{.RunID}}",
  "payload": {
    "status": "completed",
    "message": "Job {{.JobName}} finished",
    "command": "{{.Command}}",
    "environment": "{{index .Kwargs \"env\"}}"
  }
}
```

**Implementation:**
```go
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
```

### 3. LogAction

Logs a message.

**Config:**
```json
{
  "message": "Constraint check completed successfully"
}
```

**Implementation:**
```go
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
```

### 4. FailAction

Forces the job run to fail immediately.

**Config:**
```json
{
  "reason": "Required resource not available"
}
```

**Implementation:**
```go
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
```

### 5. NoOpAction (for testing)

Does nothing - useful for testing action execution flow.

**Config:**
```json
{}
```

**Implementation:**
```go
type NoOpAction struct {
    name string
}

func (n *NoOpAction) Execute(ctx *ExecutionContext) error {
    return nil
}

func (n *NoOpAction) Name() string {
    return n.name
}
```

### 6. RetryAction

Schedules an immediate retry of the current job.

**Config:**
```json
{}
```

**Implementation:**
```go
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
```

### 7. TriggerJobAction

Triggers another job with specified arguments.

**Config:**
```json
{
  "job_id": "job-123",
  "args": {
    "key1": "value1",
    "key2": "value2"
  }
}
```

**Implementation:**
```go
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
```

### 8. SlackAction

Sends a Slack notification via webhook with template support.

**Config:**
```json
{
  "webhook_url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
  "channel": "#alerts",
  "username": "Itinerary Bot",
  "text": "Job {{.JobName}} alert - Run {{.RunID}} - Command: {{.Command}}",
  "icon_emoji": ":robot_face:"
}
```

**Example with kwargs:**
```json
{
  "webhook_url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
  "channel": "#{{index .Kwargs \"env\"}}-alerts",
  "text": "Deployment to {{index .Kwargs \"region\"}} completed for job {{.JobName}}"
}
```

**Implementation:**
```go
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
```

### 9. PagerDutyAction

Creates a PagerDuty incident with template support.

**Config:**
```json
{
  "routing_key": "your-integration-key",
  "severity": "error",
  "summary": "Job {{.JobName}} failed - Run {{.RunID}}",
  "source": "itinerary-scheduler",
  "custom_details": {
    "job_id": "{{.JobID}}",
    "run_id": "{{.RunID}}",
    "command": "{{.Command}}",
    "environment": "{{index .Kwargs \"env\"}}"
  }
}
```

**Implementation:**
```go
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
```

### 10. KillAllInstancesAction

Kills all running instances of a specified job.

**Config:**
```json
{
  "job_id": "job-123"
}
```

**Implementation:**
```go
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
```

### 11. KillLatestInstanceAction

Kills the most recent instance of a specified job.

**Config:**
```json
{
  "job_id": "job-123"
}
```

**Implementation:**
```go
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
```

### 12. SkipNextInstanceAction

Skips the next scheduled run of a specified job.

**Config:**
```json
{
  "job_id": "job-123"
}
```

**Implementation:**
```go
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
```

### 13. UpdateMetadataAction

Updates job metadata in the database with template support.

**Config:**
```json
{
  "job_id": "job-123",
  "metadata": {
    "last_run": "{{.RunID}}",
    "status": "completed",
    "last_command": "{{.Command}}",
    "environment": "{{index .Kwargs \"env\"}}"
  }
}
```

**Example updating current job metadata:**
```json
{
  "job_id": "{{.JobID}}",
  "metadata": {
    "last_successful_run": "{{.RunID}}",
    "last_run_timestamp": "{{.Timestamp}}"
  }
}
```

**Implementation:**
```go
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
```

### 14. MetricAction

Records a custom metric with template support for metric names, values, and tags.

**Config:**
```json
{
  "name": "job.execution.duration",
  "value": 123.45,
  "tags": {
    "job_id": "{{.JobID}}",
    "environment": "{{index .Kwargs \"env\"}}",
    "status": "success"
  }
}
```

**Example with templated metric name:**
```json
{
  "name": "job.{{.JobName}}.runs",
  "value": 1,
  "tags": {
    "run_id": "{{.RunID}}",
    "command": "{{.Command}}"
  }
}
```

**Example with string value (converted to float):**
```json
{
  "name": "job.custom.metric",
  "value": "{{index .Kwargs \"duration\"}}",
  "tags": {}
}
```

**Implementation:**
```go
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
```

## Configuration Parsing

```go
type ActionConfig struct {
    Type   string          `json:"type"`
    Config json.RawMessage `json:"config"`
}

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
```

## Error Handling

1. **Action Execution Errors** - If an action's `Execute()` returns an error, it is logged but doesn't fail the constraint check or prevent other actions from running.

2. **Configuration Parse Errors** - Invalid configuration is caught during action creation and returns an error.

3. **Webhook Failures** - Failed webhooks return an error but don't block other actions.

4. **Timeout Handling** - Actions that support context cancellation (like DelayAction) should check `ctx.Context.Done()`.

## Thread Safety

- Actions must be stateless or use local state only
- External dependencies (webhook senders) must be thread-safe
- Multiple actions can execute concurrently for different job runs

## Database Integration

Actions are stored in the `actions` table:

```sql
CREATE TABLE actions (
    id TEXT PRIMARY KEY,
    constraint_id TEXT NOT NULL,
    action_type_id INTEGER NOT NULL,
    trigger TEXT NOT NULL,      -- 'on_met', 'on_violated'
    config TEXT,                -- JSON configuration specific to action type
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (constraint_id) REFERENCES constraints(id) ON DELETE CASCADE,
    FOREIGN KEY (action_type_id) REFERENCES action_types(id) ON DELETE CASCADE
);
```

Action types are pre-defined in the `action_types` dimension table:
- 1: Retry - rerun the job immediately
- 2: TriggerJob - trigger another with a set of arguments
- 3: Webhook - trigger an HTTP webhook with a payload
- 4: SlackAction - Sends Slack message (specialized webhook)
- 5: PagerDutyAction - Creates PagerDuty incident (specialized webhook)
- 6: KillAllInstances - Kill all instances of a specified job
- 7: KillLatestInstance - Kill the latest instance of a specified job
- 8: SkipNextInstance - Skip the next instance of a specified job
- 9: UpdateMetadataAction - Updates job metadata in database
- 10:  MetricAction - Records custom metric
