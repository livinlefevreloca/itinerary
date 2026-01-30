package orchestrator

import (
	"fmt"
	"log/slog"
	"time"

	"k8s.io/client-go/kubernetes"
)

// Orchestrator represents a single execution instance of a job
type Orchestrator struct {
	// Core identification
	runID       string
	jobID       string
	jobConfig   *Job
	scheduledAt time.Time

	// State management
	state State

	// Communication channels
	cancelChan   chan struct{}
	configUpdate chan *Job

	// Dependencies
	constraintChecker ConstraintChecker
	k8sClient         kubernetes.Interface
	logger            *slog.Logger

	// Retry tracking
	retryAttempt int
	maxRetries   int

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

// No more allowedTransitions map - enforced by type system!

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(
	runID string,
	jobConfig *Job,
	scheduledAt time.Time,
	constraintChecker ConstraintChecker,
	k8sClient kubernetes.Interface,
	logger *slog.Logger,
) *Orchestrator {
	maxRetries := 0
	if jobConfig.RetryConfig != nil {
		maxRetries = jobConfig.RetryConfig.MaxRetries
	}

	return &Orchestrator{
		runID:             runID,
		jobID:             jobConfig.ID,
		jobConfig:         jobConfig,
		scheduledAt:       scheduledAt,
		state:             &PreRunState{},
		cancelChan:        make(chan struct{}),
		configUpdate:      make(chan *Job, 1),
		constraintChecker: constraintChecker,
		k8sClient:         k8sClient,
		logger:            logger,
		maxRetries:        maxRetries,
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
func (o *Orchestrator) UpdateConfig(newConfig *Job) {
	select {
	case o.configUpdate <- newConfig:
	default:
		// Config update channel full, drop update
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
		case *ConditionPendingState:
			o.runConditionPending()
		case *ConditionRunningState:
			o.runConditionRunning()
		case *ActionPendingState:
			o.runActionPending()
		case *ActionRunningState:
			o.runActionRunning()
		case *ContainerCreatingState:
			o.runContainerCreating()
		case *RunningState:
			o.runRunning()
		case *TerminatingState:
			o.runTerminating()
		case *RetryingState:
			o.runRetrying()
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

	// Check if we need to run constraint checks
	if o.constraintChecker != nil && len(o.jobConfig.ConstraintConfig) > 0 {
		// Transition to constraint checking phase
		o.timing.ConstraintCheckStarted = time.Now()
		o.transitionTo(state.ToConditionPending())
	} else {
		// No constraints, go directly to container creation
		o.timing.ExecutionStartedAt = time.Now()
		o.transitionTo(state.ToContainerCreating())
	}
}

// runConditionPending prepares to check constraints
func (o *Orchestrator) runConditionPending() {
	state := o.state.(*ConditionPendingState)

	// Check for cancellation
	select {
	case <-o.cancelChan:
		o.transitionTo(state.ToCancelled())
		return
	default:
	}

	// Transition to actually running the constraint check
	o.transitionTo(state.ToConditionRunning())
}

// runConditionRunning executes constraint checks
func (o *Orchestrator) runConditionRunning() {
	state := o.state.(*ConditionRunningState)

	// TODO: Implement constraint checking
	// For now, just transition to container creating
	o.timing.ExecutionStartedAt = time.Now()
	o.transitionTo(state.ToContainerCreating())
}

// runActionPending prepares to execute actions
func (o *Orchestrator) runActionPending() {
	state := o.state.(*ActionPendingState)

	// Check for cancellation
	select {
	case <-o.cancelChan:
		o.transitionTo(state.ToCancelled())
		return
	default:
	}

	// Transition to actually running the action
	o.transitionTo(state.ToActionRunning())
}

// runActionRunning executes actions
func (o *Orchestrator) runActionRunning() {
	state := o.state.(*ActionRunningState)

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

// runRetrying handles retry logic
func (o *Orchestrator) runRetrying() {
	state := o.state.(*RetryingState)

	// TODO: Implement retry logic
	// For now, just transition to pending
	o.transitionTo(state.ToPending())
}

// runCompleted handles successful completion
func (o *Orchestrator) runCompleted() {
	o.logger.Info("orchestrator completed successfully",
		"runID", o.runID,
		"jobID", o.jobID)
	// TODO: Send completion message
	// TODO: Submit metrics
}

// runFailed handles failure
func (o *Orchestrator) runFailed() {
	o.logger.Info("orchestrator failed",
		"runID", o.runID,
		"jobID", o.jobID,
		"error", o.err)
	// TODO: Send completion message
	// TODO: Submit metrics
}

// runCancelled handles cancellation
func (o *Orchestrator) runCancelled() {
	o.logger.Info("orchestrator cancelled",
		"runID", o.runID,
		"jobID", o.jobID)
	// TODO: Clean up any resources
	// TODO: Send completion message
	// TODO: Submit metrics
}

// runOrphaned handles orphaned state
func (o *Orchestrator) runOrphaned() {
	o.logger.Info("orchestrator orphaned",
		"runID", o.runID,
		"jobID", o.jobID)
	// TODO: Send completion message
	// TODO: Submit metrics
}
