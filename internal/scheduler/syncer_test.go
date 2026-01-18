package scheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

// =============================================================================
// Job Run Update Buffering Tests
// =============================================================================

// TestSyncer_BufferJobRunUpdate_BelowThreshold verifies that updates are buffered without flushing when below threshold.
func TestSyncer_BufferJobRunUpdate_BelowThreshold(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	config.JobRunFlushThreshold = 500
	syncer := NewSyncer(config, logger)

	// Buffer 10 updates (below threshold)
	for i := 0; i < 10; i++ {
		update := JobRunUpdate{
			UpdateID: fmt.Sprintf("update-%d", i),
			RunID:    fmt.Sprintf("run-%d", i),
		}
		err := syncer.BufferJobRunUpdate(update)
		if err != nil {
			t.Fatalf("unexpected error buffering update: %v", err)
		}
	}

	stats := syncer.GetStats()
	if stats.BufferedJobRunUpdates != 10 {
		t.Errorf("expected 10 buffered updates, got %d", stats.BufferedJobRunUpdates)
	}

	// No flush should be requested
	select {
	case <-syncer.jobRunFlushRequest:
		t.Error("unexpected flush request")
	default:
		// Expected - no flush
	}
}

// TestSyncer_BufferJobRunUpdate_ReachesThreshold verifies that a flush is triggered when buffer reaches threshold.
func TestSyncer_BufferJobRunUpdate_ReachesThreshold(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	config.JobRunFlushThreshold = 500
	syncer := NewSyncer(config, logger)

	// Buffer exactly 500 updates (threshold)
	for i := 0; i < 500; i++ {
		update := JobRunUpdate{
			UpdateID: fmt.Sprintf("update-%d", i),
			RunID:    fmt.Sprintf("run-%d", i),
		}
		err := syncer.BufferJobRunUpdate(update)
		if err != nil {
			t.Fatalf("unexpected error buffering update: %v", err)
		}
	}

	// Flush should be requested
	select {
	case <-syncer.jobRunFlushRequest:
		// Expected - flush requested
	case <-time.After(100 * time.Millisecond):
		t.Error("expected flush request but none received")
	}
}

// TestSyncer_BufferJobRunUpdate_ExceedsMaximum verifies that an error is returned when buffer exceeds maximum size.
func TestSyncer_BufferJobRunUpdate_ExceedsMaximum(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	config.MaxBufferedJobRunUpdates = 100
	syncer := NewSyncer(config, logger)

	// Buffer 101 updates (exceeds maximum)
	var err error
	for i := 0; i < 101; i++ {
		update := JobRunUpdate{
			UpdateID: fmt.Sprintf("update-%d", i),
			RunID:    fmt.Sprintf("run-%d", i),
		}
		err = syncer.BufferJobRunUpdate(update)
		if err != nil {
			break
		}
	}

	if err == nil {
		t.Error("expected error when exceeding maximum buffer size")
	}

	if err != nil && err.Error() != fmt.Sprintf("job run update buffer exceeded maximum size: 101 > 100") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// =============================================================================
// Stats Buffering Tests
// =============================================================================

// TestSyncer_BufferStats_BelowThreshold verifies that stats are buffered without flushing when below threshold.
func TestSyncer_BufferStats_BelowThreshold(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	config.StatsFlushThreshold = 30
	syncer := NewSyncer(config, logger)

	// Buffer 10 stats (below threshold)
	for i := 0; i < 10; i++ {
		stats := SchedulerIterationStats{
			Timestamp:               time.Now(),
			ActiveOrchestratorCount: i,
		}
		syncer.BufferStats(stats)
	}

	syncerStats := syncer.GetStats()
	if syncerStats.BufferedStats != 10 {
		t.Errorf("expected 10 buffered stats, got %d", syncerStats.BufferedStats)
	}

	// No flush should be requested
	select {
	case <-syncer.statsFlushRequest:
		t.Error("unexpected flush request")
	default:
		// Expected - no flush
	}
}

// TestSyncer_BufferStats_ReachesThreshold verifies that a flush is triggered when stats buffer reaches threshold.
func TestSyncer_BufferStats_ReachesThreshold(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	config.StatsFlushThreshold = 30
	syncer := NewSyncer(config, logger)

	// Buffer exactly 30 stats (threshold)
	for i := 0; i < 30; i++ {
		stats := SchedulerIterationStats{
			Timestamp:               time.Now(),
			ActiveOrchestratorCount: i,
		}
		syncer.BufferStats(stats)
	}

	// Flush should be requested
	select {
	case <-syncer.statsFlushRequest:
		// Expected - flush requested
	case <-time.After(100 * time.Millisecond):
		t.Error("expected flush request but none received")
	}
}

// =============================================================================
// Flushing Tests
// =============================================================================

// TestSyncer_FlushJobRunUpdates_Success verifies that buffered updates are flushed to the channel and cleared.
func TestSyncer_FlushJobRunUpdates_Success(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	config.JobRunChannelSize = 200
	syncer := NewSyncer(config, logger)

	mockDB := testutil.NewMockDB()
	syncer.Start(mockDB)
	defer syncer.Shutdown()

	// Buffer 100 updates
	for i := 0; i < 100; i++ {
		update := JobRunUpdate{
			UpdateID: fmt.Sprintf("update-%d", i),
			RunID:    fmt.Sprintf("run-%d", i),
		}
		syncer.BufferJobRunUpdate(update)
	}

	// Flush
	err := syncer.FlushJobRunUpdates()
	if err != nil {
		t.Fatalf("unexpected error flushing: %v", err)
	}

	// Buffer should be cleared
	stats := syncer.GetStats()
	if stats.BufferedJobRunUpdates != 0 {
		t.Errorf("expected buffer to be cleared, got %d", stats.BufferedJobRunUpdates)
	}

	// Wait for writes to complete
	time.Sleep(100 * time.Millisecond)

	// All updates should be written
	writtenCount := mockDB.CountWrittenUpdates()
	if writtenCount != 100 {
		t.Errorf("expected 100 written updates, got %d", writtenCount)
	}
}

// TestSyncer_FlushJobRunUpdates_ChannelFull verifies that flush returns an error when the channel is full.
func TestSyncer_FlushJobRunUpdates_ChannelFull(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	config.JobRunChannelSize = 10
	syncer := NewSyncer(config, logger)

	// Do NOT start syncer - no consumer for channel

	// Buffer 20 updates
	for i := 0; i < 20; i++ {
		update := JobRunUpdate{
			UpdateID: fmt.Sprintf("update-%d", i),
			RunID:    fmt.Sprintf("run-%d", i),
		}
		syncer.BufferJobRunUpdate(update)
	}

	// Flush will fill channel and then return error
	err := syncer.FlushJobRunUpdates()
	if err == nil {
		t.Error("expected error when channel is full")
	}

	// Some updates should remain buffered
	stats := syncer.GetStats()
	if stats.BufferedJobRunUpdates == 0 {
		t.Error("expected some updates to remain buffered")
	}
}

// TestSyncer_FlushStats_Success verifies that buffered stats are flushed to the channel and cleared.
func TestSyncer_FlushStats_Success(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	syncer := NewSyncer(config, logger)

	mockDB := testutil.NewMockDB()
	syncer.Start(mockDB)
	defer syncer.Shutdown()

	// Buffer 50 stats
	for i := 0; i < 50; i++ {
		stat := SchedulerIterationStats{
			Timestamp:               time.Now(),
			ActiveOrchestratorCount: i,
		}
		syncer.BufferStats(stat)
	}

	// Flush
	err := syncer.FlushStats()
	if err != nil {
		t.Fatalf("unexpected error flushing stats: %v", err)
	}

	// Buffer should be cleared
	stats := syncer.GetStats()
	if stats.BufferedStats != 0 {
		t.Errorf("expected stats buffer to be cleared, got %d", stats.BufferedStats)
	}

	// Wait for writes to complete
	time.Sleep(100 * time.Millisecond)

	// Stats should be written (as a batch)
	writtenCount := mockDB.CountWrittenStats()
	if writtenCount != 1 {
		t.Errorf("expected 1 stats batch written, got %d", writtenCount)
	}
}

// TestSyncer_FlushStats_Empty verifies that flushing an empty stats buffer succeeds without error.
func TestSyncer_FlushStats_Empty(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	syncer := NewSyncer(config, logger)

	// Flush empty buffer
	err := syncer.FlushStats()
	if err != nil {
		t.Errorf("unexpected error flushing empty stats: %v", err)
	}
}

// =============================================================================
// Time-Based Flushing Tests
// =============================================================================

// TestSyncer_JobRunFlusher_TimeBased verifies that updates are automatically flushed after the flush interval.
func TestSyncer_JobRunFlusher_TimeBased(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	config.JobRunFlushInterval = 100 * time.Millisecond
	config.JobRunFlushThreshold = 1000 // High threshold - won't trigger
	syncer := NewSyncer(config, logger)

	mockDB := testutil.NewMockDB()
	syncer.Start(mockDB)
	defer syncer.Shutdown()

	// Buffer 10 updates (below threshold)
	for i := 0; i < 10; i++ {
		update := JobRunUpdate{
			UpdateID: fmt.Sprintf("update-%d", i),
			RunID:    fmt.Sprintf("run-%d", i),
		}
		syncer.BufferJobRunUpdate(update)
	}

	// Wait for time-based flush
	time.Sleep(200 * time.Millisecond)

	// Updates should be flushed and written
	testutil.WaitFor(t, func() bool {
		return mockDB.CountWrittenUpdates() == 10
	}, 1*time.Second, "waiting for time-based flush")
}

// TestSyncer_StatsFlusher_TimeBased verifies that stats are automatically flushed after the flush interval.
func TestSyncer_StatsFlusher_TimeBased(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	config.StatsFlushInterval = 100 * time.Millisecond
	config.StatsFlushThreshold = 1000 // High threshold - won't trigger
	syncer := NewSyncer(config, logger)

	mockDB := testutil.NewMockDB()
	syncer.Start(mockDB)
	defer syncer.Shutdown()

	// Buffer 5 stats (below threshold)
	for i := 0; i < 5; i++ {
		stat := SchedulerIterationStats{
			Timestamp:               time.Now(),
			ActiveOrchestratorCount: i,
		}
		syncer.BufferStats(stat)
	}

	// Wait for time-based flush
	time.Sleep(200 * time.Millisecond)

	// Stats should be flushed and written
	testutil.WaitFor(t, func() bool {
		return mockDB.CountWrittenStats() > 0
	}, 1*time.Second, "waiting for time-based stats flush")
}

// =============================================================================
// Database Syncer Tests
// =============================================================================

// TestSyncer_JobRunSyncer_WriteSuccess verifies that job run updates are successfully written to the database.
func TestSyncer_JobRunSyncer_WriteSuccess(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	syncer := NewSyncer(config, logger)

	mockDB := testutil.NewMockDB()
	syncer.Start(mockDB)
	defer syncer.Shutdown()

	// Send update to channel
	update := JobRunUpdate{
		UpdateID: "update-1",
		RunID:    "run-1",
	}
	syncer.jobRunChannel <- update

	// Wait for write
	time.Sleep(100 * time.Millisecond)

	// Verify written
	if mockDB.CountWrittenUpdates() != 1 {
		t.Error("expected update to be written")
	}

	// Verify debug log
	entries := logger.GetEntriesByLevel("DEBUG")
	found := false
	for _, entry := range entries {
		if entry.Message == "wrote job run update" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected debug log for successful write")
	}
}

// TestSyncer_JobRunSyncer_WriteFailure verifies that database write errors are logged appropriately.
func TestSyncer_JobRunSyncer_WriteFailure(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	syncer := NewSyncer(config, logger)

	mockDB := testutil.NewMockDB()
	mockDB.SetWriteError(fmt.Errorf("database error"))
	syncer.Start(mockDB)
	defer syncer.Shutdown()

	// Send update to channel
	update := JobRunUpdate{
		UpdateID: "update-1",
		RunID:    "run-1",
	}
	syncer.jobRunChannel <- update

	// Wait for write attempt
	time.Sleep(100 * time.Millisecond)

	// Verify error logged
	if !logger.HasError() {
		t.Error("expected error to be logged for failed write")
	}

	entries := logger.GetEntriesByLevel("ERROR")
	found := false
	for _, entry := range entries {
		if entry.Message == "failed to write job run update" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected specific error log for failed write")
	}
}

// TestSyncer_StatsSyncer_WriteFailure verifies that stats database write errors are logged appropriately.
func TestSyncer_StatsSyncer_WriteFailure(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	syncer := NewSyncer(config, logger)

	mockDB := testutil.NewMockDB()
	mockDB.SetWriteError(fmt.Errorf("database error"))
	syncer.Start(mockDB)
	defer syncer.Shutdown()

	// Send stats to channel
	statsUpdate := StatsUpdate{
		Stats: []SchedulerIterationStats{
			{
				Timestamp:               time.Now(),
				ActiveOrchestratorCount: 1,
			},
		},
	}
	syncer.statsChannel <- statsUpdate

	// Wait for write attempt
	time.Sleep(100 * time.Millisecond)

	// Verify error logged
	if !logger.HasError() {
		t.Error("expected error to be logged for failed stats write")
	}
}

// =============================================================================
// Shutdown Tests
// =============================================================================

// TestSyncer_Shutdown_FlushesAll verifies that all buffered data is flushed during shutdown.
func TestSyncer_Shutdown_FlushesAll(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	syncer := NewSyncer(config, logger)

	mockDB := testutil.NewMockDB()
	syncer.Start(mockDB)

	// Buffer 100 job run updates
	for i := 0; i < 100; i++ {
		update := JobRunUpdate{
			UpdateID: fmt.Sprintf("update-%d", i),
			RunID:    fmt.Sprintf("run-%d", i),
		}
		syncer.BufferJobRunUpdate(update)
	}

	// Buffer 10 stats
	for i := 0; i < 10; i++ {
		stat := SchedulerIterationStats{
			Timestamp:               time.Now(),
			ActiveOrchestratorCount: i,
		}
		syncer.BufferStats(stat)
	}

	// Shutdown should flush everything
	err := syncer.Shutdown()
	if err != nil {
		t.Fatalf("unexpected error during shutdown: %v", err)
	}

	// Verify all updates written
	if mockDB.CountWrittenUpdates() != 100 {
		t.Errorf("expected 100 updates written, got %d", mockDB.CountWrittenUpdates())
	}

	// Verify stats written
	if mockDB.CountWrittenStats() != 1 {
		t.Errorf("expected 1 stats batch written, got %d", mockDB.CountWrittenStats())
	}
}

// TestSyncer_Shutdown_DrainChannels verifies that shutdown drains all pending channel items before exiting.
func TestSyncer_Shutdown_DrainChannels(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	config.JobRunChannelSize = 100
	syncer := NewSyncer(config, logger)

	mockDB := testutil.NewMockDB()
	mockDB.SetWriteDelay(10 * time.Millisecond) // Slow writes
	syncer.Start(mockDB)

	// Send 50 updates directly to channel (bypassing buffer)
	for i := 0; i < 50; i++ {
		update := JobRunUpdate{
			UpdateID: fmt.Sprintf("update-%d", i),
			RunID:    fmt.Sprintf("run-%d", i),
		}
		syncer.jobRunChannel <- update
	}

	// Shutdown immediately
	// Channel drain should process all 50 updates
	err := syncer.Shutdown()
	if err != nil {
		t.Fatalf("unexpected error during shutdown: %v", err)
	}

	// All 50 should be processed
	if mockDB.CountWrittenUpdates() != 50 {
		t.Errorf("expected 50 updates processed, got %d", mockDB.CountWrittenUpdates())
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestSyncer_ConcurrentBuffering verifies that concurrent buffering operations are thread-safe.
func TestSyncer_ConcurrentBuffering(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSyncerConfig()
	syncer := NewSyncer(config, logger)

	mockDB := testutil.NewMockDB()
	syncer.Start(mockDB)
	defer syncer.Shutdown()

	// Launch 10 goroutines buffering updates concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				update := JobRunUpdate{
					UpdateID: fmt.Sprintf("update-%d-%d", id, j),
					RunID:    fmt.Sprintf("run-%d-%d", id, j),
				}
				syncer.BufferJobRunUpdate(update)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Flush all
	syncer.FlushJobRunUpdates()

	// Wait for writes
	time.Sleep(500 * time.Millisecond)

	// All 1000 updates should eventually be written
	testutil.WaitFor(t, func() bool {
		return mockDB.CountWrittenUpdates() == 1000
	}, 2*time.Second, "waiting for all updates to be written")
}
