# Constraint Module Test Specification

## Test Organization

Tests are organized into the following categories:

1. **Constraint Unit Tests** - Test individual constraint types
2. **Action Integration Tests** - Test constraint module integration with actions
3. **ConstraintChecker Integration Tests** - Test the full constraint checking flow
4. **Scheduler Communication Tests** - Test message-based communication with scheduler
5. **Configuration Parsing Tests** - Test JSON configuration parsing
6. **Error Handling Tests** - Test error conditions and recovery
7. **ExecutionContext Tests** - Test execution context and dependency injection
8. **Thread Safety Tests** - Test concurrent access patterns
9. **Integration with Orchestrator Tests** - Test orchestrator integration

## 1. Constraint Unit Tests

### TimeWindowConstraint Tests

**TestTimeWindowConstraint_WithinWindow**
- Create constraint with window 09:00-17:00
- Mock current time to 12:00
- Call Check()
- Assert Met = true
- Assert message indicates success

**TestTimeWindowConstraint_BeforeWindow**
- Create constraint with window 09:00-17:00
- Mock current time to 08:00
- Call Check()
- Assert Met = false
- Assert message indicates outside window

**TestTimeWindowConstraint_AfterWindow**
- Create constraint with window 09:00-17:00
- Mock current time to 18:00
- Call Check()
- Assert Met = false
- Assert message indicates outside window

**TestTimeWindowConstraint_Timezone**
- Create constraint with specific timezone
- Verify time calculations respect timezone
- Test across timezone boundaries

**TestTimeWindowConstraint_ShouldRecheckOnRetry**
- Create constraint with recheckOnRetry = true
- Assert ShouldRecheckOnRetry() returns true
- Create constraint with recheckOnRetry = false
- Assert ShouldRecheckOnRetry() returns false

### OtherJobRunningConstraint Tests

**TestOtherJobRunningConstraint_JobIsRunning**
- Create constraint with shouldBeRunning=false
- Create mock scheduler that returns IsRunning=true
- Call Check()
- Assert Met = false
- Verify scheduler was queried with correct JobID

**TestOtherJobRunningConstraint_JobIsNotRunning**
- Create constraint with shouldBeRunning=false
- Create mock scheduler that returns IsRunning=false
- Call Check()
- Assert Met = true

**TestOtherJobRunningConstraint_ExpectRunning**
- Create constraint with shouldBeRunning=true
- Create mock scheduler that returns IsRunning=true
- Call Check()
- Assert Met = true

**TestOtherJobRunningConstraint_SchedulerError**
- Create constraint
- Create mock scheduler that returns error
- Call Check()
- Assert error is propagated

**TestOtherJobRunningConstraint_ContextCancelled**
- Create constraint
- Cancel context before response arrives
- Call Check()
- Assert returns context.Canceled error

### OtherJobCompletedRecentlyConstraint Tests

**TestOtherJobCompletedRecently_WithinWindow**
- Create constraint with within=30m, mustSucceed=false
- Create mock scheduler that returns run completed 15m ago
- Call Check()
- Assert Met = true

**TestOtherJobCompletedRecently_OutsideWindow**
- Create constraint with within=30m
- Create mock scheduler that returns run completed 45m ago
- Call Check()
- Assert Met = false

**TestOtherJobCompletedRecently_MustSucceed_Success**
- Create constraint with mustSucceed=true
- Create mock scheduler that returns successful run
- Call Check()
- Assert Met = true

**TestOtherJobCompletedRecently_MustSucceed_Failed**
- Create constraint with mustSucceed=true
- Create mock scheduler that returns failed run
- Call Check()
- Assert Met = false

**TestOtherJobCompletedRecently_NoRuns**
- Create constraint
- Create mock scheduler that returns empty history
- Call Check()
- Assert Met = false
- Assert message indicates no recent runs

### OtherJobScheduledSoonConstraint Tests

**TestOtherJobScheduledSoon_ScheduledWithinWindow**
- Create constraint with within=10m
- Create mock scheduler that returns NextRun in 5m
- Call Check()
- Assert Met = true

**TestOtherJobScheduledSoon_ScheduledOutsideWindow**
- Create constraint with within=10m
- Create mock scheduler that returns NextRun in 30m
- Call Check()
- Assert Met = false

**TestOtherJobScheduledSoon_NoScheduledRun**
- Create constraint
- Create mock scheduler that returns NextRun=nil
- Call Check()
- Assert Met = false

**TestOtherJobScheduledSoon_ScheduledInPast**
- Create constraint with within=10m
- Create mock scheduler that returns NextRun in the past
- Call Check()
- Assert Met = false

### HTTPHealthCheckConstraint Tests

**TestHTTPHealthCheck_Success**
- Create constraint with GET request to mock HTTP server
- Mock server returns 200 OK
- Call Check()
- Assert Met = true
- Verify correct URL was called

**TestHTTPHealthCheck_Failure**
- Create constraint with GET request
- Mock server returns 500 error
- Call Check()
- Assert Met = false

**TestHTTPHealthCheck_URLTemplating**
- Create constraint with URL template "http://api.example.com/health?job={{.JobName}}"
- Create ExecutionContext with JobName="test-job"
- Call Check()
- Verify URL was rendered with correct job name

**TestHTTPHealthCheck_HeaderTemplating**
- Create constraint with header "X-Run-ID: {{.RunID}}"
- Create ExecutionContext with RunID="run-123"
- Call Check()
- Verify header was rendered correctly

**TestHTTPHealthCheck_Timeout**
- Create constraint with 100ms timeout
- Mock server with 5s delay
- Call Check()
- Assert returns timeout error
- Verify completes in ~100ms

**TestHTTPHealthCheck_ContextCancellation**
- Create constraint with long timeout
- Cancel context during request
- Call Check()
- Assert returns context.Canceled error

**TestHTTPHealthCheck_POSTMethod**
- Create constraint with POST method
- Mock server
- Call Check()
- Verify POST method was used

### MaxRuntimeConstraint Tests

**TestMaxRuntime_WithinLimit**
- Create constraint with maxDuration=2h
- Create ExecutionContext with StartTime 1h ago
- Call Check()
- Assert Met = true

**TestMaxRuntime_ExceedsLimit**
- Create constraint with maxDuration=2h
- Create ExecutionContext with StartTime 3h ago
- Call Check()
- Assert Met = false

**TestMaxRuntime_AtLimit**
- Create constraint with maxDuration=1h
- Create ExecutionContext with StartTime exactly 1h ago
- Call Check()
- Assert Met = true (equal to limit)

**TestMaxRuntime_NoStartTime**
- Create constraint
- Create ExecutionContext with StartTime=nil
- Call Check()
- Assert returns error about missing start time

**TestMaxRuntime_EvaluationPhase**
- Create constraint
- Assert EvaluationTiming() returns only DuringExecution phase

### MinRuntimeConstraint Tests

**TestMinRuntime_MeetsMinimum**
- Create constraint with minDuration=30s
- Create ExecutionContext with runtime of 45s
- Call Check()
- Assert Met = true

**TestMinRuntime_BelowMinimum**
- Create constraint with minDuration=30s
- Create ExecutionContext with runtime of 10s
- Call Check()
- Assert Met = false

**TestMinRuntime_AtMinimum**
- Create constraint with minDuration=30s
- Create ExecutionContext with runtime of exactly 30s
- Call Check()
- Assert Met = true

**TestMinRuntime_NoTiming**
- Create constraint
- Create ExecutionContext with StartTime=nil or EndTime=nil
- Call Check()
- Assert returns error about missing timing

**TestMinRuntime_EvaluationPhase**
- Create constraint
- Assert EvaluationTiming() returns only PostExecution phase

### AlwaysPassConstraint Tests

**TestAlwaysPassConstraint_AlwaysReturnsTrue**
- Create AlwaysPassConstraint
- Call Check() multiple times
- Assert Met = true every time

### AlwaysFailConstraint Tests

**TestAlwaysFailConstraint_AlwaysReturnsFalse**
- Create AlwaysFailConstraint
- Call Check() multiple times
- Assert Met = false every time

### Constraint EvaluationTiming Tests

**TestTimeWindowConstraint_EvaluationTiming**
- Create TimeWindowConstraint
- Call EvaluationTiming()
- Assert returns [EvaluationPhasePreExecution]

**TestOtherJobRunningConstraint_EvaluationTiming**
- Create OtherJobRunningConstraint
- Call EvaluationTiming()
- Assert returns [EvaluationPhasePreExecution]

**TestOtherJobCompletedRecentlyConstraint_EvaluationTiming**
- Create OtherJobCompletedRecentlyConstraint
- Call EvaluationTiming()
- Assert returns [EvaluationPhasePreExecution]

**TestOtherJobScheduledSoonConstraint_EvaluationTiming**
- Create OtherJobScheduledSoonConstraint
- Call EvaluationTiming()
- Assert returns [EvaluationPhasePreExecution]

**TestHTTPHealthCheckConstraint_EvaluationTiming**
- Create HTTPHealthCheckConstraint
- Call EvaluationTiming()
- Assert returns [EvaluationPhasePreExecution] (default)
- Note: Can be configured for multiple phases

**TestMaxRuntimeConstraint_EvaluationTiming**
- Create MaxRuntimeConstraint
- Call EvaluationTiming()
- Assert returns [EvaluationPhaseDuringExecution]

**TestMinRuntimeConstraint_EvaluationTiming**
- Create MinRuntimeConstraint
- Call EvaluationTiming()
- Assert returns [EvaluationPhasePostExecution]

**TestAlwaysPassConstraint_EvaluationTiming**
- Create AlwaysPassConstraint with specific phases
- Call EvaluationTiming()
- Assert returns configured phases

**TestAlwaysFailConstraint_EvaluationTiming**
- Create AlwaysFailConstraint with specific phases
- Call EvaluationTiming()
- Assert returns configured phases

### Constraint Name Tests

**TestConstraint_Name**
- Create each constraint type with a specific name
- Call Name()
- Assert returns the configured name

### Constraint ShouldRecheckOnRetry Tests

**TestConstraint_ShouldRecheckOnRetry_True**
- Create each constraint type with recheckOnRetry=true
- Call ShouldRecheckOnRetry()
- Assert returns true

**TestConstraint_ShouldRecheckOnRetry_False**
- Create each constraint type with recheckOnRetry=false
- Call ShouldRecheckOnRetry()
- Assert returns false

## 2. Action Integration Tests

**Note**: These tests verify that the constraint module correctly integrates with and executes actions. Detailed action implementation tests should be in the `/internal/actions/test_spec.md`. These tests focus on the constraint-action interaction.

### DelayAction Tests

**TestDelayAction_CompletesAfterDuration**
- Create DelayAction with 100ms duration
- Record start time
- Call Execute()
- Record end time
- Assert elapsed time >= 100ms
- Assert no error

**TestDelayAction_CancellationBeforeCompletion**
- Create DelayAction with 5s duration
- Create context with 100ms timeout
- Call Execute() with context
- Assert returns error (context.DeadlineExceeded)
- Assert returns before 5s elapsed

**TestDelayAction_Name**
- Create DelayAction with specific name
- Assert Name() returns expected name

### WebhookAction Tests

**TestWebhookAction_SendsWebhook**
- Create mock WebhookSender
- Create WebhookAction with URL and payload
- Create ExecutionContext with mock sender
- Call Execute()
- Assert webhook sender was called once
- Assert URL matches expected
- Assert payload matches expected

**TestWebhookAction_WebhookError**
- Create mock WebhookSender that returns error
- Create WebhookAction
- Call Execute()
- Assert error is propagated

**TestWebhookAction_LogsWebhookSend**
- Create WebhookAction with test logger
- Call Execute()
- Verify log message contains webhook URL and runID

### LogAction Tests

**TestLogAction_LogsMessage**
- Create LogAction with specific message
- Create ExecutionContext with test logger
- Call Execute()
- Verify log output contains expected message
- Verify log output contains runID

**TestLogAction_NoError**
- Create LogAction
- Call Execute()
- Assert returns nil error

### FailAction Tests

**TestFailAction_ReturnsError**
- Create FailAction with specific reason
- Call Execute()
- Assert returns error
- Assert error message contains reason

**TestFailAction_LogsFailure**
- Create FailAction with test logger
- Call Execute()
- Verify log output contains error level message
- Verify log output contains reason and runID

### NoOpAction Tests

**TestNoOpAction_DoesNothing**
- Create NoOpAction
- Call Execute()
- Assert returns nil error
- Verify no side effects

## 3. ConstraintChecker Integration Tests

### Basic Flow Tests

**TestConstraintChecker_SingleConstraintMet**
- Create config with one AlwaysPassConstraint
- Add onMet action (LogAction)
- Create constraint checker
- Call CheckPreExecution()
- Assert ShouldProceed = true
- Verify onMet action was executed

**TestConstraintChecker_SingleConstraintViolated**
- Create config with one AlwaysFailConstraint
- Add onViolation action (LogAction)
- Create constraint checker
- Call CheckPreExecution()
- Assert ShouldProceed = false
- Verify onViolation action was executed

**TestConstraintChecker_MultipleConstraintsAllMet**
- Create config with 3 AlwaysPassConstraints
- Each has onMet action
- Call CheckPreExecution()
- Assert ShouldProceed = true
- Verify all 3 onMet actions executed

**TestConstraintChecker_MultipleConstraintsOneFails**
- Create config with 3 constraints (2 pass, 1 fail)
- Call CheckPreExecution()
- Assert ShouldProceed = false
- Verify failing constraint's onViolation executed
- Verify passing constraints' onMet executed

**TestConstraintChecker_NoConstraints**
- Create empty config (no constraints)
- Create constraint checker
- Call CheckPreExecution()
- Assert ShouldProceed = true

### Action Execution Tests

**TestConstraintChecker_MultipleActionsOnViolation**
- Create constraint that fails
- Add 3 onViolation actions (Log, Webhook, Delay)
- Call CheckPreExecution()
- Verify all 3 actions executed in order

**TestConstraintChecker_ActionExecutionError**
- Create constraint that fails
- Add onViolation action that returns error
- Add second onViolation action (should still execute)
- Call CheckPreExecution()
- Assert ShouldProceed = false (constraint still failed)
- Verify error was logged
- Verify second action still executed

### ShouldRecheckOnRetry Tests

**TestConstraintChecker_ShouldRecheckOnRetry_True**
- Create config with 3 constraints
- Set recheckOnRetry = true for one constraint
- Create constraint checker
- Call ShouldRecheckOnRetry()
- Assert returns true

**TestConstraintChecker_ShouldRecheckOnRetry_False**
- Create config with 3 constraints
- Set recheckOnRetry = false for all
- Create constraint checker
- Call ShouldRecheckOnRetry()
- Assert returns false

**TestConstraintChecker_ShouldRecheckOnRetry_NoConstraints**
- Create empty config
- Create constraint checker
- Call ShouldRecheckOnRetry()
- Assert returns false

### Multi-Phase Evaluation Tests

**TestConstraintChecker_CheckPreExecution_OnlyRunsPrePhase**
- Create config with constraints for all three phases (pre, during, post)
- Call CheckPreExecution()
- Assert only pre-execution constraints were evaluated
- Verify during and post constraints were not evaluated

**TestConstraintChecker_CheckDuringExecution_OnlyRunsDuringPhase**
- Create config with constraints for all three phases
- Call CheckDuringExecution() with startTime
- Assert only during-execution constraints were evaluated
- Verify pre and post constraints were not evaluated

**TestConstraintChecker_CheckPostExecution_OnlyRunsPostPhase**
- Create config with constraints for all three phases
- Call CheckPostExecution() with timing and exitCode
- Assert only post-execution constraints were evaluated
- Verify pre and during constraints were not evaluated

**TestConstraintChecker_CheckDuringExecution_RequiresStartTime**
- Create config with during-execution constraint (MaxRuntime)
- Call CheckDuringExecution() with valid startTime
- Assert constraint is evaluated correctly

**TestConstraintChecker_CheckPostExecution_RequiresTiming**
- Create config with post-execution constraint (MinRuntime)
- Call CheckPostExecution() with startTime, endTime, exitCode
- Assert constraint is evaluated correctly

**TestConstraintChecker_MultiplePhaseConstraint**
- Create constraint that applies to multiple phases (e.g., HTTPHealthCheck for pre and post)
- Call CheckPreExecution() - verify constraint runs
- Call CheckDuringExecution() - verify constraint doesn't run
- Call CheckPostExecution() - verify constraint runs

## 4. Scheduler Communication Tests

**TestSchedulerCommunication_JobStateRequest**
- Create OtherJobRunningConstraint
- Create mock scheduler inbox that captures messages
- Call Check()
- Verify JobStateRequest was sent with correct JobID
- Verify constraint waits for response on ResponseTo channel

**TestSchedulerCommunication_JobStateResponse**
- Create OtherJobRunningConstraint
- Create mock scheduler that sends JobStateResponse
- Call Check()
- Verify constraint correctly interprets response
- Verify correct ConstraintResult returned

**TestSchedulerCommunication_JobHistoryRequest**
- Create OtherJobCompletedRecentlyConstraint
- Create mock scheduler inbox
- Call Check()
- Verify JobHistoryRequest was sent
- Verify Limit parameter is set correctly

**TestSchedulerCommunication_JobHistoryResponse**
- Create OtherJobCompletedRecentlyConstraint
- Create mock scheduler that returns multiple runs
- Call Check()
- Verify constraint processes the most recent run

**TestSchedulerCommunication_ContextCancellation**
- Create constraint that queries scheduler
- Send scheduler request but don't respond
- Cancel context
- Verify constraint returns context.Canceled error
- Verify doesn't hang waiting for response

**TestSchedulerCommunication_SchedulerSendError**
- Create constraint that queries scheduler
- Create mock inbox that returns error on Send()
- Call Check()
- Verify error is propagated

**TestSchedulerCommunication_ConcurrentRequests**
- Create multiple constraints that query scheduler
- Run checks concurrently
- Verify all requests are handled correctly
- Verify no race conditions (run with -race flag)

## 5. Configuration Parsing Tests

### Valid Configuration Tests

**TestParseConstraints_ValidSingleConstraint**
- Create JSON config with one constraint
- Parse configuration
- Assert constraint created successfully
- Assert constraint type matches
- Assert constraint name matches

**TestParseConstraints_ValidMultipleConstraints**
- Create JSON config with 3 different constraint types
- Parse configuration
- Assert 3 constraints created
- Verify each constraint has correct type and config

**TestParseConstraints_ValidActionsOnViolation**
- Create JSON config with constraint and multiple onViolation actions
- Parse configuration
- Assert actions created correctly
- Verify action types and configs

**TestParseConstraints_ValidActionsOnMet**
- Create JSON config with constraint and multiple onMet actions
- Parse configuration
- Assert actions created correctly

**TestParseConstraints_EmptyConfiguration**
- Create empty JSON config
- Parse configuration
- Assert no error
- Assert empty constraint list

### Invalid Configuration Tests

**TestParseConstraints_InvalidJSON**
- Create malformed JSON
- Attempt to parse
- Assert returns error
- Assert error message is helpful

**TestParseConstraints_UnknownConstraintType**
- Create JSON config with type="unknown_type"
- Attempt to parse
- Assert returns error
- Assert error message mentions unknown type

**TestParseConstraints_UnknownActionType**
- Create JSON config with action type="unknown_action"
- Attempt to parse
- Assert returns error
- Assert error message mentions unknown action type

**TestParseConstraints_MissingRequiredField**
- Create JSON config missing required field (e.g., constraint name)
- Attempt to parse
- Assert returns error

**TestParseConstraints_InvalidFieldType**
- Create JSON config with wrong field type (e.g., duration as string)
- Attempt to parse
- Assert returns error

### Constraint-Specific Config Parsing Tests

**TestParseTimeWindowConstraint_Valid**
- Create valid time window config
- Parse constraint
- Assert startTime, endTime, timezone correctly parsed

**TestParseTimeWindowConstraint_InvalidTimeFormat**
- Create config with invalid time format
- Attempt to parse
- Assert returns error

**TestParseTimeWindowConstraint_InvalidTimezone**
- Create config with invalid timezone
- Attempt to parse
- Assert returns error

**TestParseOtherJobRunningConstraint_Valid**
- Create valid config with jobID and shouldBeRunning
- Parse constraint
- Assert fields correctly parsed

**TestParseOtherJobRunningConstraint_MissingJobID**
- Create config without jobID
- Attempt to parse
- Assert returns error

**TestParseOtherJobCompletedRecentlyConstraint_Valid**
- Create valid config with jobID, within, mustSucceed
- Parse constraint
- Assert fields correctly parsed

**TestParseOtherJobCompletedRecentlyConstraint_InvalidDuration**
- Create config with invalid "within" duration
- Attempt to parse
- Assert returns error

**TestParseOtherJobScheduledSoonConstraint_Valid**
- Create valid config with jobID and within
- Parse constraint
- Assert fields correctly parsed

**TestParseHTTPHealthCheckConstraint_Valid**
- Create valid config with URL, method, headers, timeout
- Parse constraint
- Assert all fields correctly parsed
- Assert templates are compiled correctly

**TestParseHTTPHealthCheckConstraint_InvalidURLTemplate**
- Create config with invalid template syntax in URL
- Attempt to parse
- Assert returns error

**TestParseHTTPHealthCheckConstraint_InvalidHeaderTemplate**
- Create config with invalid template syntax in header
- Attempt to parse
- Assert returns error

**TestParseHTTPHealthCheckConstraint_InvalidTimeout**
- Create config with invalid timeout format
- Attempt to parse
- Assert returns error

**TestParseMaxRuntimeConstraint_Valid**
- Create valid config with maxDuration and checkInterval
- Parse constraint
- Assert fields correctly parsed

**TestParseMaxRuntimeConstraint_InvalidDuration**
- Create config with invalid maxDuration
- Attempt to parse
- Assert returns error

**TestParseMinRuntimeConstraint_Valid**
- Create valid config with minDuration
- Parse constraint
- Assert fields correctly parsed

**TestParseMinRuntimeConstraint_InvalidDuration**
- Create config with invalid minDuration
- Attempt to parse
- Assert returns error

**TestParseDelayAction_Valid**
- Create valid delay action config
- Parse action
- Assert duration correctly parsed

**TestParseDelayAction_InvalidDuration**
- Create config with invalid duration
- Attempt to parse
- Assert returns error

## 6. Error Handling Tests

### Constraint Check Errors

**TestConstraintChecker_ConstraintCheckError**
- Create constraint that returns error from Check()
- Create constraint checker
- Call CheckPreExecution()
- Assert error is propagated
- Verify subsequent constraints not checked

### Action Execution Errors

**TestConstraintChecker_ActionExecutionError_Logged**
- Create constraint with action that errors
- Call CheckPreExecution()
- Verify error is logged
- Verify doesn't fail entire check

**TestConstraintChecker_ActionExecutionError_SubsequentActionsRun**
- Create constraint with 3 actions (2nd one errors)
- Call CheckPreExecution()
- Verify 1st action executed
- Verify 2nd action executed (and errored)
- Verify 3rd action still executed

### Context Cancellation

**TestConstraintChecker_ContextCancelled_DuringCheck**
- Create constraint with long-running Check()
- Cancel context mid-check
- Verify Check() returns context.Canceled error

**TestConstraintChecker_ContextCancelled_DuringAction**
- Create action with long-running Execute()
- Cancel context mid-execution
- Verify action respects cancellation

## 7. ExecutionContext Tests

### Context Creation Tests

**TestExecutionContext_CreatedWithDependencies**
- Build execution context
- Assert Job is set
- Assert RunID is set
- Assert Logger is set
- Assert SchedulerInbox is set
- Assert HTTPClient is set
- Assert Context is set

**TestExecutionContext_NilDependencies**
- Following our testing principles, we should NOT test with nil dependencies
- All dependencies must be provided (real or mock)

### Dependency Injection Tests

**TestExecutionContext_SchedulerInboxUsed**
- Create mock scheduler inbox
- Create constraint that sends scheduler message (e.g., OtherJobRunningConstraint)
- Execute constraint with context
- Verify SchedulerInbox.Send() was called

**TestExecutionContext_HTTPClientUsed**
- Create HTTPHealthCheckConstraint
- Execute constraint with context
- Verify HTTPClient was used to make request

**TestExecutionContext_LoggerUsed**
- Create test logger
- Create LogAction
- Execute action with context
- Verify logger was used

## 8. Thread Safety Tests

**TestConstraintChecker_ConcurrentChecks**
- Create single constraint checker
- Launch 100 goroutines
- Each goroutine calls CheckPreExecution()
- Verify no race conditions (run with -race flag)
- Verify all checks complete successfully

**TestConstraintChecker_ConcurrentWithDifferentJobs**
- Create constraint checker
- Launch goroutines with different job configs
- Verify no data races
- Verify each check evaluates correct constraints

## 9. Integration with Orchestrator Tests

**TestConstraintChecker_OrchestratorIntegration_Success**
- Create orchestrator with constraint checker
- Configure job with passing constraints
- Start orchestrator
- Verify orchestrator transitions through constraint states
- Verify reaches execution phase

**TestConstraintChecker_OrchestratorIntegration_Failure**
- Create orchestrator with constraint checker
- Configure job with failing constraints
- Start orchestrator
- Verify orchestrator transitions to failed state
- Verify never reaches execution phase

**TestConstraintChecker_OrchestratorIntegration_WithActions**
- Create orchestrator with constraint checker
- Configure constraint with delay action
- Start orchestrator
- Verify delay is respected
- Verify orchestrator flow continues after delay

## Test Helpers

### Mock Implementations

```go
// MockSchedulerInbox for testing message sending to scheduler
type MockSchedulerInbox struct {
    messages []interface{}
    mu       sync.Mutex
}

func (m *MockSchedulerInbox) Send(msg interface{}) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.messages = append(m.messages, msg)

    // Auto-respond to known message types for testing
    switch req := msg.(type) {
    case *JobStateRequest:
        // Send mock response
        go func() {
            req.ResponseTo <- &JobStateResponse{
                JobID:     req.JobID,
                IsRunning: false,
                LastRun:   nil,
                NextRun:   nil,
            }
        }()
    case *JobHistoryRequest:
        // Send mock response
        go func() {
            req.ResponseTo <- &JobHistoryResponse{
                JobID: req.JobID,
                Runs:  []JobRunSummary{},
            }
        }()
    }

    return nil
}

func (m *MockSchedulerInbox) GetMessages() []interface{} {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.messages
}

// TestLogger for capturing log output
func createTestLogger() *slog.Logger {
    return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))
}

// Helper to create ExecutionContext for tests
func createTestExecutionContext() *ExecutionContext {
    return &ExecutionContext{
        Job:            &Job{ID: "test-job", Name: "test"},
        RunID:          "test-run-id",
        SchedulerInbox: &MockSchedulerInbox{},
        HTTPClient:     &http.Client{Timeout: 5 * time.Second},
        Logger:         createTestLogger(),
        Context:        context.Background(),
    }
}

// Helper to create constraint config JSON
func createConstraintConfigJSON(constraints []ConstraintConfig) json.RawMessage {
    config := struct {
        Constraints []ConstraintConfig `json:"constraints"`
    }{
        Constraints: constraints,
    }
    data, _ := json.Marshal(config)
    return data
}
```

## Test Coverage Goals

- **Line Coverage**: > 90%
- **Branch Coverage**: > 85%
- **All constraint types**: 100% coverage
- **All action types**: 100% coverage
- **Error paths**: All error conditions tested
- **Concurrent access**: Verified with -race flag

## Performance Benchmarks

Create benchmarks for:
- `BenchmarkConstraintChecker_SingleConstraint` - Measure check overhead
- `BenchmarkConstraintChecker_MultipleConstraints` - Scale with constraint count
- `BenchmarkActionExecution_Delay` - Measure delay action overhead
- `BenchmarkActionExecution_Webhook` - Measure webhook action overhead
- `BenchmarkConfigParsing` - Measure parsing overhead

Target: Single constraint check should complete in < 1ms (excluding action execution time)
