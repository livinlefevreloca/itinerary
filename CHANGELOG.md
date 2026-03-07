# Changelog

## PR Review Feedback Changes

### Removed
- `MinRuntimeConstraint` - removed as unnecessary
- `MaxOpenConns` from `db.Config` - removed configurable max open connections
- `Tx.CreateJob` - removed single-statement transaction wrapper; only multi-statement ops need transactions
- `ConditionPendingState`, `ActionPendingState`, `RetryingState` from orchestrator state machine - unnecessary intermediate states
- Duplicate test mocks from `constraints_test.go` (moved to `testhelpers_test.go`)

### Changed
- `MaxRuntimeConstraint` now includes `PostExecution` evaluation phase (soft time limit)
- Orchestrator state machine simplified: `Pending` goes directly to `ConditionRunning`, retries go through `Terminating` -> `Pending`
- `TerminatingState` can now transition to `PendingState` for retries
- `FailedState` can transition to `PendingState` for retries
- Config update drops in orchestrator now log a warning instead of silently dropping
- Job run channel full in `JobStateSyncer` now logs a warning before returning error
- Index copy comment clarified to explain mutation safety

### Refactored
- Split `db/jobs.go` into `db/jobs.go`, `db/constraints.go`, and `db/actions.go`
- Moved constraint test mocks to `constraints/testhelpers_test.go`
- Moved stats MockDB to `stats/testhelpers_test.go` as `MockStatsDatabaseWriter`
- Removed duplicate `MockInbox` and `MockWebhookHandler` from orchestrator tests (unused stubs)
- Consolidated orchestrator test helpers; `createTestLogger` now uses `testutil.TestLogger`
- Updated scheduler `OrchestratorStatus` enum to match simplified state machine

### Test Organization
Each package with test mocks now follows a consistent pattern:
- `testutil/mocks.go` — shared mocks with no internal package dependencies
- `<package>/testhelpers_test.go` — package-specific mocks that use package-internal types
- `actions/testhelpers.go` — exported test builder (used by action tests)
