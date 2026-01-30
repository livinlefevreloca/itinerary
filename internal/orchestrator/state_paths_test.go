package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ==============================================================================
// State Path Tests - Verify orchestrator follows expected paths
// ==============================================================================

// TestStatePaths_HappyPath verifies the normal success path
func TestStatePaths_HappyPath(t *testing.T) {
	recorder := NewStateRecorder()
	orch := NewOrchestrator(
		"test-run-id",
		&Job{ID: "test-job", Name: "test"},
		// Use past scheduled time so it starts immediately
		// We'll test actual waiting in lifecycle tests
		now().Add(-1*time.Second),
		nil, // no constraint checker
		createFakeK8sClient(),
		createTestLogger(),
	)
	orch.recorder = recorder

	// Manually step through happy path states
	preRun := &PreRunState{}
	orch.transitionTo(preRun)

	pending := preRun.ToPending()
	orch.transitionTo(pending)

	containerCreating := pending.ToContainerCreating()
	orch.transitionTo(containerCreating)

	running := containerCreating.ToRunning()
	orch.transitionTo(running)

	terminating := running.ToTerminating()
	orch.transitionTo(terminating)

	completed := terminating.ToCompleted()
	orch.transitionTo(completed)

	// Verify the path
	expected := []string{
		"prerun",
		"pending",
		"container_creating",
		"running",
		"terminating",
		"completed",
	}
	assert.Equal(t, expected, recorder.Path())
}

// TestStatePaths_WithConstraints verifies path with constraint checking
func TestStatePaths_WithConstraints(t *testing.T) {
	recorder := NewStateRecorder()
	orch := NewOrchestrator(
		"test-run-id",
		&Job{ID: "test-job", Name: "test"},
		now().Add(-1*time.Second),
		createMockConstraintChecker(true, nil),
		createFakeK8sClient(),
		createTestLogger(),
	)
	orch.recorder = recorder

	// Step through constraint checking path
	preRun := &PreRunState{}
	orch.transitionTo(preRun)

	pending := preRun.ToPending()
	orch.transitionTo(pending)

	conditionPending := pending.ToConditionPending()
	orch.transitionTo(conditionPending)

	conditionRunning := conditionPending.ToConditionRunning()
	orch.transitionTo(conditionRunning)

	containerCreating := conditionRunning.ToContainerCreating()
	orch.transitionTo(containerCreating)

	running := containerCreating.ToRunning()
	orch.transitionTo(running)

	terminating := running.ToTerminating()
	orch.transitionTo(terminating)

	completed := terminating.ToCompleted()
	orch.transitionTo(completed)

	// Verify the path
	expected := []string{
		"prerun",
		"pending",
		"condition_pending",
		"condition_running",
		"container_creating",
		"running",
		"terminating",
		"completed",
	}
	assert.Equal(t, expected, recorder.Path())
}

// TestStatePaths_WithActions verifies path with action execution
func TestStatePaths_WithActions(t *testing.T) {
	recorder := NewStateRecorder()
	orch := NewOrchestrator(
		"test-run-id",
		&Job{ID: "test-job", Name: "test"},
		now().Add(-1*time.Second),
		nil,
		createFakeK8sClient(),
		createTestLogger(),
	)
	orch.recorder = recorder

	// Step through action execution path
	preRun := &PreRunState{}
	orch.transitionTo(preRun)

	pending := preRun.ToPending()
	orch.transitionTo(pending)

	conditionPending := pending.ToConditionPending()
	orch.transitionTo(conditionPending)

	conditionRunning := conditionPending.ToConditionRunning()
	orch.transitionTo(conditionRunning)

	actionPending := conditionRunning.ToActionPending()
	orch.transitionTo(actionPending)

	actionRunning := actionPending.ToActionRunning()
	orch.transitionTo(actionRunning)

	containerCreating := actionRunning.ToContainerCreating()
	orch.transitionTo(containerCreating)

	running := containerCreating.ToRunning()
	orch.transitionTo(running)

	terminating := running.ToTerminating()
	orch.transitionTo(terminating)

	completed := terminating.ToCompleted()
	orch.transitionTo(completed)

	// Verify the path
	expected := []string{
		"prerun",
		"pending",
		"condition_pending",
		"condition_running",
		"action_pending",
		"action_running",
		"container_creating",
		"running",
		"terminating",
		"completed",
	}
	assert.Equal(t, expected, recorder.Path())
}

// TestStatePaths_WithRetry verifies path with retry
func TestStatePaths_WithRetry(t *testing.T) {
	recorder := NewStateRecorder()
	orch := NewOrchestrator(
		"test-run-id",
		&Job{
			ID:   "test-job",
			Name: "test",
			RetryConfig: &RetryConfig{
				MaxRetries: 2,
			},
		},
		now().Add(-1*time.Second),
		nil,
		createFakeK8sClient(),
		createTestLogger(),
	)
	orch.recorder = recorder

	// Step through retry path
	preRun := &PreRunState{}
	orch.transitionTo(preRun)

	pending := preRun.ToPending()
	orch.transitionTo(pending)

	containerCreating := pending.ToContainerCreating()
	orch.transitionTo(containerCreating)

	running := containerCreating.ToRunning()
	orch.transitionTo(running)

	terminating := running.ToTerminating()
	orch.transitionTo(terminating)

	// First attempt fails
	retrying := terminating.ToRetrying()
	orch.transitionTo(retrying)

	// Retry - go back to pending (skip constraint recheck)
	pending2 := retrying.ToPending()
	orch.transitionTo(pending2)

	containerCreating2 := pending2.ToContainerCreating()
	orch.transitionTo(containerCreating2)

	running2 := containerCreating2.ToRunning()
	orch.transitionTo(running2)

	terminating2 := running2.ToTerminating()
	orch.transitionTo(terminating2)

	// Second attempt succeeds
	completed := terminating2.ToCompleted()
	orch.transitionTo(completed)

	// Verify the path
	expected := []string{
		"prerun",
		"pending",
		"container_creating",
		"running",
		"terminating",
		"retrying",
		"pending",
		"container_creating",
		"running",
		"terminating",
		"completed",
	}
	assert.Equal(t, expected, recorder.Path())
}

// TestStatePaths_Cancellation verifies cancellation from various states
func TestStatePaths_Cancellation(t *testing.T) {
	tests := []struct {
		name          string
		cancelFrom    string
		expectedPath  []string
		buildPath     func(*Orchestrator) State
	}{
		{
			name:       "cancel_from_prerun",
			cancelFrom: "prerun",
			expectedPath: []string{
				"prerun",
				"cancelled",
			},
			buildPath: func(o *Orchestrator) State {
				preRun := &PreRunState{}
				o.transitionTo(preRun)
				return preRun.ToCancelled()
			},
		},
		{
			name:       "cancel_from_pending",
			cancelFrom: "pending",
			expectedPath: []string{
				"prerun",
				"pending",
				"cancelled",
			},
			buildPath: func(o *Orchestrator) State {
				preRun := &PreRunState{}
				o.transitionTo(preRun)
				pending := preRun.ToPending()
				o.transitionTo(pending)
				return pending.ToCancelled()
			},
		},
		{
			name:       "cancel_from_running",
			cancelFrom: "running",
			expectedPath: []string{
				"prerun",
				"pending",
				"container_creating",
				"running",
				"cancelled",
			},
			buildPath: func(o *Orchestrator) State {
				preRun := &PreRunState{}
				o.transitionTo(preRun)
				pending := preRun.ToPending()
				o.transitionTo(pending)
				containerCreating := pending.ToContainerCreating()
				o.transitionTo(containerCreating)
				running := containerCreating.ToRunning()
				o.transitionTo(running)
				return running.ToCancelled()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := NewStateRecorder()
			orch := NewOrchestrator(
				"test-run-id",
				&Job{ID: "test-job", Name: "test"},
				now().Add(-1*time.Second),
				nil,
				createFakeK8sClient(),
				createTestLogger(),
			)
			orch.recorder = recorder

			// Build path to cancellation point (starts from prerun)
			cancelledState := tt.buildPath(orch)
			orch.transitionTo(cancelledState)

			// Verify the path
			assert.Equal(t, tt.expectedPath, recorder.Path())
		})
	}
}

// TestStatePaths_Failure verifies failure paths
func TestStatePaths_Failure(t *testing.T) {
	recorder := NewStateRecorder()
	orch := NewOrchestrator(
		"test-run-id",
		&Job{ID: "test-job", Name: "test"},
		now().Add(-1*time.Second),
		nil,
		createFakeK8sClient(),
		createTestLogger(),
	)
	orch.recorder = recorder

	// Step through failure path
	preRun := &PreRunState{}
	orch.transitionTo(preRun)

	pending := preRun.ToPending()
	orch.transitionTo(pending)

	containerCreating := pending.ToContainerCreating()
	orch.transitionTo(containerCreating)

	// Container creation fails
	failed := containerCreating.ToFailed()
	orch.transitionTo(failed)

	// Verify the path
	expected := []string{
		"prerun",
		"pending",
		"container_creating",
		"failed",
	}
	assert.Equal(t, expected, recorder.Path())
}

// Helper function
func now() time.Time {
	return time.Now()
}
