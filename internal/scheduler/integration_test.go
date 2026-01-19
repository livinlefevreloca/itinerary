package scheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/syncer"
	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

// =============================================================================
// Integration Tests
// =============================================================================

// TestIntegration_ScheduleAndExecuteJob verifies end-to-end scheduling and execution of a single job.
func TestIntegration_ScheduleAndExecuteJob(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := testutil.NewTestLogger()
	mockDB := testutil.NewMockDB()

	// Schedule job to run every minute so it executes soon
	mockDB.SetJobs([]*testutil.Job{
		{ID: "job1", Schedule: "* * * * *"}, // Every minute
	})

	config := DefaultSchedulerConfig()
	config.PreScheduleInterval = 30 * time.Second
	config.LookaheadWindow = 5 * time.Minute
	config.GracePeriod = 10 * time.Second
	config.LoopInterval = 500 * time.Millisecond

	syncerConfig := DefaultSyncerConfig()
	syncerConfig.JobRunFlushThreshold = 1 // Flush immediately for testing

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger.Logger())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	// Start scheduler
	go scheduler.Start(mockDB)
	defer scheduler.Shutdown()

	// Wait for job to be scheduled and executed
	// Job runs at next minute boundary, plus execution time (250ms), plus flush time
	testutil.WaitFor(t, func() bool {
		return mockDB.CountWrittenUpdates() > 0
	}, 90*time.Second, "waiting for job updates")

	// Verify job run was recorded
	updates := mockDB.GetWrittenUpdates()
	if len(updates) == 0 {
		t.Fatal("expected job run updates")
	}

	// Verify update contains expected job
	foundJob1 := false
	for _, update := range updates {
		if jobUpdate, ok := update.(syncer.JobRunUpdate); ok {
			if jobUpdate.JobID == "job1" {
				foundJob1 = true
				break
			}
		}
	}

	if !foundJob1 {
		t.Error("expected update for job1")
	}
}

// TestIntegration_MultipleJobsOverlapping verifies that multiple jobs scheduled for the same time execute concurrently.
func TestIntegration_MultipleJobsOverlapping(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := testutil.NewTestLogger()
	mockDB := testutil.NewMockDB()

	// Create 3 jobs all scheduled to run every minute
	mockDB.SetJobs([]*testutil.Job{
		{ID: "job1", Schedule: "* * * * *"}, // Every minute
		{ID: "job2", Schedule: "* * * * *"}, // Every minute
		{ID: "job3", Schedule: "* * * * *"}, // Every minute
	})

	config := DefaultSchedulerConfig()
	config.PreScheduleInterval = 30 * time.Second
	config.LookaheadWindow = 5 * time.Minute
	config.LoopInterval = 500 * time.Millisecond

	syncerConfig := DefaultSyncerConfig()
	syncerConfig.JobRunFlushThreshold = 1 // Flush immediately for testing

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger.Logger())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	go scheduler.Start(mockDB)
	defer scheduler.Shutdown()

	// Wait for all jobs to be scheduled and executed
	testutil.WaitFor(t, func() bool {
		return mockDB.CountWrittenUpdates() >= 3
	}, 90*time.Second, "waiting for all job updates")

	// Verify all 3 jobs were recorded
	updates := mockDB.GetWrittenUpdates()
	jobsSeen := make(map[string]bool)

	for _, update := range updates {
		if jobUpdate, ok := update.(syncer.JobRunUpdate); ok {
			jobsSeen[jobUpdate.JobID] = true
		}
	}

	if !jobsSeen["job1"] || !jobsSeen["job2"] || !jobsSeen["job3"] {
		t.Error("expected updates for all 3 jobs")
	}
}

// TestIntegration_HeartbeatOrphaning verifies that orchestrators without heartbeats are automatically marked as orphaned.
func TestIntegration_HeartbeatOrphaning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := testutil.NewTestLogger()
	mockDB := testutil.NewMockDB()

	config := DefaultSchedulerConfig()
	config.OrchestratorHeartbeatInterval = 100 * time.Millisecond
	config.MaxMissedOrchestratorHeartbeats = 2
	config.LoopInterval = 100 * time.Millisecond

	syncerConfig := DefaultSyncerConfig()

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger.Logger())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	// Start syncer to enable background writes
	scheduler.syncer.Start(mockDB)
	defer scheduler.syncer.Shutdown()

	// Manually create an orchestrator that won't send heartbeats
	runID := "job1:1704067200"
	state := &OrchestratorState{
		RunID:            runID,
		JobID:            "job1",
		Status:           OrchestratorRunning,
		ScheduledAt:      time.Now().Add(-1 * time.Minute),
		LastHeartbeat:    time.Now(),
		MissedHeartbeats: 0,
	}
	scheduler.activeOrchestrators[runID] = state

	// Run several iterations
	for i := 0; i < 10; i++ {
		scheduler.iteration()
		time.Sleep(150 * time.Millisecond) // Longer than heartbeat interval
	}

	// Orchestrator should be marked as orphaned
	if state.Status != OrchestratorOrphaned {
		t.Errorf("expected status to be Orphaned, got %v", state.Status)
	}

	// Flush updates and wait for writes
	scheduler.syncer.FlushJobRunUpdates()
	time.Sleep(100 * time.Millisecond)

	// Orphaned update should be recorded
	foundOrphaned := false
	updates := mockDB.GetWrittenUpdates()
	for _, update := range updates {
		if jobUpdate, ok := update.(syncer.JobRunUpdate); ok {
			if jobUpdate.Status == "orphaned" {
				foundOrphaned = true
				break
			}
		}
	}

	if !foundOrphaned {
		t.Error("expected orphaned status to be recorded")
	}
}

// TestIntegration_ShutdownWithActiveJobs verifies graceful shutdown with active orchestrators and buffered updates.
func TestIntegration_ShutdownWithActiveJobs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := testutil.NewTestLogger()
	mockDB := testutil.NewMockDB()

	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger.Logger())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	// Start syncer to enable background writes
	scheduler.syncer.Start(mockDB)

	// Create 10 orchestrators
	for i := 0; i < 10; i++ {
		runID := generateRunID(fmt.Sprintf("job%d", i), time.Now())
		state := &OrchestratorState{
			RunID:       runID,
			JobID:       fmt.Sprintf("job%d", i),
			Status:      OrchestratorRunning,
			ScheduledAt: time.Now(),
			CancelChan:  make(chan struct{}),
		}
		scheduler.activeOrchestrators[runID] = state
	}

	// Buffer some updates
	for i := 0; i < 50; i++ {
		update := syncer.JobRunUpdate{
			UpdateID: fmt.Sprintf("update-%d", i),
			RunID:    fmt.Sprintf("run-%d", i),
		}
		scheduler.syncer.BufferJobRunUpdate(update)
	}

	// Shutdown
	err = scheduler.handleShutdown()
	if err != nil {
		t.Errorf("unexpected error during shutdown: %v", err)
	}

	// All updates should be flushed
	testutil.WaitFor(t, func() bool {
		return mockDB.CountWrittenUpdates() >= 50
	}, 2*time.Second, "waiting for all updates to be written")

	// All orchestrators should have cancel signal
	for _, state := range scheduler.activeOrchestrators {
		select {
		case <-state.CancelChan:
			// Expected - channel closed
		default:
			t.Error("expected cancel channel to be closed")
		}
	}
}

// TestIntegration_ConcurrentWrites verifies that syncer correctly handles concurrent database writes from multiple goroutines.
func TestIntegration_ConcurrentWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := testutil.NewTestLogger()
	mockDB := testutil.NewMockDB()

	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger.Logger())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	scheduler.syncer.Start(mockDB)
	defer scheduler.syncer.Shutdown()

	// Pre-create orchestrators in the scheduler (main thread only)
	runIDs := make([]string, 0, 100)
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			runID := fmt.Sprintf("job%d-run%d:%d", i, j, time.Now().Unix())
			runIDs = append(runIDs, runID)

			state := &OrchestratorState{
				RunID:         runID,
				JobID:         fmt.Sprintf("job%d", i),
				Status:        OrchestratorRunning,
				ScheduledAt:   time.Now(),
				LastHeartbeat: time.Now(),
			}
			scheduler.activeOrchestrators[runID] = state
		}
	}

	// Launch 10 goroutines simulating concurrent job completions
	// Use inbox.Send() instead of direct handleMessage() calls to avoid races
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				runID := runIDs[id*10+j]

				// Send completion message
				completeMsg := InboxMessage{
					Type: MsgOrchestratorComplete,
					Data: OrchestratorCompleteMsg{
						RunID:       runID,
						Success:     true,
						CompletedAt: time.Now(),
					},
				}
				scheduler.inbox.Send(completeMsg)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish sending messages
	for i := 0; i < 10; i++ {
		<-done
	}

	// Process all messages in the inbox
	scheduler.processInbox()

	// Flush all updates
	scheduler.syncer.FlushJobRunUpdates()

	// Wait for writes
	testutil.WaitFor(t, func() bool {
		return mockDB.CountWrittenUpdates() >= 100
	}, 3*time.Second, "waiting for all writes")

	// Verify at least 100 writes completed (could be more due to state changes)
	if mockDB.CountWrittenUpdates() < 100 {
		t.Errorf("expected at least 100 writes, got %d", mockDB.CountWrittenUpdates())
	}
}
