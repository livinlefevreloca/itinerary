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

    // Communication
    Inbox          MessageSender
    WebhookHandler WebhookSender

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

Sends an HTTP webhook.

**Config:**
```json
{
  "url": "https://example.com/notify",
  "payload": {"status": "completed", "message": "Job finished"}
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

    return ctx.WebhookHandler.SendWebhook(w.url, w.payload)
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
- 1: retry
- 2: kickOffJob
- 3: webhook
- 4: killAllInstances
- 5: killLatestInstance
- 6: skipNextInstance

## Future Extensions

Potential new action types:
- `SendEmailAction` - Sends email notification
- `SlackAction` - Sends Slack message (specialized webhook)
- `UpdateMetadataAction` - Updates job metadata in database
- `TriggerJobAction` - Triggers another job to run
- `PagerDutyAction` - Creates PagerDuty incident
- `MetricAction` - Records custom metric
- `ScaleResourceAction` - Scales infrastructure resources
