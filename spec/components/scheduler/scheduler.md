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
- Database reads → Index builder goroutine
- Database writes → Syncer goroutines
- Kubernetes operations → Orchestrator goroutines
- External API calls → Watcher goroutine
- Logging → slog (structured logging from stdlib)

### 3. Deterministic Iteration
Each loop iteration follows a fixed sequence of operations, making the system predictable and testable.

## Configuration

### Configuration Structure
```go
type SchedulerConfig struct {
    // How far ahead to launch orchestrators before job start time
    PreScheduleInterval time.Duration // Default: 10 seconds

    // How often the index builder queries DB and rebuilds index
    IndexRebuildInterval time.Duration // Default: 1 minute

    // How far ahead to calculate scheduled runs
    LookaheadWindow time.Duration // Default: 10 minutes

    // How far back to include runs (prevents missing near-past jobs)
    GracePeriod time.Duration // Default: 30 seconds

    // Main loop iteration interval
    LoopInterval time.Duration // Default: 1 second

    // Inbox buffer size
    InboxBufferSize int // Default: 10000

    // Timeout for sending to inbox
    InboxSendTimeout time.Duration // Default: 5 seconds

    // Orchestrator heartbeat configuration
    OrchestratorHeartbeatInterval time.Duration // Default: 10 seconds (how often orchestrators send heartbeats)
    MaxMissedOrchestratorHeartbeats           int           // Default: 3 (mark as orphaned after this many missed)
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
        PreScheduleInterval:           10 * time.Second,
        IndexRebuildInterval:          1 * time.Minute,
        LookaheadWindow:               10 * time.Minute,
        GracePeriod:                   30 * time.Second,
        LoopInterval:                  1 * time.Second,
        InboxBufferSize:               10000,
        InboxSendTimeout:              5 * time.Second,
        OrchestratorHeartbeatInterval: 10 * time.Second,
        MaxMissedOrchestratorHeartbeats:           3,
    }
}

func DefaultSyncerConfig() SyncerConfig {
    return SyncerConfig{
        MaxBufferedJobRunUpdates: 10000,
        JobRunChannelSize:        200,  // OLTP-friendly: smaller batches, more frequent flushes
        StatsChannelSize:         100,
        JobRunFlushThreshold:     100,  // OLTP-friendly: half of channel size, reduces lock contention
        JobRunFlushInterval:      1 * time.Second,
        StatsFlushThreshold:      30,   // 30 iterations (~30 seconds at 1s loop interval)
        StatsFlushInterval:       30 * time.Second,
    }
}
```

## Data Structures

### Inbox Type
```go
// Inbox provides a typed interface for the scheduler's message channel
type Inbox struct {
    ch      chan InboxMessage
    timeout time.Duration
    logger  *slog.Logger
    stats   *InboxStats
}

type InboxStats struct {
    TotalSent         int64
    TotalReceived     int64
    TimeoutCount      int64
    CurrentDepth      int
    MaxDepthSeen      int
}

func NewInbox(bufferSize int, timeout time.Duration, logger *slog.Logger) *Inbox {
    return &Inbox{
        ch:      make(chan InboxMessage, bufferSize),
        timeout: timeout,
        logger:  logger,
        stats:   &InboxStats{},
    }
}

// Send sends a message with timeout, logging on timeout
func (ib *Inbox) Send(msg InboxMessage) bool {
    select {
    case ib.ch <- msg:
        atomic.AddInt64(&ib.stats.TotalSent, 1)
        return true
    case <-time.After(ib.timeout):
        atomic.AddInt64(&ib.stats.TimeoutCount, 1)
        ib.logger.Warn("inbox send timeout",
            "msg_type", msg.Type,
            "timeout", ib.timeout,
            "current_depth", len(ib.ch))
        return false
    }
}

// TryReceive attempts to receive a message without blocking
func (ib *Inbox) TryReceive() (InboxMessage, bool) {
    select {
    case msg := <-ib.ch:
        atomic.AddInt64(&ib.stats.TotalReceived, 1)
        return msg, true
    default:
        return InboxMessage{}, false
    }
}

// Receive blocks until a message is available
func (ib *Inbox) Receive() InboxMessage {
    msg := <-ib.ch
    atomic.AddInt64(&ib.stats.TotalReceived, 1)
    return msg
}

// UpdateDepthStats updates depth statistics
func (ib *Inbox) UpdateDepthStats() {
    depth := len(ib.ch)
    ib.stats.CurrentDepth = depth
    if depth > ib.stats.MaxDepthSeen {
        ib.stats.MaxDepthSeen = depth
    }
}

// Stats returns current inbox statistics
func (ib *Inbox) Stats() InboxStats {
    return *ib.stats
}
```

### Syncer Type
```go
// Syncer handles all database write operations and buffering
type Syncer struct {
    // Configuration
    config SyncerConfig
    logger *slog.Logger

    // Job run update buffering
    jobRunUpdateBuffer  []JobRunUpdate
    jobRunChannel       chan JobRunUpdate
    jobRunFlushRequest  chan struct{} // Signal for manual flush
    lastJobRunFlush     time.Time

    // Stats buffering
    statsBuffer        []SchedulerIterationStats
    statsChannel       chan StatsUpdate
    statsFlushRequest  chan struct{} // Signal for manual flush
    lastStatsFlush     time.Time

    // Control
    shutdown chan struct{}
    wg       sync.WaitGroup // Tracks all background goroutines
}

type SyncerConfig struct {
    // Maximum buffered job run updates before stopping
    MaxBufferedJobRunUpdates int // Default: 10000

    // Channel buffer sizes
    JobRunChannelSize       int // Default: 200
    StatsChannelSize        int // Default: 100

    // Job run flushing - dual mechanism (size OR time triggers flush)
    JobRunFlushThreshold int           // Default: 100 (half of channel size)
    JobRunFlushInterval  time.Duration // Default: 1 second

    // Stats flushing - dual mechanism (size OR time triggers flush)
    StatsFlushThreshold int           // Default: 30 iterations
    StatsFlushInterval  time.Duration // Default: 30 seconds
}

func NewSyncer(config SyncerConfig, logger *slog.Logger) *Syncer {
    return &Syncer{
        config:             config,
        logger:             logger,
        jobRunUpdateBuffer: make([]JobRunUpdate, 0),
        jobRunChannel:      make(chan JobRunUpdate, config.JobRunChannelSize),
        jobRunFlushRequest: make(chan struct{}, 1),
        lastJobRunFlush:    time.Now(),
        statsBuffer:        make([]SchedulerIterationStats, 0),
        statsChannel:       make(chan StatsUpdate, config.StatsChannelSize),
        statsFlushRequest:  make(chan struct{}, 1),
        lastStatsFlush:     time.Now(),
        shutdown:           make(chan struct{}),
    }
}

// BufferJobRunUpdate adds an update to the buffer and signals flush if needed
// Flushing happens in two cases:
//   1. Size-based: When buffer reaches JobRunFlushThreshold
//   2. Time-based: After JobRunFlushInterval (handled by runJobRunFlusher)
func (s *Syncer) BufferJobRunUpdate(update JobRunUpdate) error {
    s.jobRunUpdateBuffer = append(s.jobRunUpdateBuffer, update)

    // Check if buffer exceeded maximum
    if len(s.jobRunUpdateBuffer) > s.config.MaxBufferedJobRunUpdates {
        return fmt.Errorf("job run update buffer exceeded maximum size: %d > %d",
            len(s.jobRunUpdateBuffer), s.config.MaxBufferedJobRunUpdates)
    }

    // Check if we should flush based on size
    if len(s.jobRunUpdateBuffer) >= s.config.JobRunFlushThreshold {
        s.requestFlush()
    }

    return nil
}

// requestFlush signals the flusher to flush (non-blocking)
func (s *Syncer) requestFlush() {
    select {
    case s.jobRunFlushRequest <- struct{}{}:
    default:
        // Already a flush pending
    }
}

// FlushJobRunUpdates sends all buffered updates to the syncer channel
func (s *Syncer) FlushJobRunUpdates() error {
    for _, update := range s.jobRunUpdateBuffer {
        select {
        case s.jobRunChannel <- update:
            // Successfully sent
        default:
            s.logger.Warn("job run channel full, keeping updates buffered",
                "buffered_count", len(s.jobRunUpdateBuffer))
            return fmt.Errorf("job run channel full")
        }
    }

    // All sent, clear buffer
    s.jobRunUpdateBuffer = make([]JobRunUpdate, 0)
    return nil
}

// BufferStats adds iteration stats to the buffer and signals flush if needed
// Flushing happens in two cases:
//   1. Size-based: When buffer reaches StatsFlushThreshold
//   2. Time-based: After StatsFlushInterval (handled by runStatsFlusher)
func (s *Syncer) BufferStats(stats SchedulerIterationStats) {
    s.statsBuffer = append(s.statsBuffer, stats)

    // Check if we should flush based on size
    if len(s.statsBuffer) >= s.config.StatsFlushThreshold {
        s.requestStatsFlush()
    }
}

// FlushStats sends all buffered stats to the syncer channel
func (s *Syncer) FlushStats() error {
    if len(s.statsBuffer) == 0 {
        return nil
    }

    update := StatsUpdate{
        Stats: s.statsBuffer,
    }

    select {
    case s.statsChannel <- update:
        s.statsBuffer = make([]SchedulerIterationStats, 0)
        s.logger.Debug("flushed stats", "count", len(update.Stats))
        return nil
    default:
        s.logger.Warn("stats channel full, keeping stats buffered",
            "buffered_count", len(s.statsBuffer))
        return fmt.Errorf("stats channel full")
    }
}

// requestStatsFlush signals the stats flusher to flush (non-blocking)
func (s *Syncer) requestStatsFlush() {
    select {
    case s.statsFlushRequest <- struct{}{}:
    default:
        // Already a flush pending
    }
}

// GetStats returns current syncer statistics
func (s *Syncer) GetStats() SyncerStats {
    return SyncerStats{
        BufferedJobRunUpdates: len(s.jobRunUpdateBuffer),
        BufferedStats:         len(s.statsBuffer),
    }
}

// Start launches background goroutines for syncing
func (s *Syncer) Start(db *sql.DB) {
    s.wg.Add(4) // 4 goroutines total

    go s.runJobRunFlusher()
    go s.runStatsFlusher()
    go s.runJobRunSyncer(db)
    go s.runStatsSyncer(db)
}

// runJobRunFlusher periodically flushes job run updates
func (s *Syncer) runJobRunFlusher() {
    defer s.wg.Done()

    ticker := time.NewTicker(s.config.JobRunFlushInterval)
    defer ticker.Stop()

    for {
        select {
        case <-s.shutdown:
            s.logger.Debug("job run flusher shutting down")
            return

        case <-ticker.C:
            // Periodic flush
            if len(s.jobRunUpdateBuffer) > 0 {
                if err := s.FlushJobRunUpdates(); err != nil {
                    // Error logged in FlushJobRunUpdates
                }
                s.lastJobRunFlush = time.Now()
            }

        case <-s.jobRunFlushRequest:
            // Size-based flush requested
            if err := s.FlushJobRunUpdates(); err != nil {
                // Error logged in FlushJobRunUpdates
            }
            s.lastJobRunFlush = time.Now()
        }
    }
}

// runStatsFlusher periodically flushes stats
func (s *Syncer) runStatsFlusher() {
    defer s.wg.Done()

    ticker := time.NewTicker(s.config.StatsFlushInterval)
    defer ticker.Stop()

    for {
        select {
        case <-s.shutdown:
            s.logger.Debug("stats flusher shutting down")
            return

        case <-ticker.C:
            // Periodic flush
            if len(s.statsBuffer) > 0 {
                if err := s.FlushStats(); err != nil {
                    // Error logged in FlushStats
                }
                s.lastStatsFlush = time.Now()
            }

        case <-s.statsFlushRequest:
            // Size-based flush requested
            if err := s.FlushStats(); err != nil {
                // Error logged in FlushStats
            }
            s.lastStatsFlush = time.Now()
        }
    }
}

// Shutdown performs graceful shutdown ensuring all data is persisted
func (s *Syncer) Shutdown() error {
    s.logger.Info("starting syncer shutdown")

    // Step 1: Signal shutdown to stop flushers
    // Flushers will exit their select loops
    close(s.shutdown)
    s.logger.Debug("shutdown signal sent, flushers stopping")

    // Step 2: Final flush of any remaining buffered items
    // This adds final items to channels before we close them
    s.logger.Debug("performing final flush",
        "job_run_updates", len(s.jobRunUpdateBuffer),
        "stats", len(s.statsBuffer))

    if err := s.FlushJobRunUpdates(); err != nil {
        s.logger.Warn("failed to flush job run updates on shutdown", "error", err)
    }

    if err := s.FlushStats(); err != nil {
        s.logger.Warn("failed to flush stats on shutdown", "error", err)
    }

    // Step 3: Close write channels to signal syncers "no more data"
    // The syncers use "for range" which will:
    // - Process all buffered items in the channel
    // - Exit when channel is closed and drained
    // This is Go's standard pattern for signaling completion
    s.logger.Debug("closing write channels")
    close(s.jobRunChannel)
    close(s.statsChannel)
    close(s.jobRunFlushRequest)
    close(s.statsFlushRequest)

    // Step 4: Wait for all goroutines to finish draining and exit
    // This ensures:
    // - Flushers have exited (stopped adding to buffers)
    // - Syncers have drained all items from channels
    // WaitGroup is ONLY used during shutdown for coordination
    s.logger.Debug("waiting for all goroutines to exit")
    s.wg.Wait()

    s.logger.Info("syncer shutdown complete")
    return nil
}

type SyncerStats struct {
    BufferedJobRunUpdates int
    BufferedStats         int
}
```

### Main Scheduler Type
```go
type Scheduler struct {
    // Configuration
    config SchedulerConfig
    logger *slog.Logger

    // State
    index               *index.ScheduledRunIndex
    activeOrchestrators map[string]*OrchestratorState // runID → state

    // Communication
    inbox  *Inbox
    syncer *Syncer

    // Control
    shutdown         chan struct{}
    rebuildIndexChan chan struct{}
}
```

### Orchestrator State
```go
type OrchestratorState struct {
    RunID            string
    JobID            string
    JobConfig        *Job // Current job configuration
    ScheduledAt      time.Time
    ActualStart      time.Time
    Status           OrchestratorStatus
    CancelChan       chan struct{}
    ConfigUpdate     chan *Job // For updating config while in PreRun
    CompletedAt      time.Time
    LastHeartbeat    time.Time // Last time heartbeat was received
    MissedHeartbeats int       // Consecutive missed heartbeats
}

type OrchestratorStatus int

const (
    // Pre-execution states
    OrchestratorPreRun OrchestratorStatus = iota // Created, waiting for start time
    OrchestratorPending                          // Initial pre-execution phase
    OrchestratorConditionPending                 // About to check pre-execution requirements
    OrchestratorConditionRunning                 // Checking pre-execution requirements
    OrchestratorActionPending                    // Requirement failed, about to take action
    OrchestratorActionRunning                    // Taking action based on requirement outcome

    // Execution states
    OrchestratorContainerCreating                // Creating Kubernetes pod/container
    OrchestratorRunning                          // Job executing
    OrchestratorTerminating                      // Job finishing/cleanup

    // Retry state
    OrchestratorRetrying                         // After failure, before retry

    // Terminal states
    OrchestratorCompleted                        // Completed successfully
    OrchestratorFailed                           // Failed
    OrchestratorCancelled                        // Cancelled
    OrchestratorOrphaned                         // No heartbeats, assumed dead
)

func (s OrchestratorStatus) String() string {
    switch s {
    case OrchestratorPreRun:
        return "prerun"
    case OrchestratorPending:
        return "pending"
    case OrchestratorConditionPending:
        return "condition_pending"
    case OrchestratorConditionRunning:
        return "condition_running"
    case OrchestratorActionPending:
        return "action_pending"
    case OrchestratorActionRunning:
        return "action_running"
    case OrchestratorContainerCreating:
        return "container_creating"
    case OrchestratorRunning:
        return "running"
    case OrchestratorTerminating:
        return "terminating"
    case OrchestratorRetrying:
        return "retrying"
    case OrchestratorCompleted:
        return "completed"
    case OrchestratorFailed:
        return "failed"
    case OrchestratorCancelled:
        return "cancelled"
    case OrchestratorOrphaned:
        return "orphaned"
    default:
        return "unknown"
    }
}
```

### Orchestrator Lifecycle

The orchestrator progresses through the following state transitions:

#### Normal Flow
1. **PreRun**: Waiting for scheduled time to arrive
2. **Pending**: Scheduled time reached, starting pre-execution phase
3. **ConditionPending**: About to check pre-execution requirements
4. **ConditionRunning**: Actively checking pre-execution requirements
5. **ActionPending**: Pre-execution requirement failed, about to take action
6. **ActionRunning**: Taking action based on failed requirement
7. **ContainerCreating**: Creating Kubernetes pod/container for job execution
8. **Running**: Job is actively executing
9. **Terminating**: Job finishing, performing cleanup
10. **Completed**: Job finished successfully (terminal)

#### Failure Flow
- From any non-terminal state → **Failed** (terminal)
- From **Failed** → **Retrying** (before attempting retry)
- From **Retrying** → back to **Pending** (retry attempt)

#### Cancellation Flow
- From any non-terminal state → **Cancelled** (terminal)

#### Orphaning Flow
- From any non-terminal state → **Orphaned** (terminal, no heartbeats received)

#### State Categories
- **Non-terminal states** (receive heartbeat monitoring): PreRun, Pending, ConditionPending, ConditionRunning, ActionPending, ActionRunning, ContainerCreating, Running, Terminating, Retrying
- **Terminal states** (eligible for cleanup after grace period): Completed, Failed, Cancelled, Orphaned

### Inbox Messages
```go
type InboxMessage struct {
    Type         MessageType
    Data         interface{}
    ResponseChan chan<- interface{} // Optional, for request/response
}

type MessageType int

const (
    // From orchestrators
    MsgOrchestratorStateChange MessageType = iota // Generic state change
    MsgOrchestratorComplete
    MsgOrchestratorFailed
    MsgOrchestratorHeartbeat // Periodic heartbeat from orchestrator

    // From watcher
    MsgCancelRun
    MsgUpdateRunConfig // Update config while in PreRun

    // State queries
    MsgGetOrchestratorState
    MsgGetAllActiveRuns
    MsgGetStats

    // Control
    MsgShutdown
)

func (m MessageType) String() string {
    switch m {
    case MsgOrchestratorStateChange:
        return "orchestrator_state_change"
    case MsgOrchestratorComplete:
        return "orchestrator_complete"
    case MsgOrchestratorFailed:
        return "orchestrator_failed"
    case MsgOrchestratorHeartbeat:
        return "orchestrator_heartbeat"
    case MsgCancelRun:
        return "cancel_run"
    case MsgUpdateRunConfig:
        return "update_run_config"
    case MsgGetOrchestratorState:
        return "get_orchestrator_state"
    case MsgGetAllActiveRuns:
        return "get_all_active_runs"
    case MsgGetStats:
        return "get_stats"
    case MsgShutdown:
        return "shutdown"
    default:
        return "unknown"
    }
}
```

### Specific Message Payloads
```go
type OrchestratorStateChangeMsg struct {
    RunID     string
    NewStatus OrchestratorStatus
    Timestamp time.Time
}

type OrchestratorCompleteMsg struct {
    RunID       string
    Success     bool
    CompletedAt time.Time
    Error       error
}

type OrchestratorHeartbeatMsg struct {
    RunID     string
    Timestamp time.Time
}

type CancelRunMsg struct {
    RunID string
}

type UpdateRunConfigMsg struct {
    RunID     string
    NewConfig *Job
}

type GetOrchestratorStateMsg struct {
    RunID string
}

type OrchestratorStateResponse struct {
    State *OrchestratorState
    Found bool
}

type GetAllActiveRunsMsg struct {
    // Empty
}

type AllActiveRunsResponse struct {
    Runs []*OrchestratorState
}

type GetStatsMsg struct {
    // Empty
}

type StatsResponse struct {
    SchedulerStats SchedulerStats
    InboxStats     InboxStats
    SyncerStats    SyncerStats
}
```

### Job Run Update Tracking
```go
type JobRunUpdate struct {
    UpdateID    string // UUID for idempotent database writes
    RunID       string // Deterministic format: "jobID:unixTimestamp"
    JobID       string
    ScheduledAt time.Time
    CompletedAt time.Time
    Status      string
    Success     bool
    Error       error
}
```

### Statistics
```go
type SchedulerIterationStats struct {
    Timestamp               time.Time
    IterationDuration       time.Duration
    ActiveOrchestratorCount int
    IndexSize               int
    InboxDepth              int
    MessagesProcessed       int
}

type SchedulerStats struct {
    ActiveOrchestratorCount int
    IndexSize               int
}

type StatsUpdate struct {
    Stats []SchedulerIterationStats
}
```

## Background Goroutines

### Index Builder
The index builder runs independently and rebuilds the index periodically.

```go
func (s *Scheduler) runIndexBuilder(db *sql.DB) {
    ticker := time.NewTicker(s.config.IndexRebuildInterval)
    defer ticker.Stop()

    for {
        select {
        case <-s.shutdown:
            return

        case <-ticker.C:
            s.performIndexRebuild(db)

        case <-s.rebuildIndexChan:
            // Explicit rebuild requested (e.g., schedule change)
            s.performIndexRebuild(db)
        }
    }
}

func (s *Scheduler) performIndexRebuild(db *sql.DB) {
    now := time.Now()
    start := now.Add(-s.config.GracePeriod)
    end := now.Add(s.config.LookaheadWindow)

    s.logger.Info("starting index rebuild",
        "start", start,
        "end", end)

    // 1. Query database for all job definitions
    jobs, err := queryJobDefinitions(db)
    if err != nil {
        s.logger.Error("failed to query job definitions",
            "error", err)
        return
    }

    // 2. Generate scheduled runs
    runs := []index.ScheduledRun{}
    for _, job := range jobs {
        schedule, err := cron.Parse(job.Schedule)
        if err != nil {
            s.logger.Error("failed to parse cron schedule",
                "job_id", job.ID,
                "schedule", job.Schedule,
                "error", err)
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

    // 3. Atomically swap index (lock-free!)
    s.index.Swap(runs)

    s.logger.Info("index rebuild complete",
        "run_count", len(runs),
        "job_count", len(jobs),
        "duration", time.Since(now))
}

func queryJobDefinitions(db *sql.DB) ([]*Job, error) {
    // TODO: Actual DB query
    // For now, return empty slice
    return []*Job{}, nil
}
```

### Syncer Background Goroutines
The syncer manages its own background goroutines for database writes.

```go
// runJobRunSyncer writes job run updates to the database
func (s *Syncer) runJobRunSyncer(db *sql.DB) {
    defer s.wg.Done()

    for update := range s.jobRunChannel {
        err := writeJobRunUpdate(db, update)

        if err != nil {
            s.logger.Error("failed to write job run update",
                "update_id", update.UpdateID,
                "run_id", update.RunID,
                "error", err)
        } else {
            s.logger.Debug("wrote job run update",
                "update_id", update.UpdateID,
                "run_id", update.RunID)
        }
    }

    s.logger.Debug("job run syncer shut down")
}

// runStatsSyncer writes stats updates to the database
func (s *Syncer) runStatsSyncer(db *sql.DB) {
    defer s.wg.Done()

    for update := range s.statsChannel {
        err := writeStatsUpdate(db, update)
        if err != nil {
            // Just log, stats writes are not critical
            s.logger.Error("failed to write stats update", "error", err)
        }
    }

    s.logger.Debug("stats syncer shut down")
}

// Helper functions for database writes
func writeJobRunUpdate(db *sql.DB, update JobRunUpdate) error {
    // TODO: Actual database write
    return nil
}

func writeStatsUpdate(db *sql.DB, update StatsUpdate) error {
    // TODO: Actual database write
    return nil
}
```

## Startup Sequence

### Initialization
```go
func NewScheduler(config SchedulerConfig, syncerConfig SyncerConfig, db *sql.DB, logger *slog.Logger) (*Scheduler, error) {
    // 1. Validate configuration
    if err := validateConfig(config); err != nil {
        return nil, err
    }

    // 2. Create inbox
    inbox := NewInbox(config.InboxBufferSize, config.InboxSendTimeout, logger)

    // 3. Create syncer
    syncer := NewSyncer(syncerConfig, logger)

    // 4. Build initial index (synchronously on startup)
    now := time.Now()
    start := now.Add(-config.GracePeriod)
    end := now.Add(config.LookaheadWindow)

    jobs, err := queryJobDefinitions(db)
    if err != nil {
        return nil, fmt.Errorf("failed to query jobs on startup: %w", err)
    }

    runs := []index.ScheduledRun{}
    for _, job := range jobs {
        schedule, err := cron.Parse(job.Schedule)
        if err != nil {
            logger.Error("failed to parse cron schedule on startup",
                "job_id", job.ID,
                "schedule", job.Schedule,
                "error", err)
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

    idx := index.NewScheduledRunIndex(runs)

    // 5. Initialize scheduler
    s := &Scheduler{
        config:              config,
        logger:              logger,
        index:               idx,
        activeOrchestrators: make(map[string]*OrchestratorState),
        inbox:               inbox,
        syncer:              syncer,
        shutdown:            make(chan struct{}),
        rebuildIndexChan:    make(chan struct{}, 1),
    }

    return s, nil
}
```

### Starting the Scheduler
```go
func (s *Scheduler) Start(db *sql.DB) {
    s.logger.Info("starting scheduler")

    // Start syncer background goroutines
    s.syncer.Start(db)

    // Start index builder
    go s.runIndexBuilder(db)

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
    start := time.Now()

    // Step 1: Schedule new orchestrators
    if err := s.scheduleOrchestrators(start); err != nil {
        s.logger.Error("scheduler stopping due to buffer overflow", "error", err)
        close(s.shutdown)
        return
    }

    // Step 2: Process ALL inbox messages
    messagesProcessed := s.processInbox()

    // Step 3: Check for missed heartbeats and mark orphaned
    s.checkHeartbeats(start)

    // Step 4: Clean up completed orchestrators
    s.cleanupOrchestrators(start)

    // Step 5: Record iteration statistics
    // Stats are automatically flushed by syncer based on size/time
    s.recordIterationStats(start, messagesProcessed)

    // Step 6: Update inbox depth stats
    s.inbox.UpdateDepthStats()
}
```

## Detailed Step Implementations

### Step 1: Schedule Orchestrators
```go
func (s *Scheduler) scheduleOrchestrators(now time.Time) error {
    // Query for jobs to schedule in the next PRE_SCHEDULE_INTERVAL
    start := now
    end := now.Add(s.config.PreScheduleInterval)

    runs := s.index.Query(start, end)

    for _, run := range runs {
        // Generate deterministic runID from jobID and scheduled time
        // Format: "jobID:unixTimestamp" (e.g., "job123:1704067200")
        runID := generateRunID(run.JobID, run.ScheduledAt)

        // Check if already scheduled - O(1) map lookup
        if _, exists := s.activeOrchestrators[runID]; exists {
            continue
        }

        // Note: We don't have job config here - orchestrator will request it
        // or we need to maintain a job cache in the scheduler
        // For now, create a placeholder

        // Create orchestrator state
        cancelChan := make(chan struct{})
        configUpdateChan := make(chan *Job, 1)

        state := &OrchestratorState{
            RunID:            runID,
            JobID:            run.JobID,
            JobConfig:        nil, // Will be set when orchestrator starts
            ScheduledAt:      run.ScheduledAt,
            ActualStart:      time.Time{},
            Status:           OrchestratorPreRun,
            CancelChan:       cancelChan,
            ConfigUpdate:     configUpdateChan,
            LastHeartbeat:    now, // Initialize to current time
            MissedHeartbeats: 0,
        }

        // Record in active orchestrators
        s.activeOrchestrators[runID] = state

        // Launch orchestrator goroutine
        go s.runOrchestrator(run.JobID, run.ScheduledAt, runID, cancelChan, configUpdateChan)

        // Buffer job run update
        if err := s.syncer.BufferJobRunUpdate(JobRunUpdate{
            UpdateID:    generateUpdateID(),
            RunID:       runID,
            JobID:       run.JobID,
            ScheduledAt: run.ScheduledAt,
            Status:      OrchestratorPreRun.String(),
        }); err != nil {
            return err
        }
    }

    return nil
}

func generateRunID(jobID string, scheduledAt time.Time) string {
    // Deterministic runID: jobID:unixTimestamp
    // Same job at same time always produces same runID
    return fmt.Sprintf("%s:%d", jobID, scheduledAt.Unix())
}

func generateUpdateID() string {
    // Use UUID for update deduplication in database
    return uuid.New().String()
}
```

### Step 2: Process Inbox
```go
func (s *Scheduler) processInbox() int {
    messagesProcessed := 0

    // Process all available messages (non-blocking)
    for {
        msg, ok := s.inbox.TryReceive()
        if !ok {
            break
        }

        s.handleMessage(msg)
        messagesProcessed++
    }

    return messagesProcessed
}

func (s *Scheduler) handleMessage(msg InboxMessage) {
    s.logger.Debug("handling message", "type", msg.Type.String())

    switch msg.Type {
    case MsgOrchestratorStateChange:
        s.handleOrchestratorStateChange(msg)
    case MsgOrchestratorComplete:
        s.handleOrchestratorComplete(msg)
    case MsgOrchestratorFailed:
        s.handleOrchestratorFailed(msg)
    case MsgOrchestratorHeartbeat:
        s.handleOrchestratorHeartbeat(msg)
    case MsgCancelRun:
        s.handleCancelRun(msg)
    case MsgUpdateRunConfig:
        s.handleUpdateRunConfig(msg)
    case MsgGetOrchestratorState:
        s.handleGetOrchestratorState(msg)
    case MsgGetAllActiveRuns:
        s.handleGetAllActiveRuns(msg)
    case MsgGetStats:
        s.handleGetStats(msg)
    case MsgShutdown:
        close(s.shutdown)
    default:
        s.logger.Warn("unknown message type", "type", msg.Type)
    }
}

func (s *Scheduler) handleOrchestratorHeartbeat(msg InboxMessage) {
    data := msg.Data.(OrchestratorHeartbeatMsg)

    state, exists := s.activeOrchestrators[data.RunID]
    if !exists {
        return
    }

    // Update heartbeat tracking
    state.LastHeartbeat = data.Timestamp
    state.MissedHeartbeats = 0 // Reset counter on successful heartbeat
}

func (s *Scheduler) handleOrchestratorStateChange(msg InboxMessage) {
    data := msg.Data.(OrchestratorStateChangeMsg)

    state, exists := s.activeOrchestrators[data.RunID]
    if !exists {
        return
    }

    state.Status = data.NewStatus

    // Buffer update (ignore error - will be caught in scheduleOrchestrators)
    s.syncer.BufferJobRunUpdate(JobRunUpdate{
        UpdateID: generateUpdateID(),
        RunID:    data.RunID,
        Status:   data.NewStatus.String(),
    })
}

func (s *Scheduler) handleOrchestratorComplete(msg InboxMessage) {
    data := msg.Data.(OrchestratorCompleteMsg)

    state, exists := s.activeOrchestrators[data.RunID]
    if !exists {
        return
    }

    state.Status = OrchestratorCompleted
    state.CompletedAt = data.CompletedAt

    // Buffer update (ignore error - will be caught in scheduleOrchestrators)
    s.syncer.BufferJobRunUpdate(JobRunUpdate{
        UpdateID:    generateUpdateID(),
        RunID:       data.RunID,
        CompletedAt: data.CompletedAt,
        Status:      OrchestratorCompleted.String(),
        Success:     data.Success,
        Error:       data.Error,
    })
}

func (s *Scheduler) handleOrchestratorFailed(msg InboxMessage) {
    data := msg.Data.(OrchestratorCompleteMsg)

    state, exists := s.activeOrchestrators[data.RunID]
    if !exists {
        return
    }

    state.Status = OrchestratorFailed
    state.CompletedAt = data.CompletedAt

    // Buffer update (ignore error - will be caught in scheduleOrchestrators)
    s.syncer.BufferJobRunUpdate(JobRunUpdate{
        UpdateID:    generateUpdateID(),
        RunID:       data.RunID,
        CompletedAt: data.CompletedAt,
        Status:      OrchestratorFailed.String(),
        Success:     false,
        Error:       data.Error,
    })
}

func (s *Scheduler) handleCancelRun(msg InboxMessage) {
    data := msg.Data.(CancelRunMsg)

    state, exists := s.activeOrchestrators[data.RunID]
    if !exists {
        s.logger.Warn("attempted to cancel non-existent run", "run_id", data.RunID)
        return
    }

    // Signal cancellation to orchestrator
    close(state.CancelChan)
    state.Status = OrchestratorCancelled

    s.logger.Info("cancelled run", "run_id", data.RunID)
}

func (s *Scheduler) handleUpdateRunConfig(msg InboxMessage) {
    data := msg.Data.(UpdateRunConfigMsg)

    state, exists := s.activeOrchestrators[data.RunID]
    if !exists {
        s.logger.Warn("attempted to update config for non-existent run",
            "run_id", data.RunID)
        return
    }

    // Can only update config in PreRun state
    if state.Status != OrchestratorPreRun {
        s.logger.Warn("attempted to update config for run not in PreRun state",
            "run_id", data.RunID,
            "status", state.Status.String())
        return
    }

    // Send new config to orchestrator
    select {
    case state.ConfigUpdate <- data.NewConfig:
        state.JobConfig = data.NewConfig
        s.logger.Info("updated run config",
            "run_id", data.RunID,
            "job_id", data.NewConfig.ID)
    default:
        s.logger.Warn("failed to send config update to orchestrator",
            "run_id", data.RunID)
    }
}

func (s *Scheduler) handleGetOrchestratorState(msg InboxMessage) {
    data := msg.Data.(GetOrchestratorStateMsg)

    state, found := s.activeOrchestrators[data.RunID]

    response := OrchestratorStateResponse{
        State: state,
        Found: found,
    }

    if msg.ResponseChan != nil {
        msg.ResponseChan <- response
    }
}

func (s *Scheduler) handleGetAllActiveRuns(msg InboxMessage) {
    runs := make([]*OrchestratorState, 0, len(s.activeOrchestrators))
    for _, state := range s.activeOrchestrators {
        runs = append(runs, state)
    }

    response := AllActiveRunsResponse{
        Runs: runs,
    }

    if msg.ResponseChan != nil {
        msg.ResponseChan <- response
    }
}

func (s *Scheduler) handleGetStats(msg InboxMessage) {
    response := StatsResponse{
        SchedulerStats: SchedulerStats{
            ActiveOrchestratorCount: len(s.activeOrchestrators),
            IndexSize:               s.index.Len(),
        },
        InboxStats:  s.inbox.Stats(),
        SyncerStats: s.syncer.GetStats(),
    }

    if msg.ResponseChan != nil {
        msg.ResponseChan <- response
    }
}
```

### Step 3: Check Heartbeats
```go
func (s *Scheduler) checkHeartbeats(now time.Time) {
    for runID, state := range s.activeOrchestrators {
        // Only check heartbeats for non-terminal states
        // Skip terminal states: Completed, Failed, Cancelled, Orphaned
        if state.Status == OrchestratorCompleted ||
           state.Status == OrchestratorFailed ||
           state.Status == OrchestratorCancelled ||
           state.Status == OrchestratorOrphaned {
            continue
        }

        // Check if heartbeat is overdue
        timeSinceLastHeartbeat := now.Sub(state.LastHeartbeat)
        expectedInterval := s.config.OrchestratorHeartbeatInterval

        if timeSinceLastHeartbeat > expectedInterval {
            // Increment missed heartbeat counter
            state.MissedHeartbeats++

            s.logger.Warn("missed heartbeat",
                "run_id", runID,
                "job_id", state.JobID,
                "missed_count", state.MissedHeartbeats,
                "time_since_last", timeSinceLastHeartbeat)

            // Check if we should mark as orphaned
            if state.MissedHeartbeats >= s.config.MaxMissedOrchestratorHeartbeats {
                s.logger.Error("marking orchestrator as orphaned",
                    "run_id", runID,
                    "job_id", state.JobID,
                    "missed_count", state.MissedHeartbeats)

                state.Status = OrchestratorOrphaned
                state.CompletedAt = now

                // Buffer update
                s.syncer.BufferJobRunUpdate(JobRunUpdate{
                    UpdateID:    generateUpdateID(),
                    RunID:       runID,
                    JobID:       state.JobID,
                    CompletedAt: now,
                    Status:      OrchestratorOrphaned.String(),
                    Success:     false,
                    Error:       fmt.Errorf("orchestrator orphaned after %d missed heartbeats", state.MissedHeartbeats),
                })
            }
        }
    }
}
```

### Step 4: Cleanup Orchestrators
```go
func (s *Scheduler) cleanupOrchestrators(now time.Time) {
    toDelete := []string{}

    for runID, state := range s.activeOrchestrators {
        // Only clean up terminal state orchestrators
        if state.Status != OrchestratorCompleted &&
           state.Status != OrchestratorFailed &&
           state.Status != OrchestratorCancelled &&
           state.Status != OrchestratorOrphaned {
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

    if len(toDelete) > 0 {
        s.logger.Debug("cleaned up orchestrators", "count", len(toDelete))
    }
}
```

### Step 5: Record Iteration Stats
```go
func (s *Scheduler) recordIterationStats(start time.Time, messagesProcessed int) {
    stats := SchedulerIterationStats{
        Timestamp:               start,
        IterationDuration:       time.Since(start),
        ActiveOrchestratorCount: len(s.activeOrchestrators),
        IndexSize:               s.index.Len(),
        InboxDepth:              s.inbox.Stats().CurrentDepth,
        MessagesProcessed:       messagesProcessed,
    }

    s.syncer.BufferStats(stats)
}
```

## Orchestrator Stub

```go
func (s *Scheduler) runOrchestrator(jobID string, scheduledAt time.Time, runID string,
                                    cancelChan chan struct{}, configUpdate chan *Job) {

    // Start heartbeat sender in background
    heartbeatDone := make(chan struct{})
    defer close(heartbeatDone)
    go s.sendHeartbeats(runID, heartbeatDone)

    // PreRun state: wait for scheduled time or config updates
    waitDuration := time.Until(scheduledAt)
    if waitDuration > 0 {
        for {
            select {
            case <-time.After(waitDuration):
                // Time to start
                goto START

            case newConfig := <-configUpdate:
                // Config updated while waiting
                s.logger.Info("received config update in PreRun",
                    "run_id", runID,
                    "job_id", newConfig.ID)
                // Continue waiting with updated config

            case <-cancelChan:
                // Cancelled before start
                s.inbox.Send(InboxMessage{
                    Type: MsgOrchestratorComplete,
                    Data: OrchestratorCompleteMsg{
                        RunID:       runID,
                        Success:     false,
                        CompletedAt: time.Now(),
                        Error:       errors.New("cancelled in PreRun"),
                    },
                })
                return
            }
        }
    }

START:
    // Notify state change to Pending
    s.inbox.Send(InboxMessage{
        Type: MsgOrchestratorStateChange,
        Data: OrchestratorStateChangeMsg{
            RunID:     runID,
            NewStatus: OrchestratorPending,
            Timestamp: time.Now(),
        },
    })

    // TODO: Actual orchestrator implementation
    // For now, just simulate completion
    time.Sleep(100 * time.Millisecond)

    s.inbox.Send(InboxMessage{
        Type: MsgOrchestratorComplete,
        Data: OrchestratorCompleteMsg{
            RunID:       runID,
            Success:     true,
            CompletedAt: time.Now(),
            Error:       nil,
        },
    })
}

// sendHeartbeats sends periodic heartbeat messages to the main loop
func (s *Scheduler) sendHeartbeats(runID string, done chan struct{}) {
    ticker := time.NewTicker(s.config.OrchestratorHeartbeatInterval)
    defer ticker.Stop()

    for {
        select {
        case <-done:
            // Orchestrator finished, stop sending heartbeats
            return

        case <-ticker.C:
            // Send heartbeat
            s.inbox.Send(InboxMessage{
                Type: MsgOrchestratorHeartbeat,
                Data: OrchestratorHeartbeatMsg{
                    RunID:     runID,
                    Timestamp: time.Now(),
                },
            })
        }
    }
}
```

## Shutdown Handling

```go
func (s *Scheduler) handleShutdown() {
    s.logger.Info("shutting down scheduler")

    // Cancel all active orchestrators
    for runID, state := range s.activeOrchestrators {
        if state.Status == OrchestratorPreRun ||
           state.Status == OrchestratorPending ||
           state.Status == OrchestratorRunning {
            close(state.CancelChan)
            s.logger.Debug("cancelled orchestrator", "run_id", runID)
        }
    }

    // Shutdown syncer (flushes and closes channels)
    if err := s.syncer.Shutdown(); err != nil {
        s.logger.Error("error shutting down syncer", "error", err)
    }

    // Close index builder channel
    close(s.rebuildIndexChan)

    s.logger.Info("scheduler shutdown complete")
}

func (s *Scheduler) Shutdown() {
    s.inbox.Send(InboxMessage{
        Type: MsgShutdown,
    })
}
```

## Dependencies

### Internal
- `internal/cron` - Cron expression parsing
- `internal/scheduler/index` - Scheduled run index
- `internal/testutil` - Test utilities and mocks

### Standard Library
- `time` - Time operations
- `fmt` - String formatting
- `log/slog` - Structured logging
- `database/sql` - Database interface
- `sync/atomic` - Atomic operations for stats
- `sync` - WaitGroup for coordinating goroutine shutdown

### External
- `github.com/google/uuid` - UUID generation for updateIDs (idempotent database writes)

## File Structure

```
internal/scheduler/
├── config.go              # Configuration types and defaults (SchedulerConfig, SyncerConfig)
├── inbox.go               # Inbox type with helper methods and stats
├── syncer.go              # Syncer type for database writes and buffering
├── scheduler.go           # Main Scheduler type and loop
├── messages.go            # Inbox message types and payloads
├── orchestrator.go        # Orchestrator state and stub implementation
├── indexbuilder.go        # Index builder background goroutine
├── config_test.go         # Configuration validation tests
├── inbox_test.go          # Inbox operations tests
├── syncer_test.go         # Syncer buffering/flushing tests
├── scheduler_test.go      # Main scheduler loop tests
├── orchestrator_test.go   # Orchestrator lifecycle tests
├── integration_test.go    # End-to-end integration tests
└── scheduler_bench_test.go # Performance benchmarks
```

## Testing Strategy

Will be defined in `scheduler-tests.md` but key areas:
- Index rebuild via background goroutine
- Orchestrator scheduling and deduplication
- Message handling and state queries
- Cleanup timing (grace period)
- Shutdown and cancellation
- Config updates in PreRun state
- Heartbeat monitoring
  - Heartbeat tracking and reset
  - Missed heartbeat detection
  - Orphaned orchestrator marking
  - Cleanup of orphaned orchestrators
- Job run update buffering and flushing
  - Size-based flushing (JobRunFlushThreshold reached)
  - Time-based flushing (JobRunFlushInterval elapsed)
  - Dual trigger scenarios
  - Write failure logging
- Stats buffering and flushing
  - Size-based flushing (StatsFlushThreshold reached)
  - Time-based flushing (StatsFlushInterval elapsed)
  - Dual trigger scenarios
- Inbox timeout and depth tracking
- Concurrent access (race detector)

## Resolved Design Questions

1. **RunID generation**: Deterministic format `jobID:unixTimestamp`. Same job at same time always produces same runID, enabling O(1) duplicate prevention via map lookup.
2. **UpdateID generation**: Using UUIDs via `github.com/google/uuid` for idempotent database writes.
3. **Syncer error handling**: Buffered writes with logging. Main loop buffers updates and flushes periodically. Write failures are logged but not retried. If buffer exceeds maximum, scheduler stops.
4. **Job definition reloading**: Index builder goroutine queries DB on `IndexRebuildInterval` cadence
5. **Stats aggregation**: Recorded per-iteration, buffered, synced on `STATS_PERIOD`
6. **Inbox overflow**: Senders block with timeout. Timeouts logged and tracked in stats. Inbox depth is configurable with large default (10,000).
7. **Orchestrator monitoring**: Heartbeats sent on `OrchestratorHeartbeatInterval`. Orchestrators marked as orphaned after `MaxMissedOrchestratorHeartbeats` consecutive misses.

## Future Enhancements

- Metrics/observability hooks (Prometheus, etc.)
- Configurable orchestrator pool limits
- Advanced scheduling strategies (priority, backfill)
- Multi-scheduler coordination (HA setup)
- Dynamic configuration reloading
