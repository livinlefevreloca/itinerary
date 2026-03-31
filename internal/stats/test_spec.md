# Stats Collector Test Specification

## Test Organization

Tests should be organized into the following categories:
1. Unit tests - Test individual functions in isolation
2. Integration tests - Test with actual database
3. Concurrency tests - Test thread safety and concurrent access
4. Performance tests - Test under load

## Unit Tests

### Test: NewStatsCollector
**Purpose**: Verify stats collector initialization

```go
func TestNewStatsCollector(t *testing.T)
```

**Verify**:
- StatsCollector struct is properly initialized
- Inbox is created with correct buffer size
- Config is stored correctly
- Accumulators are initialized
- Current period ID is calculated correctly
- Period start time is set

### Test: Send - Success
**Purpose**: Verify successful message sending

```go
func TestSend_Success(t *testing.T)
```

**Setup**:
- Create stats collector with buffer size 10
- Don't start the main loop (inbox won't drain)

**Test**:
- Send a scheduler stats message
- Verify Send() returns true
- Verify inbox contains the message

### Test: Send - Timeout
**Purpose**: Verify timeout behavior when inbox is full

```go
func TestSend_Timeout(t *testing.T)
```

**Setup**:
- Create stats collector with buffer size 1
- Don't start the main loop
- Fill the inbox

**Test**:
- Send another message (should timeout)
- Verify Send() returns false
- Verify timeout counter increments

### Test: Message Routing - Scheduler
**Purpose**: Verify scheduler stats are routed correctly

```go
func TestMessageRouting_Scheduler(t *testing.T)
```

**Test**:
- Send scheduler stats message
- Verify schedulerStats accumulator receives the data
- Verify other accumulators are not affected

### Test: Message Routing - Orchestrator
**Purpose**: Verify orchestrator stats are routed correctly

```go
func TestMessageRouting_Orchestrator(t *testing.T)
```

**Test**:
- Send orchestrator stats message with run_id "run-123"
- Verify orchestratorStats map has entry for "run-123"
- Send another message for same run_id
- Verify only one entry exists (not duplicated)

### Test: Message Routing - Syncer
**Purpose**: Verify syncer stats are routed correctly

```go
func TestMessageRouting_Syncer(t *testing.T)
```

**Test**:
- Send syncer stats message
- Verify syncerStats accumulator receives the data
- Verify other accumulators are not affected

### Test: SchedulerStatsAccumulator - Add
**Purpose**: Verify accumulator correctly adds data

```go
func TestSchedulerStatsAccumulator_Add(t *testing.T)
```

**Test**:
- Create new accumulator
- Add scheduler stats data (3 iterations, 5 jobs run, inbox length 10)
- Add more data (2 iterations, 3 jobs run, inbox length 15)
- Verify totals: 5 iterations, 8 jobs run
- Verify inbox length samples: [10, 15]

### Test: SchedulerStatsAccumulator - Calculations
**Purpose**: Verify min/max/avg calculations

```go
func TestSchedulerStatsAccumulator_Calculations(t *testing.T)
```

**Test**:
- Create accumulator with sample data:
  - Inbox lengths: [5, 10, 15, 20, 10]
  - Message wait times: [1ms, 2ms, 3ms, 4ms, 5ms]
- Calculate min/max/avg
- Verify:
  - Min inbox length: 5
  - Max inbox length: 20
  - Avg inbox length: 12
  - Min wait time: 1ms
  - Max wait time: 5ms
  - Avg wait time: 3ms

### Test: SyncerStatsAccumulator - Add
**Purpose**: Verify syncer accumulator correctly adds data

```go
func TestSyncerStatsAccumulator_Add(t *testing.T)
```

**Test**:
- Create new accumulator
- Add syncer stats data (10 writes, 8 succeeded, 2 failed)
- Add more data (5 writes, 5 succeeded, 0 failed)
- Verify totals: 15 writes, 13 succeeded, 2 failed

### Test: Period Transition
**Purpose**: Verify stats period transitions work correctly

```go
func TestPeriodTransition(t *testing.T)
```

**Setup**:
- Create stats collector with 1 second period duration
- Mock database

**Test**:
- Send stats for period 1
- Wait 1 second (period should transition)
- Send stats for period 2
- Verify period 1 was flushed to database
- Verify accumulator was reset for period 2
- Verify period 2 stats are separate

### Test: Flush Threshold
**Purpose**: Verify flush triggers when threshold reached

```go
func TestFlushThreshold(t *testing.T)
```

**Setup**:
- Create stats collector with flush threshold of 5
- Mock database

**Test**:
- Send 4 stats messages
- Verify no flush occurred
- Send 5th stats message
- Verify flush occurred
- Verify database write was called

### Test: Flush Timer
**Purpose**: Verify flush triggers on timer

```go
func TestFlushTimer(t *testing.T)
```

**Setup**:
- Create stats collector with 2 second flush interval
- Mock database

**Test**:
- Send 1 stats message (below threshold)
- Wait 2 seconds
- Verify flush occurred
- Verify database write was called

### Test: OrchestratorStats - Multiple Runs
**Purpose**: Verify multiple orchestrator runs are tracked separately

```go
func TestOrchestratorStats_MultipleRuns(t *testing.T)
```

**Test**:
- Send stats for run-1 (runtime 100ms, 5 constraints, 2 actions)
- Send stats for run-2 (runtime 200ms, 3 constraints, 1 action)
- Send more stats for run-1 (should update, not duplicate)
- Verify orchestratorStats map has 2 entries
- Verify run-1 has updated stats
- Verify run-2 has correct stats

### Test: CalculateMinMaxAvg - Integers
**Purpose**: Verify min/max/avg calculation for integers

```go
func TestCalculateMinMaxAvg_Integers(t *testing.T)
```

**Test cases**:
- Empty slice: min=0, max=0, avg=0
- Single value [5]: min=5, max=5, avg=5
- Multiple values [1, 5, 3, 9, 2]: min=1, max=9, avg=4
- All same [5, 5, 5]: min=5, max=5, avg=5
- Negative values [-5, -2, -8]: min=-8, max=-2, avg=-5

### Test: CalculateMinMaxAvg - Durations
**Purpose**: Verify min/max/avg calculation for durations

```go
func TestCalculateMinMaxAvg_Durations(t *testing.T)
```

**Test cases**:
- Multiple durations: [1ms, 5ms, 3ms, 9ms, 2ms]
- Verify: min=1ms, max=9ms, avg=4ms

## Integration Tests

### Test: Database Write - Scheduler Stats
**Purpose**: Verify scheduler stats are correctly written to database

```go
func TestDatabaseWrite_SchedulerStats(t *testing.T)
```

**Setup**:
- Create real database (SQLite in-memory)
- Run migrations to create schema
- Create stats collector

**Test**:
- Send scheduler stats message
- Trigger flush
- Query database for scheduler_stats
- Verify all fields match expected values
- Verify stats_period_id is correct
- Verify start_time and end_time are set

### Test: Database Write - Orchestrator Stats
**Purpose**: Verify orchestrator stats are correctly written to database

```go
func TestDatabaseWrite_OrchestratorStats(t *testing.T)
```

**Setup**:
- Create real database with schema
- Create job_run entries (foreign key requirement)

**Test**:
- Send orchestrator stats for 3 different runs
- Trigger flush
- Query database for orchestrator_stats
- Verify 3 rows were inserted
- Verify all fields match expected values
- Verify foreign key constraint to job_runs works

### Test: Database Write - Syncer Stats
**Purpose**: Verify syncer stats are correctly written to database

```go
func TestDatabaseWrite_SyncerStats(t *testing.T)
```

**Setup**:
- Create real database with schema

**Test**:
- Send syncer stats with various samples
- Trigger flush
- Query database for syncer_stats
- Verify min/max/avg calculations are correct
- Verify all fields are populated

### Test: Database Write - Multiple Periods
**Purpose**: Verify multiple stats periods can coexist

```go
func TestDatabaseWrite_MultiplePeriods(t *testing.T)
```

**Test**:
- Send stats for period 1
- Flush
- Send stats for period 2
- Flush
- Query database
- Verify 2 scheduler_stats rows (one per period)
- Verify period IDs are different
- Verify stats are separate

## Concurrency Tests

### Test: Concurrent Sends
**Purpose**: Verify multiple goroutines can send concurrently

```go
func TestConcurrentSends(t *testing.T)
```

**Test**:
- Start stats collector
- Launch 10 goroutines
- Each goroutine sends 100 stats messages
- Wait for all to complete
- Verify 1000 total messages were received
- Verify no race conditions (use -race flag)

### Test: Send During Flush
**Purpose**: Verify sends work during database flush

```go
func TestSendDuringFlush(t *testing.T)
```

**Setup**:
- Create stats collector with slow database (add sleep to writes)

**Test**:
- Send messages to trigger flush
- While flush is happening, send more messages
- Verify new messages don't block (or timeout gracefully)
- Verify new messages are included in next flush
- Verify no deadlocks

### Test: Concurrent Period Transitions
**Purpose**: Verify period transitions are thread-safe

```go
func TestConcurrentPeriodTransitions(t *testing.T)
```

**Test**:
- Configure 100ms period duration
- Send stats continuously from multiple goroutines
- Let multiple period transitions occur
- Verify no race conditions
- Verify stats are correctly attributed to periods

## Edge Case Tests

### Test: Empty Accumulator Flush
**Purpose**: Verify flush works with no stats collected

```go
func TestEmptyAccumulatorFlush(t *testing.T)
```

**Test**:
- Create stats collector
- Don't send any messages
- Trigger flush
- Verify no database writes occur
- Verify no errors

### Test: Single Sample Calculations
**Purpose**: Verify calculations work with single sample

```go
func TestSingleSampleCalculations(t *testing.T)
```

**Test**:
- Send single stats message
- Trigger flush
- Verify min = max = avg = single value
- Verify no division by zero errors

### Test: Zero Values
**Purpose**: Verify handling of zero values

```go
func TestZeroValues(t *testing.T)
```

**Test**:
- Send stats with all zero values
- Verify correct storage in database
- Verify calculations handle zeros correctly

### Test: Maximum Values
**Purpose**: Verify handling of very large values

```go
func TestMaximumValues(t *testing.T)
```

**Test**:
- Send stats with max int64 values
- Send stats with very long durations
- Verify no overflow
- Verify correct storage

## Shutdown Tests

### Test: Graceful Shutdown - Empty Inbox
**Purpose**: Verify clean shutdown with no pending messages

```go
func TestGracefulShutdown_EmptyInbox(t *testing.T)
```

**Test**:
- Start stats collector
- Call Stop()
- Verify stops cleanly
- Verify database connection closed
- Verify goroutines exit

### Test: Graceful Shutdown - Pending Messages
**Purpose**: Verify shutdown flushes pending stats

```go
func TestGracefulShutdown_PendingMessages(t *testing.T)
```

**Test**:
- Start stats collector
- Send stats messages (below flush threshold)
- Call Stop()
- Verify all pending messages are processed
- Verify flush occurred
- Verify data in database

### Test: Graceful Shutdown - During Flush
**Purpose**: Verify shutdown waits for in-progress flush

```go
func TestGracefulShutdown_DuringFlush(t *testing.T)
```

**Setup**:
- Create slow database writes

**Test**:
- Trigger flush
- Call Stop() while flush in progress
- Verify Stop() blocks until flush completes
- Verify data is fully written

### Test: Stop Idempotency
**Purpose**: Verify Stop() can be called multiple times safely

```go
func TestStopIdempotency(t *testing.T)
```

**Test**:
- Start stats collector
- Call Stop()
- Call Stop() again
- Verify no panic
- Verify no deadlock

## Test Helpers

### Mock Database
```go
type MockDB struct {
    WriteCalls   []interface{}
    FailCount    int // Fail this many times before succeeding
    WriteLatency time.Duration
}

func (m *MockDB) WriteSchedulerStats(period int64, data *SchedulerStats) error
func (m *MockDB) WriteOrchestratorStats(period int64, stats []*OrchestratorStats) error
func (m *MockDB) WriteSyncerStats(period int64, data *SyncerStats) error
```

### Test Database
```go
func setupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)

    // Run migrations
    // ...

    return db
}
```

### Stats Assertions
```go
func assertSchedulerStats(t *testing.T, db *sql.DB, periodID int64, expected SchedulerStats)
func assertOrchestratorStats(t *testing.T, db *sql.DB, periodID int64, runID string, expected OrchestratorStats)
func assertSyncerStats(t *testing.T, db *sql.DB, periodID int64, expected SyncerStats)
```

## Test Coverage Goals

- Line coverage: > 90%
- Branch coverage: > 85%
- All public methods must have tests
- All error paths must be tested
- All edge cases must be covered

## Test Execution

```bash
# Run all tests
go test -v ./internal/stats/...

# Run with race detector
go test -race ./internal/stats/...

# Run with coverage
go test -cover ./internal/stats/...
go test -coverprofile=coverage.out ./internal/stats/...
go tool cover -html=coverage.out

# Run specific test
go test -v -run TestConcurrentSends ./internal/stats/...

# Run benchmarks
go test -bench=. ./internal/stats/...
```
