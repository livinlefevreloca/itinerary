# Itinerary

## Executive summary
itinerary highly customizable scheduler that orchestrates and monitors jobs running on a kubernetes cluster. It offers:
  * A feature rich UI used to control all aspects of your job running
  * Job constraints and actions that control job behavior and handle failures
    - **Built-in constraints**: maxConcurrentRuns, catchUp, preRunHook, postRunHook, catchUpWindow, maxExpectedRunTime, maxAllowedRunTime, requirePreviousSuccess
    - **Built-in actions**: retry job, kick off another job, trigger webhook, kill running instances, skip next instance
    - **Webhook integrations**: Slack, New Relic, PagerDuty, and custom webhooks
  * Extensive tracking of job statistics run time, state change tracking,retries, failures and failure reasons
  * anomally detection for job statistics

## Project Structure

The project follows canonical Go project layout:

```
itinerary/
├── cmd/
│   └── itinerary/           # Main application entry point (planned)
│       └── main.go          # Server entry point
│
├── internal/                # Private application packages
│   ├── actions/            # Action execution system
│   │   └── spec.md              # Component specification
│   ├── config/             # Configuration management
│   │   └── config.go            # Config loading and validation
│   ├── constraints/        # Constraint evaluation system
│   │   ├── spec.md              # Component specification
│   │   ├── test_spec.md         # Test specification
│   │   ├── types.go             # Core interfaces and types
│   │   ├── checker.go           # ConstraintChecker implementation
│   │   ├── time_window.go       # TimeWindowConstraint
│   │   ├── other_job_*.go       # Job dependency constraints
│   │   ├── http_health_check.go # HTTPHealthCheckConstraint
│   │   ├── max_runtime.go       # MaxRuntimeConstraint
│   │   ├── min_runtime.go       # MinRuntimeConstraint
│   │   └── always_*.go          # Testing constraints
│   ├── cron/               # Cron expression parser
│   │   ├── spec.md              # Component specification
│   │   ├── test_spec.md         # Test specification
│   │   └── *.go                 # Parser and calculator implementation
│   ├── db/                 # Database abstraction and operations
│   │   ├── spec.md              # Component specification
│   │   ├── test_spec.md         # Test specification
│   │   └── *.go                 # DB interface, queries, schema
│   ├── inbox/              # Generic typed inbox for communication
│   │   └── inbox.go             # Inbox implementation
│   ├── orchestrator/       # Job run lifecycle management
│   │   ├── spec.md              # Component specification
│   │   ├── test_spec.md         # Test specification
│   │   └── *.go                 # Orchestrator state machine
│   ├── scheduler/          # Central scheduler component
│   │   ├── spec.md              # Component specification
│   │   ├── test_spec.md         # Test specification
│   │   ├── scheduler.go         # Main scheduler loop
│   │   ├── config.go            # Scheduler configuration
│   │   ├── messages.go          # Inbox message types
│   │   ├── types.go             # Orchestrator state types
│   │   ├── job_state_syncer.go  # Database write buffering
│   │   └── index/               # Scheduled run index (lock-free atomic)
│   │       ├── spec.md              # Component specification
│   │       ├── test_spec.md         # Test specification
│   │       ├── index.go             # Index implementation
│   │       └── scheduled_run.go     # ScheduledRun type
│   ├── stats/              # Stats collector component
│   │   ├── spec.md              # Component specification
│   │   ├── test_spec.md         # Test specification
│   │   └── *.go                 # Stats collector and accumulators
│   └── testutil/           # Shared test utilities and mocks
│       └── *.go                 # Mock implementations
│
├── tools/                  # Standalone tools
│   └── migrator/           # Database migration tool
│       ├── spec.md              # Component specification
│       └── *.go                 # Migration logic (planned)
│
├── spec.md                 # Overall architecture (this file)
├── CLAUDE.md               # Development process guide
├── go.mod                  # Module definition
├── go.sum                  # Dependency checksums
└── test.sh                 # Test runner script
```

All application code lives in `internal/` (not meant for external import). The `cmd/` directory contains executable entry points. Standalone tools live in `tools/`. Components are organized by functionality with specifications and test files co-located alongside implementation.

## Scheduler Components
This section defines the components of the scheduler and how they fit together. Each component is documented in detail in its respective spec file within `internal/`.

### Component Overview
- **Central Scheduler** (`internal/scheduler`): Event loop that coordinates all scheduling activity
- **Scheduled Run Index** (`internal/scheduler/index`): Lock-free atomic index for time-ordered job run scheduling
- **Job State Syncer** (`internal/scheduler`): Manages database writes for job execution data
- **Orchestrator** (`internal/orchestrator`): Manages individual job run lifecycle
- **Constraint Checker** (`internal/constraints`): Evaluates pre-execution, during-execution, and post-execution constraints
- **Actions** (`internal/actions`): Executes actions when constraints are met or violated
- **Stats Collector** (`internal/stats`): Centralizes statistics collection and database persistence
- **Cron Parser** (`internal/cron`): Parses cron expressions and calculates scheduled run times
- **Database Layer** (`internal/db`): Provides database abstraction and operations
- **Inbox** (`internal/inbox`): Generic typed inbox for inter-component communication
- **Config** (`internal/config`): Configuration management

### Central Scheduler
The central scheduler (`internal/scheduler`) is the heart of the application and operates as an event loop. See `internal/scheduler/spec.md` for implementation details.

#### Core Principles
* **Single Source of Truth**: The main loop owns all scheduling state. Any component that needs state must send a request to the inbox and wait for a response.
* **No I/O in Loop**: The loop never performs I/O operations. All external communication is delegated to other goroutines.
* **Lock-Free Design**: Uses atomic pointer swapping for the scheduled run index to avoid lock contention.
* **Request/Response Pattern**: Components communicate with the loop via inbox messages that can include response channels.

#### Components
* **Scheduler**: Main event loop that coordinates all activity
* **Inbox**: Typed buffered channel for inter-component messages with timeout support
* **Job State Syncer**: Buffers and flushes job run updates and stats to database
* **Index Builder**: Background goroutine that periodically rebuilds the scheduled run index

#### Startup
* Load configuration (intervals, windows, etc.)
* Load job definitions from database
* Build initial ScheduledRunIndex
* Initialize state:
  - Active orchestrators map (runID → orchestrator state)
  - Inbox (buffered channel for messages)
  - Job State Syncer (for database writes)
  - Shutdown signal channel
* Start background goroutines:
  - Index builder goroutine
  - Job State Syncer goroutines (flushers and syncers)

#### Configuration Parameters
* **PreScheduleInterval**: Time before a job starts to launch its orchestrator (default: 10 seconds)
* **IndexRebuildInterval**: How often to rebuild the index (default: 1 minute, must be < LookaheadWindow)
* **LookaheadWindow**: How far ahead to calculate runs (default: 10 minutes)
* **GracePeriod**: How far back to include runs in index to catch near-misses (default: 30 seconds)
* **LoopInterval**: How often the main loop runs (default: 1 second)
* **InboxBufferSize**: Size of inbox buffer (default: 10,000)
* **InboxSendTimeout**: Timeout for sending to inbox (default: 5 seconds)
* **OrchestratorHeartbeatInterval**: How often orchestrators send heartbeats (default: 10 seconds)
* **MaxMissedOrchestratorHeartbeats**: Number of missed heartbeats before marking orphaned (default: 3)

#### Main Loop Iteration
Each iteration processes (see `internal/scheduler/spec.md` for implementation details):

1. **Check for shutdown signal**
   - If received, cancel all active orchestrators and exit gracefully

2. **Schedule new orchestrators**
   - Query index for jobs in (now, now + PreScheduleInterval)
   - For each run not in activeOrchestrators map:
     - Generate deterministic runID (format: "jobID:unixTimestamp")
     - Launch orchestrator goroutine
     - Record in activeOrchestrators[runID] with metadata
     - Buffer job run update to syncer
   - No I/O - just goroutine spawning and memory operations

3. **Process all inbox messages**
   - Handle every message in the inbox (non-blocking, drains all available)
   - Message types:
     - OrchestratorHeartbeat: Update heartbeat tracking
     - OrchestratorStateChange: Update orchestrator status
     - OrchestratorComplete/Failed: Mark terminal state
     - CancelRun: Signal orchestrator cancellation
     - UpdateRunConfig: Update job config while in PreRun
     - GetOrchestratorState: Return orchestrator state
     - GetAllActiveRuns: Return all active orchestrators
     - GetStats: Return scheduler statistics
     - Shutdown: Trigger graceful shutdown
   - Messages may include response channels for request/response pattern

4. **Check for missed heartbeats**
   - For each non-terminal orchestrator:
     - Check time since last heartbeat
     - Increment missed heartbeat counter if overdue
     - Mark as orphaned if missed count exceeds threshold

5. **Clean up completed orchestrators**
   - Remove entries from activeOrchestrators where:
     - Orchestrator is in terminal state (Completed, Failed, Cancelled, Orphaned) AND
     - now > scheduledAt + GracePeriod
   - This prevents re-running fast jobs that complete within grace period

6. **Record iteration statistics**
   - Buffer iteration stats (duration, active count, inbox depth, messages processed)
   - Syncer automatically flushes based on size/time thresholds

7. **Update inbox depth stats**
   - Track current inbox depth and maximum seen

#### Background Goroutines
The scheduler runs several background goroutines:

1. **Index Builder** (`runIndexBuilder`):
   - Runs on IndexRebuildInterval ticker (default: 1 minute)
   - Queries database for all job definitions
   - Generates scheduled runs for window: (now - GracePeriod, now + LookaheadWindow)
   - Sorts runs by (ScheduledAt, JobID)
   - Atomically swaps index pointer (lock-free)
   - Can be triggered manually via rebuildIndexChan

2. **Job State Syncer Goroutines** (4 total):
   - **Job Run Flusher**: Periodically flushes buffered job run updates (time-based)
   - **Stats Flusher**: Periodically flushes buffered iteration stats (time-based)
   - **Job Run Syncer**: Reads from job run channel and writes to database
   - **Stats Syncer**: Reads from stats channel and writes to database
   - Dual trigger mechanism: size threshold OR time interval triggers flush

3. **Orchestrator Goroutines** (one per active job run):
   - Manage individual job run lifecycle
   - Send heartbeats on OrchestratorHeartbeatInterval
   - Send state change messages to inbox
   - Can receive cancellation signals and config updates

#### State Management
The main loop maintains:
* **activeOrchestrators**: Map of runID → OrchestratorState
  - Includes: jobID, jobConfig, scheduledAt, actualStart, status, cancelChan, configUpdate channel
  - Also tracks: completedAt, lastHeartbeat, missedHeartbeats
  - Entries removed only after reaching terminal state AND grace period expiration
* **index**: Atomic pointer to ScheduledRunIndex for lock-free time-based queries
* **inbox**: Typed inbox for inter-component messages
* **syncer**: Job State Syncer for buffered database writes

#### Orchestrator Lifecycle
1. Main loop queries index for jobs to schedule
2. Main loop creates OrchestratorState and launches orchestrator goroutine
3. Orchestrator progresses through state machine:
   - PreRun → Pending → ConditionPending → ConditionRunning → ContainerCreating → Running → Terminating → Completed/Failed
4. Orchestrator sends heartbeats and state changes to inbox
5. Main loop updates orchestrator state based on messages
6. On terminal state, entry remains in map for GracePeriod
7. After grace period expires, main loop removes from map

#### Communication Patterns
* **External → Loop**: API/CLI sends messages to inbox (via watcher goroutine)
* **Loop → Orchestrators**: Via cancel channels and config update channels
* **Orchestrators → Loop**: Via inbox messages (heartbeats, state changes, completion)
* **Loop → Job State Syncer**: Buffer job run updates and stats
* **Orchestrators → Job State Syncer**: Via syncer methods (buffer updates)
* **Loop → Stats Collector**: Via stats collector inbox (scheduler stats)
* **Orchestrators → Stats Collector**: Via stats collector inbox (orchestrator stats)
* **Job State Syncer → Stats Collector**: Via stats collector inbox (syncer stats)
* **Loop → External**: Via response channels in inbox messages


### Orchestrator
The orchestrator (`internal/orchestrator`) manages the complete lifecycle of a single job run. See `internal/orchestrator/spec.md` for full implementation details.

#### State Machine
Orchestrators follow a strict state machine with well-defined transitions:
* **Pre-execution states**: PreRun, Pending, ConditionPending, ConditionRunning, ActionPending, ActionRunning
* **Execution states**: ContainerCreating, Running, Terminating
* **Retry state**: Retrying
* **Terminal states**: Completed, Failed, Cancelled, Orphaned

All state transitions are validated through an allowed transitions map to prevent invalid states.

#### Lifecycle Phases
1. **PreRun Phase**
   - Wait for scheduled time to arrive
   - Send periodic heartbeats to scheduler
   - Handle config updates via configUpdate channel
   - Handle cancellation via cancelChan

2. **Pre-Execution Constraint Phase**
   - Transition: Pending → ConditionPending → ConditionRunning
   - Call constraintChecker.CheckPreExecution()
   - Constraint checker internally evaluates all constraints and executes associated actions
   - Returns ConstraintCheckResult with ShouldProceed boolean
   - If constraints not met, may transition to Failed or Retrying

3. **Execution Phase**
   - Transition: ContainerCreating → Running → Terminating
   - Create Kubernetes Job resource with pod spec
   - Monitor pod status via Kubernetes API
   - Send periodic heartbeats
   - Handle cancellation at any point

4. **Post-Execution Phase**
   - Transition: Terminating → Completed/Failed/Retrying
   - Retrieve pod logs and exit code
   - Call constraintChecker.CheckPostExecution()
   - Determine final state based on exit code and constraints

5. **Retry Phase** (if configured)
   - Transition: Failed → Retrying → Pending or ConditionPending
   - Check retry configuration (max retries, backoff)
   - Ask constraint checker: ShouldRecheckOnRetry()
   - If yes: transition to ConditionPending (re-check constraints)
   - If no: transition to Pending (skip constraint re-check)

#### Communication
* **Heartbeats**: Sent on OrchestratorHeartbeatInterval to prove liveness
* **State Changes**: Sent to scheduler inbox to update orchestrator state
* **Completion**: Final message with success/failure and error details
* **Database Updates**: Buffer job run updates via Job State Syncer
* **Statistics**: Send orchestrator metrics to Stats Collector on completion

#### Constraint and Action Integration
* Orchestrators delegate constraint evaluation to ConstraintChecker interface
* ConstraintChecker internally handles action execution for onViolation/onMet triggers
* Orchestrator only receives ShouldProceed boolean - all complexity is encapsulated
* See `internal/constraints/spec.md` for constraint implementation details
* See `internal/actions/spec.md` for action implementation details


### Job State Syncer
The Job State Syncer (`internal/scheduler/job_state_syncer.go`) is responsible for buffering and persisting job execution data and scheduler statistics to the database. It is part of the scheduler package.

#### Architecture
* **Two-stage buffering**: In-memory buffers → channels → database writers
* **Dual trigger flushing**: Size threshold OR time interval triggers flush
* **Four background goroutines**:
  - Job Run Flusher: Time-based flushing of buffered job run updates
  - Stats Flusher: Time-based flushing of buffered iteration stats
  - Job Run Syncer: Reads from job run channel and writes to database
  - Stats Syncer: Reads from stats channel and writes to database

#### Data Types
* **Job Run Updates**: State changes, start/end times, success/failure status
  - UpdateID: UUID for idempotent database writes
  - RunID: Deterministic format "jobID:unixTimestamp"
  - Status, timestamps, success flag, error message
* **Scheduler Iteration Stats**: Per-iteration metrics
  - Timestamp, duration, active orchestrator count
  - Index size, inbox depth, messages processed

#### Flush Configuration
* **Job Run Updates**:
  - Channel size: 200 (default)
  - Flush threshold: 100 updates (size-based trigger)
  - Flush interval: 1 second (time-based trigger)
  - Maximum buffered: 10,000 (safety limit)
* **Iteration Stats**:
  - Channel size: 100 (default)
  - Flush threshold: 30 stats (size-based trigger)
  - Flush interval: 30 seconds (time-based trigger)

#### Benefits
* No I/O in scheduler main loop
* Efficient batched database writes
* Reduced database contention
* Bounded memory usage
* Backpressure handling (scheduler stops if buffer exceeds maximum)

#### Graceful Shutdown
1. Signal shutdown to stop flushers
2. Final flush of remaining buffered items
3. Close write channels
4. Wait for syncers to drain channels and exit
5. All pending data is persisted before shutdown completes

### Stats Collector
The Stats Collector (`internal/stats`) is a standalone component that centralizes all statistics collection and database writing. See `internal/stats/spec.md` for full implementation details.

#### Architecture
* **Inbox-based communication**: Buffered channel for stats messages
* **Stats period tracking**: Time-based windows for aggregation
* **Accumulators**: Per-component stats accumulators that perform intermediate calculations
* **Main loop**: Blocks on inbox, routes messages to appropriate accumulator

#### Stats Sources
* **Scheduler**: Iteration metrics, active orchestrator count, inbox depth
* **Orchestrators**: Runtime, constraints checked, actions taken (per run)
* **Job State Syncer**: Write metrics, buffer sizes, queue depths
* **Stats Collector itself**: Self-monitoring (messages processed, flush count, etc.)

#### Configuration
* **InboxBufferSize**: 1000 (default)
* **InboxSendTimeout**: 5 seconds (default)
* **FlushInterval**: 30 seconds (default, time-based trigger)
* **FlushThreshold**: 100 messages (default, size-based trigger)
* **StatsPeriodDuration**: 30 seconds (default, window size)

#### Accumulators
Each accumulator:
* Receives stats data via main loop routing
* Performs aggregations (sums, min/max/avg calculations)
* Tracks samples for statistical calculations
* Flushes to database on period completion or threshold
* Resets after successful flush

#### Database Writes
* **scheduler_stats**: Per-period scheduler metrics
* **orchestrator_stats**: Per-run orchestrator metrics
* **syncer_stats**: Per-period syncer metrics (formerly writer_stats)
* **stats_collector_stats**: Per-period stats collector self-monitoring

#### Benefits
* Centralizes all stats logic in one component
* No stats-related I/O in scheduler loop or orchestrators
* Consistent aggregation and calculations across all components
* Easy to add new metrics without modifying multiple components
* Self-monitoring to track stats collector health

### Scheduled Run Index
The Scheduled Run Index (`internal/scheduler/index`) provides efficient time-based queries for scheduled job runs. See `internal/scheduler/index/spec.md` for full implementation details.

#### Design
* **Lock-free atomic swapping**: Uses atomic.Pointer for concurrent access without locks
* **Sorted slice**: Runs sorted by (ScheduledAt, JobID) for binary search
* **Bulk rebuild strategy**: Periodically rebuild entire index rather than incremental updates
* **O(log n + k) queries**: Binary search to find start position + linear scan for results

#### Operations
* **Build**: Create index from sorted runs (sorts unsorted input)
* **Query**: Find all runs in time window [start, end)
* **Len**: Get count of scheduled runs
* **Swap**: Atomically replace index with new sorted runs

#### Performance Characteristics
* Build 1M runs: < 150ms (including sort)
* Query: < 1ms even with 1M runs in index
* Memory: ~40 bytes per run (~40MB for 1M runs)
* No lock contention on read path

#### Integration with Scheduler
* Index builder goroutine queries database for job definitions
* Generates runs for window: (now - GracePeriod, now + LookaheadWindow)
* Sorts runs by time and job ID
* Atomically swaps index pointer
* Main loop queries index for runs to schedule

### Cron Parser
The Cron Parser (`internal/cron`) parses standard 5-field cron expressions and calculates scheduled run times. See `internal/cron/spec.md` for full implementation details.

#### Format
* Standard 5-field cron: minute hour day-of-month month day-of-week
* Supported syntax: * (any), , (list), - (range), / (step)
* Examples: `0 0 * * *` (daily), `*/15 * * * *` (every 15 minutes)

#### API
* **Parse(expr string)**: Parse cron expression into CronSchedule
* **Next(after time.Time, count int)**: Calculate next N occurrences
* **Between(start, end time.Time)**: Calculate all occurrences in window

#### Performance
* Parse 10,000 schedules: ~5ms
* Calculate 1 hour of runs for 10,000 schedules: ~5ms
* Efficient enough to rebuild entire index every 30-60 seconds

### Database Layer
The Database Layer (`internal/db`) provides a shared abstraction for all database operations. See `internal/db/spec.md` for full implementation details.

#### Support
* **Primary**: PostgreSQL (production)
* **Development/Testing**: SQLite (in-memory and file-based)
* **Optional**: MySQL (alternative production option)

#### Core Features
* Connection management with pooling
* Transaction support (Begin/Commit/Rollback)
* Type-safe query interface for all tables
* Database-agnostic SQL dialect handling
* In-memory SQLite for tests

#### Schema Tables
See database schema section below for detailed table definitions.

### Constraint Checker
The Constraint Checker (`internal/constraints`) provides a pluggable system for evaluating constraints on job runs. See `internal/constraints/spec.md` for full implementation details.

#### Design Principles
* **Opaque to Orchestrator**: Orchestrator receives simple ShouldProceed boolean
* **Interface-Based**: Constraints and actions implement interfaces
* **Type-Safe**: Each constraint type is its own struct
* **Multi-Phase Evaluation**: Pre-execution, during-execution, and post-execution
* **Per-Constraint Configuration**: Each constraint specifies when to evaluate and what actions to take

#### Constraint Types Implemented
1. **TimeWindowConstraint**: Check if within time window
2. **OtherJobRunningConstraint**: Check if another job is running
3. **OtherJobCompletedRecentlyConstraint**: Check if job completed recently
4. **OtherJobScheduledSoonConstraint**: Check if job scheduled soon
5. **HTTPHealthCheckConstraint**: Make HTTP request and check response
6. **MaxRuntimeConstraint**: Check runtime limit (during execution)
7. **MinRuntimeConstraint**: Check minimum runtime (post execution)
8. **AlwaysPassConstraint**: Always succeeds (testing)
9. **AlwaysFailConstraint**: Always fails (testing)

#### Integration
* Orchestrator calls CheckPreExecution(), CheckDuringExecution(), CheckPostExecution()
* Constraint checker evaluates all applicable constraints for that phase
* For each constraint result (met or violated), executes associated action list
* Returns ConstraintCheckResult with ShouldProceed boolean

### Action System
The Action System (`internal/actions`) provides action execution in response to constraint evaluation. See `internal/actions/spec.md` for full implementation details.

#### Design Principles
* **Interface-Based**: All actions implement Action interface
* **Isolated Execution**: Each action runs independently
* **Type-Safe**: Each action type is its own struct
* **Context-Based Communication**: Actions receive execution context with dependencies

#### Action Types
1. **DelayAction**: Pause execution for duration
2. **WebhookAction**: Send HTTP webhook
3. **LogAction**: Log a message
4. **FailAction**: Force job run to fail
5. **NoOpAction**: Do nothing (testing)

#### Integration
* Actions are triggered by constraints (onViolation or onMet)
* Constraint checker executes actions internally
* Actions receive ExecutionContext with job info, inbox, webhook handler, logger

### Supporting Components

#### Inbox
Generic typed inbox implementation (`internal/inbox`) for inter-component communication with timeout support.

#### Config
Configuration management (`internal/config`) for application settings.

## Job Constraints and Actions

The Itinerary scheduler provides a pluggable constraint and action system. Constraints are evaluated at different phases of the job lifecycle (pre-execution, during-execution, post-execution). When constraints are met or violated, associated actions are executed.

### Constraint System Architecture
* **Interface-based**: All constraints implement the Constraint interface
* **Multi-phase evaluation**: Each constraint specifies which phases it applies to
* **Configuration-driven**: Constraints are stored in database and loaded at runtime
* **Action triggers**: Each constraint can specify onViolation and onMet action lists

### Implemented Constraint Types

See `internal/constraints/spec.md` for full implementation details.

**Pre-execution constraints:**
* `time_window` - Job must run within specified time window (e.g., business hours only)
* `other_job_running` - Check if another job is currently running (or not running)
* `other_job_completed_recently` - Requires another job completed within time window
* `other_job_scheduled_soon` - Check if another job is scheduled soon
* `http_health_check` - Make HTTP request to health check endpoint (supports templating)

**During-execution constraints:**
* `max_runtime` - Maximum allowed runtime before action is taken

**Post-execution constraints:**
* `min_runtime` - Minimum expected runtime (detect jobs that exit too quickly)

**Testing constraints:**
* `always_pass` - Always succeeds (for testing action execution)
* `always_fail` - Always fails (for testing violation actions)

### Action System Architecture
* **Interface-based**: All actions implement the Action interface
* **Isolated execution**: Each action runs independently
* **Trigger-driven**: Actions specify when they trigger (on_met or on_violated)
* **Context-provided**: Actions receive dependencies via ExecutionContext

### Implemented Action Types

See `internal/actions/spec.md` for full implementation details.

**Available actions:**
* `delay` - Pause execution for specified duration
* `webhook` - Send HTTP webhook with configurable payload
* `log` - Log a message
* `fail` - Force job run to fail with reason
* `noop` - Do nothing (for testing)

### Future Constraint Types
Planned but not yet implemented:
* `maxConcurrentRuns` - Limit concurrent runs of this job
* `requirePreviousSuccess` - Require specific job completed successfully
* `catchUp` / `catchUpWindow` - Handle missed scheduled runs
* `preRunHook` / `postRunHook` - Webhook-based pre/post checks

### Future Action Types
Planned but not yet implemented:
* `retry` - Retry the current job run
* `kickOffJob` - Start another job
* `killAllInstances` - Kill all running instances
* `killLatestInstance` - Kill most recent instance
* `skipNextInstance` - Skip next scheduled run
* `sendEmail` - Email notification
* `slack` - Specialized Slack integration
* `pagerduty` - PagerDuty incident creation

## Application Startup Sequence

The application follows this startup sequence:

1. **Parse Configuration**
   - Command-line flags (database connection, migrations path)
   - Environment variables
   - Configuration file (if present)

2. **Open Database Connection**
   - Connect using configured driver (PostgreSQL, MySQL, or SQLite)
   - Verify connection with ping
   - Apply connection pool settings

3. **Run Database Migrations**
   - Automatically run on every startup (unless `--skip-migrations` flag is set)
   - Uses migrator tool (`tools/migrator`)
   - Acquires advisory lock to prevent concurrent migrations
   - Applies all pending migrations in order (with dependency validation)
   - Logs current schema version
   - Fails fast if migrations fail (application won't start with outdated schema)

4. **Initialize Components**
   - Create Stats Collector and start goroutine
   - Create Job State Syncer and start 4 background goroutines (flushers and syncers)
   - Initialize constraint checker
   - Initialize action executor

5. **Initialize Scheduler**
   - Load job definitions from database
   - Build initial ScheduledRunIndex (synchronously on startup)
   - Create inbox with configured buffer size
   - Initialize active orchestrators map
   - Start scheduler main loop
   - Start index builder background goroutine

6. **Start HTTP API Server** (future)
   - Expose REST API for job management
   - Health check endpoints
   - Metrics endpoints

7. **Graceful Shutdown**
   - Listen for SIGINT/SIGTERM
   - Send shutdown message to scheduler inbox
   - Scheduler main loop:
     - Cancels all active orchestrators
     - Stops accepting new jobs
     - Exits main loop
   - Shutdown Job State Syncer:
     - Stop flushers
     - Final flush of buffered data
     - Close channels
     - Wait for syncers to drain
   - Shutdown Stats Collector:
     - Stop accepting new stats
     - Flush pending stats
     - Close database connection
   - Close database connections
   - Exit cleanly

### Command-Line Flags (Planned)

```bash
itinerary \
  --db-driver=postgres \
  --db-dsn="postgres://user:pass@localhost/itinerary?sslmode=disable" \
  --migrations-dir=./migrations \
  --skip-migrations=false \
  --config-file=./config.yaml
```

See `internal/config` for configuration management implementation.

## Database Layer
### Database Abstraction
The database layer (`internal/db`) provides abstraction over relational databases using only the Go standard library `database/sql` package. See `internal/db/spec.md` for full implementation details.

### Supported Databases
* **PostgreSQL**: Primary production database
* **SQLite**: Development and testing (in-memory and file-based)
* **MySQL**: Optional production alternative

### Connection Management
* Connection pooling with configurable limits
* Transaction support (Begin/Commit/Rollback)
* Type-safe query interface
* Database-specific SQL dialect handling

### Migrations
Migrations are managed by the migrator tool (`tools/migrator`). See `tools/migrator/spec.md` for full implementation details.

* SQL-based migrations in `<version>_<description>.sql` format
* Automatic execution on application startup (unless `--skip-migrations` flag)
* Advisory locks prevent concurrent migrations
* Version tracking in `schema_migrations` table
* Support for non-transactional migrations
* Migration dependency declarations for branch management
### Core Schema Tables

See `internal/db/spec.md` for complete schema definitions and SQL.

#### Dimension Tables (Reference Data)
* **constraint_types**: Define types of constraints (manually assigned IDs, never deleted)
* **action_types**: Define types of actions (manually assigned IDs, never deleted)

#### Job Configuration Tables
* **jobs**: Job definitions with name, schedule (cron), pod_spec (JSON)
* **constraints**: Constraint instances attached to jobs (references constraint_types)
* **actions**: Action instances attached to constraints (references action_types)

#### Job Execution Tables
* **job_runs**: Individual job executions
  - Primary key: (job_id, scheduled_at)
  - Unique index: run_id (format: "jobID:unixTimestamp")
  - Fields: job_id, run_id, scheduled_at, started_at, completed_at, status, success, error, trigger
  - Trigger values: 'scheduled', 'manual', 'retry', 'action'
* **constraint_runs**: Records of constraint evaluations
  - Links to job_runs and constraints
  - Fields: success, violated, in_error, error, details (JSON)
* **action_runs**: Records of action executions (future)
  - Links to job_runs, actions, and optional constraint_runs
  - Fields: trigger, executed_at, success, error, details (JSON)
#### Statistics Tables

See `internal/stats/spec.md` and `internal/db/spec.md` for complete definitions.

* **scheduler_stats**: Per-period scheduler metrics
  - stats_period_id (pk), start_time, end_time
  - iterations, run_jobs, late_jobs, missed_jobs, jobs_cancelled
  - Inbox metrics: min/max/avg inbox length, empty inbox time, time in inbox

* **orchestrator_stats**: Per-run orchestrator metrics
  - run_id (pk), stats_period_id (fk)
  - runtime, constraints_checked, actions_taken

* **syncer_stats**: Per-period Job State Syncer metrics
  - stats_period_id (pk), start_time, end_time
  - Write metrics: total_writes, writes_succeeded, writes_failed
  - Queue metrics: min/max/avg writes in flight, queued writes
  - Inbox metrics: min/max/avg inbox length, time in inbox, time in write queue

* **stats_collector_stats**: Per-period Stats Collector self-monitoring
  - stats_period_id (pk), start_time, end_time
  - Message metrics: messages_received, messages_processed by source
  - Flush metrics: periods_completed, database_flushes, flush_errors
  - Inbox metrics: min/max/avg inbox length
  - Processing time: min/max/avg processing time (microseconds)

#### Future Tables
* **webhook_deliveries**: Track individual webhook delivery attempts
* **webhook_handler_stats**: Per-period webhook handler metrics

### Database Indexes

See `internal/db/spec.md` for complete indexing strategy. The project takes a conservative approach: indexes are only added when proven necessary through query patterns and performance profiling.

#### Current Indexes

**Unique Indexes:**
* `idx_job_runs_run_id` - Unique index on job_runs.run_id for GetJobRunByRunID lookups

**Foreign Key Indexes:**
All foreign keys have indexes to optimize JOIN operations and constraint checks:
* `idx_constraints_job_id` - constraints.job_id → jobs
* `idx_actions_constraint_id` - actions.constraint_id → constraints
* `idx_job_runs_job_id` - job_runs.job_id → jobs
* `idx_constraint_runs_run_id` - constraint_runs.run_id → job_runs
* `idx_constraint_runs_constraint_id` - constraint_runs.constraint_id → constraints
* `idx_orchestrator_stats_stats_period_id` - orchestrator_stats.stats_period_id → scheduler_stats

**Note:** Primary keys automatically create indexes. Composite primary keys (e.g., job_runs) index all PK columns together.

#### Future Indexes

Additional indexes will be added based on:
* UI query patterns (filtering, searching, sorting)
* Performance profiling results
* Proven high-selectivity predicates

### Dimension Table Management

**Critical rules for dimension tables**:
1. **Never delete rows** - historical data may reference them
2. **Never change IDs** - breaks referential integrity
3. **Never renumber** - IDs are immutable once assigned
4. **Use explicit IDs** - assign IDs manually in migrations to avoid conflicts
5. **Use upsert syntax** - INSERT OR IGNORE (SQLite), ON CONFLICT DO NOTHING (PostgreSQL), INSERT IGNORE (MySQL)

**Adding new types**:
```sql
-- In a new migration file (e.g., 003_add_email_action.sql)
-- +migrate Up
INSERT OR IGNORE INTO action_types (id, name) VALUES (7, 'sendEmail');
INSERT OR IGNORE INTO constraint_types (id, name) VALUES (9, 'minHealthyInstances');
```

**Deprecating types** (never delete):
```sql
-- Add deprecated flag in migration
ALTER TABLE constraint_types ADD COLUMN deprecated BOOLEAN DEFAULT FALSE;
UPDATE constraint_types SET deprecated = TRUE WHERE id = 5;
```


## UI components
* The UI is a web based UI. The UI should be in typescript with react using vite
### Currently running jobs screen
* A screen displaying running jobs.
* Each job has a card that displays stats about it (job name, start time, etc)
* The job card links to each individual job run page

### Full Job run history
* Job run history displayed as a [gantt chart](https://en.wikipedia.org/wiki/Gantt_chart)
* There should be a several filters on this page
  - time range in which to display jobs (starttime, endtime) this should be able to be set relativley (since 5 mins ago) or precisley (2026-01-01T00:00:00 to 2026-01-01T00:05:00)
  - job run length (min and max)
  - job name
  - job tags
* Jobs on gantt chart should be color coded by success, failure and still running


### Individual job run history
* Job Run hisotry displaying simimlar to the the cronitor page which can be seen in these screenshots
  - ![top of page](/Users/adam/Projects/personal/golang/itinerary/spec/top.png)
  - ![bottom of page](/Users/adam/Projects/personal/golang/itinerary/spec/bottom.png)

### Job run page
* A page displaying information specific to a job run
* start time, current run time, args, constraint violations (if any), and actions taken
* links to Job Page

### Manually run a job page
* Run a job (if allowed) with the arguements passed overwritten

### Job page
* Information on a job (not a specific run)
* All job configuration

### Job definitions page
* A page listing all define jobs.
* paginated
* allow search based on
  - name
  - tags
  - schedule

### New Job page
* A page for defining a new job
* A job definition takes
  - name
  - namespace
  - schedule
  - image(s)
  - command(s)
  - serviceaccount(s)
  - env
  - envSecret
  - podSpec
  - tags
  - constraints (maxConcurrentRuns, catchUp, hooks, timeouts, dependencies)
  - resources

### Edit job page
  * Edit all of the above attributes in the newJob page



## Implementation Status

### Fully Implemented Components
* **Cron Parser** (`internal/cron`): Complete with full test suite
* **Scheduled Run Index** (`internal/scheduler/index`): Lock-free atomic index with benchmarks
* **Constraint Checker** (`internal/constraints`): All constraint types implemented and tested
* **Database Layer** (`internal/db`): Connection management, schema definitions, query interface
* **Stats Collector** (`internal/stats`): Complete with accumulators and database persistence
* **Inbox** (`internal/inbox`): Generic typed inbox with timeout support
* **Job State Syncer** (`internal/scheduler/job_state_syncer.go`): Buffering and flushing logic complete
* **Scheduler Core** (`internal/scheduler`): Main loop, message handling, heartbeat monitoring
* **Test Utilities** (`internal/testutil`): Mock implementations for testing

### Partially Implemented
* **Orchestrator** (`internal/orchestrator`): State machine designed, partial implementation
* **Actions** (`internal/actions`): Basic actions implemented, more types needed
* **Migrator** (`tools/migrator`): Specification complete, implementation pending

### Planned Components
* **Main Application** (`cmd/itinerary`): Entry point and startup logic
* **HTTP API Server**: REST API for job management and monitoring
* **Web UI**: React-based user interface for job management
* **Webhook Handler**: Standalone component for webhook delivery
* **Kubernetes Integration**: Job creation and pod monitoring

## Future: UI Web API
* The UI talks to a backend web API which defines the following endpoints
  - GET /runs/<run_id> - Get information on a specific run of a job
  - GET /runs/<job_id> - Get information on runs of a given job given an id
    params:
      - start_time
      - end_time
  - GET /runs - Get a list of runs satisfying the criteria
      params:
        - job_id (optional)
        - name (optional)
        - tags (optional)
        - status (optional)
        - started_after_time (optional)
        - ended_before_time (optional)
        - running_after_time (optional)
        - running_before_time (optional)
  - POST /runs/<job_id> - kick off a run of a given job with the provided args
      params:
        - args (optional)
        - resources (optional)
        - start-time (optional)
  - DELETE /runs/<run_id> - Cancel the run of a job matching the ID
  - GET /runs/<run_id>/violations - Get constraint violations for a specific run
  - GET /jobs/<job_id> - get information on a specific job
  - GET /jobs - get information on all jobs
    params:
      - name_pattern (optional)
      - tags (optional)
      - tag_pattern (optional)
      - schedule (optional)
  - POST /jobs - create a new job with the given parameters
    params:
      - name
      - schedule
      - podSpec (optional)
      * namespace (optional)
      * images (optional)
      * commands
      * serviceaccounts (optional)
      * env (optional)
      * envSecret (optional)
      - tags (optional)
      - constraints (optional)
      - is manually runnable
      - is modifiable for manual run
  - PUT /jobs/<job-id> - update an existing job
    params:
      - name
      - schedule
      - podSpec (optional)
      * namespace (optional)
      * images (optional)
      * commands
      * serviceaccounts (optional)
      * env (optional)
      * envSecret (optional)
      - tags (optional)
      - constraints (optional)
      - is manually runnable
      - manual run modifiable fields
