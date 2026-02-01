# Constraint Module Specification

## Overview

The constraint module provides a pluggable system for evaluating constraints on job runs at three key points: before execution (pre-execution), during execution (mid-execution), and after execution (post-execution). Constraints can trigger actions when they are met or violated. The orchestrator delegates all constraint evaluation to this module through the `ConstraintChecker` interface.

## Design Principles

1. **Opaque to Orchestrator** - The orchestrator calls constraint check methods and receives a simple `ShouldProceed` boolean. All complexity is encapsulated in this module.

2. **Interface-Based** - Both constraints and actions implement interfaces, allowing easy extension with new types.

3. **Type-Safe** - Each constraint type and action type is its own struct with compile-time guarantees.

4. **Multi-Phase Evaluation** - Constraints can be evaluated at three points:
   - **Pre-execution**: Before job starts (e.g., check dependencies, time windows)
   - **During execution**: While job is running (e.g., runtime limits, health checks)
   - **Post-execution**: After job completes (e.g., verify outputs, record metrics)

5. **Per-Constraint Configuration** - Each constraint can specify:
   - `evaluationTiming`: When to evaluate (pre, during, post, or multiple)
   - `recheckOnRetry`: Whether to re-evaluate on retry
   - `onViolation`: Actions to take when constraint is violated
   - `onMet`: Actions to take when constraint is met

6. **Scheduler as Source of Truth** - Constraints query the scheduler (not the database) for live job state information via message passing. The scheduler maintains the in-memory source of truth for what jobs are running, scheduled, and recently completed.

7. **Direct I/O Allowed** - Constraints run in the orchestrator goroutine and can perform I/O directly (HTTP requests, external API calls). They do not run in the scheduler loop hotpath.

## Core Interfaces

### ConstraintChecker

The main interface exposed to the orchestrator:

```go
type ConstraintChecker interface {
    // CheckPreExecution evaluates constraints before job starts
    CheckPreExecution(ctx context.Context, job *Job, runID string) (ConstraintCheckResult, error)

    // CheckDuringExecution evaluates constraints while job is running
    // Should be called periodically during execution
    CheckDuringExecution(ctx context.Context, job *Job, runID string, startTime time.Time) (ConstraintCheckResult, error)

    // CheckPostExecution evaluates constraints after job completes
    CheckPostExecution(ctx context.Context, job *Job, runID string, startTime, endTime time.Time, exitCode int) (ConstraintCheckResult, error)

    // ShouldRecheckOnRetry returns whether constraints should be re-evaluated on retry
    ShouldRecheckOnRetry(job *Job) bool
}

type ConstraintCheckResult struct {
    ShouldProceed bool   // false if constraints prevent execution/continuation
    Message       string // Summary of constraint evaluation and actions taken
}
```

### Constraint Interface

Each constraint type implements this interface:

```go
type Constraint interface {
    // Check evaluates the constraint and returns whether it is met
    Check(ctx *ExecutionContext) (ConstraintResult, error)

    // Name returns the human-readable name of this constraint
    Name() string

    // EvaluationTiming returns when this constraint should be evaluated
    EvaluationTiming() []EvaluationPhase

    // ShouldRecheckOnRetry returns whether this constraint should be re-evaluated on retry
    ShouldRecheckOnRetry() bool
}

type EvaluationPhase string

const (
    EvaluationPhasePreExecution    EvaluationPhase = "pre"
    EvaluationPhaseDuringExecution EvaluationPhase = "during"
    EvaluationPhasePostExecution   EvaluationPhase = "post"
)

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

    // Execution timing (for during/post execution checks)
    StartTime *time.Time
    EndTime   *time.Time
    ExitCode  *int

    // Message-based communication with scheduler for job state queries
    SchedulerInbox MessageSender

    // HTTP client for making external requests
    HTTPClient *http.Client

    // Webhook handler for sending notifications
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

### Scheduler Communication

Constraints query the scheduler for live job state via request/response messages. The scheduler is the source of truth for what's currently running, not the database.

```go
// Example: Check if another job is running
type JobStateRequest struct {
    JobID      string
    ResponseTo chan interface{}
}

type JobStateResponse struct {
    JobID     string
    IsRunning bool
    LastRun   *time.Time
    NextRun   *time.Time
}

// In constraint implementation:
func (c *OtherJobRunningConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
    responseChan := make(chan interface{}, 1)
    request := &JobStateRequest{
        JobID:      c.otherJobID,
        ResponseTo: responseChan,
    }

    if err := ctx.SchedulerInbox.Send(request); err != nil {
        return ConstraintResult{}, err
    }

    select {
    case resp := <-responseChan:
        state := resp.(*JobStateResponse)
        return ConstraintResult{
            Met:     !state.IsRunning, // Met if job is NOT running
            Message: fmt.Sprintf("job %s running: %v", c.otherJobID, state.IsRunning),
        }, nil
    case <-ctx.Context.Done():
        return ConstraintResult{}, ctx.Context.Err()
    }
}
```

## Configuration Format

Constraints are stored in the database and loaded by the constraint checker. Example configurations:

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
            "url": "https://hooks.slack.com/...",
            "payload": {"text": "Job delayed until business hours"}
          }
        }
      ]
    },
    {
      "type": "other_job_running",
      "name": "wait_for_etl",
      "recheckOnRetry": true,
      "config": {
        "jobID": "data-import-etl",
        "shouldBeRunning": false
      },
      "onViolation": [
        {
          "type": "delay",
          "config": {
            "duration": "5m"
          }
        }
      ],
      "onMet": [
        {
          "type": "log",
          "config": {
            "message": "ETL job not running, safe to proceed"
          }
        }
      ]
    },
    {
      "type": "other_job_completed_recently",
      "name": "require_upstream_success",
      "recheckOnRetry": false,
      "config": {
        "jobID": "upstream-job",
        "within": "30m",
        "mustSucceed": true
      },
      "onViolation": [
        {
          "type": "fail",
          "config": {
            "reason": "Upstream job has not completed successfully in last 30m"
          }
        }
      ]
    },
    {
      "type": "max_runtime",
      "name": "kill_on_timeout",
      "recheckOnRetry": false,
      "config": {
        "maxDuration": "2h",
        "checkInterval": "5m"
      },
      "onViolation": [
        {
          "type": "webhook",
          "config": {
            "url": "https://hooks.slack.com/...",
            "payload": {"text": "Job exceeded 2h runtime limit"}
          }
        },
        {
          "type": "kill_job"
        }
      ]
    },
    {
      "type": "http_health_check",
      "name": "api_availability",
      "recheckOnRetry": true,
      "config": {
        "url": "https://api.example.com/health?job={{.JobName}}",
        "method": "GET",
        "headers": {
          "X-Run-ID": "{{.RunID}}"
        },
        "timeout": "5s"
      },
      "onViolation": [
        {
          "type": "fail",
          "config": {
            "reason": "API health check failed"
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
    return c.checkConstraints(ctx, job, runID, EvaluationPhasePreExecution, nil, nil, nil)
}

func (c *DefaultConstraintChecker) CheckDuringExecution(ctx context.Context, job *Job, runID string, startTime time.Time) (ConstraintCheckResult, error) {
    return c.checkConstraints(ctx, job, runID, EvaluationPhaseDuringExecution, &startTime, nil, nil)
}

func (c *DefaultConstraintChecker) CheckPostExecution(ctx context.Context, job *Job, runID string, startTime, endTime time.Time, exitCode int) (ConstraintCheckResult, error) {
    return c.checkConstraints(ctx, job, runID, EvaluationPhasePostExecution, &startTime, &endTime, &exitCode)
}

func (c *DefaultConstraintChecker) checkConstraints(
    ctx context.Context,
    job *Job,
    runID string,
    phase EvaluationPhase,
    startTime *time.Time,
    endTime *time.Time,
    exitCode *int,
) (ConstraintCheckResult, error) {
    execCtx := c.buildExecutionContext(ctx, job, runID, startTime, endTime, exitCode)

    allMet := true
    messages := []string{}

    for _, cwa := range c.constraints {
        // Skip if constraint doesn't apply to this phase
        if !c.appliesToPhase(cwa.Constraint, phase) {
            continue
        }

        result, err := cwa.Constraint.Check(execCtx)
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

func (c *DefaultConstraintChecker) appliesToPhase(constraint Constraint, phase EvaluationPhase) bool {
    phases := constraint.EvaluationTiming()
    for _, p := range phases {
        if p == phase {
            return true
        }
    }
    return false
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

**Evaluation Phase**: Pre-execution

Checks if current time is within a specified window. Useful for ensuring jobs only run during business hours, maintenance windows, etc.

**Config:**
```json
{
  "startTime": "09:00",
  "endTime": "17:00",
  "timezone": "America/New_York"
}
```

**Implementation:**
```go
type TimeWindowConstraint struct {
    name      string
    startTime time.Time // Daily start time
    endTime   time.Time // Daily end time
    timezone  *time.Location
    recheck   bool
}

func (t *TimeWindowConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
    now := time.Now().In(t.timezone)

    // Convert now to today's window times
    start := time.Date(now.Year(), now.Month(), now.Day(),
        t.startTime.Hour(), t.startTime.Minute(), 0, 0, t.timezone)
    end := time.Date(now.Year(), now.Month(), now.Day(),
        t.endTime.Hour(), t.endTime.Minute(), 0, 0, t.timezone)

    met := now.After(start) && now.Before(end)

    return ConstraintResult{
        Met:     met,
        Message: fmt.Sprintf("time window check [%s-%s]: %v",
            start.Format("15:04"), end.Format("15:04"), met),
    }, nil
}

func (t *TimeWindowConstraint) EvaluationTiming() []EvaluationPhase {
    return []EvaluationPhase{EvaluationPhasePreExecution}
}
```

### 2. OtherJobRunningConstraint

**Evaluation Phase**: Pre-execution

Checks if another job is currently running. Useful for preventing concurrent execution of dependent jobs or jobs that compete for resources.

**Config:**
```json
{
  "jobID": "data-import-job",
  "shouldBeRunning": false
}
```

**Implementation:**
```go
type OtherJobRunningConstraint struct {
    name            string
    otherJobID      string
    shouldBeRunning bool // true = met when running, false = met when NOT running
    recheck         bool
}

func (o *OtherJobRunningConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
    request := &JobStateRequest{
        JobID:      o.otherJobID,
        ResponseTo: make(chan interface{}, 1),
    }

    if err := ctx.SchedulerInbox.Send(request); err != nil {
        return ConstraintResult{}, err
    }

    select {
    case resp := <-request.ResponseTo:
        state := resp.(*JobStateResponse)
        met := state.IsRunning == o.shouldBeRunning

        return ConstraintResult{
            Met: met,
            Message: fmt.Sprintf("job %s running=%v (expected=%v)",
                o.otherJobID, state.IsRunning, o.shouldBeRunning),
        }, nil
    case <-ctx.Context.Done():
        return ConstraintResult{}, ctx.Context.Err()
    }
}

func (o *OtherJobRunningConstraint) EvaluationTiming() []EvaluationPhase {
    return []EvaluationPhase{EvaluationPhasePreExecution}
}
```

### 3. OtherJobCompletedRecentlyConstraint

**Evaluation Phase**: Pre-execution

Checks if another job completed within a specified time window. Useful for dependency chains where a job should only run after its dependencies complete.

**Config:**
```json
{
  "jobID": "upstream-etl-job",
  "within": "30m",
  "mustSucceed": true
}
```

**Implementation:**
```go
type OtherJobCompletedRecentlyConstraint struct {
    name        string
    otherJobID  string
    within      time.Duration
    mustSucceed bool
    recheck     bool
}

func (o *OtherJobCompletedRecentlyConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
    request := &JobHistoryRequest{
        JobID:      o.otherJobID,
        Limit:      1,
        ResponseTo: make(chan interface{}, 1),
    }

    if err := ctx.SchedulerInbox.Send(request); err != nil {
        return ConstraintResult{}, err
    }

    select {
    case resp := <-request.ResponseTo:
        history := resp.(*JobHistoryResponse)

        if len(history.Runs) == 0 {
            return ConstraintResult{
                Met:     false,
                Message: fmt.Sprintf("job %s has no recent runs", o.otherJobID),
            }, nil
        }

        lastRun := history.Runs[0]
        timeSinceCompletion := time.Since(lastRun.CompletedAt)

        met := timeSinceCompletion <= o.within
        if o.mustSucceed {
            met = met && lastRun.Success
        }

        return ConstraintResult{
            Met: met,
            Message: fmt.Sprintf("job %s last completed %v ago (success=%v)",
                o.otherJobID, timeSinceCompletion.Round(time.Second), lastRun.Success),
        }, nil
    case <-ctx.Context.Done():
        return ConstraintResult{}, ctx.Context.Err()
    }
}

func (o *OtherJobCompletedRecentlyConstraint) EvaluationTiming() []EvaluationPhase {
    return []EvaluationPhase{EvaluationPhasePreExecution}
}
```

### 4. OtherJobScheduledSoonConstraint

**Evaluation Phase**: Pre-execution

Checks if another job is scheduled to run within a specified time window. Useful for avoiding resource conflicts or ensuring jobs run in a specific order.

**Config:**
```json
{
  "jobID": "high-priority-job",
  "within": "10m"
}
```

**Implementation:**
```go
type OtherJobScheduledSoonConstraint struct {
    name       string
    otherJobID string
    within     time.Duration
    recheck    bool
}

func (o *OtherJobScheduledSoonConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
    request := &JobStateRequest{
        JobID:      o.otherJobID,
        ResponseTo: make(chan interface{}, 1),
    }

    if err := ctx.SchedulerInbox.Send(request); err != nil {
        return ConstraintResult{}, err
    }

    select {
    case resp := <-request.ResponseTo:
        state := resp.(*JobStateResponse)

        if state.NextRun == nil {
            return ConstraintResult{
                Met:     false,
                Message: fmt.Sprintf("job %s has no scheduled runs", o.otherJobID),
            }, nil
        }

        timeUntilRun := time.Until(*state.NextRun)
        met := timeUntilRun > 0 && timeUntilRun <= o.within

        return ConstraintResult{
            Met: met,
            Message: fmt.Sprintf("job %s scheduled in %v",
                o.otherJobID, timeUntilRun.Round(time.Second)),
        }, nil
    case <-ctx.Context.Done():
        return ConstraintResult{}, ctx.Context.Err()
    }
}

func (o *OtherJobScheduledSoonConstraint) EvaluationTiming() []EvaluationPhase {
    return []EvaluationPhase{EvaluationPhasePreExecution}
}
```

### 5. HTTPHealthCheckConstraint

**Evaluation Phase**: Pre-execution

Makes an HTTP request to an endpoint and checks for a 200 response. Supports templating for dynamic values based on job context.

**Config:**
```json
{
  "url": "https://api.example.com/health?job={{.JobName}}",
  "method": "GET",
  "headers": {
    "X-Job-Name": "{{.JobName}}",
    "X-Run-ID": "{{.RunID}}"
  },
  "body": null,
  "timeout": "5s"
}
```

**Implementation:**
```go
type HTTPHealthCheckConstraint struct {
    name            string
    urlTemplate     *template.Template
    method          string
    headerTemplates map[string]*template.Template
    bodyTemplate    *template.Template
    timeout         time.Duration
    recheck         bool
}

func (h *HTTPHealthCheckConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
    // Template data
    data := map[string]interface{}{
        "JobID":   ctx.Job.ID,
        "JobName": ctx.Job.Name,
        "RunID":   ctx.RunID,
    }

    // Execute URL template
    var urlBuf bytes.Buffer
    if err := h.urlTemplate.Execute(&urlBuf, data); err != nil {
        return ConstraintResult{}, err
    }
    url := urlBuf.String()

    // Render headers
    headers := make(map[string]string)
    for key, tmpl := range h.headerTemplates {
        var buf bytes.Buffer
        if err := tmpl.Execute(&buf, data); err != nil {
            return ConstraintResult{}, err
        }
        headers[key] = buf.String()
    }

    // Make HTTP request directly
    reqCtx, cancel := context.WithTimeout(ctx.Context, h.timeout)
    defer cancel()

    req, err := http.NewRequestWithContext(reqCtx, h.method, url, nil)
    if err != nil {
        return ConstraintResult{}, err
    }

    for key, value := range headers {
        req.Header.Set(key, value)
    }

    resp, err := ctx.HTTPClient.Do(req)
    if err != nil {
        return ConstraintResult{}, err
    }
    defer resp.Body.Close()

    met := resp.StatusCode == 200

    return ConstraintResult{
        Met: met,
        Message: fmt.Sprintf("HTTP %s %s returned %d",
            h.method, url, resp.StatusCode),
    }, nil
}

func (h *HTTPHealthCheckConstraint) EvaluationTiming() []EvaluationPhase {
    return []EvaluationPhase{EvaluationPhasePreExecution}
}
```

### 6. MaxRuntimeConstraint

**Evaluation Phase**: During execution

Checks if job has been running for less than a specified duration. Can be used for expected runtime (warning) or maximum allowed runtime (hard limit).

**Config:**
```json
{
  "maxDuration": "2h",
  "checkInterval": "5m"
}
```

**Implementation:**
```go
type MaxRuntimeConstraint struct {
    name          string
    maxDuration   time.Duration
    checkInterval time.Duration
}

func (m *MaxRuntimeConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
    if ctx.StartTime == nil {
        return ConstraintResult{}, fmt.Errorf("start time not available")
    }

    elapsed := time.Since(*ctx.StartTime)
    met := elapsed <= m.maxDuration

    return ConstraintResult{
        Met: met,
        Message: fmt.Sprintf("runtime %v / %v",
            elapsed.Round(time.Second), m.maxDuration),
    }, nil
}

func (m *MaxRuntimeConstraint) EvaluationTiming() []EvaluationPhase {
    return []EvaluationPhase{EvaluationPhaseDuringExecution}
}
```

### 7. MinRuntimeConstraint

**Evaluation Phase**: Post-execution

Checks if job ran for at least a specified duration. Useful for detecting jobs that exit too quickly (potential errors).

**Config:**
```json
{
  "minDuration": "30s"
}
```

**Implementation:**
```go
type MinRuntimeConstraint struct {
    name        string
    minDuration time.Duration
}

func (m *MinRuntimeConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
    if ctx.StartTime == nil || ctx.EndTime == nil {
        return ConstraintResult{}, fmt.Errorf("start/end time not available")
    }

    runtime := ctx.EndTime.Sub(*ctx.StartTime)
    met := runtime >= m.minDuration

    return ConstraintResult{
        Met: met,
        Message: fmt.Sprintf("runtime %v (minimum %v)",
            runtime.Round(time.Second), m.minDuration),
    }, nil
}

func (m *MinRuntimeConstraint) EvaluationTiming() []EvaluationPhase {
    return []EvaluationPhase{EvaluationPhasePostExecution}
}
```

### 8. AlwaysPassConstraint (for testing)

**Evaluation Phase**: Any

Always returns true - useful for testing action execution.

```go
type AlwaysPassConstraint struct {
    name    string
    recheck bool
    phases  []EvaluationPhase
}

func (a *AlwaysPassConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
    return ConstraintResult{Met: true, Message: "always pass"}, nil
}

func (a *AlwaysPassConstraint) EvaluationTiming() []EvaluationPhase {
    return a.phases
}
```

### 9. AlwaysFailConstraint (for testing)

**Evaluation Phase**: Any

Always returns false - useful for testing violation actions.

```go
type AlwaysFailConstraint struct {
    name    string
    recheck bool
    phases  []EvaluationPhase
}

func (a *AlwaysFailConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
    return ConstraintResult{Met: false, Message: "always fail"}, nil
}

func (a *AlwaysFailConstraint) EvaluationTiming() []EvaluationPhase {
    return a.phases
}
```

**Note**: Action type implementations are defined in the `actions` module. See `/internal/actions/spec.md` for details on available action types and their configurations.

## Configuration Parsing

```go
type ConstraintConfig struct {
    Type           string                 `json:"type"`
    Name           string                 `json:"name"`
    RecheckOnRetry bool                   `json:"recheckOnRetry"`
    Config         json.RawMessage        `json:"config"`
    OnViolation    []actions.ActionConfig `json:"onViolation"`
    OnMet          []actions.ActionConfig `json:"onMet"`
}

// Load constraints from database for a job
func LoadConstraintsForJob(database *db.DB, jobID string) ([]ConstraintWithActions, error) {
    // Get all constraints for this job
    dbConstraints, err := database.GetConstraintsByJob(jobID)
    if err != nil {
        return nil, err
    }

    result := make([]ConstraintWithActions, len(dbConstraints))

    for i, dbConstraint := range dbConstraints {
        // Get constraint type to determine how to parse
        constraintType, err := database.GetConstraintType(dbConstraint.ConstraintTypeID)
        if err != nil {
            return nil, err
        }

        // Create constraint instance based on type
        constraint, err := createConstraintFromDB(&dbConstraint, constraintType)
        if err != nil {
            return nil, fmt.Errorf("failed to create constraint %s: %w", dbConstraint.ID, err)
        }

        // Get all actions for this constraint
        dbActions, err := database.GetActionsByConstraint(dbConstraint.ID)
        if err != nil {
            return nil, err
        }

        // Separate actions by trigger type
        var onViolation, onMet []Action
        for _, dbAction := range dbActions {
            actionType, err := database.GetActionType(dbAction.ActionTypeID)
            if err != nil {
                return nil, err
            }

            action, err := actions.CreateActionFromDB(&dbAction, actionType)
            if err != nil {
                return nil, fmt.Errorf("failed to create action %s: %w", dbAction.ID, err)
            }

            switch dbAction.Trigger {
            case "on_violated":
                onViolation = append(onViolation, action)
            case "on_met":
                onMet = append(onMet, action)
            }
        }

        result[i] = ConstraintWithActions{
            Constraint:     constraint,
            OnViolation:    onViolation,
            OnMet:          onMet,
            RecheckOnRetry: parseRecheckOnRetry(dbConstraint.Config),
        }
    }

    return result, nil
}

func createConstraintFromDB(dbConstraint *db.Constraint, constraintType *db.ConstraintType) (Constraint, error) {
    switch constraintType.Name {
    case "time_window":
        return parseTimeWindowConstraint(dbConstraint)
    case "other_job_running":
        return parseOtherJobRunningConstraint(dbConstraint)
    case "other_job_completed_recently":
        return parseOtherJobCompletedRecentlyConstraint(dbConstraint)
    case "other_job_scheduled_soon":
        return parseOtherJobScheduledSoonConstraint(dbConstraint)
    case "http_health_check":
        return parseHTTPHealthCheckConstraint(dbConstraint)
    case "max_runtime":
        return parseMaxRuntimeConstraint(dbConstraint)
    case "min_runtime":
        return parseMinRuntimeConstraint(dbConstraint)
    case "always_pass":
        return &AlwaysPassConstraint{name: dbConstraint.ID}, nil
    case "always_fail":
        return &AlwaysFailConstraint{name: dbConstraint.ID}, nil
    default:
        return nil, fmt.Errorf("unknown constraint type: %s", constraintType.Name)
    }
}

// Example parser for TimeWindowConstraint
func parseTimeWindowConstraint(dbConstraint *db.Constraint) (*TimeWindowConstraint, error) {
    if dbConstraint.Config == nil {
        return nil, fmt.Errorf("config required for time_window constraint")
    }

    var config struct {
        StartTime string `json:"startTime"`
        EndTime   string `json:"endTime"`
        Timezone  string `json:"timezone"`
    }

    if err := json.Unmarshal([]byte(*dbConstraint.Config), &config); err != nil {
        return nil, err
    }

    // Parse times and timezone
    // ... implementation details

    return &TimeWindowConstraint{
        name:      dbConstraint.ID,
        startTime: parsedStart,
        endTime:   parsedEnd,
        timezone:  tz,
        recheck:   parseRecheckOnRetry(dbConstraint.Config),
    }, nil
}
```

## Integration with Orchestrator

The orchestrator creates a constraint checker during initialization:

```go
// In orchestrator package
import "github.com/livinlefevreloca/itinerary/internal/constraints"

func NewOrchestrator(...) *Orchestrator {
    constraintChecker := constraints.NewConstraintChecker(
        jobConfig.ConstraintConfig,
        logger,
    )

    return &Orchestrator{
        constraintChecker: constraintChecker,
        ...
    }
}
```

### Pre-Execution Constraints

Checked before starting the job:

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

### During-Execution Constraints

Checked periodically while the job is running (e.g., for runtime limits):

```go
func (o *Orchestrator) runContainerRunning() {
    state := o.state.(*ContainerRunningState)

    // Check during-execution constraints on an interval
    ticker := time.NewTicker(1 * time.Minute) // or configurable interval
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            ctx := o.buildConstraintContext()
            result, err := o.constraintChecker.CheckDuringExecution(
                ctx,
                o.jobConfig,
                o.runID,
                o.timing.ExecutionStartedAt,
            )

            if err != nil {
                o.logger.Error("during-execution constraint check failed", "error", err)
                continue
            }

            if !result.ShouldProceed {
                // Constraint violated - trigger actions (may kill job)
                o.logger.Warn("during-execution constraint violated",
                    "message", result.Message)
                // Actions are executed by the constraint checker
                // If a kill action was executed, the container will stop
            }

        case <-o.containerExitChan:
            // Container exited, move to next state
            return
        }
    }
}
```

### Post-Execution Constraints

Checked after the job completes:

```go
func (o *Orchestrator) runContainerExited() {
    state := o.state.(*ContainerExitedState)

    ctx := o.buildConstraintContext()
    result, err := o.constraintChecker.CheckPostExecution(
        ctx,
        o.jobConfig,
        o.runID,
        o.timing.ExecutionStartedAt,
        o.timing.ExecutionCompletedAt,
        o.exitCode,
    )

    if err != nil {
        o.logger.Error("post-execution constraint check failed", "error", err)
    }

    if !result.ShouldProceed {
        o.logger.Info("post-execution constraints not met",
            "message", result.Message)
        // Actions already executed by constraint checker
    }

    // Continue to next state regardless
    o.transitionTo(state.ToCompleted())
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

## Scheduler Request/Response Messages

Constraints query the scheduler for live job state via typed request/response messages. The scheduler maintains in-memory state and is the source of truth for job execution status.

```go
// Job state query - used to check if a job is running, when it last ran, when it runs next
type JobStateRequest struct {
    JobID      string
    ResponseTo chan interface{}
}

type JobStateResponse struct {
    JobID     string
    IsRunning bool
    LastRun   *JobRunSummary
    NextRun   *time.Time
}

type JobRunSummary struct {
    RunID       string
    StartedAt   time.Time
    CompletedAt time.Time
    Success     bool
}

// Job history query - used to get recent run history
type JobHistoryRequest struct {
    JobID      string
    Limit      int
    ResponseTo chan interface{}
}

type JobHistoryResponse struct {
    JobID string
    Runs  []JobRunSummary
}
```

The scheduler handles these requests by:
1. Receiving the request message in its inbox
2. Querying its internal in-memory state (scheduled runs index, running jobs map)
3. Sending the response back on the provided channel

**Note**: Constraints do NOT query the database for job state - the scheduler's in-memory state is the source of truth for what's currently running, scheduled, and recently completed.

## Future Extensions

Potential new constraint types:
- `MaxConcurrentRunsConstraint` - Limits concurrent runs of this job
- `ExitCodeConstraint` - Checks job exit code (post-execution)
- `DataAvailableConstraint` - Checks if input data is ready via HTTP/webhook
- `QuotaConstraint` - Checks resource quotas via API
- `RateLimitConstraint` - Enforces rate limits on job execution
- `MaintenanceWindowConstraint` - Prevents execution during maintenance windows
- `DiskSpaceConstraint` - Checks available disk space via system metrics
- `CPULoadConstraint` - Checks CPU load before starting resource-intensive jobs

For potential new action types, see `/internal/actions/spec.md`.
