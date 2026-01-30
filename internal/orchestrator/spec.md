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
            o.stateTransitionTime = time.Now()
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
- State transitions are atomic operations
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

**Phase 2: Pre-Execution Constraints**
- State: `Pending → ConditionPending`
- Check if job has pre-execution constraints defined
- Transition: `ConditionPending → ConditionRunning`
- Evaluate each constraint:
  - `maxConcurrentRuns`: Check if too many instances running
  - `requirePreviousSuccess`: Check if previous run succeeded
  - `catchUp`: Determine if this is a catch-up run
- Transition: `ConditionRunning → ActionPending` (if constraint violated) or `ConditionRunning → ContainerCreating` (if all pass)

**Phase 3: Pre-Execution Actions**
- State: `ActionPending → ActionRunning`
- Execute action based on constraint violation:
  - `skipJob`: Skip this run
  - `retryLater`: Delay execution
  - `killOldInstances`: Terminate running instances
  - `waitForCompletion`: Wait for running instances to finish
- Transition: `ActionRunning → ContainerCreating` or `ActionRunning → Failed`

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

### 3. Constraint Checking

#### Pre-Execution Constraints
```go
func checkPreExecutionConstraints(job *Job, runID string) ([]ConstraintViolation, error)
```

**Constraints to check:**
- `maxConcurrentRuns`: Query active orchestrators for same job
- `requirePreviousSuccess`: Query database for last run status
- `catchUpWindow`: Check if run is within catch-up window
- `preRunHook`: Execute HTTP webhook before running

**Returns:**
- List of violated constraints
- Error if check fails

#### Post-Execution Constraints
```go
func checkPostExecutionConstraints(job *Job, startTime, endTime time.Time) ([]ConstraintViolation, error)
```

**Constraints to check:**
- `maxExpectedRunTime`: Compare runtime to threshold
- `maxAllowedRunTime`: Check if hard limit exceeded
- `postRunHook`: Execute HTTP webhook after completion

### 4. Action Execution

```go
func executeAction(action Action, job *Job, runID string) error
```

**Actions to support:**
- `skipJob`: Mark as skipped and complete
- `retryLater`: Delay execution by configured amount
- `killOldInstances`: Cancel older running instances
- `waitForCompletion`: Block until other instances finish
- `triggerWebhook`: Send webhook notification
- `startAnotherJob`: Trigger another job to run

### 5. Retry Logic

```go
type RetryState struct {
    MaxRetries    int
    CurrentAttempt int
    RetryDelay    time.Duration
    BackoffMultiplier float64
}

func shouldRetry(job *Job, retryState *RetryState) bool
func calculateRetryDelay(retryState *RetryState) time.Duration
```

**Retry behavior:**
- Check if job has retry configuration
- Track attempt number in orchestrator state
- Calculate exponential backoff delay
- Maximum retry attempts configurable
- Send state change to `Retrying` before retry

### 6. Database Updates

The orchestrator sends updates to the JobStateSyncer via the scheduler for all state changes:

```go
type JobRunUpdate struct {
    UpdateID    string    // UUID for deduplication
    RunID       string    // Deterministic: "jobID:timestamp"
    JobID       string
    ScheduledAt time.Time
    StartedAt   time.Time
    CompletedAt time.Time
    Status      string
    Success     bool
    ExitCode    int
    Error       string
    Logs        string
    RetryAttempt int
}
```

**When to send updates:**
- State transitions (every state change)
- Terminal states (Completed, Failed, Cancelled)
- Error conditions

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
    stateTransitionTime time.Time

    // State machine validation
    transitionLock     sync.Mutex  // Protects state transitions

    // Communication channels
    cancelChan     chan struct{}
    configUpdate   chan *Job
    schedulerInbox *inbox.Inbox

    // Metrics tracking
    metrics        *OrchestratorMetrics
    stateTracker   *StateTracker

    // Retry tracking
    retryAttempt   int
    maxRetries     int

    // Kubernetes client
    k8sClient      kubernetes.Interface

    // Timing
    createdAt      time.Time
    startedAt      time.Time
    completedAt    time.Time

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
    k8sClient kubernetes.Interface,
    logger *slog.Logger,
) *Orchestrator {
    return &Orchestrator{
        runID:           runID,
        jobID:           jobID,
        jobConfig:       jobConfig,
        scheduledAt:     scheduledAt,
        state:           OrchestratorPreRun,
        stateTransitionTime: time.Now(),
        cancelChan:      make(chan struct{}),
        configUpdate:    make(chan *Job, 1),
        schedulerInbox:  schedulerInbox,
        metrics:         NewOrchestratorMetrics(runID, jobID, scheduledAt),
        stateTracker:    NewStateTracker(),
        retryAttempt:    0,
        maxRetries:      getMaxRetries(jobConfig),
        k8sClient:       k8sClient,
        createdAt:       time.Now(),
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

    // Constraints
    Constraints JobConstraints

    // Actions
    Actions JobActions

    // Retry configuration
    RetryConfig *RetryConfig
}

type JobConstraints struct {
    MaxConcurrentRuns    int
    RequirePreviousSuccess bool
    CatchUp              bool
    CatchUpWindow        time.Duration
    MaxExpectedRunTime   time.Duration
    MaxAllowedRunTime    time.Duration
    PreRunHook           *WebhookConfig
    PostRunHook          *WebhookConfig
}

type JobActions struct {
    OnConstraintViolation Action
    OnFailure             Action
    OnSuccess             Action
}

type Action struct {
    Type       ActionType
    Parameters map[string]string
}

type ActionType string

const (
    ActionSkip            ActionType = "skip"
    ActionRetryLater      ActionType = "retry_later"
    ActionKillOld         ActionType = "kill_old"
    ActionWait            ActionType = "wait"
    ActionWebhook         ActionType = "webhook"
    ActionTriggerJob      ActionType = "trigger_job"
)

type RetryConfig struct {
    MaxRetries        int
    InitialDelay      time.Duration
    BackoffMultiplier float64
    MaxDelay          time.Duration
}
```

### Constraint Violation
```go
type ConstraintViolation struct {
    Constraint string
    Message    string
    Timestamp  time.Time
}
```

## Implementation Strategy

### Phase 1: Basic Lifecycle (MVP)
1. Implement PreRun → Pending → ContainerCreating → Running → Terminating → Completed flow
2. Kubernetes job creation and monitoring
3. Basic heartbeat mechanism
4. Basic cancellation handling
5. Database update on terminal states

### Phase 2: Constraint Checking
1. Implement constraint checking framework
2. Add `maxConcurrentRuns` constraint
3. Add `requirePreviousSuccess` constraint
4. Add execution time constraints
5. Database updates for constraint violations

### Phase 3: Action Execution
1. Implement action execution framework
2. Add `skip` action
3. Add `killOld` action
4. Add `wait` action
5. Database updates for action executions

### Phase 4: Retry Logic
1. Implement retry state tracking
2. Add exponential backoff
3. Handle retry transitions
4. Track retry attempts in database

### Phase 5: Webhook Integration
1. Implement pre-run hooks
2. Implement post-run hooks
3. Add webhook delivery tracking

## External Dependencies

### Kubernetes Client
- Use `k8s.io/client-go` for Kubernetes API interaction
- Create jobs, watch pods, retrieve logs
- Handle API errors gracefully

### Job State Syncer
- Send all state updates via scheduler's JobStateSyncer
- Use `JobRunUpdate` for persistence

### Scheduler Inbox
- Send heartbeats
- Send state changes
- Send completion notifications

## Error Handling

### Kubernetes Errors
- Job creation failure → `Failed` state
- Pod start timeout → `Failed` state
- Pod crash/error → Retry or `Failed` state

### Network Errors
- Transient errors → retry with backoff
- Persistent errors → `Failed` state

### Constraint Check Errors
- Cannot check constraint → fail-safe decision (skip or allow)
- Log error and continue

### Action Execution Errors
- Action fails → log error and continue to execution
- Critical action failure → `Failed` state

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
    PreRunDuration            time.Duration  // Time waiting for scheduled time
    ConstraintCheckDuration   time.Duration  // Time checking constraints
    ActionExecutionDuration   time.Duration  // Time executing actions
    ContainerCreationDuration time.Duration  // Time creating Kubernetes resources
    ExecutionDuration         time.Duration  // Time job was running
    TerminationDuration       time.Duration  // Time in cleanup/termination
    TotalDuration             time.Duration  // End-to-end time

    // State transition counts
    StateTransitionCount      int            // Total number of state changes
    StateTransitionTimestamps map[OrchestratorStatus]time.Time

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
    PodStartLatency           time.Duration  // Time from creation to running
    PodExitCode               int            // Final pod exit code

    // Heartbeat metrics
    HeartbeatsSent            int            // Number of heartbeats sent

    // Outcome
    FinalState                OrchestratorStatus
    Success                   bool
    ErrorMessage              string
}
```

#### State Duration Tracking
The orchestrator maintains timestamps for each state entry:
```go
type StateTracker struct {
    currentState      OrchestratorStatus
    stateEntryTime    time.Time
    stateDurations    map[OrchestratorStatus]time.Duration
}

func (st *StateTracker) enterState(state OrchestratorStatus) {
    now := time.Now()

    // Record duration of previous state
    if st.currentState != 0 {
        duration := now.Sub(st.stateEntryTime)
        st.stateDurations[st.currentState] += duration
    }

    st.currentState = state
    st.stateEntryTime = now
}
```

#### Metrics Collection Points

**On State Transition:**
- Record timestamp
- Increment state transition counter
- Calculate duration in previous state

**On Constraint Check:**
- Increment constraints checked counter
- Record constraint evaluation time
- If violated, increment violations counter

**On Action Execution:**
- Increment actions executed counter
- Record action execution time
- If failed, increment action failures counter

**On Kubernetes Operation:**
- Increment API call counter
- Record operation latency

**On Heartbeat:**
- Increment heartbeats sent counter

**On Completion:**
- Calculate all phase durations
- Calculate total duration
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
