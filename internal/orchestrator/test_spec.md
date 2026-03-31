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
- Verify Retrying → Pending is valid (skip constraint re-check)
- Verify Retrying → ConditionPending is valid (re-check constraints)
- Verify Retrying → Failed is valid (no retries left)
- Verify Retrying → Completed is invalid
```

#### Phase Timing Tests
```go
TestPhaseTiming_Boundaries
- Orchestrator goes through complete lifecycle
- Verify phase boundary timestamps are captured:
  - createdAt when orchestrator created
  - constraintCheckStarted when entering ConditionPending
  - executionStartedAt when pod starts running
  - completedAt when reaching terminal state

TestPhaseTiming_Durations
- Run orchestrator with known delays in each phase
- Verify calculated phase durations are approximately correct:
  - PreRunDuration = constraintCheckStarted - createdAt
  - ConstraintCheckDuration = executionStartedAt - constraintCheckStarted
  - ExecutionDuration = completedAt - executionStartedAt
  - TotalDuration = completedAt - createdAt
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

TestRetry_RecheckConstraints
- Mock constraint checker returns ShouldRecheckOnRetry() = true
- First attempt fails
- Should transition Retrying → ConditionPending (not Pending)
- Should call CheckPreExecution() again before retry attempt
- Verify constraint checker called on retry

TestRetry_SkipConstraintRecheck
- Mock constraint checker returns ShouldRecheckOnRetry() = false
- First attempt fails
- Should transition Retrying → Pending (skip ConditionPending)
- Should not call CheckPreExecution() on retry
- Should go directly to execution
```

### 6. Metrics Collection Tests

```go
TestMetrics_PhaseTimings
- Orchestrator goes through all phases
- Verify phase timing metrics are populated:
  - PreRunDuration > 0
  - ConstraintCheckDuration > 0
  - ExecutionDuration > 0
  - TotalDuration equals sum of phase durations

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

**Note:** Integration tests verify the complete orchestrator lifecycle with all components working together, but still use `fake.Clientset` for Kubernetes operations.

```go
TestIntegration_HappyPath
- Complete end-to-end orchestrator lifecycle
- PreRun → Pending → ContainerCreating → Running → Terminating → Completed
- Verify fake Kubernetes job created and completes
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

### 9. Mock Tests

**Note:** All orchestrator tests use `fake.Clientset` from `k8s.io/client-go/kubernetes/fake` for Kubernetes operations. This provides a fully functional in-memory Kubernetes client without requiring an actual cluster. Real integration tests with Docker-in-Docker will be added later for the full scheduler system.

#### Mock Kubernetes Client Tests
```go
TestMock_FullLifecycle
- Use fake.NewSimpleClientset()
- Simulate complete lifecycle with pod state changes
- Verify state transitions
- Verify all messages sent

TestMock_JobCreation
- Create Kubernetes job from pod spec using fake client
- Verify job is created in fake client
- Verify labels are set correctly (job-id, run-id, scheduled-at)
- Verify pod spec matches job config

TestMock_JobCreation_InvalidSpec
- Job config has invalid pod spec
- Should return error
- Should transition to Failed

TestMock_PodWatcher_Success
- Create fake pod in Pending state
- Transition to Running, then Succeeded
- Should track each phase transition
- Should capture exit code 0
- Should transition orchestrator states accordingly

TestMock_PodWatcher_Failure
- Create fake pod that transitions to Failed
- Should capture exit code (non-zero)
- Should transition to Failed or Retrying

TestMock_PodWatcher_PodDeleted
- Delete fake pod while orchestrator running
- Should detect deletion via watch
- Should handle as failure

TestMock_PodFailureScenarios
- Test various pod failure scenarios (exit codes 1-255)
- Verify appropriate orchestrator handling for each

TestMock_KubernetesAPIErrors
- Simulate K8s API errors from fake client
- Verify error handling
- Verify retry logic

TestMock_LogRetrieval
- Fake pod completes successfully
- Mock log retrieval from fake client
- Verify logs are captured

TestMock_Cleanup
- Job completes
- Verify job resource deleted from fake client
- Handle cleanup errors gracefully

TestMock_CleanupOnCancellation
- Cancel orchestrator during execution
- Verify job deleted immediately from fake client
```

#### Mock Constraint/Action Tests
```go
TestMock_ConstraintChecks
- Mock ConstraintChecker interface
- Test orchestrator's constraint checking phase in isolation
- Verify correct state transitions based on constraint results

TestMock_ActionExecution
- Mock ConstraintChecker with various action scenarios
- Verify orchestrator handles action outcomes correctly
- Verify state transitions match action results
```

## Helper Functions

```go
// Test utilities
func createTestOrchestrator(t *testing.T, jobConfig *Job) *Orchestrator
func waitForState(t *testing.T, orch *Orchestrator, state OrchestratorStatus, timeout time.Duration)
func triggerCancellation(orch *Orchestrator)
func createFakeK8sClient() *fake.Clientset  // Returns fake.NewSimpleClientset()
func createMockSchedulerInbox() *inbox.Inbox
func createMockWebhookHandler() *webhook.Handler
func createMockConstraintChecker(shouldProceed bool, err error) *MockConstraintChecker
func createMockConstraintCheckerWithRecheck(shouldProceed bool, recheckOnRetry bool, err error) *MockConstraintChecker
func verifyJobCreated(t *testing.T, fakeClient *fake.Clientset, jobName string)
func verifyJobDeleted(t *testing.T, fakeClient *fake.Clientset, jobName string)
func simulatePodStateChange(fakeClient *fake.Clientset, podName string, newPhase corev1.PodPhase)
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
- ✅ Kubernetes integration is tested using `fake.Clientset` (real cluster tests via DinD will be added later for full scheduler)
