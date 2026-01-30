# Orchestrator Test Suite

## Overview
Comprehensive test suite for the Orchestrator component. Focus on state machine correctness, lifecycle management, Kubernetes integration, constraint/action systems, retry logic, and metrics collection.

## Test Categories

### 1. State Machine Tests

#### State Transition Validation Tests
```go
TestStateTransition_ValidTransitions
- Verify all valid transitions from allowedTransitions map
- Each valid transition should succeed
- State should update correctly
- Previous state should be tracked

TestStateTransition_InvalidTransitions
- Attempt invalid transitions (e.g., PreRun → Running)
- Should return error
- State should remain unchanged
- Error message should be descriptive

TestStateTransition_TerminalStates
- Attempt transitions from Completed → any state
- Attempt transitions from Failed → any state (except Retrying if retries remain)
- Attempt transitions from Cancelled → any state
- Attempt transitions from Orphaned → any state
- All should return error
- State should remain terminal

TestStateTransition_Concurrency
- Multiple goroutines attempt state transitions simultaneously
- Only valid transitions should succeed
- State should remain consistent
- No race conditions (run with -race flag)

TestStateTransition_PreRunPhase
- Verify PreRun → Pending is valid
- Verify PreRun → Cancelled is valid
- Verify PreRun → anything else is invalid

TestStateTransition_ConstraintPhase
- Verify Pending → ConditionPending is valid
- Verify ConditionPending → ConditionRunning is valid
- Verify ConditionRunning → ActionPending is valid (constraint violated)
- Verify ConditionRunning → ContainerCreating is valid (all passed)
- Verify ConditionRunning → Completed is invalid

TestStateTransition_ExecutionPhase
- Verify ContainerCreating → Running is valid
- Verify Running → Terminating is valid
- Verify Terminating → Completed is valid
- Verify Terminating → Failed is valid
- Verify Terminating → Retrying is valid

TestStateTransition_RetryPhase
- Verify Failed → Retrying is valid (if retries remain)
- Verify Retrying → Pending is valid
- Verify Retrying → Failed is valid (no retries left)
- Verify Retrying → Completed is invalid
```

#### State Duration Tracking Tests
```go
TestStateTracker_DurationTracking
- Transition through multiple states
- Verify duration is recorded for each state
- Verify durations sum to total elapsed time

TestStateTracker_StateDurations
- Stay in Running state for 100ms
- Stay in Terminating state for 50ms
- Verify durations are approximately correct (within margin)

TestStateTracker_MultipleVisits
- Transition Pending → ConditionPending → Pending (retry)
- Verify durations accumulate for revisited states
```

### 2. Lifecycle Tests

#### PreRun Phase Tests
```go
TestLifecycle_PreRun_WaitForScheduledTime
- Create orchestrator with scheduledAt 1 second in future
- Verify stays in PreRun state
- Verify heartbeats are sent periodically
- Verify transitions to Pending when scheduled time arrives

TestLifecycle_PreRun_AlreadyPast
- Create orchestrator with scheduledAt in the past
- Should immediately transition to Pending
- Should not wait

TestLifecycle_PreRun_ConfigUpdate
- Send config update while in PreRun
- Verify config is updated
- Verify remains in PreRun state
- Verify continues to scheduled time

TestLifecycle_PreRun_Cancellation
- Cancel orchestrator while in PreRun
- Should transition to Cancelled
- Should send completion message
- Should exit cleanly
```

#### Constraint Checking Phase Tests
```go
TestLifecycle_Constraints_NoConstraints
- Job with no constraint config
- Should skip constraint checking phase
- Should go directly from Pending → ContainerCreating

TestLifecycle_Constraints_AllPass
- Mock ConstraintChecker returns empty violations list
- Should transition Pending → ConditionPending → ConditionRunning → ContainerCreating
- Should record zero violations in metrics

TestLifecycle_Constraints_ViolationsReturned
- Mock ConstraintChecker returns ShouldProceed=false
- Should transition to Failed or Retrying
- Should record constraint check failure in metrics

TestLifecycle_Constraints_ViolationWithActionsResolve
- Mock ConstraintChecker initially has violations
- Constraint checker internally executes onViolation actions
- Actions resolve the issue (e.g., wait for resources)
- ConstraintChecker returns ShouldProceed=true
- Should transition to ContainerCreating
- Should record successful constraint resolution in metrics

TestLifecycle_Constraints_CheckError
- Mock ConstraintChecker returns error
- Should handle gracefully (fail-safe or fail-closed based on config)
- Should log error
- Should record error in metrics
```

#### Action Communication Tests
```go
TestLifecycle_Action_StateChangeMessages
- Mock ConstraintChecker sends state change messages during action execution
- Should transition orchestrator to ActionPending when actions start
- Should transition to ActionRunning while actions execute
- Should transition to appropriate state when actions complete
- Verify orchestrator correctly handles state change messages from actions

TestLifecycle_Action_LongRunningAction
- Mock ConstraintChecker executes long-running action (e.g., wait 5 seconds)
- Orchestrator should remain in ActionRunning state
- Should continue sending heartbeats during action execution
- Should eventually complete when action finishes

TestLifecycle_Action_CancellationDuringAction
- Mock ConstraintChecker is executing action
- Cancel orchestrator while in ActionRunning state
- Action execution should be cancelled
- Should transition to Cancelled
```

#### Execution Phase Tests
```go
TestLifecycle_Execution_Success
- Create Kubernetes job
- Job completes successfully (exit code 0)
- Should transition ContainerCreating → Running → Terminating → Completed
- Should record success metrics

TestLifecycle_Execution_Failure
- Create Kubernetes job
- Job fails (exit code 1)
- Should transition to Failed or Retrying (if retries configured)

TestLifecycle_Execution_Timeout
- Job exceeds maxAllowedRunTime
- Should kill job
- Should transition to Failed

TestLifecycle_Execution_PodStartTimeout
- Pod never transitions to Running
- Should timeout after threshold
- Should transition to Failed

TestLifecycle_Execution_Cancellation
- Cancel orchestrator during execution
- Should delete Kubernetes job
- Should transition to Cancelled
- Should clean up resources
```

#### Termination Phase Tests
```go
TestLifecycle_Termination_Success
- Job completes with exit code 0
- Should retrieve logs (if configured)
- Should check post-execution constraints
- Should transition to Completed

TestLifecycle_Termination_WithRetry
- Job fails but retries configured
- Should transition to Retrying
- Should calculate retry delay
- Should eventually transition to Pending

TestLifecycle_Termination_NoRetriesLeft
- Job fails, no retries remaining
- Should transition to Failed
- Should record final failure
```

### 3. Heartbeat Tests

```go
TestHeartbeat_Periodic
- Orchestrator sends heartbeat every interval
- Verify heartbeat messages sent to inbox
- Verify heartbeat includes correct runID and timestamp

TestHeartbeat_AllPhases
- Verify heartbeats sent in PreRun phase
- Verify heartbeats sent in ConditionRunning phase
- Verify heartbeats sent in Running phase
- Verify heartbeats stop after terminal state

TestHeartbeat_StopsOnTerminal
- Orchestrator reaches Completed state
- Verify no more heartbeats sent
- Verify heartbeat counter is final

TestHeartbeat_Frequency
- Verify heartbeat interval is respected
- Count heartbeats over 10 second period
- Should match expected count ±1
```

### 4. Cancellation Tests

```go
TestCancellation_PreRun
- Cancel while waiting for scheduled time
- Should transition to Cancelled immediately
- Should send completion message
- Should exit goroutine

TestCancellation_ConstraintChecking
- Cancel while checking constraints
- Should abort constraint checks
- Should transition to Cancelled
- Should not proceed to execution

TestCancellation_ActionExecution
- Cancel while executing action
- Should abort action
- Should transition to Cancelled
- Should clean up any action resources

TestCancellation_ContainerCreating
- Cancel while creating Kubernetes resources
- Should abort creation
- Should delete any partially created resources
- Should transition to Cancelled

TestCancellation_Running
- Cancel while job is executing
- Should delete Kubernetes job
- Should transition to Cancelled
- Should verify pod is terminated

TestCancellation_Terminating
- Cancel during cleanup phase
- Should complete cleanup
- Should transition to Cancelled (not Completed)

TestCancellation_AlreadyTerminal
- Attempt cancel on Completed orchestrator
- Should be no-op
- Should remain in Completed state

TestCancellation_Multiple
- Cancel same orchestrator multiple times
- Should handle gracefully
- Should only process first cancellation
```

### 5. Retry Tests

```go
TestRetry_Configuration
- Job with MaxRetries=3
- First attempt fails
- Should transition to Retrying
- Should track retry attempt number

TestRetry_ExponentialBackoff
- Job with backoff multiplier 2.0
- First retry: 1 second delay
- Second retry: 2 second delay
- Third retry: 4 second delay
- Verify delays are approximately correct

TestRetry_MaxRetries
- Job with MaxRetries=2
- Fail 3 times
- Should attempt total of 3 times (initial + 2 retries)
- Should transition to Failed after last attempt

TestRetry_SuccessOnRetry
- Job fails first attempt
- Job succeeds on second attempt
- Should transition to Completed
- Should record retry metrics

TestRetry_NoRetryConfig
- Job with no retry configuration
- First failure should transition to Failed
- Should not transition to Retrying

TestRetry_MaxDelayLimit
- Job with MaxDelay=5s and high backoff
- Calculated delay exceeds MaxDelay
- Should cap at MaxDelay

TestRetry_CancellationDuringRetry
- Cancel orchestrator while in Retrying state
- Should transition to Cancelled
- Should not attempt retry
```

### 6. Kubernetes Integration Tests

#### Job Creation Tests
```go
TestKubernetes_CreateJob
- Create Kubernetes job from pod spec
- Verify job is created in cluster
- Verify labels are set correctly (job-id, run-id, scheduled-at)
- Verify pod spec matches job config

TestKubernetes_CreateJob_InvalidSpec
- Job config has invalid pod spec
- Should return error
- Should transition to Failed

TestKubernetes_CreateJob_AlreadyExists
- Job with same name already exists
- Should handle gracefully
- Should either use existing or create with unique name

TestKubernetes_CreateJob_NamespaceNotFound
- Target namespace doesn't exist
- Should return error
- Should transition to Failed
```

#### Pod Monitoring Tests
```go
TestKubernetes_PodWatcher_Success
- Job pod transitions Pending → Running → Succeeded
- Should track each phase transition
- Should capture exit code 0
- Should transition orchestrator states accordingly

TestKubernetes_PodWatcher_Failure
- Job pod transitions Pending → Running → Failed
- Should capture exit code (non-zero)
- Should transition to Failed or Retrying

TestKubernetes_PodWatcher_PodDeleted
- Pod is deleted externally while orchestrator running
- Should detect deletion
- Should handle as failure

TestKubernetes_PodWatcher_MultipleRestarts
- Pod restarts multiple times (CrashLoopBackOff)
- Should track restart count
- Should eventually timeout or fail
```

#### Log Retrieval Tests
```go
TestKubernetes_RetrieveLogs_Success
- Job completes successfully
- Retrieve pod logs
- Verify logs are captured
- Verify logs stored/returned correctly

TestKubernetes_RetrieveLogs_PodGone
- Attempt to retrieve logs after pod deleted
- Should handle gracefully
- Should log warning but not fail orchestrator

TestKubernetes_RetrieveLogs_TooLarge
- Pod logs exceed size limit
- Should truncate logs
- Should include truncation marker
```

#### Cleanup Tests
```go
TestKubernetes_Cleanup_Success
- Job completes
- Should delete job resource
- Should verify pod is cleaned up
- Should handle cleanup errors gracefully

TestKubernetes_Cleanup_OnCancellation
- Cancel orchestrator
- Should delete job immediately
- Should not wait for completion

TestKubernetes_Cleanup_OrphanedJob
- Orchestrator marked orphaned
- Job should remain running (handled by scheduler/operator)
```

### 7. Metrics Collection Tests

```go
TestMetrics_PhaseTimings
- Orchestrator goes through all phases
- Verify timing metrics for each phase
- PreRunDuration, ConstraintCheckDuration, etc. should be > 0
- Sum of phase durations should equal total duration

TestMetrics_StateTransitions
- Transition through multiple states
- Verify StateTransitionCount is accurate
- Verify all states have timestamps

TestMetrics_ConstraintMetrics
- Check 5 constraints, 2 violated
- ConstraintsChecked should be 5
- ConstraintsViolated should be 2

TestMetrics_ActionMetrics
- Execute 3 actions, 1 fails
- ActionsExecuted should be 3
- ActionsFailed should be 1

TestMetrics_RetryMetrics
- Fail twice, succeed on third attempt
- RetryAttempt should be 2 (third attempt)
- MaxRetries should reflect configuration

TestMetrics_KubernetesMetrics
- Track Kubernetes API calls
- KubernetesAPICalls should count create, watch, delete operations
- PodStartLatency should be time from create to running

TestMetrics_HeartbeatCount
- Run orchestrator for 10 seconds with 1s heartbeat interval
- HeartbeatsSent should be ~10

TestMetrics_FinalState
- Complete successfully: FinalState=Completed, Success=true
- Fail: FinalState=Failed, Success=false
- Cancel: FinalState=Cancelled, Success=false

TestMetrics_Submission
- Orchestrator completes
- Verify metrics submitted to stats collector
- Verify all fields populated correctly
```

### 7. Error Handling Tests

```go
TestError_DatabaseUnavailable
- Cannot check constraints (database down)
- Should handle gracefully
- Should decide fail-safe or fail-closed
- Should record error

TestError_KubernetesAPIError
- Kubernetes API returns error on job creation
- Should retry with backoff
- Should eventually fail if persistent

TestError_NetworkTimeout
- Action executor call times out
- Should handle gracefully
- Should record timeout

TestError_PanicRecovery
- Action execution panics
- Should recover from panic
- Should transition to Failed
- Should log panic details

TestError_InvalidJobConfig
- Job config is invalid/corrupted
- Should detect early
- Should transition to Failed
- Should not attempt execution

TestError_ResourceExhaustion
- Kubernetes cluster has no resources
- Pod stays in Pending
- Should timeout appropriately
- Should record error reason
```

### 8. Integration Tests

```go
TestIntegration_HappyPath
- Complete end-to-end orchestrator lifecycle
- PreRun → Pending → ContainerCreating → Running → Terminating → Completed
- Verify Kubernetes job created and completes
- Verify all metrics collected
- Verify completion message sent

TestIntegration_WithConstraints
- Job with mock constraint checker
- Mock returns constraint violations initially
- Constraint checker internally executes onViolation actions
- Actions resolve violations
- Eventually returns ShouldProceed=true and proceeds to execution
- Verify complete flow with metrics

TestIntegration_WithRetry
- Job fails first attempt
- Retry configured
- Second attempt succeeds
- Verify complete retry flow
- Verify retry metrics

TestIntegration_ConcurrentOrchestrators
- Launch 10 orchestrators simultaneously
- Some pass constraint checks, some require actions
- All complete successfully
- Verify no race conditions
- Verify all metrics correct

TestIntegration_LongRunningJob
- Job runs for 30 seconds
- Verify heartbeats throughout
- Verify no timeouts
- Verify successful completion

TestIntegration_MultipleRetries
- Job fails 3 times, succeeds on 4th
- Verify complete retry flow
- Verify exponential backoff
- Verify metrics track all attempts
```

### 9. Mock Tests (No Real Kubernetes)

```go
TestMock_FullLifecycle
- Use mock Kubernetes client
- Simulate complete lifecycle
- Verify state transitions
- Verify all messages sent

TestMock_PodFailureScenarios
- Mock pod failures (exit codes 1-255)
- Verify appropriate handling for each

TestMock_KubernetesClientErrors
- Mock various K8s API errors
- Verify error handling
- Verify retry logic

TestMock_ConstraintChecks
- Mock ConstraintChecker interface
- Test orchestrator's constraint checking phase in isolation
- Verify correct state transitions based on constraint results
```

## Helper Functions

```go
// Test utilities
func createTestOrchestrator(t *testing.T, jobConfig *Job) *Orchestrator
func waitForState(t *testing.T, orch *Orchestrator, state OrchestratorStatus, timeout time.Duration)
func triggerCancellation(orch *Orchestrator)
func createMockK8sClient() kubernetes.Interface
func createMockSchedulerInbox() *inbox.Inbox
func createMockWebhookHandler() *webhook.Handler
func createMockConstraintChecker() ConstraintChecker
func verifyJobDeleted(t *testing.T, k8sClient kubernetes.Interface, jobName string)
func countHeartbeats(inbox *inbox.Inbox) int
func waitForCompletion(t *testing.T, orch *Orchestrator, timeout time.Duration) OrchestratorStatus
```

## Success Criteria

Tests must:
- ✅ All unit tests pass
- ✅ All integration tests pass
- ✅ No data races detected with `go test -race`
- ✅ Code coverage > 85%
- ✅ State machine transitions are exhaustively tested
- ✅ All error paths are tested
- ✅ Metrics collection is verified
- ✅ Cancellation is tested in all phases
- ✅ Kubernetes integration is tested (with mocks and real cluster)
