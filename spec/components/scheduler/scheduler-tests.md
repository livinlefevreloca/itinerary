# Central Scheduler Test Suite

## Overview
Comprehensive test suite for the Central Scheduler component. Tests cover configuration, inbox operations, syncer buffering/flushing, main loop operations, orchestrator lifecycle, heartbeat monitoring, and shutdown behavior.

## Test Organization

```
internal/scheduler/
├── config_test.go           # Configuration validation tests
├── inbox_test.go            # Inbox operations tests
├── syncer_test.go           # Syncer buffering/flushing tests
├── scheduler_test.go        # Main scheduler loop tests
├── orchestrator_test.go     # Orchestrator lifecycle tests
├── integration_test.go      # End-to-end integration tests
└── scheduler_bench_test.go  # Performance benchmarks
```

## Configuration Tests

### Default Configuration Tests
```go
func TestDefaultSchedulerConfig(t *testing.T)
func TestDefaultSyncerConfig(t *testing.T)
```

These tests verify that default configurations:
1. **Have positive values**: All duration/count values are > 0 where required
2. **Pass validation**: Defaults satisfy all validation rules

The tests do NOT check specific default values (e.g., "must be 10s") to avoid brittle tests. When defaults change, the tests continue to pass as long as they remain valid.

### Validation Tests
```go
func TestValidateConfig_*(t *testing.T)
```

Test cases for validation logic:
1. **IndexRebuildInterval >= LookaheadWindow**: Returns error
2. **Negative PreScheduleInterval**: Returns error
3. **Negative LookaheadWindow**: Returns error
4. **Negative GracePeriod**: Returns error
5. **Zero InboxBufferSize**: Returns error
6. **Zero MaxBufferedJobRunUpdates**: Returns error
7. **Negative OrchestratorHeartbeatInterval**: Returns error
8. **Zero MaxMissedHeartbeats**: Returns error

## Inbox Tests

### Basic Operations
```go
func TestInbox_Send_Success(t *testing.T)
```
- Create inbox with buffer size 10
- Send 5 messages
- All sends succeed (return true)
- Stats show TotalSent = 5

```go
func TestInbox_Send_Timeout(t *testing.T)
```
- Create inbox with buffer size 2, timeout 10ms
- Send 2 messages (fills buffer)
- Third send times out
- Returns false
- Stats show TimeoutCount = 1

```go
func TestInbox_TryReceive_Success(t *testing.T)
```
- Send 3 messages
- TryReceive 3 times, all succeed
- Each returns (message, true)
- Stats show TotalReceived = 3

```go
func TestInbox_TryReceive_Empty(t *testing.T)
```
- Empty inbox
- TryReceive returns (zero message, false)

```go
func TestInbox_Receive_Blocking(t *testing.T)
```
- Start goroutine that receives (blocks)
- Send message after 50ms
- Goroutine unblocks and receives message

### Statistics
```go
func TestInbox_UpdateDepthStats(t *testing.T)
```
- Send 5 messages to inbox with size 10
- UpdateDepthStats()
- Stats.CurrentDepth = 5
- Send 3 more (total 8)
- UpdateDepthStats()
- Stats.CurrentDepth = 8
- Stats.MaxDepthSeen = 8
- Receive 4 messages (leaving 4)
- UpdateDepthStats()
- Stats.CurrentDepth = 4
- Stats.MaxDepthSeen still = 8

## Syncer Tests

### Buffering Tests
```go
func TestSyncer_BufferJobRunUpdate_BelowThreshold(t *testing.T)
```
- Buffer 10 updates (threshold = 500)
- No flush triggered
- GetStats().BufferedJobRunUpdates = 10

```go
func TestSyncer_BufferJobRunUpdate_ReachesThreshold(t *testing.T)
```
- Buffer 500 updates (threshold = 500)
- Flush request should be signaled
- Check jobRunFlushRequest channel has message

```go
func TestSyncer_BufferJobRunUpdate_ExceedsMaximum(t *testing.T)
```
- Set MaxBufferedJobRunUpdates = 100
- Buffer 101 updates
- Returns error "buffer exceeded maximum size"

```go
func TestSyncer_BufferStats_BelowThreshold(t *testing.T)
```
- Buffer 10 stats (threshold = 30)
- No flush triggered
- GetStats().BufferedStats = 10

```go
func TestSyncer_BufferStats_ReachesThreshold(t *testing.T)
```
- Buffer 30 stats (threshold = 30)
- Flush request should be signaled
- Check statsFlushRequest channel has message

### Flushing Tests - Size-Based
```go
func TestSyncer_FlushJobRunUpdates_Success(t *testing.T)
```
- Start syncer with mock DB
- Buffer 100 updates
- Call FlushJobRunUpdates()
- All 100 sent to jobRunChannel
- Buffer is cleared (length = 0)

```go
func TestSyncer_FlushJobRunUpdates_ChannelFull(t *testing.T)
```
- Create syncer with JobRunChannelSize = 10
- Do NOT start syncer (no consumer)
- Buffer 20 updates
- Call FlushJobRunUpdates()
- First 10 sent successfully
- Returns error "channel full"
- Buffer still contains remaining 10

```go
func TestSyncer_FlushStats_Success(t *testing.T)
```
- Start syncer with mock DB
- Buffer 50 stats
- Call FlushStats()
- Stats sent to statsChannel
- Buffer is cleared

```go
func TestSyncer_FlushStats_Empty(t *testing.T)
```
- Empty buffer
- Call FlushStats()
- Returns nil (no error)
- No channel operations

### Flushing Tests - Time-Based
```go
func TestSyncer_JobRunFlusher_TimeBased(t *testing.T)
```
- Create syncer with JobRunFlushInterval = 100ms
- Start syncer
- Buffer 10 updates (below threshold)
- Wait 150ms
- Verify updates flushed to channel
- Syncer goroutine consumed them and called writeJobRunUpdate

```go
func TestSyncer_StatsFlusher_TimeBased(t *testing.T)
```
- Create syncer with StatsFlushInterval = 100ms
- Start syncer
- Buffer 5 stats (below threshold)
- Wait 150ms
- Verify stats flushed to channel

### Flushing Tests - Dual Trigger
```go
func TestSyncer_JobRunFlusher_SizeTrigger(t *testing.T)
```
- Start syncer with JobRunFlushThreshold = 10
- Buffer 10 updates rapidly
- Flush triggered by size, not time
- Verify immediate flush

```go
func TestSyncer_JobRunFlusher_ManualFlush(t *testing.T)
```
- Start syncer
- Buffer 5 updates
- Call requestFlush() (non-blocking send to jobRunFlushRequest)
- Verify flush happens immediately

### Write Error Logging
```go
func TestSyncer_JobRunSyncer_WriteSuccess(t *testing.T)
```
- Mock DB that succeeds
- Start syncer
- Send update to jobRunChannel
- Verify writeJobRunUpdate called
- Verify debug log "wrote job run update"

```go
func TestSyncer_JobRunSyncer_WriteFailure(t *testing.T)
```
- Mock DB that returns error
- Start syncer
- Send update to jobRunChannel
- Verify error logged at error level
- Update is not retried

```go
func TestSyncer_StatsSyncer_WriteFailure(t *testing.T)
```
- Mock DB that returns error
- Start syncer
- Send stats to statsChannel
- Verify error logged
- Stats write failure is non-critical

### Shutdown Tests
```go
func TestSyncer_Shutdown_FlushesAll(t *testing.T)
```
- Start syncer
- Buffer 100 job run updates
- Buffer 10 stats
- Call Shutdown()
- Verify all updates written to DB
- Verify all stats written to DB
- Verify all goroutines exited

```go
func TestSyncer_Shutdown_DrainChannels(t *testing.T)
```
- Start syncer
- Send 50 updates directly to jobRunChannel (bypassing buffer)
- Call Shutdown() immediately
- Verify all 50 updates processed before exit
- Tests "for range" drain behavior

```go
func TestSyncer_Shutdown_Order(t *testing.T)
```
- Start syncer
- Buffer updates
- Call Shutdown()
- Verify order:
  1. Shutdown signal sent
  2. Final flush called
  3. Channels closed
  4. WaitGroup waits
  5. Returns

## Scheduler Main Loop Tests

### Initialization Tests
```go
func TestNewScheduler_Success(t *testing.T)
```
- Create scheduler with valid config and mock DB
- Verify initial index built from DB
- Verify inbox created
- Verify syncer created
- Verify activeOrchestrators map initialized

```go
func TestNewScheduler_DBQueryFails(t *testing.T)
```
- Mock DB that fails on queryJobDefinitions
- NewScheduler returns error
- Error contains "failed to query jobs on startup"

```go
func TestNewScheduler_InvalidCronSchedule(t *testing.T)
```
- Mock DB returns job with invalid cron expression
- Logs error for that job
- Continues with other jobs
- Does not return error (startup continues)

### Index Rebuilding Tests
```go
func TestScheduler_IndexBuilder_Periodic(t *testing.T)
```
- Start scheduler with IndexRebuildInterval = 100ms
- Mock DB with 10 jobs
- Wait for 2 rebuild cycles
- Verify performIndexRebuild called twice
- Verify index.Swap called with new runs

```go
func TestScheduler_IndexBuilder_ExplicitRebuild(t *testing.T)
```
- Start scheduler
- Send to rebuildIndexChan
- Verify rebuild happens immediately (not waiting for interval)

```go
func TestScheduler_IndexBuilder_LookaheadWindow(t *testing.T)
```
- LookaheadWindow = 10 minutes
- GracePeriod = 30 seconds
- Mock "now" as 12:00:00
- Verify generateScheduledRuns called with:
  - start = 11:59:30 (now - gracePeriod)
  - end = 12:10:00 (now + lookahead)

```go
func TestScheduler_IndexBuilder_InvalidCronSchedule(t *testing.T)
```
- Start scheduler with 3 jobs
- One job has invalid cron expression
- Trigger index rebuild
- Logs error for invalid job
- Index built with 2 valid jobs (invalid job skipped)
- Rebuild completes successfully

### RunID Generation Tests
```go
func TestGenerateRunID_Deterministic(t *testing.T)
```
- jobID = "job123"
- scheduledAt = time.Unix(1704067200, 0) // 2024-01-01 00:00:00 UTC
- runID = generateRunID(jobID, scheduledAt)
- Verify runID = "job123:1704067200"
- Call again with same inputs
- Verify produces same runID (deterministic)

```go
func TestGenerateRunID_DifferentJobs(t *testing.T)
```
- Same scheduledAt for two different jobs
- job1 runID = "job1:1704067200"
- job2 runID = "job2:1704067200"
- Verify they are different

```go
func TestGenerateRunID_DifferentTimes(t *testing.T)
```
- Same job at different times
- runID1 = "job1:1704067200"
- runID2 = "job1:1704067260"
- Verify they are different

### Orchestrator Scheduling Tests
```go
func TestScheduler_ScheduleOrchestrators_LaunchesNew(t *testing.T)
```
- Index contains 3 runs in next 10 seconds:
  - (job1, 12:00:05) → runID "job1:1704067205"
  - (job2, 12:00:08) → runID "job2:1704067208"
  - (job3, 12:00:09) → runID "job3:1704067209"
- Run iteration
- Verify 3 orchestrators launched
- Verify 3 entries in activeOrchestrators map with correct runIDs
- Each has status OrchestratorPreRun
- Each has LastHeartbeat = now

```go
func TestScheduler_ScheduleOrchestrators_SkipsDuplicates(t *testing.T)
```
- Orchestrator with runID "job1:1704067200" already exists in activeOrchestrators
- Index query returns (JobID="job1", ScheduledAt=12:00:00 UTC)
- generateRunID produces same runID: "job1:1704067200"
- Map lookup finds existing orchestrator (O(1) operation)
- Orchestrator is NOT launched again
- activeOrchestrators still has only 1 entry

```go
func TestScheduler_ScheduleOrchestrators_GracePeriodOverlap(t *testing.T)
```
- Scenario: IndexRebuildInterval=10s, GracePeriod=30s
- Time 12:00:00: Job scheduled and starts
- Time 12:00:05: Job completes (stays in activeOrchestrators until 12:00:30)
- Time 12:00:10: Index rebuilds with window (11:59:40, 12:10:10) - includes 12:00:00 run
- Next iteration queries index and finds 12:00:00 run again
- Verify orchestrator NOT launched again (deduplication works)
- Only 1 orchestrator exists for this job/time

```go
func TestScheduler_ScheduleOrchestrators_BuffersUpdates(t *testing.T)
```
- Schedule 5 orchestrators
- Verify 5 JobRunUpdate calls to syncer.BufferJobRunUpdate
- Each update has status "prerun"
- Each has unique UpdateID

```go
func TestScheduler_ScheduleOrchestrators_BufferOverflow(t *testing.T)
```
- Mock syncer that returns error on BufferJobRunUpdate
- Schedule orchestrator
- Iteration returns error
- Shutdown signal sent

### Message Processing Tests
```go
func TestScheduler_ProcessInbox_EmptyInbox(t *testing.T)
```
- Empty inbox
- processInbox() returns 0

```go
func TestScheduler_ProcessInbox_MultipleMessages(t *testing.T)
```
- Send 10 messages to inbox
- Call processInbox()
- Returns 10
- All messages handled

```go
func TestScheduler_HandleOrchestratorStateChange(t *testing.T)
```
- Create orchestrator in PreRun state
- Send MsgOrchestratorStateChange to Pending
- Verify state.Status = OrchestratorPending
- Verify JobRunUpdate buffered

```go
func TestScheduler_HandleOrchestratorComplete(t *testing.T)
```
- Create running orchestrator
- Send MsgOrchestratorComplete
- Verify state.Status = OrchestratorCompleted
- Verify state.CompletedAt set
- Verify JobRunUpdate buffered with success=true

```go
func TestScheduler_HandleOrchestratorFailed(t *testing.T)
```
- Create running orchestrator
- Send MsgOrchestratorFailed with error
- Verify state.Status = OrchestratorFailed
- Verify JobRunUpdate buffered with success=false and error

```go
func TestScheduler_HandleCancelRun(t *testing.T)
```
- Create orchestrator
- Send MsgCancelRun
- Verify state.CancelChan closed
- Verify state.Status = OrchestratorCancelled

```go
func TestScheduler_HandleCancelRun_NonExistent(t *testing.T)
```
- Send MsgCancelRun for non-existent runID
- Logs warning
- No panic

```go
func TestScheduler_HandleUpdateRunConfig_PreRun(t *testing.T)
```
- Create orchestrator in PreRun state
- Send MsgUpdateRunConfig with new Job
- Verify new config sent to state.ConfigUpdate channel
- Verify state.JobConfig updated

```go
func TestScheduler_HandleUpdateRunConfig_NotPreRun(t *testing.T)
```
- Create orchestrator in Running state
- Send MsgUpdateRunConfig
- Logs warning
- Config not updated

```go
func TestScheduler_HandleGetOrchestratorState(t *testing.T)
```
- Create orchestrator
- Send MsgGetOrchestratorState with response channel
- Verify response contains state and found=true

```go
func TestScheduler_HandleGetAllActiveRuns(t *testing.T)
```
- Create 5 orchestrators
- Send MsgGetAllActiveRuns with response channel
- Verify response contains all 5 states

```go
func TestScheduler_HandleGetStats(t *testing.T)
```
- Create scheduler with 3 orchestrators
- Index with 100 runs
- Send MsgGetStats with response channel
- Verify response contains:
  - SchedulerStats.ActiveOrchestratorCount = 3
  - SchedulerStats.IndexSize = 100
  - InboxStats
  - SyncerStats

```go
func TestScheduler_HandleShutdown(t *testing.T)
```
- Send MsgShutdown
- Verify shutdown channel closed

### Heartbeat Monitoring Tests
```go
func TestScheduler_CheckHeartbeats_NoMissed(t *testing.T)
```
- Create orchestrator with LastHeartbeat = now - 5 seconds
- HeartbeatInterval = 10 seconds
- Run checkHeartbeats(now)
- No missed heartbeats
- Status unchanged

```go
func TestScheduler_CheckHeartbeats_OneMissed(t *testing.T)
```
- Create orchestrator with LastHeartbeat = now - 15 seconds
- HeartbeatInterval = 10 seconds
- Run checkHeartbeats(now)
- MissedHeartbeats = 1
- Warning logged
- Status unchanged (not orphaned yet)

```go
func TestScheduler_CheckHeartbeats_Orphaned(t *testing.T)
```
- Create orchestrator with LastHeartbeat = now - 60 seconds
- MissedHeartbeats = 4 (one more will trigger orphan)
- MaxMissedHeartbeats = 5
- HeartbeatInterval = 10 seconds
- Run checkHeartbeats(now)
- MissedHeartbeats = 5
- Status = OrchestratorOrphaned
- Error logged
- JobRunUpdate buffered with orphaned status

```go
func TestScheduler_CheckHeartbeats_SkipsCompleted(t *testing.T)
```
- Create orchestrator with Status = OrchestratorCompleted
- LastHeartbeat is very old
- Run checkHeartbeats(now)
- No changes (completed orchestrators not checked)

```go
func TestScheduler_HandleHeartbeat_ResetsCounter(t *testing.T)
```
- Create orchestrator with MissedHeartbeats = 3
- Send MsgOrchestratorHeartbeat
- Verify MissedHeartbeats = 0
- Verify LastHeartbeat updated to message timestamp

### Cleanup Tests
```go
func TestScheduler_CleanupOrchestrators_NoCleanup(t *testing.T)
```
- Create completed orchestrator
- ScheduledAt = now - 10 seconds
- GracePeriod = 30 seconds
- Run cleanupOrchestrators(now)
- Orchestrator NOT removed (grace period not expired)

```go
func TestScheduler_CleanupOrchestrators_AfterGrace(t *testing.T)
```
- Create completed orchestrator
- ScheduledAt = now - 40 seconds
- GracePeriod = 30 seconds
- Run cleanupOrchestrators(now)
- Orchestrator removed from map

```go
func TestScheduler_CleanupOrchestrators_MultipleStates(t *testing.T)
```
- Create 4 orchestrators, all past grace period:
  - OrchestratorCompleted
  - OrchestratorFailed
  - OrchestratorCancelled
  - OrchestratorOrphaned
- Run cleanupOrchestrators
- All 4 removed

```go
func TestScheduler_CleanupOrchestrators_SkipsActive(t *testing.T)
```
- Create 3 orchestrators past grace period:
  - OrchestratorPreRun
  - OrchestratorPending
  - OrchestratorRunning
- Run cleanupOrchestrators
- None removed (still active)

### Stats Recording Tests
```go
func TestScheduler_RecordIterationStats(t *testing.T)
```
- Create scheduler with 5 orchestrators
- Index with 200 runs
- Inbox depth = 3
- Process 7 messages
- Run recordIterationStats(start, 7)
- Verify syncer.BufferStats called with:
  - Timestamp = start
  - ActiveOrchestratorCount = 5
  - IndexSize = 200
  - InboxDepth = 3
  - MessagesProcessed = 7
  - IterationDuration > 0

### Full Iteration Tests
```go
func TestScheduler_Iteration_AllSteps(t *testing.T)
```
- Mock index with 2 runs
- Run single iteration
- Verify steps executed in order:
  1. scheduleOrchestrators
  2. processInbox
  3. checkHeartbeats
  4. cleanupOrchestrators
  5. recordIterationStats
  6. inbox.UpdateDepthStats

```go
func TestScheduler_Iteration_BufferOverflow(t *testing.T)
```
- Mock syncer that fails on buffer
- Run iteration
- Verify shutdown signal sent
- Loop exits

## Orchestrator Tests

### PreRun State Tests
```go
func TestOrchestrator_WaitsForScheduledTime(t *testing.T)
```
- Create orchestrator with scheduledAt = now + 200ms
- Start orchestrator
- Verify it waits approximately 200ms
- Then transitions to Pending
- Sends MsgOrchestratorStateChange

```go
func TestOrchestrator_ConfigUpdate_InPreRun(t *testing.T)
```
- Create orchestrator with scheduledAt = now + 500ms
- Start orchestrator
- After 100ms, send new config via configUpdate channel
- Verify orchestrator receives config
- Continues waiting for scheduled time

```go
func TestOrchestrator_Cancel_InPreRun(t *testing.T)
```
- Create orchestrator with scheduledAt = now + 500ms
- Start orchestrator
- After 100ms, close cancelChan
- Verify orchestrator exits immediately
- Sends MsgOrchestratorComplete with success=false, error="cancelled in PreRun"

```go
func TestOrchestrator_ScheduledTimeInPast(t *testing.T)
```
- Create orchestrator with scheduledAt = now - 1 second
- Start orchestrator
- Immediately transitions to Pending (no wait)

### Heartbeat Tests
```go
func TestOrchestrator_SendsHeartbeats(t *testing.T)
```
- Create orchestrator with HeartbeatInterval = 100ms
- Start orchestrator
- Collect heartbeat messages from inbox
- Verify heartbeats sent at ~100ms intervals
- Each heartbeat has correct runID and timestamp

```go
func TestOrchestrator_StopsHeartbeatsOnCompletion(t *testing.T)
```
- Create orchestrator that completes quickly
- Orchestrator completes and closes heartbeatDone channel
- Wait 500ms
- Verify no more heartbeats sent after completion

### State Transition Tests
```go
func TestOrchestrator_PreRunToPending(t *testing.T)
```
- Create orchestrator
- Wait for scheduled time
- Verify MsgOrchestratorStateChange sent with NewStatus=OrchestratorPending

```go
func TestOrchestrator_Completion(t *testing.T)
```
- Orchestrator stub simulates work
- Completes
- Verify MsgOrchestratorComplete sent
- Contains runID, success=true, completedAt, error=nil

## Shutdown Tests

### Graceful Shutdown
```go
func TestScheduler_Shutdown_CancelsOrchestrators(t *testing.T)
```
- Start scheduler
- Create 3 active orchestrators (PreRun, Pending, Running)
- Call Shutdown()
- Verify all 3 orchestrators' CancelChan closed
- Verify orchestrators exit

```go
func TestScheduler_Shutdown_ShutsSyncer(t *testing.T)
```
- Start scheduler
- Buffer updates in syncer
- Call Shutdown()
- Verify syncer.Shutdown() called
- Verify all updates flushed to DB

```go
func TestScheduler_Shutdown_ClosesChannels(t *testing.T)
```
- Start scheduler
- Call Shutdown()
- Verify rebuildIndexChan closed
- Verify index builder goroutine exits

```go
func TestScheduler_Shutdown_ViaMessage(t *testing.T)
```
- Start scheduler
- Send MsgShutdown to inbox
- Verify handleShutdown() called
- Full shutdown sequence executes

## Integration Tests

### End-to-End Scheduling
```go
func TestIntegration_ScheduleAndExecuteJob(t *testing.T)
```
- Create real scheduler with in-memory DB
- Insert job with cron "*/5 * * * *" (every 5 minutes)
- Mock time to be 12:04:55 (5 seconds before trigger)
- Start scheduler
- Wait for orchestrator to be scheduled
- Wait for orchestrator to transition states
- Verify:
  - Orchestrator created in PreRun
  - Transitions to Pending at 12:05:00
  - Heartbeats sent
  - Completes
  - JobRunUpdate written to DB
  - Orchestrator cleaned up after grace period

```go
func TestIntegration_MultipleJobsOverlapping(t *testing.T)
```
- Create 3 jobs with different schedules
- All scheduled to run within 10 seconds
- Start scheduler
- Verify all 3 orchestrators launched
- All complete successfully
- No conflicts

```go
func TestIntegration_HeartbeatOrphaning(t *testing.T)
```
- Start scheduler with MaxMissedHeartbeats = 2
- Create orchestrator
- Mock orchestrator to stop sending heartbeats
- Wait for 3 heartbeat intervals
- Verify orchestrator marked as orphaned
- Verify orphaned update written to DB

```go
func TestIntegration_IndexRebuild_NewJobAppears(t *testing.T)
```
- Start scheduler
- Initial index has 5 jobs
- After 2 seconds, insert new job to DB
- Trigger index rebuild
- New job appears in index
- Orchestrator scheduled for new job

```go
func TestIntegration_ConcurrentWrites(t *testing.T)
```
- Start scheduler
- Schedule 100 jobs to run simultaneously
- All complete at similar times
- Verify no data races (run with -race)
- All updates written to DB

```go
func TestIntegration_ShutdownWithActiveJobs(t *testing.T)
```
- Start scheduler
- Launch 10 orchestrators
- Immediately call Shutdown()
- Verify all orchestrators cancelled
- Verify all pending updates flushed
- Clean exit

## Concurrent Access Tests

### Race Detection
```go
func TestScheduler_ConcurrentStateReads(t *testing.T)
```
- Start scheduler with 50 orchestrators
- Launch 10 goroutines reading state via MsgGetAllActiveRuns
- No data races (run with -race flag)

```go
func TestScheduler_ConcurrentMessageSending(t *testing.T)
```
- Start scheduler
- Launch 20 goroutines sending various messages to inbox
- All messages processed
- No data races

```go
func TestSyncer_ConcurrentBuffering(t *testing.T)
```
- Start syncer
- Launch 10 goroutines buffering updates
- No data races
- All updates eventually flushed

## Benchmark Tests

### Throughput Benchmarks
```go
func BenchmarkScheduler_ScheduleOrchestrators(b *testing.B)
```
- Measure time to schedule N orchestrators
- Test with N = 10, 100, 1000, 10000
- Report orchestrators/second

```go
func BenchmarkScheduler_ProcessInbox(b *testing.B)
```
- Pre-fill inbox with N messages
- Measure time to process all
- Test with N = 100, 1000, 10000
- Report messages/second

```go
func BenchmarkSyncer_BufferAndFlush(b *testing.B)
```
- Measure time to buffer N updates and flush
- Test with N = 100, 1000, 10000
- Report updates/second

### Latency Benchmarks
```go
func BenchmarkScheduler_SingleIteration(b *testing.B)
```
- Measure time for single iteration
- With varying numbers of active orchestrators (0, 10, 100, 1000)
- With varying inbox depths (0, 10, 100)
- Report p50, p95, p99 latencies

```go
func BenchmarkInbox_SendReceive(b *testing.B)
```
- Measure round-trip time for send/receive
- Single goroutine
- Report latency in microseconds

```go
func BenchmarkOrchestrator_Lifecycle(b *testing.B)
```
- Measure time from orchestrator creation to completion
- Includes all state transitions
- Report total lifecycle time

### Memory Benchmarks
```go
func BenchmarkScheduler_MemoryUsage(b *testing.B)
```
- Create scheduler with N active orchestrators
- Test with N = 100, 1000, 10000
- Report memory usage per orchestrator

```go
func BenchmarkSyncer_BufferMemory(b *testing.B)
```
- Buffer N updates
- Test with N = 1000, 10000, 100000
- Report memory per update

## Test Helpers

### Mock Database
```go
type MockDB struct {
    jobs          []*Job
    writtenUpdates []JobRunUpdate
    writtenStats   []StatsUpdate
    queryError    error
    writeError    error
}

func (m *MockDB) QueryJobDefinitions() ([]*Job, error)
func (m *MockDB) WriteJobRunUpdate(update JobRunUpdate) error
func (m *MockDB) WriteStatsUpdate(update StatsUpdate) error
```

### Mock Time
```go
type MockClock struct {
    current time.Time
}

func (m *MockClock) Now() time.Time
func (m *MockClock) Advance(d time.Duration)
```

### Test Utilities
```go
func createTestScheduler(t *testing.T, config SchedulerConfig) *Scheduler
func createTestOrchestrator(t *testing.T, scheduledAt time.Time) *OrchestratorState
func waitForOrchestrator(t *testing.T, s *Scheduler, runID string, timeout time.Duration) *OrchestratorState
func collectMessages(t *testing.T, inbox *Inbox, count int, timeout time.Duration) []InboxMessage
```

## Success Criteria

Tests must:
- ✅ All unit tests pass
- ✅ All integration tests pass
- ✅ No data races detected with `go test -race`
- ✅ Code coverage > 85%
- ✅ All benchmarks complete without errors
- ✅ Deterministic runID generation works correctly (same job + time = same runID)
- ✅ No duplicate orchestrators can be created (O(1) map lookup prevents it)
- ✅ Performance targets met:
  - Single iteration < 10ms for 1000 orchestrators
  - Inbox send/receive < 100µs
  - Schedule 10,000 orchestrators < 100ms
  - Process 10,000 messages < 50ms
  - Buffer and flush 10,000 updates < 100ms
