# PR Review - Open Discussion Items

Items from the PR review that require broader architectural decisions before implementing.

## 1. Move Action interface to actions/ package

**Comment:** The `Action` interface in `constraints/types.go` should live in `actions/`.

**Problem:** The `constraints` and `actions` packages each define their own `ExecutionContext` with different fields. The `Action` interface in constraints operates on `constraints.ExecutionContext`, while the one in actions operates on `actions.ExecutionContext`. Moving the interface to a single location would require unifying `ExecutionContext` or introducing a shared interface package.

**Options:**
- Create a shared `types` package that both `constraints` and `actions` import for `Action` and `ExecutionContext`
- Merge `constraints` and `actions` into a single package
- Define a minimal shared `Action` interface in a common package and adapt in each consumer

## 2. Remove duplicate Job struct definitions

**Comment:** Don't add conflicting definitions of structs. We don't need simplified versions of things.

**Problem:** `Job` is defined in four places with different fields:
- `constraints.Job`: ID, Name, Args, Kwargs
- `actions.Job`: ID, Name
- `orchestrator.Job`: ID, Name, Schedule, PodSpec, ConstraintConfig, ActionConfig, RetryConfig
- `scheduler.Job`: ID, Name, Schedule, PodSpec

Each package uses the fields relevant to its domain. A single canonical `Job` type would need to live in a shared package to avoid import cycles.

**Options:**
- Create a `model` or `types` package with a single `Job` struct containing all fields
- Use the `db.Job` as the canonical type and have other packages reference it
- Keep domain-specific views but derive them from a single source type

## 3. Evaluate ShouldRecheckOnRetry

**Comment:** Is this method useful? Don't we need to check each constraint individually always?

**Current usage:** The orchestrator's `ConstraintChecker` interface includes `ShouldRecheckOnRetry(job)` to decide whether retries should re-enter the constraint checking flow or skip directly to execution. Each individual constraint also has `ShouldRecheckOnRetry() bool` on the `Constraint` interface.

**Question:** With the `RetryingState` removed, retry flow now goes `Terminating`/`Failed` -> `Pending`. The `Pending` state already decides whether to check constraints based on whether constraints exist. If we always re-check constraints on retry, we can remove `ShouldRecheckOnRetry` from both interfaces entirely.

**Decision needed:** Should retries always re-check all constraints, or is the per-constraint opt-in still valuable?

## 4. Default evaluation phases to all-inclusive

**Comment:** In general all evaluation phases should be allowed unless excluded.

**Current behavior:** Each constraint explicitly lists which phases it applies to via `EvaluationTiming() []EvaluationPhase`. The checker skips constraints that don't list the current phase.

**Proposed change:** Invert the logic so constraints run in all phases by default and optionally exclude specific phases. This would require:
- Changing the `Constraint` interface (e.g., `ExcludedPhases()` instead of `EvaluationTiming()`)
- Updating all constraint implementations
- Updating the checker's `appliesToPhase` logic
- Updating all tests

**Trade-off:** More inclusive by default means constraints must explicitly opt out, which is safer but may cause unexpected behavior if a constraint isn't designed for a given phase (e.g., a pre-execution constraint running post-execution).

## 5. Reusable send-and-wait pattern on scheduler inbox

**Comment:** We do the send-message-wait-for-response pattern in many places. We want a method on the scheduler inbox that sends a message, waits for the response, and passes it to a callback. Also an async fire-and-forget version.

**Current pattern** (repeated in `other_job_running.go`, `other_job_completed_recently.go`, `other_job_scheduled_soon.go`):
```go
responseChan := make(chan interface{}, 1)
request := &JobStateRequest{JobID: id, ResponseTo: responseChan}
ctx.SchedulerInbox.Send(request)
select {
case resp := <-responseChan:
    // handle response
case <-ctx.Context.Done():
    return ctx.Context.Err()
}
```

**Proposed API:**
```go
// Synchronous: send and wait for response with context cancellation
func (inbox *Inbox) SendAndWait(ctx context.Context, msg interface{}) (interface{}, error)

// Async: send and forget
func (inbox *Inbox) SendAsync(msg interface{}) error
```

**Considerations:**
- The `MessageSender` interface in constraints would need to be extended
- Response routing needs a convention (currently uses `ResponseTo` channel embedded in the request)
- The callback variant vs return-value variant is a design choice
