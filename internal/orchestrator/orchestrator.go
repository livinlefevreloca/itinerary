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
	state         OrchestratorStatus
	previousState OrchestratorStatus

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

	// Phase timing boundaries
	createdAt              time.Time
	constraintCheckStarted time.Time
	executionStartedAt     time.Time
	completedAt            time.Time

	// Kubernetes tracking
	podName string
	jobName string

	// Execution results
	exitCode int
	err      error
}

// allowedTransitions defines valid state transitions
var allowedTransitions = map[OrchestratorStatus][]OrchestratorStatus{
	// PreRun can go to Pending or be Cancelled
	OrchestratorPreRun: {
		OrchestratorPending,
		OrchestratorCancelled,
	},

	// Pending starts constraint checking or goes to container creation
	OrchestratorPending: {
		OrchestratorConditionPending,
		OrchestratorContainerCreating,
		OrchestratorCancelled,
	},

	// ConditionPending transitions to running the check
	OrchestratorConditionPending: {
		OrchestratorConditionRunning,
		OrchestratorCancelled,
	},

	// ConditionRunning can proceed, need action, or be cancelled
	OrchestratorConditionRunning: {
		OrchestratorActionPending,
		OrchestratorContainerCreating,
		OrchestratorFailed,
		OrchestratorCancelled,
	},

	// ActionPending transitions to running the action
	OrchestratorActionPending: {
		OrchestratorActionRunning,
		OrchestratorCancelled,
	},

	// ActionRunning can complete or fail
	OrchestratorActionRunning: {
		OrchestratorContainerCreating,
		OrchestratorCompleted,
		OrchestratorFailed,
		OrchestratorCancelled,
	},

	// ContainerCreating transitions to running or fails
	OrchestratorContainerCreating: {
		OrchestratorRunning,
		OrchestratorFailed,
		OrchestratorRetrying,
		OrchestratorCancelled,
	},

	// Running transitions to terminating
	OrchestratorRunning: {
		OrchestratorTerminating,
		OrchestratorCancelled,
	},

	// Terminating decides final outcome
	OrchestratorTerminating: {
		OrchestratorCompleted,
		OrchestratorFailed,
		OrchestratorRetrying,
		OrchestratorCancelled,
	},

	// Retrying goes back to constraint check or directly to pending
	OrchestratorRetrying: {
		OrchestratorConditionPending,
		OrchestratorPending,
		OrchestratorFailed,
		OrchestratorCancelled,
	},

	// Terminal states - no transitions allowed (except Failed can retry)
	OrchestratorCompleted: {},
	OrchestratorFailed: {
		OrchestratorRetrying, // Can transition to retrying if retries remain
	},
	OrchestratorCancelled: {},
	OrchestratorOrphaned:  {},
}

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
		state:             OrchestratorPreRun,
		previousState:     OrchestratorPreRun,
		cancelChan:        make(chan struct{}),
		configUpdate:      make(chan *Job, 1),
		constraintChecker: constraintChecker,
		k8sClient:         k8sClient,
		logger:            logger,
		maxRetries:        maxRetries,
		createdAt:         time.Now(),
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
func (o *Orchestrator) GetState() OrchestratorStatus {
	return o.state
}

// GetPreviousState returns the previous state (for testing)
func (o *Orchestrator) GetPreviousState() OrchestratorStatus {
	return o.previousState
}

// transitionTo attempts to transition to a new state
func (o *Orchestrator) transitionTo(newState OrchestratorStatus) error {
	// Check if transition is allowed
	allowed := allowedTransitions[o.state]
	isAllowed := false
	for _, allowedState := range allowed {
		if newState == allowedState {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		return fmt.Errorf("invalid state transition: %s -> %s",
			o.state.String(), newState.String())
	}

	// Log the transition
	o.logger.Info("state transition",
		"from", o.state.String(),
		"to", newState.String(),
		"runID", o.runID)

	// Perform the transition
	o.previousState = o.state
	o.state = newState

	return nil
}

// run is the main orchestrator loop
func (o *Orchestrator) run() {
	defer func() {
		if r := recover(); r != nil {
			o.logger.Error("orchestrator panic recovered",
				"runID", o.runID,
				"panic", r)
			// Try to transition to failed state
			_ = o.transitionTo(OrchestratorFailed)
			o.runFailed()
		}
	}()

	for {
		switch o.state {
		case OrchestratorPreRun:
			o.runPreRun()
		case OrchestratorPending:
			o.runPending()
		case OrchestratorConditionPending:
			o.runConditionPending()
		case OrchestratorConditionRunning:
			o.runConditionRunning()
		case OrchestratorActionPending:
			o.runActionPending()
		case OrchestratorActionRunning:
			o.runActionRunning()
		case OrchestratorContainerCreating:
			o.runContainerCreating()
		case OrchestratorRunning:
			o.runRunning()
		case OrchestratorTerminating:
			o.runTerminating()
		case OrchestratorRetrying:
			o.runRetrying()
		case OrchestratorCompleted:
			o.runCompleted()
			return
		case OrchestratorFailed:
			o.runFailed()
			return
		case OrchestratorCancelled:
			o.runCancelled()
			return
		case OrchestratorOrphaned:
			o.runOrphaned()
			return
		default:
			o.logger.Error("unknown state",
				"state", o.state,
				"runID", o.runID)
			_ = o.transitionTo(OrchestratorFailed)
		}
	}
}

// runPreRun waits for the scheduled time
func (o *Orchestrator) runPreRun() {
	// Check if scheduled time has already passed
	now := time.Now()
	if now.After(o.scheduledAt) || now.Equal(o.scheduledAt) {
		// Scheduled time has passed, move to pending immediately
		if err := o.transitionTo(OrchestratorPending); err != nil {
			o.logger.Error("failed to transition to pending",
				"error", err,
				"runID", o.runID)
			_ = o.transitionTo(OrchestratorFailed)
		}
		return
	}

	// Wait for scheduled time or cancellation
	waitDuration := o.scheduledAt.Sub(now)
	timer := time.NewTimer(waitDuration)
	defer timer.Stop()

	select {
	case <-timer.C:
		// Scheduled time reached
		if err := o.transitionTo(OrchestratorPending); err != nil {
			o.logger.Error("failed to transition to pending",
				"error", err,
				"runID", o.runID)
			_ = o.transitionTo(OrchestratorFailed)
		}
	case newConfig := <-o.configUpdate:
		// Config updated while waiting
		o.jobConfig = newConfig
		// Stay in PreRun state, continue waiting
	case <-o.cancelChan:
		// Cancelled while waiting
		_ = o.transitionTo(OrchestratorCancelled)
	}
}

// runPending determines whether to check constraints or go directly to execution
func (o *Orchestrator) runPending() {
	// Check if we need to run constraint checks
	if o.constraintChecker != nil && len(o.jobConfig.ConstraintConfig) > 0 {
		// Transition to constraint checking phase
		o.constraintCheckStarted = time.Now()
		if err := o.transitionTo(OrchestratorConditionPending); err != nil {
			o.logger.Error("failed to transition to condition pending",
				"error", err,
				"runID", o.runID)
			_ = o.transitionTo(OrchestratorFailed)
		}
	} else {
		// No constraints, go directly to container creation
		o.executionStartedAt = time.Now()
		if err := o.transitionTo(OrchestratorContainerCreating); err != nil {
			o.logger.Error("failed to transition to container creating",
				"error", err,
				"runID", o.runID)
			_ = o.transitionTo(OrchestratorFailed)
		}
	}
}

// runConditionPending prepares to check constraints
func (o *Orchestrator) runConditionPending() {
	// Check for cancellation
	select {
	case <-o.cancelChan:
		_ = o.transitionTo(OrchestratorCancelled)
		return
	default:
	}

	// Transition to actually running the constraint check
	if err := o.transitionTo(OrchestratorConditionRunning); err != nil {
		o.logger.Error("failed to transition to condition running",
			"error", err,
			"runID", o.runID)
		_ = o.transitionTo(OrchestratorFailed)
	}
}

// runConditionRunning executes constraint checks
func (o *Orchestrator) runConditionRunning() {
	// TODO: Implement constraint checking
	// For now, just transition to container creating
	o.executionStartedAt = time.Now()
	if err := o.transitionTo(OrchestratorContainerCreating); err != nil {
		o.logger.Error("failed to transition to container creating",
			"error", err,
			"runID", o.runID)
		_ = o.transitionTo(OrchestratorFailed)
	}
}

// runActionPending prepares to execute actions
func (o *Orchestrator) runActionPending() {
	// Check for cancellation
	select {
	case <-o.cancelChan:
		_ = o.transitionTo(OrchestratorCancelled)
		return
	default:
	}

	// Transition to actually running the action
	if err := o.transitionTo(OrchestratorActionRunning); err != nil {
		o.logger.Error("failed to transition to action running",
			"error", err,
			"runID", o.runID)
		_ = o.transitionTo(OrchestratorFailed)
	}
}

// runActionRunning executes actions
func (o *Orchestrator) runActionRunning() {
	// TODO: Implement action execution
	// For now, just transition to container creating
	o.executionStartedAt = time.Now()
	if err := o.transitionTo(OrchestratorContainerCreating); err != nil {
		o.logger.Error("failed to transition to container creating",
			"error", err,
			"runID", o.runID)
		_ = o.transitionTo(OrchestratorFailed)
	}
}

// runContainerCreating creates the Kubernetes job
func (o *Orchestrator) runContainerCreating() {
	// TODO: Implement Kubernetes job creation
	// For now, just transition to running
	if err := o.transitionTo(OrchestratorRunning); err != nil {
		o.logger.Error("failed to transition to running",
			"error", err,
			"runID", o.runID)
		_ = o.transitionTo(OrchestratorFailed)
	}
}

// runRunning monitors the executing job
func (o *Orchestrator) runRunning() {
	// TODO: Implement job monitoring
	// For now, just transition to terminating
	if err := o.transitionTo(OrchestratorTerminating); err != nil {
		o.logger.Error("failed to transition to terminating",
			"error", err,
			"runID", o.runID)
		_ = o.transitionTo(OrchestratorFailed)
	}
}

// runTerminating handles job completion and cleanup
func (o *Orchestrator) runTerminating() {
	// TODO: Implement termination logic
	// For now, just transition to completed
	o.completedAt = time.Now()
	if err := o.transitionTo(OrchestratorCompleted); err != nil {
		o.logger.Error("failed to transition to completed",
			"error", err,
			"runID", o.runID)
		_ = o.transitionTo(OrchestratorFailed)
	}
}

// runRetrying handles retry logic
func (o *Orchestrator) runRetrying() {
	// TODO: Implement retry logic
	// For now, just transition to pending
	if err := o.transitionTo(OrchestratorPending); err != nil {
		o.logger.Error("failed to transition to pending",
			"error", err,
			"runID", o.runID)
		_ = o.transitionTo(OrchestratorFailed)
	}
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
