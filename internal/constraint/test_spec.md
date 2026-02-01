# Constraint Module Test Specification

## Test Organization

Tests are organized into the following categories:

1. **Constraint Unit Tests** - Test individual constraint types
2. **Action Unit Tests** - Test individual action types
3. **ConstraintChecker Integration Tests** - Test the full constraint checking flow
4. **Configuration Parsing Tests** - Test JSON configuration parsing
5. **Error Handling Tests** - Test error conditions and recovery
6. **Context Tests** - Test execution context and dependency injection

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

### ResourceAvailableConstraint Tests

**TestResourceAvailableConstraint_Available**
- Create mock ResourceChecker that returns available=true
- Create constraint with mock checker
- Call Check()
- Assert Met = true
- Verify checker was called with correct parameters

**TestResourceAvailableConstraint_Unavailable**
- Create mock ResourceChecker that returns available=false
- Create constraint with mock checker
- Call Check()
- Assert Met = false

**TestResourceAvailableConstraint_CheckerError**
- Create mock ResourceChecker that returns error
- Create constraint with mock checker
- Call Check()
- Assert error is propagated

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

## 2. Action Unit Tests

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

### Post-Execution Tests

**TestConstraintChecker_CheckPostExecution_Success**
- Create config with post-execution constraint
- Call CheckPostExecution() with exitCode=0
- Assert appropriate result

**TestConstraintChecker_CheckPostExecution_Failure**
- Create config with post-execution constraint
- Call CheckPostExecution() with exitCode=1
- Assert appropriate result

## 4. Configuration Parsing Tests

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

**TestParseDelayAction_Valid**
- Create valid delay action config
- Parse action
- Assert duration correctly parsed

**TestParseDelayAction_InvalidDuration**
- Create config with invalid duration
- Attempt to parse
- Assert returns error

## 5. Error Handling Tests

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

## 6. ExecutionContext Tests

### Context Creation Tests

**TestExecutionContext_CreatedWithDependencies**
- Build execution context
- Assert Job is set
- Assert RunID is set
- Assert Logger is set
- Assert Inbox is set
- Assert WebhookHandler is set
- Assert Context is set

**TestExecutionContext_NilDependencies**
- Following our testing principles, we should NOT test with nil dependencies
- All dependencies must be provided (real or mock)

### Dependency Injection Tests

**TestExecutionContext_InboxUsed**
- Create mock inbox
- Create action that sends message
- Execute action with context
- Verify inbox.Send() was called

**TestExecutionContext_WebhookHandlerUsed**
- Create mock webhook handler
- Create WebhookAction
- Execute action with context
- Verify handler.SendWebhook() was called

**TestExecutionContext_LoggerUsed**
- Create test logger
- Create LogAction
- Execute action with context
- Verify logger was used

## 7. Thread Safety Tests

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

## 8. Integration with Orchestrator Tests

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
// MockResourceChecker for testing resource constraints
type MockResourceChecker struct {
    available bool
    err       error
}

func (m *MockResourceChecker) IsAvailable(resourceType, resourceID string) (bool, error) {
    return m.available, m.err
}

// MockInbox for testing message sending
type MockInbox struct {
    messages []interface{}
    mu       sync.Mutex
}

func (m *MockInbox) Send(msg interface{}) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.messages = append(m.messages, msg)
    return nil
}

func (m *MockInbox) GetMessages() []interface{} {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.messages
}

// MockWebhookHandler for testing webhooks
type MockWebhookHandler struct {
    calls []WebhookCall
    err   error
    mu    sync.Mutex
}

type WebhookCall struct {
    URL     string
    Payload interface{}
}

func (m *MockWebhookHandler) SendWebhook(url string, payload interface{}) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = append(m.calls, WebhookCall{URL: url, Payload: payload})
    return m.err
}

func (m *MockWebhookHandler) GetCalls() []WebhookCall {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.calls
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
        Inbox:          &MockInbox{},
        WebhookHandler: &MockWebhookHandler{},
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
