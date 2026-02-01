# Constraint Module Specification

## Overview

The constraint module provides a pluggable system for evaluating pre-execution and post-execution constraints on job runs, and executing associated actions when constraints are met or violated. The orchestrator delegates all constraint evaluation to this module through the `ConstraintChecker` interface.

## Design Principles

1. **Opaque to Orchestrator** - The orchestrator calls `CheckPreExecution()` and receives a simple `ShouldProceed` boolean. All complexity is encapsulated in this module.

2. **Interface-Based** - Both constraints and actions implement interfaces, allowing easy extension with new types.

3. **Type-Safe** - Each constraint type and action type is its own struct with compile-time guarantees.

4. **Per-Constraint Configuration** - Each constraint can specify:
   - `recheckOnRetry`: Whether to re-evaluate on retry
   - `onViolation`: Actions to take when constraint is violated
   - `onMet`: Actions to take when constraint is met

5. **Context-Based Communication** - Constraints receive an execution context with dependencies (inbox, webhook handler, logger).

## Core Interfaces

### ConstraintChecker

The main interface exposed to the orchestrator (already defined in orchestrator package):

```go
type ConstraintChecker interface {
    CheckPreExecution(ctx context.Context, job *Job, runID string) (ConstraintCheckResult, error)
    CheckPostExecution(ctx context.Context, job *Job, runID string, startTime, endTime time.Time, exitCode int) (ConstraintCheckResult, error)
    ShouldRecheckOnRetry(job *Job) bool
}

type ConstraintCheckResult struct {
    ShouldProceed bool   // false if constraints prevent execution
    Message       string // Summary of constraint evaluation and actions taken
}
```

### Constraint Interface

Each constraint type implements this interface:

```go
type Constraint interface {
    // Check evaluates the constraint and returns whether it is met
    Check(ctx context.Context) (ConstraintResult, error)

    // Name returns the human-readable name of this constraint
    Name() string

    // ShouldRecheckOnRetry returns whether this constraint should be re-evaluated on retry
    ShouldRecheckOnRetry() bool
}

type ConstraintResult struct {
    Met     bool   // true if constraint is satisfied
    Message string // Description of constraint evaluation
}
```

### Action Interface

Each action type implements this interface:

```go
type Action interface {
    // Execute performs the action
    Execute(ctx context.Context) error

    // Name returns the human-readable name of this action
    Name() string
}
```

### ExecutionContext

Provides dependencies to constraints and actions:

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

Constraints are configured as JSON in the `Job.ConstraintConfig` field:

```json
{
  "constraints": [
    {
      "type": "time_window",
      "name": "business_hours_only",
      "recheckOnRetry": true,
      "config": {
        "startTime": "09:00",
        "endTime": "17:00",
        "timezone": "America/New_York"
      },
      "onViolation": [
        {
          "type": "delay",
          "config": {
            "duration": "1h"
          }
        },
        {
          "type": "webhook",
          "config": {
            "url": "https://example.com/notify",
            "payload": {"status": "delayed"}
          }
        }
      ],
      "onMet": [
        {
          "type": "log",
          "config": {
            "message": "Time window constraint satisfied"
          }
        }
      ]
    },
    {
      "type": "resource_available",
      "name": "database_available",
      "recheckOnRetry": false,
      "config": {
        "resourceType": "database",
        "resourceID": "prod-db-1"
      },
      "onViolation": [
        {
          "type": "fail",
          "config": {
            "reason": "Database not available"
          }
        }
      ]
    }
  ]
}
```

## Implementation Structure

### ConstraintChecker Implementation

```go
type DefaultConstraintChecker struct {
    constraints []ConstraintWithActions
    logger      *slog.Logger
}

type ConstraintWithActions struct {
    Constraint      Constraint
    OnViolation     []Action
    OnMet           []Action
    RecheckOnRetry  bool
}

func NewConstraintChecker(config json.RawMessage, logger *slog.Logger) (*DefaultConstraintChecker, error) {
    // Parse configuration
    // Create constraint instances based on type
    // Create action instances for each constraint
    // Return configured checker
}

func (c *DefaultConstraintChecker) CheckPreExecution(ctx context.Context, job *Job, runID string) (ConstraintCheckResult, error) {
    execCtx := c.buildExecutionContext(ctx, job, runID)

    allMet := true
    messages := []string{}

    for _, cwa := range c.constraints {
        result, err := cwa.Constraint.Check(execCtx.Context)
        if err != nil {
            return ConstraintCheckResult{}, err
        }

        messages = append(messages, result.Message)

        if result.Met {
            // Execute onMet actions
            for _, action := range cwa.OnMet {
                if err := action.Execute(execCtx); err != nil {
                    c.logger.Error("failed to execute onMet action",
                        "constraint", cwa.Constraint.Name(),
                        "action", action.Name(),
                        "error", err)
                }
            }
        } else {
            allMet = false

            // Execute onViolation actions
            for _, action := range cwa.OnViolation {
                if err := action.Execute(execCtx); err != nil {
                    c.logger.Error("failed to execute onViolation action",
                        "constraint", cwa.Constraint.Name(),
                        "action", action.Name(),
                        "error", err)
                }
            }
        }
    }

    return ConstraintCheckResult{
        ShouldProceed: allMet,
        Message:       strings.Join(messages, "; "),
    }, nil
}

func (c *DefaultConstraintChecker) ShouldRecheckOnRetry(job *Job) bool {
    for _, cwa := range c.constraints {
        if cwa.RecheckOnRetry {
            return true
        }
    }
    return false
}
```

## Constraint Types

### 1. TimeWindowConstraint

Checks if current time is within a specified window.

```go
type TimeWindowConstraint struct {
    name      string
    startTime time.Time // Daily start time
    endTime   time.Time // Daily end time
    timezone  *time.Location
    recheck   bool
}

func (t *TimeWindowConstraint) Check(ctx context.Context) (ConstraintResult, error) {
    now := time.Now().In(t.timezone)

    // Convert now to today's window times
    start := time.Date(now.Year(), now.Month(), now.Day(),
        t.startTime.Hour(), t.startTime.Minute(), 0, 0, t.timezone)
    end := time.Date(now.Year(), now.Month(), now.Day(),
        t.endTime.Hour(), t.endTime.Minute(), 0, 0, t.timezone)

    met := now.After(start) && now.Before(end)

    return ConstraintResult{
        Met:     met,
        Message: fmt.Sprintf("time window check: %s", met),
    }, nil
}

func (t *TimeWindowConstraint) Name() string {
    return t.name
}

func (t *TimeWindowConstraint) ShouldRecheckOnRetry() bool {
    return t.recheck
}
```

### 2. ResourceAvailableConstraint

Checks if a required resource is available.

```go
type ResourceAvailableConstraint struct {
    name         string
    resourceType string
    resourceID   string
    checker      ResourceChecker
    recheck      bool
}

type ResourceChecker interface {
    IsAvailable(resourceType, resourceID string) (bool, error)
}

func (r *ResourceAvailableConstraint) Check(ctx context.Context) (ConstraintResult, error) {
    available, err := r.checker.IsAvailable(r.resourceType, r.resourceID)
    if err != nil {
        return ConstraintResult{}, err
    }

    return ConstraintResult{
        Met:     available,
        Message: fmt.Sprintf("resource %s:%s available: %v", r.resourceType, r.resourceID, available),
    }, nil
}
```

### 3. AlwaysPassConstraint (for testing)

Always returns true - useful for testing action execution.

```go
type AlwaysPassConstraint struct {
    name    string
    recheck bool
}

func (a *AlwaysPassConstraint) Check(ctx context.Context) (ConstraintResult, error) {
    return ConstraintResult{Met: true, Message: "always pass"}, nil
}
```

### 4. AlwaysFailConstraint (for testing)

Always returns false - useful for testing violation actions.

```go
type AlwaysFailConstraint struct {
    name    string
    recheck bool
}

func (a *AlwaysFailConstraint) Check(ctx context.Context) (ConstraintResult, error) {
    return ConstraintResult{Met: false, Message: "always fail"}, nil
}
```

## Action Types

### 1. DelayAction

Pauses execution for a specified duration.

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

Does nothing - useful for testing.

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
type ConstraintConfig struct {
    Type           string          `json:"type"`
    Name           string          `json:"name"`
    RecheckOnRetry bool            `json:"recheckOnRetry"`
    Config         json.RawMessage `json:"config"`
    OnViolation    []ActionConfig  `json:"onViolation"`
    OnMet          []ActionConfig  `json:"onMet"`
}

type ActionConfig struct {
    Type   string          `json:"type"`
    Config json.RawMessage `json:"config"`
}

func parseConstraints(configData json.RawMessage) ([]ConstraintWithActions, error) {
    var parsed struct {
        Constraints []ConstraintConfig `json:"constraints"`
    }

    if err := json.Unmarshal(configData, &parsed); err != nil {
        return nil, err
    }

    result := make([]ConstraintWithActions, len(parsed.Constraints))

    for i, cc := range parsed.Constraints {
        // Create constraint based on type
        constraint, err := createConstraint(cc)
        if err != nil {
            return nil, err
        }

        // Create onViolation actions
        onViolation := make([]Action, len(cc.OnViolation))
        for j, ac := range cc.OnViolation {
            action, err := createAction(ac)
            if err != nil {
                return nil, err
            }
            onViolation[j] = action
        }

        // Create onMet actions
        onMet := make([]Action, len(cc.OnMet))
        for j, ac := range cc.OnMet {
            action, err := createAction(ac)
            if err != nil {
                return nil, err
            }
            onMet[j] = action
        }

        result[i] = ConstraintWithActions{
            Constraint:     constraint,
            OnViolation:    onViolation,
            OnMet:          onMet,
            RecheckOnRetry: cc.RecheckOnRetry,
        }
    }

    return result, nil
}

func createConstraint(config ConstraintConfig) (Constraint, error) {
    switch config.Type {
    case "time_window":
        return parseTimeWindowConstraint(config)
    case "resource_available":
        return parseResourceAvailableConstraint(config)
    case "always_pass":
        return &AlwaysPassConstraint{name: config.Name, recheck: config.RecheckOnRetry}, nil
    case "always_fail":
        return &AlwaysFailConstraint{name: config.Name, recheck: config.RecheckOnRetry}, nil
    default:
        return nil, fmt.Errorf("unknown constraint type: %s", config.Type)
    }
}

func createAction(config ActionConfig) (Action, error) {
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
```

## Integration with Orchestrator

The orchestrator creates a constraint checker during initialization:

```go
// In orchestrator package
import "github.com/livinlefevreloca/itinerary/internal/constraint"

func NewOrchestrator(...) *Orchestrator {
    constraintChecker := constraint.NewConstraintChecker(
        jobConfig.ConstraintConfig,
        logger,
    )

    return &Orchestrator{
        constraintChecker: constraintChecker,
        ...
    }
}
```

When checking constraints:

```go
func (o *Orchestrator) runConditionRunning() {
    state := o.state.(*ConditionRunningState)

    ctx := o.buildConstraintContext()
    result, err := o.constraintChecker.CheckPreExecution(ctx, o.jobConfig, o.runID)

    if err != nil {
        o.logger.Error("constraint check failed", "error", err)
        o.transitionTo(state.ToFailed())
        return
    }

    if result.ShouldProceed {
        o.timing.ExecutionStartedAt = time.Now()
        o.transitionTo(state.ToContainerCreating())
    } else {
        o.logger.Info("constraints not met, cannot proceed",
            "message", result.Message)
        o.transitionTo(state.ToFailed())
    }
}
```

## Error Handling

1. **Constraint Check Errors** - If a constraint's `Check()` returns an error, the entire check fails and the orchestrator transitions to failed state.

2. **Action Execution Errors** - If an action's `Execute()` returns an error, it is logged but doesn't fail the constraint check. This allows other actions to run.

3. **Configuration Parse Errors** - Invalid configuration is caught during `NewConstraintChecker()` and returns an error, preventing orchestrator creation.

## Thread Safety

- Each orchestrator goroutine has its own constraint checker instance
- No shared state between orchestrators
- Constraint/action implementations must be stateless or use local state only
- External dependencies (resource checkers, webhook senders) must be thread-safe

## Future Extensions

Potential new constraint types:
- `ConcurrencyConstraint` - Limits concurrent runs of a job
- `DependencyConstraint` - Checks if dependent jobs completed
- `DataAvailableConstraint` - Checks if input data is ready
- `QuotaConstraint` - Checks resource quotas

Potential new action types:
- `SendEmailAction` - Sends email notification
- `SlackAction` - Sends Slack message
- `UpdateMetadataAction` - Updates job metadata
- `TriggerJobAction` - Triggers another job
