# Orchestrator Implementation Specification

## Overview
The Orchestrator manages the complete lifecycle of a single job run, from pre-execution constraint checking through execution to post-execution cleanup. Each job run gets its own orchestrator goroutine that operates independently while coordinating with the central scheduler via inbox messages.

## Design Principles

### 1. Single Responsibility
Each orchestrator manages exactly one job run from creation to terminal state.

### 2. Heartbeat-Based Liveness
Orchestrators prove they're alive by sending periodic heartbeats to the scheduler. Missing heartbeats triggers orphan detection.

### 3. Graceful Cancellation
Orchestrators can be cancelled at any point and will clean up gracefully before terminating.

### 4. State Machine
The orchestrator follows a strict state machine with well-defined transitions and terminal states.

### 5. No Shared State
Orchestrators communicate only via inbox messages - no shared memory with the scheduler loop.

## State Machine

### States

```go
type OrchestratorStatus int

const (
    // Pre-execution states
    OrchestratorPreRun           OrchestratorStatus = iota // Waiting for scheduled time
    OrchestratorPending                                    // Scheduled time reached
    OrchestratorConditionPending                           // About to check constraints
    OrchestratorConditionRunning                           // Checking constraints
    OrchestratorActionPending                              // Constraint violated, about to act
    OrchestratorActionRunning                              // Taking action

    // Execution states
    OrchestratorContainerCreating                          // Creating Kubernetes pod
    OrchestratorRunning                                    // Job executing
    OrchestratorTerminating                                // Finishing/cleanup

    // Retry state
    OrchestratorRetrying                                   // Before retry attempt

    // Terminal states
    OrchestratorCompleted                                  // Success
    OrchestratorFailed                                     // Failed
    OrchestratorCancelled                                  // User cancelled
    OrchestratorOrphaned                                   // No heartbeats (set by scheduler)
)
```

### State Transitions

#### Happy Path
```
PreRun → Pending → ConditionPending → ConditionRunning →
ContainerCreating → Running → Terminating → Completed
```

#### With Pre-Execution Actions
```
PreRun → Pending → ConditionPending → ConditionRunning →
ActionPending → ActionRunning → ContainerCreating → Running →
Terminating → Completed
```

#### With Retry
```
... → Running → Failed → Retrying → Pending → ... → Completed
```

#### Cancellation (from any non-terminal state)
```
<any> → Cancelled
```

#### Orphaning (detected by scheduler)
```
<any> → Orphaned
```

### Terminal States
States from which no further transitions occur:
- `Completed`
- `Failed` (only if no retries remain)
- `Cancelled`
- `Orphaned`

### State Machine Validation

All state transitions must be validated to prevent invalid states. The orchestrator enforces valid transitions through a state machine validator.

#### Allowed Transitions Map
```go
var allowedTransitions = map[OrchestratorStatus][]OrchestratorStatus{
    OrchestratorPreRun: {
        OrchestratorPending,
        OrchestratorCancelled,
    },
    OrchestratorPending: {
        OrchestratorConditionPending,
        OrchestratorContainerCreating,  // Skip constraints if none defined
        OrchestratorCancelled,
        OrchestratorFailed,
    },
    OrchestratorConditionPending: {
        OrchestratorConditionRunning,
        OrchestratorCancelled,
        OrchestratorFailed,
    },
    OrchestratorConditionRunning: {
        OrchestratorActionPending,      // Constraint violated
        OrchestratorContainerCreating,  // All constraints passed
        OrchestratorCancelled,
        OrchestratorFailed,
    },
    OrchestratorActionPending: {
        OrchestratorActionRunning,
        OrchestratorCancelled,
        OrchestratorFailed,
    },
    OrchestratorActionRunning: {
        OrchestratorContainerCreating,  // Action completed, proceed
        OrchestratorCompleted,          // Action: skip job
        OrchestratorCancelled,
        OrchestratorFailed,
    },
    OrchestratorContainerCreating: {
        OrchestratorRunning,
        OrchestratorCancelled,
        OrchestratorFailed,
    },
    OrchestratorRunning: {
        OrchestratorTerminating,
        OrchestratorCancelled,
        OrchestratorFailed,
    },
    OrchestratorTerminating: {
        OrchestratorCompleted,
        OrchestratorFailed,
        OrchestratorRetrying,
        OrchestratorCancelled,
    },
    OrchestratorRetrying: {
        OrchestratorPending,            // Retry attempt
        OrchestratorFailed,             // No retries left
        OrchestratorCancelled,
    },
    // Terminal states - no transitions allowed
    OrchestratorCompleted:  {},
    OrchestratorFailed:     {},
    OrchestratorCancelled:  {},
    OrchestratorOrphaned:   {},
}
```

#### Transition Validation Function
```go
func (o *Orchestrator) transitionTo(newState OrchestratorStatus) error {
    allowed := allowedTransitions[o.state]

    for _, allowedState := range allowed {
        if newState == allowedState {
            o.previousState = o.state
            o.state = newState
            return nil
        }
    }

    return fmt.Errorf("invalid state transition: %s -> %s",
        o.state.String(), newState.String())
}
```

#### Transition Enforcement
- All state changes MUST go through `transitionTo()`
- Direct assignment to state field is prohibited
- Invalid transitions return error and are logged
- **Lock-free design**: Each orchestrator runs in a single goroutine, so no synchronization needed
- Previous state is tracked for debugging

## Core Functionality

### 1. Lifecycle Management

#### runOrchestrator Function
```go
func (s *Scheduler) runOrchestrator(
    jobID string,
    scheduledAt time.Time,
    runID string,
    cancelChan chan struct{},
    configUpdate chan *Job,
)
```

**Responsibilities:**
- Wait until scheduled time (PreRun phase)
- Check pre-execution constraints
- Execute pre-execution actions if needed
- Create Kubernetes job/pod
- Monitor execution
- Handle retries
- Report completion

**Heartbeat Behavior:**
- Send heartbeat every `OrchestratorHeartbeatInterval` (default: 10s)
- Continue heartbeats in all non-terminal states
- Stop heartbeats when reaching terminal state

#### State Transitions

**Phase 1: PreRun**
- State: `OrchestratorPreRun`
- Wait until `scheduledAt` time arrives
- Send periodic heartbeats
- Handle config updates via `configUpdate` channel
- Handle cancellation via `cancelChan`
- Transition: `PreRun → Pending` when scheduled time arrives

**Phase 2: Pre-Execution Constraints and Actions**
- State: `Pending → ConditionPending`
- Check if job has pre-execution constraints defined
- Transition: `ConditionPending → ConditionRunning`
- Call `constraintChecker.CheckPreExecution()`
  - Constraint checker internally evaluates all constraints
  - For each constraint, calls either onViolation or onMet action list
  - Actions may send messages back to orchestrator or scheduler
  - Returns `ConstraintCheckResult` with ShouldProceed boolean
- Transition based on result:
  - `ShouldProceed = true`: `ConditionRunning → ContainerCreating`
  - `ShouldProceed = false`: `ConditionRunning → Failed` or `ConditionRunning → Retrying`

**Note:** The constraint checker may internally transition orchestrator through `ActionPending` and `ActionRunning` states by sending state change messages while actions execute. This provides visibility into long-running actions but is managed by the constraint/action module, not the orchestrator itself.

**Phase 4: Execution**
- State: `ContainerCreating`
- Create Kubernetes Job resource with pod spec from job config
- Wait for pod to be created
- Transition: `ContainerCreating → Running` when pod starts

- State: `Running`
- Monitor pod status
- Send periodic heartbeats
- Handle cancellation
- Transition: `Running → Terminating` when pod completes

**Phase 5: Termination**
- State: `Terminating`
- Retrieve pod logs (if configured)
- Determine success/failure from exit code
- Check post-execution constraints:
  - `maxExpectedRunTime`: Did it run too long?
  - `maxAllowedRunTime`: Did it exceed hard limit?
- Execute post-execution actions if needed
- Transition: `Terminating → Completed` or `Terminating → Failed` or `Terminating → Retrying`

**Phase 6: Retry Logic**
- State: `Retrying`
- Check retry configuration
- Increment retry attempt counter
- Wait for retry delay
- Transition: `Retrying → Pending` (new attempt)

### 2. Kubernetes Integration

#### Job Creation
```go
func createKubernetesJob(jobConfig *Job, runID string) error
```

**Responsibilities:**
- Build Kubernetes Job manifest from job config
- Apply labels: `job-id`, `run-id`, `scheduled-at`
- Set pod spec from job configuration
- Create job in cluster
- Return error if creation fails

#### Pod Monitoring
```go
func watchPodStatus(runID string) (exitCode int, err error)
```

**Responsibilities:**
- Watch pod events for the job
- Track pod phases: Pending, Running, Succeeded, Failed
- Capture exit code on completion
- Return timeout error if pod doesn't start within threshold

#### Log Retrieval
```go
func retrievePodLogs(runID string) (logs string, err error)
```

**Responsibilities:**
- Retrieve pod logs after completion
- Store logs to database or external storage
- Handle log truncation if too large

### 3. Constraint-Action System Interface

The orchestrator delegates constraint evaluation and action execution to a pluggable constraint checker module.

```go
type ConstraintChecker interface {
    // CheckPreExecution evaluates all pre-execution constraints
    // Internally handles calling onViolation/onMet actions for each constraint
    // Returns result indicating whether execution should proceed
    CheckPreExecution(ctx context.Context, job *Job, runID string) (ConstraintCheckResult, error)

    // CheckPostExecution evaluates all post-execution constraints
    CheckPostExecution(ctx context.Context, job *Job, runID string, startTime, endTime time.Time, exitCode int) (ConstraintCheckResult, error)
}

type ConstraintCheckResult struct {
    ShouldProceed bool   // false if constraints prevent execution
    Message       string // Summary of constraint evaluation and actions taken
}
```

**Orchestrator responsibilities:**
- Call `CheckPreExecution()` during ConditionRunning state
- If `ShouldProceed` is true, transition to ContainerCreating
- If `ShouldProceed` is false, transition to Failed or Retrying (based on result)
- Call `CheckPostExecution()` during Terminating state
- Record constraint check results in metrics

**How Constraint-Action System Actually Works (separate module - not part of orchestrator):**

In the job configuration, constraints are defined with associated action lists:
```yaml
constraints:
  - constraint1:
      config: {...}
      onViolation:
        - action1
        - action2
      onMet:
        - action3
```

**Implementation details (separate module):**
- Each `Constraint` struct implements a `Constraint` interface with:
  - `Check()`: Evaluates the constraint
  - `OnViolation()`: Runs the action list when constraint is violated
  - `OnMet()`: Runs the action list when constraint is met

- Each `Action` struct implements an `Action` interface with:
  - `Take()`: Executes the action

**Communication & Context:**
- All constraints need access to an inbox for sending messages to the scheduler loop and orchestrator
- All constraints need access to a webhook handler (webhooks are a specific action type)
- This is provided via a Context that gets passed from constraint into each action
- The orchestrator provides these dependencies when calling the constraint checker

**From Orchestrator's Perspective:**
The orchestrator simply calls `CheckPreExecution()` and receives back a `ShouldProceed` boolean. All the complexity of:
- Evaluating each constraint
- Calling appropriate action lists (onViolation/onMet)
- Executing individual actions
- Determining if execution should proceed

...happens inside the constraint/action module. The orchestrator doesn't need to understand these details.

### 4. Retry Logic

The orchestrator coordinates retry attempts based on job configuration.

```go
type RetryConfig struct {
    MaxRetries        int
    InitialDelay      time.Duration
    BackoffMultiplier float64
    MaxDelay          time.Duration
}

func shouldRetry(retryConfig *RetryConfig, currentAttempt int) bool {
    return currentAttempt < retryConfig.MaxRetries
}

func calculateRetryDelay(retryConfig *RetryConfig, attempt int) time.Duration {
    // Exponential backoff: InitialDelay * (BackoffMultiplier ^ attempt)
    delay := retryConfig.InitialDelay * time.Duration(math.Pow(retryConfig.BackoffMultiplier, float64(attempt)))
    if delay > retryConfig.MaxDelay {
        return retryConfig.MaxDelay
    }
    return delay
}
```

**Retry behavior:**
- On job failure, check if retries are configured and remaining
- If yes: transition to `Retrying`, calculate delay, wait, then transition to `Pending`
- If no: transition to `Failed`
- Track attempt number in orchestrator metrics
- Each retry is a full lifecycle restart from Pending state

### 6. Database Updates

The orchestrator sends updates to the JobStateSyncer for important state changes:

```go
type JobRunUpdate struct {
    UpdateID     string    // UUID for deduplication
    RunID        string    // Deterministic: "jobID:timestamp"
    JobID        string
    ScheduledAt  time.Time
    StartedAt    time.Time
    CompletedAt  time.Time
    Status       string
    Success      bool
    ExitCode     int
    Error        string
    RetryAttempt int
}
```

**When to send updates:**
- Job starts executing (StartedAt timestamp)
- Terminal states reached (Completed, Failed, Cancelled)
- Retry attempts (increment RetryAttempt)

**Not included:**
- Every state transition (too verbose for database)
- Metrics (sent separately to stats collector)

### 7. Heartbeat Mechanism

```go
func sendHeartbeat(inbox *inbox.Inbox, runID string)
```

**Behavior:**
- Send heartbeat every `OrchestratorHeartbeatInterval`
- Include current timestamp
- Scheduler tracks last heartbeat time
- Missing `MaxMissedOrchestratorHeartbeats` triggers orphan status

**Heartbeat message:**
```go
type OrchestratorHeartbeatMsg struct {
    RunID     string
    Timestamp time.Time
}
```

### 8. Cancellation Handling

The orchestrator monitors `cancelChan` at every phase:

**Cancellation response:**
1. Stop current operation
2. Clean up Kubernetes resources (delete job/pod)
3. Send completion message with `Cancelled` status
4. Exit goroutine

## Data Structures

### Orchestrator Structure

```go
type Orchestrator struct {
    // Identity
    runID       string
    jobID       string
    jobConfig   *Job
    scheduledAt time.Time

    // State management
    state              OrchestratorStatus
    previousState      OrchestratorStatus

    // Communication channels
    cancelChan     chan struct{}
    configUpdate   chan *Job
    schedulerInbox *inbox.Inbox

    // Dependencies for constraint/action system
    webhookHandler *webhook.Handler  // Passed to constraints for webhook actions
    constraintChecker ConstraintChecker // For evaluating constraints

    // Metrics tracking
    metrics        *OrchestratorMetrics

    // Retry tracking
    retryAttempt   int
    maxRetries     int

    // Kubernetes client
    k8sClient      kubernetes.Interface

    // Phase timing (mark boundaries, calculate durations)
    createdAt               time.Time
    constraintCheckStarted  time.Time
    executionStartedAt      time.Time
    completedAt             time.Time

    // Logging
    logger         *slog.Logger
}
```

### Orchestrator Creation

```go
func NewOrchestrator(
    runID string,
    jobID string,
    jobConfig *Job,
    scheduledAt time.Time,
    schedulerInbox *inbox.Inbox,
    webhookHandler *webhook.Handler,
    constraintChecker ConstraintChecker,
    k8sClient kubernetes.Interface,
    logger *slog.Logger,
) *Orchestrator {
    now := time.Now()
    return &Orchestrator{
        runID:           runID,
        jobID:           jobID,
        jobConfig:       jobConfig,
        scheduledAt:     scheduledAt,
        state:           OrchestratorPreRun,
        cancelChan:      make(chan struct{}),
        configUpdate:    make(chan *Job, 1),
        schedulerInbox:  schedulerInbox,
        webhookHandler:  webhookHandler,
        constraintChecker: constraintChecker,
        metrics:         NewOrchestratorMetrics(runID, jobID, scheduledAt),
        retryAttempt:    0,
        maxRetries:      getMaxRetries(jobConfig),
        k8sClient:       k8sClient,
        createdAt:       now,
        logger:          logger.With("run_id", runID, "job_id", jobID),
    }
}
```

### Job Configuration
```go
type Job struct {
    ID       string
    Name     string
    Schedule string // Cron expression
    PodSpec  string // Kubernetes pod spec (JSON or YAML)

    // Constraint/Action configuration (opaque to orchestrator)
    // Interpreted by constraint and action modules
    ConstraintConfig json.RawMessage  // Constraint checker interprets this
    ActionConfig     json.RawMessage  // Action executor interprets this

    // Retry configuration (orchestrator needs to understand this)
    RetryConfig *RetryConfig
}

type RetryConfig struct {
    MaxRetries        int
    InitialDelay      time.Duration
    BackoffMultiplier float64
    MaxDelay          time.Duration
}
```

**Design notes:**
- `ConstraintConfig` and `ActionConfig` are opaque JSON
- Orchestrator passes them to constraint/action modules
- Orchestrator only needs to understand `RetryConfig` to coordinate retries
- This allows constraint/action implementations to evolve independently

## Implementation Strategy

### Phase 1: Basic Lifecycle (MVP)
1. Implement state machine with transition validation
2. Implement PreRun → Pending → ContainerCreating → Running → Terminating → Completed flow (no constraints/actions)
3. Kubernetes job creation and monitoring
4. Heartbeat mechanism
5. Cancellation handling
6. Database updates on terminal states
7. Basic metrics collection

### Phase 2: Constraint/Action Integration
1. Define ConstraintChecker interface
2. Define ActionExecutor interface
3. Add constraint checking phase (Pending → ConditionPending → ConditionRunning)
4. Add action execution phase (ActionPending → ActionRunning)
5. Integrate with lifecycle flow
6. Record constraint violations and action executions in metrics

### Phase 3: Retry Logic
1. Implement retry state tracking
2. Add exponential backoff calculation
3. Handle Failed → Retrying → Pending transitions
4. Track retry attempts in metrics

### Phase 4: Full Metrics & Observability
1. Complete OrchestratorMetrics implementation
2. Phase boundary timing (minimal time.Now() calls)
3. Submit metrics to stats collector on completion

**Note:** Specific constraint types and action types will be implemented in separate modules (not part of orchestrator)

## External Dependencies

### Kubernetes Client
- Use `k8s.io/client-go` for Kubernetes API interaction
- Create jobs, watch pods, retrieve logs
- Handle API errors gracefully

### Constraint Checker (interface)
- Separate module implements `ConstraintChecker` interface
- Orchestrator calls interface methods
- No direct dependency on constraint implementation

### Action Executor (interface)
- Separate module implements `ActionExecutor` interface
- Orchestrator calls interface methods
- No direct dependency on action implementation

### Job State Syncer
- Send state updates via scheduler's JobStateSyncer
- Use `JobRunUpdate` for persistence

### Stats Collector
- Send OrchestratorMetrics on completion
- Track per-run execution data

### Scheduler Inbox
- Send heartbeats
- Send state change notifications
- Send completion messages

## Error Handling

### Kubernetes Errors
- Job creation failure → `Failed` state
- Pod start timeout → `Failed` state
- Pod crash/error → Retry or `Failed` state

### Network Errors
- Transient errors → retry with backoff
- Persistent errors → `Failed` state

### Constraint Checker Errors
- Interface returns error
- Orchestrator decides: fail-safe (allow) or fail-closed (deny)
- Log error and record in metrics

### Action Executor Errors
- Interface returns error
- Orchestrator transitions to `Failed` state
- Log error and record in metrics

## Performance Considerations

### Goroutine Management
- One goroutine per active job run
- Graceful shutdown on cancellation
- No goroutine leaks

### Kubernetes API Calls
- Use watch API instead of polling
- Implement client-side rate limiting
- Cache pod status locally

### Database Updates
- Buffer updates through JobStateSyncer
- Batch writes where possible
- Use deterministic update IDs for deduplication

## Testing Strategy

### Unit Tests
- State transition logic
- Constraint checking
- Action execution
- Retry logic
- Heartbeat mechanism

### Integration Tests
- Full lifecycle with mock Kubernetes
- Cancellation scenarios
- Timeout scenarios
- Orphan detection

### End-to-End Tests
- Real Kubernetes cluster tests
- Multi-constraint scenarios
- Retry scenarios
- Concurrent execution limits

## Monitoring & Observability

### Orchestrator-Level Metrics

Each orchestrator tracks metrics about its own execution lifecycle. These are sent to the stats collector upon completion.

#### Timing Metrics
```go
type OrchestratorMetrics struct {
    RunID       string
    JobID       string
    ScheduledAt time.Time

    // Phase timing (duration spent in each phase)
    // Calculated from phase boundary timestamps
    PreRunDuration            time.Duration  // Time waiting for scheduled time
    ConstraintCheckDuration   time.Duration  // Time checking constraints + executing actions
    ExecutionDuration         time.Duration  // Time job was running (pod running)
    TerminationDuration       time.Duration  // Time in cleanup/termination
    TotalDuration             time.Duration  // End-to-end time

    // Constraint/Action metrics
    ConstraintsChecked        int            // Number of constraints evaluated
    ConstraintsViolated       int            // Number of constraints that failed
    ActionsExecuted           int            // Number of actions taken
    ActionsFailed             int            // Number of actions that failed

    // Retry metrics
    RetryAttempt              int            // Current retry attempt (0 = first attempt)
    MaxRetries                int            // Maximum retries configured

    // Kubernetes metrics
    KubernetesAPICalls        int            // Number of K8s API calls made
    PodStartLatency           time.Duration  // Time from job creation to pod running
    PodExitCode               int            // Final pod exit code

    // Heartbeat metrics
    HeartbeatsSent            int            // Number of heartbeats sent

    // Outcome
    FinalState                OrchestratorStatus
    Success                   bool
    ErrorMessage              string
}
```

#### Phase Timing Strategy

To minimize performance overhead from frequent `time.Now()` calls, the orchestrator only captures timestamps at **phase boundaries**, not at every state transition.

**Phase Boundary Timestamps:**
```go
// In Orchestrator struct:
createdAt               time.Time  // When orchestrator created (PreRun start)
constraintCheckStarted  time.Time  // When entering ConditionPending
executionStartedAt      time.Time  // When pod starts running
completedAt             time.Time  // When reaching terminal state
```

**Duration Calculations (on completion):**
```go
metrics.PreRunDuration = o.constraintCheckStarted.Sub(o.createdAt)
metrics.ConstraintCheckDuration = o.executionStartedAt.Sub(o.constraintCheckStarted)
metrics.ExecutionDuration = o.completedAt.Sub(o.executionStartedAt)
metrics.TotalDuration = o.completedAt.Sub(o.createdAt)
```

This reduces `time.Now()` calls from ~20 per run to ~4-5 per run.

#### Metrics Collection Points

**On Constraint Check:**
- Increment constraints checked counter
- If violated, increment violations counter

**On Action Execution:**
- Increment actions executed counter
- If failed, increment action failures counter

**On Kubernetes Operation:**
- Increment API call counter

**On Heartbeat:**
- Increment heartbeats sent counter

**On Completion:**
- Calculate phase durations from boundary timestamps
- Package all metrics
- Send to stats collector

### Logging
- State transitions (INFO level)
- Constraint violations (WARN level)
- Action executions (INFO level)
- Errors (ERROR level)
- Heartbeats (DEBUG level)

### Structured Logging Fields
- `run_id`: Unique run identifier
- `job_id`: Job identifier
- `scheduled_at`: Scheduled execution time
- `state`: Current orchestrator state
- `attempt`: Retry attempt number
