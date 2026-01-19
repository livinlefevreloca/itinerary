package scheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/scheduler/index"
	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

// =============================================================================
// Benchmark Tests
// =============================================================================

// BenchmarkGenerateRunID measures runID generation performance.
func BenchmarkGenerateRunID(b *testing.B) {
	jobID := "job123"
	scheduledAt := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateRunID(jobID, scheduledAt)
	}
}

// BenchmarkInbox_SendReceive measures inbox send and receive throughput.
func BenchmarkInbox_SendReceive(b *testing.B) {
	logger := testutil.NewTestLogger()
	inbox := NewInbox(10000, 100*time.Millisecond, logger.Logger())

	msg := InboxMessage{
		Type: MsgShutdown,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inbox.Send(msg)
		inbox.TryReceive()
	}
}

// BenchmarkSyncer_BufferAndFlush_100 measures syncer throughput with 100 updates per batch.
func BenchmarkSyncer_BufferAndFlush_100(b *testing.B) {
	benchmarkSyncerBufferAndFlush(b, 100)
}

// BenchmarkSyncer_BufferAndFlush_1000 measures syncer throughput with 1000 updates per batch.
func BenchmarkSyncer_BufferAndFlush_1000(b *testing.B) {
	benchmarkSyncerBufferAndFlush(b, 1000)
}

// BenchmarkSyncer_BufferAndFlush_10000 measures syncer throughput with 10000 updates per batch.
func BenchmarkSyncer_BufferAndFlush_10000(b *testing.B) {
	benchmarkSyncerBufferAndFlush(b, 10000)
}

func benchmarkSyncerBufferAndFlush(b *testing.B, count int) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	config.JobRunChannelSize = count * 2
	syncer, _ := NewSyncer(config, logger.Logger())

	mockDB := testutil.NewMockDB()
	syncer.Start(mockDB)
	defer syncer.Shutdown()

	updates := make([]JobRunUpdate, count)
	for i := 0; i < count; i++ {
		updates[i] = JobRunUpdate{
			UpdateID: fmt.Sprintf("update-%d", i),
			RunID:    fmt.Sprintf("run-%d", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Buffer
		for j := 0; j < count; j++ {
			syncer.BufferJobRunUpdate(updates[j])
		}

		// Flush
		syncer.FlushJobRunUpdates()

		// Wait for writes
		for mockDB.CountWrittenUpdates() < count {
			time.Sleep(1 * time.Millisecond)
		}

		mockDB.ClearWritten()
	}
}

// BenchmarkScheduler_ProcessInbox_100 measures inbox processing performance with 100 messages.
func BenchmarkScheduler_ProcessInbox_100(b *testing.B) {
	benchmarkProcessInbox(b, 100)
}

// BenchmarkScheduler_ProcessInbox_1000 measures inbox processing performance with 1000 messages.
func BenchmarkScheduler_ProcessInbox_1000(b *testing.B) {
	benchmarkProcessInbox(b, 1000)
}

// BenchmarkScheduler_ProcessInbox_10000 measures inbox processing performance with 10000 messages.
func BenchmarkScheduler_ProcessInbox_10000(b *testing.B) {
	benchmarkProcessInbox(b, 10000)
}

func benchmarkProcessInbox(b *testing.B, messageCount int) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	config.InboxBufferSize = messageCount * 2
	syncerConfig := DefaultSyncerConfig()

	scheduler := createBenchScheduler(b, config, syncerConfig, logger)

	// Create a test orchestrator for heartbeat messages
	now := time.Now()
	runID := "benchmark-job:1704067200"
	scheduler.activeOrchestrators[runID] = &OrchestratorState{
		RunID:            runID,
		JobID:            "benchmark-job",
		Status:           OrchestratorRunning,
		ScheduledAt:      now,
		LastHeartbeat:    now,
		MissedHeartbeats: 0,
	}

	// Prepare messages - use heartbeat messages to avoid channel close panics
	messages := make([]InboxMessage, messageCount)
	for i := 0; i < messageCount; i++ {
		messages[i] = InboxMessage{
			Type: MsgOrchestratorHeartbeat,
			Data: OrchestratorHeartbeatMsg{
				RunID:     runID,
				Timestamp: now,
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Fill inbox
		for j := 0; j < messageCount; j++ {
			scheduler.inbox.Send(messages[j])
		}

		// Process
		scheduler.processInbox()
	}
}

// BenchmarkScheduler_ScheduleOrchestrators_10 measures orchestrator scheduling performance with 10 jobs.
func BenchmarkScheduler_ScheduleOrchestrators_10(b *testing.B) {
	benchmarkScheduleOrchestrators(b, 10)
}

// BenchmarkScheduler_ScheduleOrchestrators_100 measures orchestrator scheduling performance with 100 jobs.
func BenchmarkScheduler_ScheduleOrchestrators_100(b *testing.B) {
	benchmarkScheduleOrchestrators(b, 100)
}

// BenchmarkScheduler_ScheduleOrchestrators_1000 measures orchestrator scheduling performance with 1000 jobs.
func BenchmarkScheduler_ScheduleOrchestrators_1000(b *testing.B) {
	benchmarkScheduleOrchestrators(b, 1000)
}

func benchmarkScheduleOrchestrators(b *testing.B, count int) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()
	syncerConfig.MaxBufferedJobRunUpdates = count * 10

	scheduler := createBenchScheduler(b, config, syncerConfig, logger)

	// Create mock index with runs
	now := time.Now()
	runs := make([]index.ScheduledRun, count)
	for i := 0; i < count; i++ {
		runs[i] = index.ScheduledRun{
			JobID:       fmt.Sprintf("job%d", i),
			ScheduledAt: now.Add(time.Duration(i) * time.Second),
		}
	}

	// Mock index that returns these runs
	scheduler.index = index.NewScheduledRunIndex(runs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := scheduler.scheduleOrchestrators(now)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}

		// Clear for next iteration
		scheduler.activeOrchestrators = make(map[string]*OrchestratorState)
		scheduler.syncer.jobRunUpdateBuffer = make([]JobRunUpdate, 0)
	}
}

// BenchmarkScheduler_SingleIteration_Empty measures scheduler iteration performance with no active orchestrators.
func BenchmarkScheduler_SingleIteration_Empty(b *testing.B) {
	benchmarkSingleIteration(b, 0)
}

// BenchmarkScheduler_SingleIteration_10Orchestrators measures scheduler iteration performance with 10 active orchestrators.
func BenchmarkScheduler_SingleIteration_10Orchestrators(b *testing.B) {
	benchmarkSingleIteration(b, 10)
}

// BenchmarkScheduler_SingleIteration_100Orchestrators measures scheduler iteration performance with 100 active orchestrators.
func BenchmarkScheduler_SingleIteration_100Orchestrators(b *testing.B) {
	benchmarkSingleIteration(b, 100)
}

// BenchmarkScheduler_SingleIteration_1000Orchestrators measures scheduler iteration performance with 1000 active orchestrators.
func BenchmarkScheduler_SingleIteration_1000Orchestrators(b *testing.B) {
	benchmarkSingleIteration(b, 1000)
}

func benchmarkSingleIteration(b *testing.B, orchestratorCount int) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createBenchScheduler(b, config, syncerConfig, logger)

	// Create orchestrators
	now := time.Now()
	for i := 0; i < orchestratorCount; i++ {
		runID := fmt.Sprintf("job%d:%d", i, now.Unix())
		scheduler.activeOrchestrators[runID] = &OrchestratorState{
			RunID:            runID,
			JobID:            fmt.Sprintf("job%d", i),
			Status:           OrchestratorRunning,
			ScheduledAt:      now,
			LastHeartbeat:    now,
			MissedHeartbeats: 0,
		}
	}

	// Mock empty index
	scheduler.index = index.NewScheduledRunIndex([]index.ScheduledRun{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.iteration()
	}
}

// BenchmarkScheduler_CheckHeartbeats_1000 measures heartbeat checking performance with 1000 orchestrators.
func BenchmarkScheduler_CheckHeartbeats_1000(b *testing.B) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createBenchScheduler(b, config, syncerConfig, logger)

	// Create 1000 orchestrators
	now := time.Now()
	for i := 0; i < 1000; i++ {
		runID := fmt.Sprintf("job%d:%d", i, now.Unix())
		scheduler.activeOrchestrators[runID] = &OrchestratorState{
			RunID:            runID,
			JobID:            fmt.Sprintf("job%d", i),
			Status:           OrchestratorRunning,
			ScheduledAt:      now,
			LastHeartbeat:    now.Add(-5 * time.Second),
			MissedHeartbeats: 0,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.checkHeartbeats(now)
	}
}

// BenchmarkScheduler_CleanupOrchestrators_1000 measures cleanup performance with 1000 completed orchestrators.
func BenchmarkScheduler_CleanupOrchestrators_1000(b *testing.B) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	config.GracePeriod = 30 * time.Second
	syncerConfig := DefaultSyncerConfig()

	scheduler := createBenchScheduler(b, config, syncerConfig, logger)

	// Create 1000 completed orchestrators past grace period
	now := time.Now()
	pastTime := now.Add(-40 * time.Second)

	for i := 0; i < 1000; i++ {
		runID := fmt.Sprintf("job%d:%d", i, pastTime.Unix())
		scheduler.activeOrchestrators[runID] = &OrchestratorState{
			RunID:       runID,
			JobID:       fmt.Sprintf("job%d", i),
			Status:      OrchestratorCompleted,
			ScheduledAt: pastTime,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Make copy for each iteration
		if i > 0 {
			scheduler.activeOrchestrators = make(map[string]*OrchestratorState)
			for j := 0; j < 1000; j++ {
				runID := fmt.Sprintf("job%d:%d", j, pastTime.Unix())
				scheduler.activeOrchestrators[runID] = &OrchestratorState{
					RunID:       runID,
					JobID:       fmt.Sprintf("job%d", j),
					Status:      OrchestratorCompleted,
					ScheduledAt: pastTime,
				}
			}
		}

		scheduler.cleanupOrchestrators(now)
	}
}

// Mock index for benchmarking
type MockIndex struct {
	runs []index.ScheduledRun
}

func (m *MockIndex) Query(start, end time.Time) []index.ScheduledRun {
	return m.runs
}

func (m *MockIndex) Len() int {
	return len(m.runs)
}

func (m *MockIndex) Swap(runs []index.ScheduledRun) {
	m.runs = runs
}

// Helper for benchmarks
type BenchmarkT interface {
	Fatalf(format string, args ...interface{})
}

func createBenchScheduler(t BenchmarkT, config SchedulerConfig, syncerConfig SyncerConfig, logger *testutil.TestLogger) *Scheduler {
	inbox := NewInbox(config.InboxBufferSize, config.InboxSendTimeout, logger.Logger())
	syncer, _ := NewSyncer(syncerConfig, logger.Logger())

	return &Scheduler{
		config:              config,
		logger:  logger.Logger(),
		index:               nil,
		activeOrchestrators: make(map[string]*OrchestratorState),
		inbox:               inbox,
		syncer:              syncer,
		shutdown:            make(chan struct{}),
		rebuildIndexChan:    make(chan struct{}, 1),
	}
}
