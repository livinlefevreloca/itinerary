package scheduler

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

// =============================================================================
// RunID Generation Tests
// =============================================================================

// TestGenerateRunID_Deterministic verifies that runID generation is deterministic for deduplication.
// This is critical: same job+time must always produce the same runID to prevent duplicate runs.
func TestGenerateRunID_Deterministic(t *testing.T) {
	jobID := "job123"
	scheduledAt := time.Unix(1704067200, 0) // 2024-01-01 00:00:00 UTC

	runID1 := generateRunID(jobID, scheduledAt)
	runID2 := generateRunID(jobID, scheduledAt)

	expected := "job123:1704067200"
	if runID1 != expected {
		t.Errorf("expected runID to be %s, got %s", expected, runID1)
	}

	if runID1 != runID2 {
		t.Error("expected deterministic runID generation")
	}
}

// TestGenerateRunID_DifferentJobs verifies that different jobIDs produce different runIDs.
func TestGenerateRunID_DifferentJobs(t *testing.T) {
	scheduledAt := time.Unix(1704067200, 0)

	runID1 := generateRunID("job1", scheduledAt)
	runID2 := generateRunID("job2", scheduledAt)

	if runID1 == runID2 {
		t.Error("expected different runIDs for different jobs")
	}

	if runID1 != "job1:1704067200" {
		t.Errorf("expected job1:1704067200, got %s", runID1)
	}

	if runID2 != "job2:1704067200" {
		t.Errorf("expected job2:1704067200, got %s", runID2)
	}
}

// TestGenerateRunID_DifferentTimes verifies that the same job at different times produces different runIDs.
func TestGenerateRunID_DifferentTimes(t *testing.T) {
	jobID := "job1"

	runID1 := generateRunID(jobID, time.Unix(1704067200, 0))
	runID2 := generateRunID(jobID, time.Unix(1704067260, 0))

	if runID1 == runID2 {
		t.Error("expected different runIDs for different times")
	}

	if runID1 != "job1:1704067200" {
		t.Errorf("expected job1:1704067200, got %s", runID1)
	}

	if runID2 != "job1:1704067260" {
		t.Errorf("expected job1:1704067260, got %s", runID2)
	}
}

// =============================================================================
// Initialization Tests
// =============================================================================

// TestNewScheduler_Success verifies that a new scheduler can be created successfully.
func TestNewScheduler_Success(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockDB := testutil.NewMockDB()
	mockDB.SetJobs([]*testutil.Job{
		{ID: "job1", Schedule: "*/5 * * * *"},
		{ID: "job2", Schedule: "0 * * * *"},
	})

	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger.Logger())
	if err != nil {
		t.Fatalf("unexpected error creating scheduler: %v", err)
	}

	if scheduler == nil {
		t.Fatal("expected scheduler to be created")
	}

	if scheduler.index == nil {
		t.Error("expected index to be initialized")
	}

	if scheduler.inbox == nil {
		t.Error("expected inbox to be initialized")
	}

	if scheduler.syncer == nil {
		t.Error("expected syncer to be initialized")
	}

	if scheduler.activeOrchestrators == nil {
		t.Error("expected activeOrchestrators map to be initialized")
	}
}

// TestNewScheduler_DBQueryFails verifies that initialization fails gracefully when database query fails.
func TestNewScheduler_DBQueryFails(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockDB := testutil.NewMockDB()
	mockDB.SetQueryError(fmt.Errorf("database connection failed"))

	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	_, err := NewScheduler(config, syncerConfig, mockDB, logger.Logger())
	if err == nil {
		t.Error("expected error when DB query fails")
		t.Logf("Error logs: %+v", logger.GetEntriesByLevel("ERROR"))
	}

	if err != nil && !strings.Contains(err.Error(), "failed to query jobs on startup") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestNewScheduler_InvalidCronSchedule verifies that invalid cron schedules are logged but don't prevent initialization.
func TestNewScheduler_InvalidCronSchedule(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockDB := testutil.NewMockDB()
	mockDB.SetJobs([]*testutil.Job{
		{ID: "job1", Schedule: "*/5 * * * *"},       // Valid
		{ID: "job2", Schedule: "invalid cron expr"}, // Invalid
		{ID: "job3", Schedule: "0 * * * *"},         // Valid
	})

	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler, err := NewScheduler(config, syncerConfig, mockDB, logger.Logger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should log error for invalid job
	errorLogs := logger.GetEntriesByLevel("ERROR")
	found := false
	for _, entry := range errorLogs {
		if entry.Message == "failed to parse cron schedule" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error log for invalid cron schedule")
	}

	// Scheduler should still be created with valid jobs
	if scheduler == nil {
		t.Fatal("expected scheduler to be created despite invalid job")
	}
}

// =============================================================================
// Message Processing Tests
// =============================================================================

// TestScheduler_HandleOrchestratorStateChange verifies that orchestrator state changes are processed correctly.
func TestScheduler_HandleOrchestratorStateChange(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create orchestrator
	runID := "job1:1704067200"
	state := &OrchestratorState{
		RunID:      runID,
		JobID:      "job1",
		Status:     OrchestratorPreRun,
		CancelChan: make(chan struct{}),
	}
	scheduler.activeOrchestrators[runID] = state

	// Send state change message
	msg := InboxMessage{
		Type: MsgOrchestratorStateChange,
		Data: OrchestratorStateChangeMsg{
			RunID:     runID,
			NewStatus: OrchestratorPending,
			Timestamp: time.Now(),
		},
	}

	scheduler.handleMessage(msg)

	// Verify status updated
	if state.Status != OrchestratorPending {
		t.Errorf("expected status to be Pending, got %v", state.Status)
	}

	// Verify update buffered
	stats := scheduler.syncer.GetStats()
	if stats.BufferedJobRunUpdates != 1 {
		t.Errorf("expected 1 buffered update, got %d", stats.BufferedJobRunUpdates)
	}
}

// TestScheduler_HandleOrchestratorComplete verifies that orchestrator completion messages update state and buffer database writes.
func TestScheduler_HandleOrchestratorComplete(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create running orchestrator
	runID := "job1:1704067200"
	state := &OrchestratorState{
		RunID:      runID,
		JobID:      "job1",
		Status:     OrchestratorRunning,
		CancelChan: make(chan struct{}),
	}
	scheduler.activeOrchestrators[runID] = state

	// Send completion message
	completedAt := time.Now()
	msg := InboxMessage{
		Type: MsgOrchestratorComplete,
		Data: OrchestratorCompleteMsg{
			RunID:       runID,
			Success:     true,
			CompletedAt: completedAt,
			Error:       nil,
		},
	}

	scheduler.handleMessage(msg)

	// Verify status updated
	if state.Status != OrchestratorCompleted {
		t.Errorf("expected status to be Completed, got %v", state.Status)
	}

	if !state.CompletedAt.Equal(completedAt) {
		t.Error("expected CompletedAt to be set")
	}

	// Verify update buffered
	updates := scheduler.syncer.GetStats().BufferedJobRunUpdates
	if updates != 1 {
		t.Errorf("expected 1 buffered update, got %d", updates)
	}
}

// TestScheduler_HandleCancelRun verifies that cancel messages close the orchestrator's cancel channel and update status.
func TestScheduler_HandleCancelRun(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create orchestrator
	runID := "job1:1704067200"
	cancelChan := make(chan struct{})
	state := &OrchestratorState{
		RunID:      runID,
		JobID:      "job1",
		Status:     OrchestratorRunning,
		CancelChan: cancelChan,
	}
	scheduler.activeOrchestrators[runID] = state

	// Send cancel message
	msg := InboxMessage{
		Type: MsgCancelRun,
		Data: CancelRunMsg{
			RunID: runID,
		},
	}

	scheduler.handleMessage(msg)

	// Verify cancel channel closed
	select {
	case <-cancelChan:
		// Expected - channel closed
	default:
		t.Error("expected cancel channel to be closed")
	}

	// Verify status updated
	if state.Status != OrchestratorCancelled {
		t.Errorf("expected status to be Cancelled, got %v", state.Status)
	}
}

// TestScheduler_HandleCancelRun_NonExistent verifies that cancelling non-existent runs logs a warning without panicking.
func TestScheduler_HandleCancelRun_NonExistent(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Send cancel for non-existent run
	msg := InboxMessage{
		Type: MsgCancelRun,
		Data: CancelRunMsg{
			RunID: "nonexistent",
		},
	}

	// Should not panic
	scheduler.handleMessage(msg)

	// Should log warning
	if !logger.HasWarning() {
		t.Error("expected warning for cancelling non-existent run")
	}
}

// =============================================================================
// Heartbeat Monitoring Tests
// =============================================================================

// TestScheduler_CheckHeartbeats_OneMissed verifies that stale heartbeats increment the missed counter and log warnings.
func TestScheduler_CheckHeartbeats_OneMissed(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	config.OrchestratorHeartbeatInterval = 10 * time.Second
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create orchestrator with old heartbeat
	now := time.Now()
	runID := "job1:1704067200"
	state := &OrchestratorState{
		RunID:            runID,
		JobID:            "job1",
		Status:           OrchestratorRunning,
		LastHeartbeat:    now.Add(-15 * time.Second), // 15 seconds ago
		MissedHeartbeats: 0,
	}
	scheduler.activeOrchestrators[runID] = state

	// Check heartbeats
	scheduler.checkHeartbeats(now)

	// One missed heartbeat
	if state.MissedHeartbeats != 1 {
		t.Errorf("expected 1 missed heartbeat, got %d", state.MissedHeartbeats)
	}

	// Status still running (not orphaned yet)
	if state.Status != OrchestratorRunning {
		t.Errorf("expected status to remain Running, got %v", state.Status)
	}

	// Debug log should be recorded
	if !logger.HasDebug() {
		t.Error("expected debug log for missed heartbeat")
	}
}

// TestScheduler_CheckHeartbeats_Orphaned verifies that orchestrators are marked orphaned after exceeding max missed heartbeats.
func TestScheduler_CheckHeartbeats_Orphaned(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	config.OrchestratorHeartbeatInterval = 10 * time.Second
	config.MaxMissedOrchestratorHeartbeats = 3
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create orchestrator with many missed heartbeats
	now := time.Now()
	runID := "job1:1704067200"
	state := &OrchestratorState{
		RunID:            runID,
		JobID:            "job1",
		Status:           OrchestratorRunning,
		LastHeartbeat:    now.Add(-40 * time.Second), // 40 seconds ago
		MissedHeartbeats: 2,                          // One more will trigger orphan
	}
	scheduler.activeOrchestrators[runID] = state

	// Check heartbeats
	scheduler.checkHeartbeats(now)

	// Should be marked as orphaned
	if state.MissedHeartbeats != 3 {
		t.Errorf("expected 3 missed heartbeats, got %d", state.MissedHeartbeats)
	}

	if state.Status != OrchestratorOrphaned {
		t.Errorf("expected status to be Orphaned, got %v", state.Status)
	}

	// Error should be logged
	if !logger.HasError() {
		t.Error("expected error for orphaned orchestrator")
	}

	// Update should be buffered
	updates := scheduler.syncer.GetStats().BufferedJobRunUpdates
	if updates != 1 {
		t.Errorf("expected 1 buffered update for orphaned status, got %d", updates)
	}
}

// TestScheduler_CheckHeartbeats_SkipsCompleted verifies that completed orchestrators are excluded from heartbeat monitoring.
func TestScheduler_CheckHeartbeats_SkipsCompleted(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create completed orchestrator with very old heartbeat
	now := time.Now()
	runID := "job1:1704067200"
	state := &OrchestratorState{
		RunID:            runID,
		JobID:            "job1",
		Status:           OrchestratorCompleted,
		LastHeartbeat:    now.Add(-1 * time.Hour), // Very old
		MissedHeartbeats: 0,
	}
	scheduler.activeOrchestrators[runID] = state

	// Check heartbeats
	scheduler.checkHeartbeats(now)

	// Should not be checked (completed orchestrators skipped)
	if state.MissedHeartbeats != 0 {
		t.Error("completed orchestrator should not have heartbeat checked")
	}
}

// TestScheduler_CheckHeartbeats_SkipsAllTerminalStates verifies that all terminal states are excluded from heartbeat monitoring.
func TestScheduler_CheckHeartbeats_SkipsAllTerminalStates(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create orchestrators in all terminal states with very old heartbeats
	now := time.Now()
	terminalStates := []OrchestratorStatus{
		OrchestratorCompleted,
		OrchestratorFailed,
		OrchestratorCancelled,
		OrchestratorOrphaned,
	}

	for i, status := range terminalStates {
		runID := fmt.Sprintf("job%d:1704067200", i)
		scheduler.activeOrchestrators[runID] = &OrchestratorState{
			RunID:            runID,
			JobID:            fmt.Sprintf("job%d", i),
			Status:           status,
			LastHeartbeat:    now.Add(-1 * time.Hour), // Very old
			MissedHeartbeats: 0,
		}
	}

	// Check heartbeats
	scheduler.checkHeartbeats(now)

	// All terminal states should be skipped (no missed heartbeats incremented)
	for runID, state := range scheduler.activeOrchestrators {
		if state.MissedHeartbeats != 0 {
			t.Errorf("terminal state orchestrator %s should not have heartbeat checked, but has %d missed",
				runID, state.MissedHeartbeats)
		}
	}
}

// TestScheduler_HandleHeartbeat_ResetsCounter verifies that receiving a heartbeat resets the missed heartbeat counter.
func TestScheduler_HandleHeartbeat_ResetsCounter(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create orchestrator with missed heartbeats
	runID := "job1:1704067200"
	state := &OrchestratorState{
		RunID:            runID,
		JobID:            "job1",
		Status:           OrchestratorRunning,
		MissedHeartbeats: 3,
	}
	scheduler.activeOrchestrators[runID] = state

	// Send heartbeat
	heartbeatTime := time.Now()
	msg := InboxMessage{
		Type: MsgOrchestratorHeartbeat,
		Data: OrchestratorHeartbeatMsg{
			RunID:     runID,
			Timestamp: heartbeatTime,
		},
	}

	scheduler.handleMessage(msg)

	// Counter should be reset
	if state.MissedHeartbeats != 0 {
		t.Errorf("expected missed heartbeats to be reset to 0, got %d", state.MissedHeartbeats)
	}

	// Timestamp should be updated
	if !state.LastHeartbeat.Equal(heartbeatTime) {
		t.Error("expected LastHeartbeat to be updated")
	}
}

// =============================================================================
// Cleanup Tests
// =============================================================================

// TestScheduler_CleanupOrchestrators_NoCleanup verifies that orchestrators within grace period are not cleaned up.
func TestScheduler_CleanupOrchestrators_NoCleanup(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	config.GracePeriod = 30 * time.Second
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create completed orchestrator within grace period
	now := time.Now()
	runID := "job1:1704067200"
	state := &OrchestratorState{
		RunID:       runID,
		JobID:       "job1",
		Status:      OrchestratorCompleted,
		ScheduledAt: now.Add(-10 * time.Second), // 10 seconds ago
	}
	scheduler.activeOrchestrators[runID] = state

	// Cleanup
	scheduler.cleanupOrchestrators(now)

	// Should NOT be removed (still in grace period)
	if _, exists := scheduler.activeOrchestrators[runID]; !exists {
		t.Error("orchestrator should not be cleaned up within grace period")
	}
}

// TestScheduler_CleanupOrchestrators_AfterGrace verifies that terminal orchestrators past grace period are removed.
func TestScheduler_CleanupOrchestrators_AfterGrace(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	config.GracePeriod = 30 * time.Second
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create completed orchestrator past grace period
	now := time.Now()
	runID := "job1:1704067200"
	state := &OrchestratorState{
		RunID:       runID,
		JobID:       "job1",
		Status:      OrchestratorCompleted,
		ScheduledAt: now.Add(-40 * time.Second), // 40 seconds ago
	}
	scheduler.activeOrchestrators[runID] = state

	// Cleanup
	scheduler.cleanupOrchestrators(now)

	// Should be removed
	if _, exists := scheduler.activeOrchestrators[runID]; exists {
		t.Error("orchestrator should be cleaned up after grace period")
	}
}

// TestScheduler_CleanupOrchestrators_MultipleStates verifies that all terminal states are cleaned up after grace period.
func TestScheduler_CleanupOrchestrators_MultipleStates(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	config.GracePeriod = 30 * time.Second
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create orchestrators in various terminal states, all past grace period
	now := time.Now()
	pastTime := now.Add(-40 * time.Second)

	states := []OrchestratorStatus{
		OrchestratorCompleted,
		OrchestratorFailed,
		OrchestratorCancelled,
		OrchestratorOrphaned,
	}

	for i, status := range states {
		runID := fmt.Sprintf("job%d:%d", i, pastTime.Unix())
		scheduler.activeOrchestrators[runID] = &OrchestratorState{
			RunID:       runID,
			Status:      status,
			ScheduledAt: pastTime,
		}
	}

	// Cleanup
	scheduler.cleanupOrchestrators(now)

	// All should be removed
	if len(scheduler.activeOrchestrators) != 0 {
		t.Errorf("expected all orchestrators to be cleaned up, got %d remaining", len(scheduler.activeOrchestrators))
	}
}

// TestScheduler_CleanupOrchestrators_SkipsActive verifies that active orchestrators are never cleaned up regardless of age.
func TestScheduler_CleanupOrchestrators_SkipsActive(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	config.GracePeriod = 30 * time.Second
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Create non-terminal orchestrators, all past grace period
	now := time.Now()
	pastTime := now.Add(-40 * time.Second)

	nonTerminalStates := []OrchestratorStatus{
		OrchestratorPreRun,
		OrchestratorPending,
		OrchestratorConditionPending,
		OrchestratorConditionRunning,
		OrchestratorActionPending,
		OrchestratorActionRunning,
		OrchestratorContainerCreating,
		OrchestratorRunning,
		OrchestratorTerminating,
		OrchestratorRetrying,
	}

	for i, status := range nonTerminalStates {
		runID := fmt.Sprintf("job%d:%d", i, pastTime.Unix())
		scheduler.activeOrchestrators[runID] = &OrchestratorState{
			RunID:       runID,
			Status:      status,
			ScheduledAt: pastTime,
		}
	}

	// Cleanup
	scheduler.cleanupOrchestrators(now)

	// None should be removed (all non-terminal)
	if len(scheduler.activeOrchestrators) != len(nonTerminalStates) {
		t.Errorf("expected %d non-terminal orchestrators to remain, got %d",
			len(nonTerminalStates), len(scheduler.activeOrchestrators))
	}
}

// Helper functions

func createTestScheduler(t *testing.T, config SchedulerConfig, syncerConfig SyncerConfig, logger *testutil.TestLogger) *Scheduler {
	inbox := NewInbox(config.InboxBufferSize, config.InboxSendTimeout, logger.Logger())
	syncer, _ := NewSyncer(syncerConfig, logger.Logger())

	return &Scheduler{
		config:              config,
		logger:  logger.Logger(),
		index:               nil, // Not needed for most tests
		activeOrchestrators: make(map[string]*OrchestratorState),
		inbox:               inbox,
		syncer:              syncer,
		shutdown:            make(chan struct{}),
		rebuildIndexChan:    make(chan struct{}, 1),
	}
}
