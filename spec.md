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
│   └── itinerary/           # Main application entry point
│       └── main.go          # Server/CLI entry point
│
├── internal/                # Private application packages
│   ├── cron/               # Cron expression parser
│   │   ├── spec.md              # Component specification
│   │   └── test_spec.md         # Test specification
│   ├── db/                 # Database abstraction and operations
│   │   ├── spec.md              # Component specification
│   │   └── test_spec.md         # Test specification
│   ├── inbox/              # Generic inbox for component communication
│   ├── scheduler/          # Central scheduler component
│   │   ├── spec.md              # Component specification
│   │   ├── test_spec.md         # Test specification
│   │   └── index/          # Scheduled run index (lock-free atomic)
│   │       ├── spec.md              # Component specification
│   │       └── test_spec.md         # Test specification
│   ├── stats/              # Stats collector component
│   │   ├── spec.md              # Component specification
│   │   └── test_spec.md         # Test specification
│   ├── syncer/             # Database syncer component
│   │   └── spec.md              # Component specification
│   └── testutil/           # Shared test utilities and mocks
│
├── tools/                  # Standalone tools
│   └── migrator/           # Database migration tool
│       ├── spec.md              # Component specification
│       └── test_spec.md         # Test specification
│
├── spec/                   # Specifications and UI mockups
│   ├── components/         # Component specifications
│   │   ├── syncer.md               # Syncer specification
│   │   ├── stats-collector.md      # Stats Collector specification
│   │   └── webhook-handler.md      # Webhook Handler specification
│   ├── top.png             # UI mockups
│   └── bottom.png
│
├── spec.md                 # Overall architecture (this file)
├── go.mod                  # Module definition
└── test.sh                 # Test runner script
```

All application code lives in `internal/` (not meant for external import). The `cmd/` directory contains executable entry points. Standalone tools live in `tools/`. Components are organized by functionality with specifications and test files co-located alongside implementation.

## Scheduler Components
This section defines the components of the scheduler and how they fit together. Each component is documented in detail in the `spec/components/` directory.

### Component Overview
- **Central Scheduler**: Event loop that coordinates all scheduling activity
- **Orchestrator**: Manages individual job run lifecycle
- **Syncer**: Manages database writes for job execution data (job runs, constraint violations, action runs)
- **Stats Collector**: Centralizes statistics collection and database persistence
- **Webhook Handler**: Handles all webhook deliveries asynchronously

### Central scheduler
The central scheduler is the heart of the application and operates as an event loop.

#### Core Principles
* **Single Source of Truth**: The main loop owns all scheduling state. Any component that needs state must send a request to the inbox and wait for a response.
* **No I/O in Loop**: The loop never performs I/O operations. All external communication is delegated to other goroutines.
* **Request/Response Pattern**: Components communicate with the loop via inbox messages that can include response channels.

#### Startup
* Load configuration (intervals, windows, etc.)
* Load job definitions from database (read-only connection)
* Build initial ScheduledRunIndex
* Initialize state:
  - Active orchestrators map (runID → orchestrator state)
  - Inbox (buffered channel for events)
  - Shutdown signal channel

#### Configuration Parameters
* **PRE_SCHEDULE_INTERVAL**: Time before a job starts to launch its orchestrator (default: 10 seconds, configurable)
* **INDEX_REBUILD_INTERVAL**: How often to rebuild the index (default: 1 minute, configurable, must be < LOOKAHEAD_WINDOW)
* **LOOKAHEAD_WINDOW**: How far ahead to calculate runs (default: 10 minutes, configurable)
* **GRACE_PERIOD**: How far back to include runs in index to catch near-misses (default: 30 seconds, configurable)
* **LOOP_INTERVAL**: How often the main loop runs (default: 1 second)

#### Main Loop Iteration
Each iteration processes:

1. **Check for shutdown signal**
   - If received, cancel all active orchestrators and exit gracefully

2. **Rebuild index if needed**
   - If (now - lastRebuild) > INDEX_REBUILD_INTERVAL
   - Generate runs for window: (now - GRACE_PERIOD, now + LOOKAHEAD_WINDOW)
   - Atomically swap index
   - No I/O - uses in-memory job list

3. **Schedule new orchestrators**
   - Query index for jobs in (now, now + PRE_SCHEDULE_INTERVAL)
   - For each run not in activeOrchestrators map:
     - Generate unique runID
     - Launch orchestrator goroutine
     - Record in activeOrchestrators[runID] with metadata
   - No I/O - just goroutine spawning

4. **Process all inbox messages**
   - Handle every message in the inbox (not priority-based)
   - Message types:
     - State queries (respond with current state)
     - Cancellation requests (signal orchestrator)
     - Orchestrator completion notifications
     - Schedule update notifications (trigger rebuild)
     - Stats requests
   - Messages may include response channels for request/response pattern

5. **Clean up completed orchestrators**
   - Remove entries from activeOrchestrators where:
     - Orchestrator has completed AND
     - now > startTime + GRACE_PERIOD
   - This prevents re-running fast jobs that complete within grace period

6. **Record state changes**
   - Collect all state changes from this iteration
   - Send to syncer goroutine for database persistence
   - No I/O in main loop - syncer handles writes

7. **Update statistics**
   - Track loop iteration metrics
   - Send to Stats Collector via its inbox

#### State Management
The main loop maintains:
* **activeOrchestrators**: Map of runID → orchestrator metadata
  - Includes: jobID, scheduledTime, actualStartTime, status, cancel channel
  - Entries removed only after completion AND grace period expiration
* **jobDefinitions**: In-memory copy of all job schedules
* **index**: ScheduledRunIndex for efficient time-based queries
* **stats**: Current iteration statistics

#### Orchestrator Lifecycle
1. Main loop creates orchestrator goroutine
2. Orchestrator runs (pre-exec, exec, post-exec phases)
3. Orchestrator sends completion message to inbox
4. Main loop marks as completed in activeOrchestrators
5. After GRACE_PERIOD expires, main loop removes from map

#### Communication Patterns
* **External → Loop**: Watcher goroutine receives gRPC requests, sends to inbox
* **Loop → Orchestrators**: Via cancel channels
* **Orchestrators → Loop**: Via inbox messages (completion, errors)
* **Loop → Syncer**: Via syncer inbox channel (job run updates)
* **Orchestrators → Syncer**: Via syncer inbox channel (job run updates, constraint violations, action runs)
* **Loop → Stats Collector**: Via stats collector inbox (scheduler stats)
* **Orchestrators → Stats Collector**: Via stats collector inbox (orchestrator stats)
* **Orchestrators → Webhook Handler**: Via webhook handler inbox (webhook requests)
* **Syncer → Stats Collector**: Via stats collector inbox (syncer stats)
* **Webhook Handler → Stats Collector**: Via stats collector inbox (webhook stats)
* **Loop → External**: Via response channels in inbox messages


### Orchestrator
* An orchestrator is a go routine that is responsible for a given run of a given job.
* It is passed the required information about the job and starts its pre execution loop. This involves
  - checking for cancel notifications from the main loop
  - checking if the starttime has passed
  - checking for a global shutdown signal
* Once starttime has passed it evaluates pre-execution constraints (maxConcurrentRuns, preRunHook, requirePreviousSuccess)
* If constraints are satisfied it starts the job execution
* After that it starts its execution loop within the execution loop it:
  - waits on kubernetes events from the job it started with a timeout
  - checks execution-time constraints (maxExpectedRunTime, maxAllowedRunTime)
  - takes built-in actions if constraints are violated (retry, webhook, kill)
* Throughout execution, orchestrator records events to Syncer:
  - Job run state changes (started, running, completed)
  - Constraint violations (when constraints fail)
  - Action runs (when actions are executed with success/failure)
* Once the job is done if it has failed we notify the main loop, record the failure and exit
* Once the job concludes if successful we evaluate post-execution constraints (postRunHook)
* Actions are taken by:
  - Sending messages to the main loop (kill instances, skip next run)
  - Sending webhook requests to Webhook Handler (asynchronous, non-blocking)
  - Direct actions by orchestrator (retry, start another job)
* On completion, orchestrator sends final job run update to Syncer and stats to Stats Collector


### Syncer
* The syncer is the component responsible for writing job execution data to the database
* See detailed specification: `spec/components/syncer.md`
* Unlike the Stats Collector (which handles statistics), the Syncer handles transactional job execution data
* It operates with an inbox-based architecture, blocking on incoming messages
* The syncer handles three types of database writes:
  - **Job run updates**: State changes, start/end times, success/failure status
  - **Constraint violations**: Records when constraints are violated (separate from action_taken)
  - **Action runs**: Records when actions are executed (retry, webhook, kill, skip) with success/failure
* Architecture:
  - Inbox-based communication (buffered channel)
  - Blocks on inbox waiting for messages
  - Independent batches for each message type
  - Flushes based on size threshold OR time interval (dual trigger per type)
  - Backpressure handling (blocks senders when overloaded)
* The syncer uses buffering to reduce database contention:
  - Batches updates by type (job runs, violations, actions)
  - Uses transactions for atomic multi-row updates
  - Different flush thresholds for different message types
* Benefits:
  - No I/O in scheduler loop or orchestrators
  - Efficient batched database writes
  - Guaranteed delivery (no data loss)
  - Clear backpressure signals
* On shutdown, the syncer flushes all pending updates before exiting

### Stats Collector
* The Stats Collector is a standalone component that centralizes all statistics collection and database writing
* See high-level specification: `spec/components/stats-collector.md`
* See implementation specification: `internal/stats/spec.md`
* See test specification: `internal/stats/test_spec.md`
* Key responsibilities:
  - Receive statistics from all components (scheduler, orchestrators, syncer, webhook handler)
  - Perform intermediate calculations and aggregations
  - Write statistics to database in batches
  - Track statistics periods (time-based windows)
* Architecture:
  - Inbox-based communication (buffered channel)
  - Blocks on inbox waiting for stats messages
  - Flushes to database on threshold or timer
  - Graceful shutdown with pending stats flush
* Benefits:
  - Centralizes all stats logic
  - No stats-related I/O in other components
  - Consistent aggregation and calculations
  - Easy to add new metrics

### Webhook Handler
* The Webhook Handler is a standalone component that handles all webhook deliveries
* See detailed specification: `spec/components/webhook-handler.md`
* Key responsibilities:
  - Receive webhook requests from orchestrators and other components
  - Send HTTP requests to configured webhook endpoints
  - Implement retry logic with exponential backoff
  - Track webhook delivery statistics
  - Support multiple webhook types (Slack, New Relic, PagerDuty, custom)
* Architecture:
  - Inbox-based communication (buffered channel)
  - Blocks on inbox waiting for webhook requests
  - Asynchronous delivery (doesn't block senders)
  - Concurrent delivery with configurable limits
  - Rate limiting to protect external services
* Benefits:
  - Orchestrators don't block on webhook delivery
  - Centralized retry and error handling
  - Unified webhook statistics
  - Easy to add new webhook integrations

## Job Constraints and Actions

### Built-in Constraints
Constraints are conditions evaluated at different points in the job lifecycle. All constraints are built into the orchestrator.

**Pre-execution constraints:**
* `maxConcurrentRuns` - Maximum number of concurrent runs allowed for this job
* `requirePreviousSuccess` - Requires that a specified job with specified args has completed successfully before this job can run
* `preRunHook` - Webhook that must return 200 before the job is allowed to run
* `catchUp` - Whether the job should run if its scheduled time was missed (evaluated within catchUpWindow)
* `catchUpWindow` - Time window to look backwards for missed job runs

**Execution-time constraints:**
* `maxExpectedRunTime` - Duration after which job is marked as "behindSchedule" (can trigger webhook)
* `maxAllowedRunTime` - Duration after which job is hard-killed and marked as failed

**Post-execution constraints:**
* `postRunHook` - Webhook that must return 200 for the job to be considered successful

### Built-in Actions
Actions are behaviors that can be triggered when constraints are violated or jobs fail. Actions are executed by the orchestrator either directly or by sending messages to the main scheduler loop.

**Available actions:**
* `retry` - Retry the current job run (orchestrator restarts the job)
* `kickOffJob` - Start another job (orchestrator spawns new job)
* `webhook` - Trigger a webhook notification
* `killAllInstances` - Kill all currently running instances of the job (message to main loop)
* `killLatestInstance` - Kill the most recent running instance of the job (message to main loop)
* `skipNextInstance` - Skip the next scheduled run of the job (message to main loop)

### Webhook Integrations
Webhooks can be triggered as actions or as part of constraint evaluation.

**Built-in webhook types:**
* **Slack** - Send messages to Slack channels
* **New Relic** - Send events to New Relic
* **PagerDuty** - Create/update PagerDuty incidents
* **Custom** - User-defined webhook with template variables

**Available template variables for custom webhooks:**
* `cluster` - Kubernetes cluster name
* `jobName` - Job name
* `jobNamespace` - Job namespace
* `jobCommand` - Job command
* `jobArgs` - Job arguments
* `runID` - Current run ID
* `status` - Current job status
* `error` - Error message (if failed)

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
   - Acquires advisory lock to prevent concurrent migrations
   - Applies all pending migrations in order
   - Logs current schema version
   - Fails fast if migrations fail (application won't start with outdated schema)

4. **Initialize Components**
   - Start Stats Collector goroutine
   - Start Webhook Handler goroutine
   - Start Syncer goroutine

5. **Initialize Scheduler**
   - Load job definitions from database
   - Build initial ScheduledRunIndex
   - Start main scheduler loop

6. **Start HTTP API Server**
   - Expose REST API for job management
   - Health check endpoints
   - Metrics endpoints

7. **Graceful Shutdown**
   - Listen for SIGINT/SIGTERM
   - Signal main scheduler loop to stop
   - Stop accepting new jobs
   - Allow running jobs to complete (with timeout)
   - Stop Webhook Handler (complete in-flight webhooks)
   - Stop Syncer (flush pending updates)
   - Stop Stats Collector (flush pending stats)
   - Close database connections
   - Exit cleanly

### Command-Line Flags

```bash
itinerary \
  --db-driver=postgres \
  --db-dsn="postgres://user:pass@localhost/itinerary?sslmode=disable" \
  --migrations-dir=./migrations \
  --skip-migrations=false
```

## Database connection
### Database abstraction
* itinerary should be able to run with an relational store
* We will likely need to use a library to abstract over the database connection
* All connections besides the `Recorder` connection should be read only
* Migrations are run automatically on startup before the scheduler initializes
### Schema definition
* constraint_types (dimension table - read-only reference data)
    - id pk (manually assigned, never reused)
    - name (unique constraint type identifier)
    - Built-in types: maxConcurrentRuns (1), catchUp (2), preRunHook (3), postRunHook (4), catchUpWindow (5), maxExpectedRunTime (6), maxAllowedRunTime (7), requirePreviousSuccess (8)
    - New types added via migrations using INSERT OR IGNORE / ON CONFLICT DO NOTHING
  action_types (dimension table - read-only reference data)
    - id pk (manually assigned, never reused)
    - name (unique action type identifier)
    - Built-in types: retry (1), kickOffJob (2), webhook (3), killAllInstances (4), killLatestInstance (5), skipNextInstance (6)
    - New types added via migrations using INSERT OR IGNORE / ON CONFLICT DO NOTHING
  job
    - id pk
    - name
    - schedule
    - podSpec (JSON)
    - constraints (JSON) - map of constraint_type_id → constraint config
    - created_at
    - updated_at
  job_actions
    - id pk
    - job_id (fk job.id)
    - action_type_id (fk action_types.id)
    - trigger (on_failure, on_violation, on_success, etc.)
    - constraint_type_id (fk constraint_types.id, nullable) - when trigger is on_violation, specifies which constraint violation triggers this action
    - config (JSON) - action-specific configuration (webhook URL, retry count, etc.)
  job_run
    - job_id (fk job.id) (pk with scheduled_at)
    - run_id (unique)
    - scheduled_at (pk with job_id)
    - started_at
    - completed_at
    - status
    - success
    - error
  constraint_violations
    - id pk
    - run_id (fk job_runs.run_id)
    - constraint_type_id (fk constraint_types.id)
    - violation_time
    - details (JSON)
  action_runs
    - id pk
    - run_id (fk job_runs.run_id)
    - action_type_id (fk action_types.id)
    - trigger (on_failure, on_violation, on_success, manual)
    - constraint_violation_id (fk constraint_violations.id, nullable) - references the violation that triggered this action, if applicable
    - executed_at
    - success
    - error
    - details (JSON) - action-specific details (webhook response, retry count, etc.)
  scheduler_stats
    - stats_period_id (pk)
    - start_time
    - end_time
    - iterations
    - run_jobs
    - late_jobs
    - time_passed_run_time
    - missed_jobs
    - time_passed_grace_period
    - jobs_cancelled
    - min_inbox_length
    - max_inbox_length
    - avg_inbox_length
    - empty_inbox_time
    - avg_time_in_inbox
    - min_time_in_inbox
    - max_time_in_inbox
  orchestrator_stats
   - run_id
   - stats_period_id
   - runtime
   - constraints_checked
   - actions_taken
  syncer_stats (formerly writer_stats)
    - stats_period_id
    - total_writes
    - writes_succeeded
    - writes_failed
    - avg_writes_in_flight
    - max_writes_in_flight
    - min_writes_in_flight
    - avg_queued_writes
    - max_queued_writes
    - min_queued_writes
    - avg_inbox_length
    - max_inbox_length
    - min_inbox_length
    - avg_time_in_write_queue
    - max_time_in_write_queue
    - min_time_in_write_queue
    - avg_time_in_inbox
    - max_time_in_inbox
    - min_time_in_inbox
  webhook_deliveries (future)
    - id pk
    - run_id (fk job_runs.run_id)
    - webhook_type (slack, newrelic, pagerduty, custom)
    - trigger (on_failure, on_violation, on_success, manual)
    - url
    - attempt_count
    - status_code
    - success
    - error
    - request_duration (milliseconds)
    - created_at
    - delivered_at
  webhook_handler_stats (future)
    - stats_period_id pk
    - start_time
    - end_time
    - webhooks_sent
    - webhooks_succeeded
    - webhooks_failed
    - total_retries
    - avg_delivery_time (milliseconds)
    - max_delivery_time (milliseconds)
    - min_delivery_time (milliseconds)
    - avg_inbox_length
    - max_inbox_length
    - min_inbox_length

### Indexes
* idx_job_runs_run_id - unique index on job_runs.run_id
* idx_job_actions_job_id - index on job_actions.job_id (FK)
* idx_job_actions_action_type_id - index on job_actions.action_type_id (FK)
* idx_job_actions_constraint_type_id - index on job_actions.constraint_type_id (FK)
* idx_job_runs_job_id - index on job_runs.job_id (FK)
* idx_constraint_violations_run_id - index on constraint_violations.run_id (FK)
* idx_constraint_violations_constraint_type_id - index on constraint_violations.constraint_type_id (FK)
* idx_action_runs_run_id - index on action_runs.run_id (FK)
* idx_action_runs_action_type_id - index on action_runs.action_type_id (FK)
* idx_action_runs_constraint_violation_id - index on action_runs.constraint_violation_id (FK)
* idx_orchestrator_stats_stats_period_id - index on orchestrator_stats.stats_period_id (FK)
* idx_webhook_deliveries_run_id - index on webhook_deliveries.run_id (FK) (future)
* idx_webhook_deliveries_created_at - index on webhook_deliveries.created_at (future)

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



## UI web API
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
