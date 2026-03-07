package orchestrator

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/constraints"
	"github.com/livinlefevreloca/itinerary/internal/db"
	"k8s.io/client-go/kubernetes"
)

// Orchestrator represents a single execution instance of a job
type Orchestrator struct {
	// Core identification
	runID       string
	jobID       string
	jobConfig   *db.Job
	scheduledAt time.Time

	// State management
	state State

	// Communication channels
	cancelChan   chan struct{}
	configUpdate chan *db.Job

	// Dependencies
	constraintChecker constraints.ConstraintChecker
	k8sClient         kubernetes.Interface
	logger            *slog.Logger

	// Phase timing
	timing PhaseTiming

	// Kubernetes tracking
	podName string
	jobName string

	// Execution results
	exitCode int
	err      error

	// Optional state recorder for testing
	recorder *StateRecorder
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(
	runID string,
	jobConfig *db.Job,
	scheduledAt time.Time,
	checker constraints.ConstraintChecker,
	k8sClient kubernetes.Interface,
	logger *slog.Logger,
) *Orchestrator {
	return &Orchestrator{
		runID:             runID,
		jobID:             jobConfig.ID,
		jobConfig:         jobConfig,
		scheduledAt:       scheduledAt,
		state:             &PreRunState{},
		cancelChan:        make(chan struct{}),
		configUpdate:      make(chan *db.Job, 1),
		constraintChecker: checker,
		k8sClient:         k8sClient,
		logger:            logger,
		timing: PhaseTiming{
			CreatedAt: time.Now(),
		},
	}
}

// Start begins the orchestrator execution
func (o *Orchestrator) Start() {
	go o.run()
}

// Cancel requests cancellation of the orchestrator
func (o *Orchestrator) Cancel() {
	close(o.cancelChan)
}

// UpdateConfig sends a config update to the orchestrator
func (o *Orchestrator) UpdateConfig(newConfig *db.Job) {
	select {
	case o.configUpdate <- newConfig:
	default:
		o.logger.Warn("config update channel full, dropping update",
			"runID", o.runID,
			"jobID", o.jobID)
	}
}

// GetState returns the current state (for testing)
func (o *Orchestrator) GetState() State {
	return o.state
}

// GetStateName returns the current state name (for testing)
func (o *Orchestrator) GetStateName() string {
	return o.state.Name()
}

// transitionTo performs a state transition and logs it
func (o *Orchestrator) transitionTo(newState State) {
	oldStateName := o.state.Name()
	o.state = newState

	// Record state for testing if recorder is present
	if o.recorder != nil {
		o.recorder.Record(newState)
	}

	// Log the transition
	o.logger.Info("state transition",
		"from", oldStateName,
		"to", newState.Name(),
		"runID", o.runID)
}

// run is the main orchestrator loop
func (o *Orchestrator) run() {
	defer func() {
		if r := recover(); r != nil {
			o.logger.Error("orchestrator panic recovered",
				"runID", o.runID,
				"panic", r)
			// Transition to failed state
			o.transitionTo(&FailedState{})
			o.runFailed()
		}
	}()

	for {
		switch o.state.(type) {
		case *PreRunState:
			o.runPreRun()
		case *PendingState:
			o.runPending()
		case *ConditionRunningState:
			o.runConditionRunning()
		case *ActionRunningState:
			o.runActionRunning()
		case *ContainerCreatingState:
			o.runContainerCreating()
		case *RunningState:
			o.runRunning()
		case *TerminatingState:
			o.runTerminating()
		case *CompletedState:
			o.runCompleted()
			return
		case *FailedState:
			o.runFailed()
			return
		case *CancelledState:
			o.runCancelled()
			return
		case *OrphanedState:
			o.runOrphaned()
			return
		default:
			o.logger.Error("unknown state type",
				"state", fmt.Sprintf("%T", o.state),
				"runID", o.runID)
			o.transitionTo(&FailedState{})
		}
	}
}

// runPreRun waits for the scheduled time
func (o *Orchestrator) runPreRun() {
	state := o.state.(*PreRunState)

	// Check if scheduled time has already passed
	now := time.Now()
	if now.After(o.scheduledAt) || now.Equal(o.scheduledAt) {
		// Scheduled time has passed, move to pending immediately
		o.transitionTo(state.ToPending())
		return
	}

	// Wait for scheduled time or cancellation
	waitDuration := o.scheduledAt.Sub(now)
	timer := time.NewTimer(waitDuration)
	defer timer.Stop()

	select {
	case <-timer.C:
		// Scheduled time reached
		o.transitionTo(state.ToPending())
	case newConfig := <-o.configUpdate:
		// Config updated while waiting
		o.jobConfig = newConfig
		// Stay in PreRun state, continue waiting
	case <-o.cancelChan:
		// Cancelled while waiting
		o.transitionTo(state.ToCancelled())
	}
}

// runPending determines whether to check constraints or go directly to execution
func (o *Orchestrator) runPending() {
	state := o.state.(*PendingState)

	// Always run through constraint checking — the checker handles
	// the case where no constraints are configured
	o.timing.ConstraintCheckStarted = time.Now()
	o.transitionTo(state.ToConditionRunning())
}

// runConditionRunning executes constraint checks
func (o *Orchestrator) runConditionRunning() {
	state := o.state.(*ConditionRunningState)

	// Check for cancellation
	select {
	case <-o.cancelChan:
		o.transitionTo(state.ToCancelled())
		return
	default:
	}

	// TODO: Implement constraint checking
	// For now, just transition to container creating
	o.timing.ExecutionStartedAt = time.Now()
	o.transitionTo(state.ToContainerCreating())
}

// runActionRunning executes actions
func (o *Orchestrator) runActionRunning() {
	state := o.state.(*ActionRunningState)

	// Check for cancellation
	select {
	case <-o.cancelChan:
		o.transitionTo(state.ToCancelled())
		return
	default:
	}

	// TODO: Implement action execution
	// For now, just transition to container creating
	o.timing.ExecutionStartedAt = time.Now()
	o.transitionTo(state.ToContainerCreating())
}

// runContainerCreating creates the Kubernetes job
func (o *Orchestrator) runContainerCreating() {
	state := o.state.(*ContainerCreatingState)

	// TODO: Implement Kubernetes job creation
	// For now, just transition to running
	o.transitionTo(state.ToRunning())
}

// runRunning monitors the executing job
func (o *Orchestrator) runRunning() {
	state := o.state.(*RunningState)

	// TODO: Implement job monitoring
	// For now, just transition to terminating
	o.transitionTo(state.ToTerminating())
}

// runTerminating handles job completion and cleanup
func (o *Orchestrator) runTerminating() {
	state := o.state.(*TerminatingState)

	// TODO: Implement termination logic
	// For now, just transition to completed
	o.timing.CompletedAt = time.Now()
	o.transitionTo(state.ToCompleted())
}

// runCompleted handles successful completion
func (o *Orchestrator) runCompleted() {
	o.logger.Info("orchestrator completed successfully",
		"runID", o.runID,
		"jobID", o.jobID)
}

// runFailed handles failure
func (o *Orchestrator) runFailed() {
	o.logger.Info("orchestrator failed",
		"runID", o.runID,
		"jobID", o.jobID,
		"error", o.err)
}

// runCancelled handles cancellation
func (o *Orchestrator) runCancelled() {
	o.logger.Info("orchestrator cancelled",
		"runID", o.runID,
		"jobID", o.jobID)
}

// runOrphaned handles orphaned state
func (o *Orchestrator) runOrphaned() {
	o.logger.Info("orchestrator orphaned",
		"runID", o.runID,
		"jobID", o.jobID)
}
