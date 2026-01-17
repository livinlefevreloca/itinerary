# Central Scheduler Implementation Plan

## Overview
The Central Scheduler is the core component of Itinerary. It operates as an event loop that maintains all scheduling state, coordinates job orchestrators, and ensures no I/O operations occur in the main loop.

## Core Design Principles

### 1. Single Source of Truth
The scheduler loop owns all scheduling state. Any component needing state information must:
- Send a request message to the scheduler's inbox
- Include a response channel in the message
- Wait for the scheduler to respond with the requested state

### 2. No I/O in Main Loop
The main loop is strictly computational. All I/O operations are delegated to other goroutines:
- Database reads/writes → Syncer goroutines
- Kubernetes operations → Orchestrator goroutines
- External API calls → Watcher goroutine
- Logging → Async logger goroutine (if needed)

### 3. Deterministic Iteration
Each loop iteration follows a fixed sequence of operations, making the system predictable and testable.

## Configuration

### Configuration Structure
```go
type SchedulerConfig struct {
    // How far ahead to launch orchestrators before job start time
    PreScheduleInterval time.Duration // Default: 10 seconds

    // How often to rebuild the scheduled run index
    IndexRebuildInterval time.Duration // Default: 1 minute

    // How far ahead to calculate scheduled runs
    LookaheadWindow time.Duration // Default: 10 minutes

    // How far back to include runs (prevents missing near-past jobs)
    GracePeriod time.Duration // Default: 30 seconds

    // Main loop iteration interval
    LoopInterval time.Duration // Default: 1 second

    // Inbox buffer size
    InboxBufferSize int // Default: 1000
}
```

### Validation Rules
- `IndexRebuildInterval < LookaheadWindow` (required)
- `GracePeriod > LoopInterval` (recommended)
- `PreScheduleInterval >= LoopInterval` (recommended)

### Defaults
```go
func DefaultSchedulerConfig() SchedulerConfig {
    return SchedulerConfig{
        PreScheduleInterval:  10 * time.Second,
        IndexRebuildInterval: 1 * time.Minute,
        LookaheadWindow:      10 * time.Minute,
        GracePeriod:          30 * time.Second,
        LoopInterval:         1 * time.Second,
        InboxBufferSize:      1000,
    }
}
```

## Data Structures

### Main Scheduler Type
```go
type Scheduler struct {
    // Configuration
    config SchedulerConfig

    // State
    jobDefinitions  map[string]*Job           // jobID → Job
    index           *index.ScheduledRunIndex
    activeOrchestrators map[string]*OrchestratorState // runID → state
    lastRebuildTime time.Time
    stats           SchedulerStats

    // Communication channels
    inbox      chan InboxMessage
    shutdown   chan struct{}

    // Syncer channels (for database writes)
    jobRunSyncer chan JobRunUpdate
    statsSyncer  chan StatsUpdate

    // For tracking what needs rebuilding
    needsRebuild bool
}
```

### Orchestrator State
```go
type OrchestratorState struct {
    RunID        string
    JobID        string
    ScheduledAt  time.Time
    ActualStart  time.Time
    Status       OrchestratorStatus
    CancelChan   chan struct{}
    CompletedAt  time.Time // Set when orchestrator completes
}

type OrchestratorStatus int

const (
    OrchestratorPending OrchestratorStatus = iota
    OrchestratorRunning
    OrchestratorCompleted
    OrchestratorFailed
    OrchestratorCancelled
)
```

### Inbox Messages
```go
type InboxMessage struct {
    Type MessageType
    Data interface{}
    ResponseChan chan<- interface{} // Optional, for request/response
}

type MessageType int

const (
    // From orchestrators
    MsgOrchestratorComplete MessageType = iota
    MsgOrchestratorFailed

    // From watcher
    MsgCancelRun
    MsgScheduleUpdated
    MsgJobAdded
    MsgJobRemoved
    MsgJobModified

    // State queries
    MsgGetOrchestratorState
    MsgGetAllActiveRuns
    MsgGetJobStatus

    // Control
    MsgShutdown
)
```

### Specific Message Payloads
```go
type OrchestratorCompleteMsg struct {
    RunID       string
    Success     bool
    CompletedAt time.Time
    Error       error
}

type CancelRunMsg struct {
    RunID string
}

type ScheduleUpdatedMsg struct {
    // Empty - just signals rebuild needed
}

type JobModifiedMsg struct {
    JobID string
    Job   *Job
}

type GetOrchestratorStateMsg struct {
    RunID string
}

// Response sent via ResponseChan
type OrchestratorStateResponse struct {
    State *OrchestratorState
    Found bool
}
```

## Startup Sequence

### Initialization
```go
func NewScheduler(config SchedulerConfig, jobs []*Job) (*Scheduler, error) {
    // 1. Validate configuration
    if err := validateConfig(config); err != nil {
        return nil, err
    }

    // 2. Build job definitions map
    jobDefs := make(map[string]*Job)
    for _, job := range jobs {
        jobDefs[job.ID] = job
    }

    // 3. Build initial index
    now := time.Now()
    start := now.Add(-config.GracePeriod)
    end := now.Add(config.LookaheadWindow)
    runs := generateScheduledRuns(jobs, start, end)
    idx := index.NewScheduledRunIndex(runs)

    // 4. Initialize scheduler
    s := &Scheduler{
        config:              config,
        jobDefinitions:      jobDefs,
        index:               idx,
        activeOrchestrators: make(map[string]*OrchestratorState),
        lastRebuildTime:     now,
        inbox:               make(chan InboxMessage, config.InboxBufferSize),
        shutdown:            make(chan struct{}),
        jobRunSyncer:        make(chan JobRunUpdate, 100),
        statsSyncer:         make(chan StatsUpdate, 100),
        needsRebuild:        false,
    }

    return s, nil
}
```

### Starting the Scheduler
```go
func (s *Scheduler) Start() {
    // Launch syncer goroutines
    go s.runJobRunSyncer()
    go s.runStatsSyncer()

    // Start main loop
    s.run()
}
```

## Main Loop Implementation

### Loop Structure
```go
func (s *Scheduler) run() {
    ticker := time.NewTicker(s.config.LoopInterval)
    defer ticker.Stop()

    for {
        select {
        case <-s.shutdown:
            s.handleShutdown()
            return

        case <-ticker.C:
            s.iteration()
        }
    }
}
```

### Single Iteration
```go
func (s *Scheduler) iteration() {
    now := time.Now()

    // Step 1: Rebuild index if needed
    if s.shouldRebuildIndex(now) {
        s.rebuildIndex(now)
    }

    // Step 2: Schedule new orchestrators
    s.scheduleOrchestrators(now)

    // Step 3: Process ALL inbox messages
    s.processInbox()

    // Step 4: Clean up completed orchestrators
    s.cleanupOrchestrators(now)

    // Step 5: Update statistics
    s.updateStats(now)
}
```

## Detailed Step Implementations

### Step 1: Rebuild Index
```go
func (s *Scheduler) shouldRebuildIndex(now time.Time) bool {
    if s.needsRebuild {
        return true
    }
    return now.Sub(s.lastRebuildTime) >= s.config.IndexRebuildInterval
}

func (s *Scheduler) rebuildIndex(now time.Time) {
    // Calculate window
    start := now.Add(-s.config.GracePeriod)
    end := now.Add(s.config.LookaheadWindow)

    // Generate runs from all jobs (no I/O, uses in-memory jobDefinitions)
    jobs := make([]*Job, 0, len(s.jobDefinitions))
    for _, job := range s.jobDefinitions {
        jobs = append(jobs, job)
    }

    runs := generateScheduledRuns(jobs, start, end)

    // Atomically swap index
    s.index.Swap(runs)

    // Update state
    s.lastRebuildTime = now
    s.needsRebuild = false
}

func generateScheduledRuns(jobs []*Job, start, end time.Time) []index.ScheduledRun {
    runs := []index.ScheduledRun{}

    for _, job := range jobs {
        schedule, err := cron.Parse(job.Schedule)
        if err != nil {
            // Log error, skip this job
            continue
        }

        times := schedule.Between(start, end)
        for _, t := range times {
            runs = append(runs, index.ScheduledRun{
                JobID:       job.ID,
                ScheduledAt: t,
            })
        }
    }

    return runs
}
```

### Step 2: Schedule Orchestrators
```go
func (s *Scheduler) scheduleOrchestrators(now time.Time) {
    // Query for jobs to schedule in the next PRE_SCHEDULE_INTERVAL
    start := now
    end := now.Add(s.config.PreScheduleInterval)

    runs := s.index.Query(start, end)

    for _, run := range runs {
        // Generate unique runID
        runID := generateRunID(run.JobID, run.ScheduledAt)

        // Check if already scheduled
        if _, exists := s.activeOrchestrators[runID]; exists {
            continue
        }

        // Get job definition
        job, exists := s.jobDefinitions[run.JobID]
        if !exists {
            // Job was deleted, skip
            continue
        }

        // Create orchestrator state
        cancelChan := make(chan struct{})
        state := &OrchestratorState{
            RunID:       runID,
            JobID:       run.JobID,
            ScheduledAt: run.ScheduledAt,
            ActualStart: time.Time{}, // Not started yet
            Status:      OrchestratorPending,
            CancelChan:  cancelChan,
        }

        // Record in active orchestrators
        s.activeOrchestrators[runID] = state

        // Launch orchestrator goroutine
        go s.runOrchestrator(job, run.ScheduledAt, runID, cancelChan)

        // Send update to syncer
        s.jobRunSyncer <- JobRunUpdate{
            RunID:       runID,
            JobID:       run.JobID,
            ScheduledAt: run.ScheduledAt,
            Status:      "pending",
        }
    }
}

func generateRunID(jobID string, scheduledAt time.Time) string {
    // Format: jobID + timestamp in RFC3339Nano for uniqueness
    return fmt.Sprintf("%s_%s", jobID, scheduledAt.Format(time.RFC3339Nano))
}
```

### Step 3: Process Inbox
```go
func (s *Scheduler) processInbox() {
    // Process all available messages (non-blocking)
    for {
        select {
        case msg := <-s.inbox:
            s.handleMessage(msg)
        default:
            // No more messages, continue
            return
        }
    }
}

func (s *Scheduler) handleMessage(msg InboxMessage) {
    switch msg.Type {
    case MsgOrchestratorComplete:
        s.handleOrchestratorComplete(msg)
    case MsgOrchestratorFailed:
        s.handleOrchestratorFailed(msg)
    case MsgCancelRun:
        s.handleCancelRun(msg)
    case MsgScheduleUpdated:
        s.handleScheduleUpdated(msg)
    case MsgJobAdded:
        s.handleJobAdded(msg)
    case MsgJobRemoved:
        s.handleJobRemoved(msg)
    case MsgJobModified:
        s.handleJobModified(msg)
    case MsgGetOrchestratorState:
        s.handleGetOrchestratorState(msg)
    case MsgGetAllActiveRuns:
        s.handleGetAllActiveRuns(msg)
    case MsgShutdown:
        close(s.shutdown)
    }
}

func (s *Scheduler) handleOrchestratorComplete(msg InboxMessage) {
    data := msg.Data.(OrchestratorCompleteMsg)

    state, exists := s.activeOrchestrators[data.RunID]
    if !exists {
        return
    }

    // Update state
    state.Status = OrchestratorCompleted
    state.CompletedAt = data.CompletedAt

    // Send to syncer
    s.jobRunSyncer <- JobRunUpdate{
        RunID:       data.RunID,
        CompletedAt: data.CompletedAt,
        Status:      "completed",
        Success:     data.Success,
        Error:       data.Error,
    }
}

func (s *Scheduler) handleCancelRun(msg InboxMessage) {
    data := msg.Data.(CancelRunMsg)

    state, exists := s.activeOrchestrators[data.RunID]
    if !exists {
        return
    }

    // Signal cancellation to orchestrator
    close(state.CancelChan)
    state.Status = OrchestratorCancelled
}

func (s *Scheduler) handleScheduleUpdated(msg InboxMessage) {
    // Mark that we need to rebuild on next iteration
    s.needsRebuild = true
}

func (s *Scheduler) handleJobModified(msg InboxMessage) {
    data := msg.Data.(JobModifiedMsg)

    // Update in-memory job definition
    s.jobDefinitions[data.JobID] = data.Job

    // Mark for rebuild
    s.needsRebuild = true
}

func (s *Scheduler) handleGetOrchestratorState(msg InboxMessage) {
    data := msg.Data.(GetOrchestratorStateMsg)

    state, found := s.activeOrchestrators[data.RunID]

    response := OrchestratorStateResponse{
        State: state,
        Found: found,
    }

    // Send response back
    if msg.ResponseChan != nil {
        msg.ResponseChan <- response
    }
}
```

### Step 4: Cleanup Orchestrators
```go
func (s *Scheduler) cleanupOrchestrators(now time.Time) {
    toDelete := []string{}

    for runID, state := range s.activeOrchestrators {
        // Only clean up completed orchestrators
        if state.Status != OrchestratorCompleted &&
           state.Status != OrchestratorFailed &&
           state.Status != OrchestratorCancelled {
            continue
        }

        // Check if grace period has expired
        if now.After(state.ScheduledAt.Add(s.config.GracePeriod)) {
            toDelete = append(toDelete, runID)
        }
    }

    // Delete from map
    for _, runID := range toDelete {
        delete(s.activeOrchestrators, runID)
    }
}
```

### Step 5: Update Statistics
```go
func (s *Scheduler) updateStats(now time.Time) {
    s.stats.LastIterationTime = now
    s.stats.ActiveOrchestratorCount = len(s.activeOrchestrators)
    s.stats.IndexSize = s.index.Len()

    // Send to stats syncer (non-blocking)
    select {
    case s.statsSyncer <- StatsUpdate{
        Timestamp:               now,
        ActiveOrchestratorCount: s.stats.ActiveOrchestratorCount,
        IndexSize:               s.stats.IndexSize,
    }:
    default:
        // Syncer is backed up, skip this update
    }
}
```

## Orchestrator Stub

For now, the orchestrator is a placeholder that will be fully implemented later:

```go
func (s *Scheduler) runOrchestrator(job *Job, scheduledAt time.Time, runID string, cancelChan chan struct{}) {
    // Wait until scheduled time
    waitDuration := time.Until(scheduledAt)
    if waitDuration > 0 {
        select {
        case <-time.After(waitDuration):
            // Time to start
        case <-cancelChan:
            // Cancelled before start
            s.inbox <- InboxMessage{
                Type: MsgOrchestratorComplete,
                Data: OrchestratorCompleteMsg{
                    RunID:       runID,
                    Success:     false,
                    CompletedAt: time.Now(),
                    Error:       errors.New("cancelled"),
                },
            }
            return
        }
    }

    // TODO: Actual orchestrator implementation
    // For now, just simulate completion
    time.Sleep(100 * time.Millisecond)

    s.inbox <- InboxMessage{
        Type: MsgOrchestratorComplete,
        Data: OrchestratorCompleteMsg{
            RunID:       runID,
            Success:     true,
            CompletedAt: time.Now(),
            Error:       nil,
        },
    }
}
```

## Shutdown Handling

```go
func (s *Scheduler) handleShutdown() {
    // Cancel all active orchestrators
    for _, state := range s.activeOrchestrators {
        if state.Status == OrchestratorPending || state.Status == OrchestratorRunning {
            close(state.CancelChan)
        }
    }

    // Close syncer channels
    close(s.jobRunSyncer)
    close(s.statsSyncer)

    // Wait for syncers to finish (implementation detail)
    // ...
}

func (s *Scheduler) Shutdown() {
    s.inbox <- InboxMessage{
        Type: MsgShutdown,
    }
}
```

## Syncer Stubs

These will be fully implemented later but need basic structure:

```go
type JobRunUpdate struct {
    RunID       string
    JobID       string
    ScheduledAt time.Time
    CompletedAt time.Time
    Status      string
    Success     bool
    Error       error
}

type StatsUpdate struct {
    Timestamp               time.Time
    ActiveOrchestratorCount int
    IndexSize               int
}

func (s *Scheduler) runJobRunSyncer() {
    for update := range s.jobRunSyncer {
        // TODO: Write to database
        _ = update
    }
}

func (s *Scheduler) runStatsSyncer() {
    for update := range s.statsSyncer {
        // TODO: Write to database
        _ = update
    }
}
```

## Dependencies

### Internal
- `lib/cron` - Cron expression parsing
- `lib/scheduler/index` - Scheduled run index

### Standard Library
- `time` - Time operations
- `fmt` - String formatting
- `sync` - (only in syncers if needed)

### External
- None for core scheduler (syncers may need database library later)

## File Structure

```
lib/scheduler/
├── config.go           # Configuration types and defaults
├── scheduler.go        # Main Scheduler type and loop
├── messages.go         # Inbox message types
├── orchestrator.go     # Orchestrator state and stub implementation
├── syncer.go          # Syncer stub implementations
└── scheduler_test.go  # Tests (to be defined)
```

## Testing Strategy

Will be defined in `scheduler-tests.md` but key areas:
- Index rebuild timing and correctness
- Orchestrator scheduling and deduplication
- Message handling and state queries
- Cleanup timing (grace period)
- Shutdown and cancellation
- Concurrent access (race detector)

## Open Questions

1. **RunID generation**: Is `jobID_timestamp` sufficient or do we need UUIDs?
2. **Syncer error handling**: What happens if database writes fail?
3. **Job definition reloading**: Should we periodically reload from DB or only on explicit updates?
4. **Stats aggregation**: Should stats be per-iteration or aggregated over windows?
5. **Inbox overflow**: What happens if inbox fills up? Drop messages? Block senders?

## Future Enhancements

- Metrics/observability hooks
- Configurable orchestrator pool limits
- Advanced scheduling strategies (priority, backfill)
- Multi-scheduler coordination (HA setup)
