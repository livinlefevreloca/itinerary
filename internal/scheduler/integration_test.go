package scheduler

import (
	"testing"
	"time"

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

	// Round to next minute for cron scheduling
	now := time.Now()
	futureTime := now.Truncate(time.Minute).Add(time.Minute)

	// Create cron expression for that exact minute
	// Format uses Go time format constants: 4=minute, 15=hour, 2=day, 1=month, *=any day-of-week
	cronExpr := futureTime.Format("4 15 2 1 *")

	mockDB.SetJobs([]*testutil.Job{
		{ID: "job1", Schedule: cronExpr},
	})

	config := DefaultSchedulerConfig()
	config.PreScheduleInterval = 30 * time.Second
	config.LookaheadWindow = 5 * time.Minute
	config.GracePeriod = 10 * time.Second
	config.LoopInterval = 500 * time.Millisecond

	syncerConfig := DefaultSyncerConfig()

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger)
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	// Start scheduler
	go scheduler.Start(mockDB)
	defer scheduler.Shutdown()

	// Wait for job to be scheduled and executed
	testutil.WaitFor(t, func() bool {
		return mockDB.CountWrittenUpdates() > 0
	}, 3*time.Second, "waiting for job updates")

	// Verify job run was recorded
	updates := mockDB.GetWrittenUpdates()
	if len(updates) == 0 {
		t.Fatal("expected job run updates")
	}

	// Verify update contains expected job
	foundJob1 := false
	for _, update := range updates {
		if update.JobID == "job1" {
			foundJob1 = true
			break
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

	// Create 3 jobs all scheduled for same time in the future
	now := time.Now()
	futureTime := now.Truncate(time.Minute).Add(time.Minute)
	// Format uses Go time format constants: 4=minute, 15=hour, 2=day, 1=month, *=any day-of-week
	cronExpr := futureTime.Format("4 15 2 1 *")

	mockDB.SetJobs([]*testutil.Job{
		{ID: "job1", Schedule: cronExpr},
		{ID: "job2", Schedule: cronExpr},
		{ID: "job3", Schedule: cronExpr},
	})

	config := DefaultSchedulerConfig()
	config.PreScheduleInterval = 30 * time.Second
	config.LookaheadWindow = 5 * time.Minute
	config.LoopInterval = 500 * time.Millisecond

	syncerConfig := DefaultSyncerConfig()

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger)
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	go scheduler.Start(mockDB)
	defer scheduler.Shutdown()

	// Wait for all jobs to be scheduled
	testutil.WaitFor(t, func() bool {
		return mockDB.CountWrittenUpdates() >= 3
	}, 3*time.Second, "waiting for all job updates")

	// Verify all 3 jobs were recorded
	updates := mockDB.GetWrittenUpdates()
	jobsSeen := make(map[string]bool)

	for _, update := range updates {
		jobsSeen[update.JobID] = true
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

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger)
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

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

	// Orphaned update should be recorded
	foundOrphaned := false
	updates := mockDB.GetWrittenUpdates()
	for _, update := range updates {
		if update.Status == "orphaned" {
			foundOrphaned = true
			break
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

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger)
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	// Create 10 orchestrators
	for i := 0; i < 10; i++ {
		runID := testutil.GenerateRunID(fmt.Sprintf("job%d", i), time.Now())
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
		update := JobRunUpdate{
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

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger)
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	scheduler.syncer.Start(mockDB)
	defer scheduler.syncer.Shutdown()

	// Launch 10 goroutines simulating concurrent job completions
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				runID := fmt.Sprintf("job%d-run%d:%d", id, j, time.Now().Unix())

				msg := InboxMessage{
					Type: MsgOrchestratorComplete,
					Data: OrchestratorCompleteMsg{
						RunID:       runID,
						Success:     true,
						CompletedAt: time.Now(),
					},
				}

				// Create state
				state := &OrchestratorState{
					RunID:  runID,
					JobID:  fmt.Sprintf("job%d", id),
					Status: OrchestratorRunning,
				}
				scheduler.activeOrchestrators[runID] = state

				// Handle message
				scheduler.handleMessage(msg)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Flush all updates
	scheduler.syncer.FlushJobRunUpdates()

	// Wait for writes
	testutil.WaitFor(t, func() bool {
		return mockDB.CountWrittenUpdates() == 100
	}, 3*time.Second, "waiting for all writes")

	// Verify all 100 writes completed
	if mockDB.CountWrittenUpdates() != 100 {
		t.Errorf("expected 100 writes, got %d", mockDB.CountWrittenUpdates())
	}
}
