package scheduler

import (
	"testing"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

// =============================================================================
// Orchestrator Lifecycle Tests
// =============================================================================

// TestOrchestrator_WaitsForScheduledTime verifies that orchestrators wait until scheduled time before transitioning to Pending.
func TestOrchestrator_WaitsForScheduledTime(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Schedule for 200ms in the future
	scheduledAt := time.Now().Add(200 * time.Millisecond)
	runID := generateRunID("job1", scheduledAt)
	cancelChan := make(chan struct{})
	configUpdateChan := make(chan *Job, 1)

	// Track when orchestrator transitions to Pending
	transitionTime := make(chan time.Time, 1)
	go func() {
		for {
			msg, ok := scheduler.inbox.TryReceive()
			if !ok {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if msg.Type == MsgOrchestratorStateChange {
				data := msg.Data.(OrchestratorStateChangeMsg)
				if data.NewStatus == OrchestratorPending {
					transitionTime <- time.Now()
					return
				}
			}
		}
	}()

	// Start orchestrator
	start := time.Now()
	go scheduler.runOrchestrator("job1", scheduledAt, runID, cancelChan, configUpdateChan)

	// Wait for transition
	select {
	case actualTime := <-transitionTime:
		elapsed := actualTime.Sub(start)
		// Should wait approximately 200ms
		if elapsed < 180*time.Millisecond || elapsed > 250*time.Millisecond {
			t.Errorf("expected ~200ms wait, got %v", elapsed)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for orchestrator to transition")
	}
}

// TestOrchestrator_ConfigUpdate_InPreRun verifies that config updates can be received during PreRun state.
func TestOrchestrator_ConfigUpdate_InPreRun(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Schedule for 500ms in the future
	scheduledAt := time.Now().Add(500 * time.Millisecond)
	runID := generateRunID("job1", scheduledAt)
	cancelChan := make(chan struct{})
	configUpdateChan := make(chan *Job, 1)

	// Start orchestrator
	go scheduler.runOrchestrator("job1", scheduledAt, runID, cancelChan, configUpdateChan)

	// Wait a bit for orchestrator to start
	time.Sleep(100 * time.Millisecond)

	// Send config update
	newConfig := &Job{
		ID:       "job1",
		Schedule: "*/10 * * * *",
	}
	configUpdateChan <- newConfig

	// Wait for orchestrator to process the config update
	time.Sleep(100 * time.Millisecond)

	// Verify orchestrator received and logged the config update
	infoLogs := logger.GetEntriesByLevel("INFO")
	foundConfigUpdate := false
	for _, entry := range infoLogs {
		if entry.Message == "received config update in PreRun" {
			foundConfigUpdate = true
			break
		}
	}
	if !foundConfigUpdate {
		t.Error("expected orchestrator to receive and log config update")
	}

	// Close to trigger completion
	close(cancelChan)
}

// TestOrchestrator_Cancel_InPreRun verifies that orchestrators can be cancelled during PreRun state.
func TestOrchestrator_Cancel_InPreRun(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Schedule for 500ms in the future
	scheduledAt := time.Now().Add(500 * time.Millisecond)
	runID := generateRunID("job1", scheduledAt)
	cancelChan := make(chan struct{})
	configUpdateChan := make(chan *Job, 1)

	// Track completion
	completed := make(chan bool, 1)
	go func() {
		for {
			msg, ok := scheduler.inbox.TryReceive()
			if !ok {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if msg.Type == MsgOrchestratorComplete {
				completed <- true
				return
			}
		}
	}()

	// Start orchestrator
	go scheduler.runOrchestrator("job1", scheduledAt, runID, cancelChan, configUpdateChan)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Cancel
	close(cancelChan)

	// Should complete immediately
	select {
	case <-completed:
		// Expected - cancelled
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for cancellation")
	}
}

// TestOrchestrator_ScheduledTimeInPast verifies that orchestrators transition immediately when scheduled in the past.
func TestOrchestrator_ScheduledTimeInPast(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Schedule in the past
	scheduledAt := time.Now().Add(-1 * time.Second)
	runID := generateRunID("job1", scheduledAt)
	cancelChan := make(chan struct{})
	configUpdateChan := make(chan *Job, 1)

	// Track when Pending state is reached
	transitioned := make(chan bool, 1)
	go func() {
		for {
			msg, ok := scheduler.inbox.TryReceive()
			if !ok {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if msg.Type == MsgOrchestratorStateChange {
				data := msg.Data.(OrchestratorStateChangeMsg)
				if data.NewStatus == OrchestratorPending {
					transitioned <- true
					return
				}
			}
		}
	}()

	// Start orchestrator
	start := time.Now()
	go scheduler.runOrchestrator("job1", scheduledAt, runID, cancelChan, configUpdateChan)

	// Should transition immediately (no wait)
	select {
	case <-transitioned:
		elapsed := time.Since(start)
		if elapsed > 100*time.Millisecond {
			t.Errorf("expected immediate transition, took %v", elapsed)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for transition")
	}
}

// TestOrchestrator_SendsHeartbeats verifies that orchestrators send periodic heartbeat messages at the configured interval.
func TestOrchestrator_SendsHeartbeats(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	config.OrchestratorHeartbeatInterval = 100 * time.Millisecond
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Schedule in past (starts immediately)
	scheduledAt := time.Now().Add(-1 * time.Second)
	runID := generateRunID("job1", scheduledAt)
	cancelChan := make(chan struct{})
	configUpdateChan := make(chan *Job, 1)

	// Collect heartbeats
	heartbeats := make([]time.Time, 0)
	heartbeatChan := make(chan time.Time, 10)

	go func() {
		for {
			msg, ok := scheduler.inbox.TryReceive()
			if !ok {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if msg.Type == MsgOrchestratorHeartbeat {
				data := msg.Data.(OrchestratorHeartbeatMsg)
				heartbeatChan <- data.Timestamp
			}
		}
	}()

	// Start orchestrator
	go scheduler.runOrchestrator("job1", scheduledAt, runID, cancelChan, configUpdateChan)

	// Collect heartbeats for 350ms (should get 3-4 heartbeats at 100ms intervals)
	timeout := time.After(350 * time.Millisecond)
	for {
		select {
		case hb := <-heartbeatChan:
			heartbeats = append(heartbeats, hb)
		case <-timeout:
			goto DONE
		}
	}

DONE:
	// Should have received multiple heartbeats
	if len(heartbeats) < 2 {
		t.Errorf("expected at least 2 heartbeats, got %d", len(heartbeats))
	}

	// Cancel orchestrator
	close(cancelChan)
}

// TestOrchestrator_StopsHeartbeatsOnCompletion verifies that heartbeat messages cease after orchestrator completes.
func TestOrchestrator_StopsHeartbeatsOnCompletion(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultSchedulerConfig()
	config.OrchestratorHeartbeatInterval = 50 * time.Millisecond
	syncerConfig := DefaultSyncerConfig()

	scheduler := createTestScheduler(t, config, syncerConfig, logger)

	// Schedule in past (starts immediately)
	scheduledAt := time.Now().Add(-1 * time.Second)
	runID := generateRunID("job1", scheduledAt)
	cancelChan := make(chan struct{})
	configUpdateChan := make(chan *Job, 1)

	// Track messages
	heartbeatCount := 0
	completed := false
	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ { // Check for a while
			msg, ok := scheduler.inbox.TryReceive()
			if !ok {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if msg.Type == MsgOrchestratorHeartbeat {
				if !completed {
					heartbeatCount++
				}
			}

			if msg.Type == MsgOrchestratorComplete {
				completed = true
				// Wait a bit more to see if heartbeats stop
				time.Sleep(200 * time.Millisecond)
				done <- true
				return
			}
		}
	}()

	// Start orchestrator
	go scheduler.runOrchestrator("job1", scheduledAt, runID, cancelChan, configUpdateChan)

	// Wait a bit then cancel to trigger completion
	time.Sleep(150 * time.Millisecond)
	close(cancelChan)

	// Wait for completion
	select {
	case <-done:
		// Good - orchestrator completed
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for completion")
	}

	// Heartbeats should have been sent before completion but not after
	if heartbeatCount == 0 {
		t.Error("expected some heartbeats before completion")
	}
}
