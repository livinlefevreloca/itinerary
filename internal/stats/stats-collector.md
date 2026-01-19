# Stats Collector Implementation Specification

## Overview
The Stats Collector is a standalone component that centralizes all statistics collection and database writing for the Itinerary scheduler. It receives statistics from all other components via an inbox and handles all intermediate calculations and database persistence.

## Package Location
`internal/stats`

## Core Types

### StatsCollector
```go
type StatsCollector struct {
    db              *sql.DB
    inbox           *inbox.Inbox[StatsMessage]
    config          Config
    logger          *slog.Logger

    // Current stats period tracking
    currentPeriod   int64
    periodStartTime time.Time

    // Accumulators for current period
    schedulerStats      *SchedulerStatsAccumulator
    orchestratorStats   map[string]*OrchestratorStats // keyed by run_id
    syncerStats         *SyncerStatsAccumulator
    webhookStats        *WebhookStatsAccumulator

    // Flush timer
    flushTicker *time.Ticker

    // Shutdown coordination
    done chan struct{}
    wg   sync.WaitGroup
}
```

### StatsMessage
```go
type StatsMessage struct {
    Source      StatsSource
    Timestamp   time.Time
    Data        interface{} // Actual type depends on Source
}

type StatsSource int

const (
    StatsSourceScheduler StatsSource = iota
    StatsSourceOrchestrator
    StatsSourceSyncer
    StatsSourceWebhookHandler
)
```

### Stats Data Types

#### Scheduler Stats
```go
type SchedulerStatsData struct {
    Iterations          int
    JobsRun             int
    LateJobs            int
    MissedJobs          int
    JobsCancelled       int
    InboxLength         int
    InboxEmptyTime      time.Duration
    MessageWaitTime     time.Duration // Time message spent in inbox
}
```

#### Orchestrator Stats
```go
type OrchestratorStatsData struct {
    RunID              string
    Runtime            time.Duration
    ConstraintsChecked int
    ActionsTaken       int
}
```

#### Syncer Stats
```go
type SyncerStatsData struct {
    TotalWrites       int
    WritesSucceeded   int
    WritesFailed      int
    WritesInFlight    int
    QueuedWrites      int
    InboxLength       int
    TimeInQueue       time.Duration
    TimeInInbox       time.Duration
}
```

#### Webhook Stats (future)
```go
type WebhookStatsData struct {
    WebhooksSent      int
    WebhooksSucceeded int
    WebhooksFailed    int
    TotalRetries      int
    DeliveryTime      time.Duration
    InboxLength       int
}
```

## Configuration

```go
type Config struct {
    // Inbox configuration
    InboxBufferSize   int           // Default: 1000
    InboxSendTimeout  time.Duration // Default: 5s

    // Flush configuration
    FlushInterval     time.Duration // Default: 30s
    FlushThreshold    int           // Default: 100 stats messages

    // Stats period configuration
    StatsPeriodDuration time.Duration // Default: 30s
}

func DefaultConfig() Config {
    return Config{
        InboxBufferSize:     1000,
        InboxSendTimeout:    5 * time.Second,
        FlushInterval:       30 * time.Second,
        FlushThreshold:      100,
        StatsPeriodDuration: 30 * time.Second,
    }
}
```

## Core Methods

### Initialization
```go
// NewStatsCollector creates a new stats collector
func NewStatsCollector(db *sql.DB, config Config, logger *slog.Logger) *StatsCollector

// Start begins the stats collection loop
func (sc *StatsCollector) Start() error

// Stop gracefully shuts down the stats collector
func (sc *StatsCollector) Stop() error

// Send sends a stats message to the collector (non-blocking with timeout)
func (sc *StatsCollector) Send(msg StatsMessage) bool
```

### Main Loop
```go
// run is the main loop that processes stats messages
func (sc *StatsCollector) run()
```

The main loop:
1. Checks if stats period has expired
2. If period expired, flushes current period and starts new one
3. Receives messages from inbox (blocking)
4. Routes message to appropriate accumulator based on Source
5. Checks if flush threshold reached
6. If threshold reached, flushes to database
7. Handles shutdown signal

## Accumulators

### Scheduler Stats Accumulator
```go
type SchedulerStatsAccumulator struct {
    Iterations       int
    JobsRun          int
    LateJobs         int
    MissedJobs       int
    JobsCancelled    int

    // Inbox metrics
    InboxLengthSamples []int
    EmptyInboxTime     time.Duration
    MessageWaitTimes   []time.Duration
}

func (acc *SchedulerStatsAccumulator) Add(data SchedulerStatsData)
func (acc *SchedulerStatsAccumulator) Flush(db *sql.DB, periodID int64) error
func (acc *SchedulerStatsAccumulator) Reset()
```

Flush calculates:
- Min/Max/Avg inbox length from samples
- Total empty inbox time
- Min/Max/Avg message wait time

### Orchestrator Stats Accumulator
Orchestrator stats are stored per run_id and flushed individually:
```go
type OrchestratorStats struct {
    RunID              string
    Runtime            time.Duration
    ConstraintsChecked int
    ActionsTaken       int
}

func flushOrchestratorStats(db *sql.DB, periodID int64, stats map[string]*OrchestratorStats) error
```

### Syncer Stats Accumulator
```go
type SyncerStatsAccumulator struct {
    TotalWrites     int
    WritesSucceeded int
    WritesFailed    int

    // Samples for min/max/avg
    WritesInFlightSamples []int
    QueuedWritesSamples   []int
    InboxLengthSamples    []int
    TimeInQueueSamples    []time.Duration
    TimeInInboxSamples    []time.Duration
}

func (acc *SyncerStatsAccumulator) Add(data SyncerStatsData)
func (acc *SyncerStatsAccumulator) Flush(db *sql.DB, periodID int64) error
func (acc *SyncerStatsAccumulator) Reset()
```

### Webhook Stats Accumulator (future)
```go
type WebhookStatsAccumulator struct {
    WebhooksSent      int
    WebhooksSucceeded int
    WebhooksFailed    int
    TotalRetries      int

    // Samples for min/max/avg
    DeliveryTimeSamples []time.Duration
    InboxLengthSamples  []int
}

func (acc *WebhookStatsAccumulator) Add(data WebhookStatsData)
func (acc *WebhookStatsAccumulator) Flush(db *sql.DB, periodID int64) error
func (acc *WebhookStatsAccumulator) Reset()
```

## Database Operations

### Stats Period Management
```go
// getCurrentPeriodID returns the stats period ID for the current time
func (sc *StatsCollector) getCurrentPeriodID() int64

// startNewPeriod flushes current period and starts a new one
func (sc *StatsCollector) startNewPeriod() error
```

Stats periods are calculated as:
```go
periodID = timestamp / statsPeriodDuration
```

This ensures all components use the same period IDs for the same time windows.

### Database Writes

All writes use transactions for atomicity.

#### Scheduler Stats
```sql
INSERT INTO scheduler_stats (
    stats_period_id, start_time, end_time,
    iterations, run_jobs, late_jobs, missed_jobs, jobs_cancelled,
    min_inbox_length, max_inbox_length, avg_inbox_length,
    empty_inbox_time, avg_time_in_inbox, min_time_in_inbox, max_time_in_inbox
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
```

#### Orchestrator Stats (batch insert)
```sql
INSERT INTO orchestrator_stats (
    run_id, stats_period_id, runtime, constraints_checked, actions_taken
) VALUES (?, ?, ?, ?, ?);
```

#### Syncer Stats
```sql
INSERT INTO syncer_stats (
    stats_period_id, total_writes, writes_succeeded, writes_failed,
    avg_writes_in_flight, max_writes_in_flight, min_writes_in_flight,
    avg_queued_writes, max_queued_writes, min_queued_writes,
    avg_inbox_length, max_inbox_length, min_inbox_length,
    avg_time_in_write_queue, max_time_in_write_queue, min_time_in_write_queue,
    avg_time_in_inbox, max_time_in_inbox, min_time_in_inbox
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
```

## Intermediate Calculations

### Min/Max/Avg
For any slice of integers or durations:
```go
func calculateMinMaxAvg[T constraints.Integer | constraints.Float](samples []T) (min, max T, avg float64)
```

### Percentiles (future enhancement)
```go
func calculatePercentile[T constraints.Integer | constraints.Float](samples []T, percentile float64) T
```

## Error Handling

### Database Write Failures
- Log error with full context
- Retry with exponential backoff (3 attempts)
- If all retries fail, log critical error and continue
- Don't block on failed writes
- Stats are best-effort, not critical path

### Memory Pressure
- If accumulator samples exceed threshold (e.g., 10,000), force flush
- Log warning about memory pressure
- This prevents unbounded memory growth if database is down

### Inbox Full
- Senders use timeout when sending
- If inbox full, timeout occurs
- Sender logs warning
- Stats Collector logs inbox full warnings
- Monitor inbox utilization

## Shutdown Sequence

1. Receive shutdown signal on `done` channel
2. Stop accepting new messages (close inbox)
3. Process all remaining messages in inbox
4. Flush all accumulators to database
5. Close database connection
6. Signal completion via `wg.Done()`

## Thread Safety

- Inbox is thread-safe via channels
- Accumulators are only accessed by main loop (single goroutine)
- No mutex needed for accumulators
- Database writes use transactions

## Performance Characteristics

### Memory Usage
- Bounded by inbox buffer size + accumulator sample sizes
- Typical: ~100KB per stats period
- Maximum: ~1MB if all accumulators at capacity

### CPU Usage
- Minimal CPU usage
- Main work is during flush (calculations + database writes)
- Expected: < 1% CPU utilization

### Latency
- Stats are not time-critical
- Flush latency: < 100ms typical
- Database write latency: < 500ms typical
- Stats period: 30 seconds (configurable)

## Integration Points

### Scheduler Integration
```go
// In scheduler main loop:
statsCollector.Send(StatsMessage{
    Source:    StatsSourceScheduler,
    Timestamp: time.Now(),
    Data: SchedulerStatsData{
        Iterations:      1,
        JobsRun:         jobsRunThisIteration,
        LateJobs:        lateJobsThisIteration,
        InboxLength:     inbox.Len(),
        MessageWaitTime: msgWaitTime,
    },
})
```

### Orchestrator Integration
```go
// In orchestrator completion:
statsCollector.Send(StatsMessage{
    Source:    StatsSourceOrchestrator,
    Timestamp: time.Now(),
    Data: OrchestratorStatsData{
        RunID:              runID,
        Runtime:            time.Since(startTime),
        ConstraintsChecked: constraintsChecked,
        ActionsTaken:       actionsTaken,
    },
})
```

### Syncer Integration
```go
// In syncer periodic reporting:
statsCollector.Send(StatsMessage{
    Source:    StatsSourceSyncer,
    Timestamp: time.Now(),
    Data: SyncerStatsData{
        TotalWrites:     totalWrites,
        WritesSucceeded: writesSucceeded,
        WritesFailed:    writesFailed,
        WritesInFlight:  currentWritesInFlight,
        QueuedWrites:    currentQueuedWrites,
        InboxLength:     syncer.inbox.Len(),
    },
})
```

## Testing Strategy

See `stats-collector-tests.md` for detailed test specifications.

Key testing areas:
- Message routing to correct accumulator
- Min/max/avg calculations
- Period transitions
- Flush triggers (threshold and timer)
- Database writes
- Error handling and retries
- Shutdown with pending stats
- Memory limits

## Future Enhancements

1. **Percentile calculations** (p50, p95, p99)
2. **Histogram buckets** for latency distributions
3. **Real-time stats queries** via request/response pattern
4. **Stats aggregation** across multiple periods
5. **Stats export** to external systems (Prometheus, DataDog)
6. **Alerting** based on stats thresholds
