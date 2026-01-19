# Syncer

## Overview
The Syncer is a standalone component responsible for persisting all job execution data to the database. It receives messages from the scheduler and orchestrators via an inbox and handles all transactional writes for job runs, constraint violations, and action executions.

## Purpose
The Syncer separates database write operations from the scheduler's hot path and orchestrator logic. This provides:
- No I/O blocking in scheduler main loop
- No I/O blocking in orchestrators
- Efficient batched database writes
- Transaction management for atomic updates
- Backpressure handling when database is slow
- Single source of truth for job execution data persistence

## Architecture

### Communication Pattern
- **Inbox-based**: Syncer has a buffered inbox channel
- **Blocking receiver**: Sits in a loop blocking on the inbox
- **Fire-and-forget**: Other components send messages without waiting for acknowledgment
- **Batched writes**: Accumulates messages and flushes in batches

### Component Lifecycle
1. **Initialization**
   - Create inbox channel with configured buffer size
   - Initialize batch buffers for each message type
   - Initialize flush timers
   - Start goroutine

2. **Main Loop**
   - Block on inbox channel waiting for messages
   - When message received:
     - Add to appropriate batch buffer (job runs, violations, or actions)
     - Check if flush threshold reached for any buffer
     - If threshold reached OR timer expired, flush that buffer
   - Multiple flush timers run concurrently (one per message type)
   - Send syncer statistics to Stats Collector periodically

3. **Shutdown**
   - Receive shutdown signal
   - Flush all pending batches to database
   - Complete in-flight transactions
   - Close database connections
   - Exit cleanly

## Message Types

### SyncerMessage
```go
type SyncerMessage struct {
    Type      SyncerMessageType
    Timestamp time.Time
    Data      interface{} // Type depends on MessageType
}

type SyncerMessageType int

const (
    MessageTypeJobRunUpdate SyncerMessageType = iota
    MessageTypeConstraintViolation
    MessageTypeActionRun
)
```

### Job Run Update
```go
type JobRunUpdate struct {
    RunID       string
    JobID       string
    ScheduledAt time.Time
    StartedAt   *time.Time
    CompletedAt *time.Time
    Status      string
    Success     *bool
    Error       *string
}
```

### Constraint Violation
```go
type ConstraintViolation struct {
    RunID            string
    ConstraintTypeID int
    ViolationTime    time.Time
    Details          map[string]interface{} // JSON
}
```

### Action Run
```go
type ActionRun struct {
    RunID                  string
    ActionTypeID           int
    Trigger                string // on_failure, on_violation, on_success, manual
    ConstraintViolationID  *string
    ExecutedAt             time.Time
    Success                bool
    Error                  *string
    Details                map[string]interface{} // JSON
}
```

## Configuration

### Syncer Config
```go
type SyncerConfig struct {
    // Inbox configuration
    InboxBufferSize          int           // Inbox channel buffer size (default: 1000)
    InboxSendTimeout         time.Duration // Timeout for sending to inbox (default: 5s)

    // Job run flushing
    JobRunFlushThreshold     int           // Flush when this many updates (default: 100)
    JobRunFlushInterval      time.Duration // Flush after this time (default: 1s)
    JobRunChannelSize        int           // Internal buffer size (default: 200)

    // Constraint violation flushing
    ViolationFlushThreshold  int           // Flush when this many violations (default: 50)
    ViolationFlushInterval   time.Duration // Flush after this time (default: 2s)
    ViolationChannelSize     int           // Internal buffer size (default: 100)

    // Action run flushing
    ActionFlushThreshold     int           // Flush when this many actions (default: 50)
    ActionFlushInterval      time.Duration // Flush after this time (default: 2s)
    ActionChannelSize        int           // Internal buffer size (default: 100)

    // Backpressure configuration
    MaxBufferedJobRunUpdates int           // Stop accepting when exceeded (default: 10000)

    // Stats reporting
    StatsReportInterval      time.Duration // How often to send stats (default: 30s)
}
```

## Database Operations

### Job Run Updates
- **Table**: `job_runs`
- **Operation**: UPSERT (INSERT or UPDATE)
- **Key**: (job_id, scheduled_at)
- **Batching**: Multiple rows per transaction
- **Transaction**: Always use transaction for batch

```sql
-- SQLite
INSERT INTO job_runs (job_id, run_id, scheduled_at, started_at, completed_at, status, success, error)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (job_id, scheduled_at) DO UPDATE SET
    started_at = excluded.started_at,
    completed_at = excluded.completed_at,
    status = excluded.status,
    success = excluded.success,
    error = excluded.error;
```

### Constraint Violations
- **Table**: `constraint_violations`
- **Operation**: INSERT
- **Batching**: Multiple rows per transaction
- **Transaction**: Always use transaction for batch

```sql
INSERT INTO constraint_violations (run_id, constraint_type_id, violation_time, details)
VALUES (?, ?, ?, ?);
```

### Action Runs
- **Table**: `action_runs`
- **Operation**: INSERT
- **Batching**: Multiple rows per transaction
- **Transaction**: Always use transaction for batch

```sql
INSERT INTO action_runs (run_id, action_type_id, trigger, constraint_violation_id, executed_at, success, error, details)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);
```

## Batching Strategy

### Dual-Trigger Flushing
Each message type has two flush triggers:
- **Size threshold**: Flush when batch reaches N messages
- **Time interval**: Flush after T duration since last flush

Whichever trigger fires first causes the flush.

### Independent Batches
Each message type is batched independently:
- Job run updates accumulate in one batch
- Constraint violations accumulate in another batch
- Action runs accumulate in a third batch

This allows different flush rates for different message types.

### Flush Priority
When multiple batches are ready to flush:
1. Job run updates (highest priority - critical state)
2. Constraint violations (medium priority - audit trail)
3. Action runs (medium priority - audit trail)

## Backpressure Handling

### When Database is Slow
If database writes are slow, the Syncer:
1. Accumulates more messages in batch buffers
2. Monitors total buffered count
3. When approaching limit:
   - Flushes more aggressively (lower thresholds)
   - Logs warning about backpressure
4. When limit exceeded:
   - Sends block on inbox (senders must wait)
   - Logs error about backpressure
5. After successful flush, resume normal operation

### Backpressure Signals
The Syncer does NOT drop messages. Instead:
- Senders may experience timeout on send
- Orchestrators should handle send timeout gracefully
- Main loop should handle send timeout gracefully
- This prevents data loss

## Error Handling

### Database Write Failures
- Log error with full context
- Retry failed batch with exponential backoff
- If retry fails after N attempts:
  - Log critical error
  - Write failed batch to disk for manual recovery
  - Continue processing new messages
- Never drop data silently

### Constraint Violations (Database)
- Foreign key violations indicate bug (invalid run_id)
- Log critical error with full message context
- Write to dead letter queue (disk file)
- Continue processing

### Disk Full
- If disk is full, can't write anything
- Log critical error
- Attempt to free space (delete old logs)
- If still failing, pause accepting messages
- Don't crash - wait for operator intervention

## Statistics

The Syncer reports to Stats Collector:
- Total writes (by type)
- Writes succeeded/failed (by type)
- Average batch size (by type)
- Flush trigger hit (size vs time, by type)
- Writes in flight
- Queued writes (by type)
- Inbox length (current, min, max, avg)
- Time in queue (min, max, avg)
- Backpressure events
- Database write latency (min, max, avg, p95, p99)

## Interface

### Sending Messages
```go
// From scheduler or orchestrator:
syncer.Send(SyncerMessage{
    Type:      MessageTypeJobRunUpdate,
    Timestamp: time.Now(),
    Data: JobRunUpdate{
        RunID:       "run-123",
        JobID:       "job-456",
        ScheduledAt: scheduledTime,
        StartedAt:   &startTime,
        Status:      "running",
    },
})

// With timeout:
select {
case syncer.Inbox <- msg:
    // Sent successfully
case <-time.After(5 * time.Second):
    // Timeout - syncer is backed up
    log.Error("failed to send to syncer: timeout")
}
```

### Initialization
```go
// In main.go startup:
syncer := NewSyncer(db, statsCollector, config.Syncer)
syncer.Start()
defer syncer.Stop()
```

## Database Schema

Uses existing tables:
- `job_runs`
- `constraint_violations`
- `action_runs`

See main spec.md for full schema details.

## Testing Requirements

### Unit Tests
- Message routing to correct batch buffer
- Flush threshold triggers
- Flush timer triggers
- Batch construction (SQL generation)
- Database placeholder handling (PostgreSQL vs MySQL vs SQLite)
- Backpressure detection
- Stats calculation

### Integration Tests
- Actual database writes (all three message types)
- Concurrent senders
- Batching behavior under load
- Flush timing verification
- Backpressure handling
- Graceful shutdown with pending batches
- Database failure recovery
- Transaction rollback on error

### Load Tests
- High-volume job run updates
- Many concurrent orchestrators
- Sustained write load
- Backpressure behavior
- Database slow response handling

## Performance Characteristics

### Expected Throughput
- Job run updates: 1000/second
- Constraint violations: 100/second
- Action runs: 100/second

### Latency Requirements
- Inbox send: < 1ms (except under backpressure)
- Time to database: < 2 seconds (p99)
- Backpressure recovery: < 10 seconds after database recovery

### Resource Usage
- Memory: O(buffer_sizes) bounded by configuration
- Database connections: 1 connection
- Goroutines: 1 main loop + 3 flush timers = 4 total

## Migration from Current Implementation

### Current State
Currently, the scheduler has a Syncer component, but it may need updates to:
1. Handle constraint violations
2. Handle action runs
3. Use separate batches for each message type
4. Implement proper backpressure

### Migration Steps
1. **Add new message types**
   - Define ConstraintViolation and ActionRun structs
   - Add to SyncerMessage union type

2. **Add new batch buffers**
   - Create separate buffers for violations and actions
   - Initialize flush timers for each

3. **Implement database operations**
   - Add INSERT statements for violations and actions
   - Use database-specific placeholder syntax

4. **Update orchestrators**
   - Send constraint violations to Syncer (instead of direct writes)
   - Send action runs to Syncer (instead of direct writes)

5. **Update configuration**
   - Add violation and action flush thresholds
   - Add violation and action flush intervals

6. **Test migration**
   - Verify all messages reach database
   - Verify batching works correctly
   - Verify backpressure handling

## Benefits

### Performance
- Batched writes reduce database load
- Transaction overhead amortized across batch
- Non-blocking senders (unless backpressure)

### Reliability
- No data loss (blocking backpressure vs dropping)
- Automatic retry on transient failures
- Dead letter queue for permanent failures

### Observability
- Centralized write statistics
- Easy to monitor database write health
- Clear backpressure signals

### Maintainability
- Single place for all write logic
- Easy to change batching strategy
- Easy to add new message types
- Testable in isolation
