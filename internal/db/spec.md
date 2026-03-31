# Database Layer

## Overview

The database layer provides a shared abstraction for all database operations in the Itinerary application. It handles connection management, query execution, transaction support, and provides a consistent interface across PostgreSQL, MySQL, and SQLite.

## Goals

1. **Single source of truth** - All database operations go through this layer
2. **Database agnostic** - Support PostgreSQL (primary), MySQL, and SQLite (dev/test)
3. **Type-safe** - Strong typing for all database operations
4. **Minimal dependencies** - Use only stdlib `database/sql` package
5. **Performance** - Connection pooling, prepared statements
6. **Safety** - Parameterized queries, transaction support

## Component Location

```
internal/
└── db/
    ├── db.go           # Connection management and database interface
    ├── db_test.go      # Tests
    ├── schema.go       # Schema definitions and types
    ├── jobs.go         # Job-related queries
    ├── runs.go         # Job run queries
    ├── stats.go        # Statistics queries
    └── migrations/     # SQL migration files
```

## Database Support

### Primary: PostgreSQL
- Production database
- Full feature support
- Advisory locks for migrations
- JSONB support for flexible schemas

### Development: SQLite
- Local development and testing
- In-memory mode for tests
- File-based for local dev
- Most features supported

### Optional: MySQL
- Alternative production option
- Core features supported
- Some PostgreSQL-specific features may differ

## Connection Management

### Database Interface

```go
// DB is the main database interface used throughout the application
type DB struct {
    *sql.DB
    driver string // "postgres", "mysql", or "sqlite3"
}

// Open creates a new database connection
func Open(driver, dsn string) (*DB, error)

// Close closes the database connection
func (db *DB) Close() error

// Ping verifies the database connection is alive
func (db *DB) Ping() error

// Driver returns the database driver name
func (db *DB) Driver() string
```

### Connection Configuration

```go
type Config struct {
    Driver          string        // "postgres", "mysql", "sqlite3"
    DSN             string        // Data source name
    MaxOpenConns    int           // Maximum open connections (default: 25)
    MaxIdleConns    int           // Maximum idle connections (default: 5)
    ConnMaxLifetime time.Duration // Connection max lifetime (default: 5m)
    ConnMaxIdleTime time.Duration // Connection max idle time (default: 5m)
}

// OpenWithConfig creates a connection with custom configuration
func OpenWithConfig(config Config) (*DB, error)
```

### Transaction Support

```go
// Tx wraps sql.Tx with additional context
type Tx struct {
    *sql.Tx
    db *DB
}

// Begin starts a new transaction
func (db *DB) Begin() (*Tx, error)

// Commit commits the transaction
func (tx *Tx) Commit() error

// Rollback rolls back the transaction
func (tx *Tx) Rollback() error

// WithTransaction executes a function within a transaction
// Automatically commits on success, rolls back on error
func (db *DB) WithTransaction(fn func(*Tx) error) error
```

## Schema Definition

### Core Tables

**Note**: The schema uses dimension tables (`constraint_types` and `action_types`) to define built-in types, with separate tables (`constraints` and `actions`) for instances.

#### Constraints Table
Stores constraint instances attached to jobs. Each constraint is a specific configuration of a constraint type.

```sql
CREATE TABLE constraints (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    constraint_type_id INTEGER NOT NULL,
    config TEXT,                  -- JSON configuration specific to constraint type
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,
    FOREIGN KEY (constraint_type_id) REFERENCES constraint_types(id) ON DELETE CASCADE
);
```

```go
type Constraint struct {
    ID               string
    JobID            string
    ConstraintTypeID int
    Config           *string // JSON
    CreatedAt        time.Time
}
```

**Config JSON structure example (maxConcurrentRuns):**
```json
{
  "value": 5
}
```

**Config JSON structure example (maxExpectedRunTime):**
```json
{
  "value": "2h"
}
```

#### Actions Table
Stores action instances that can be triggered when constraints are met or violated. Actions belong to specific constraints.

```sql
CREATE TABLE actions (
    id TEXT PRIMARY KEY,
    constraint_id TEXT NOT NULL,
    action_type_id INTEGER NOT NULL,
    trigger TEXT NOT NULL,      -- 'on_met', 'on_violated'
    config TEXT,                -- JSON configuration specific to action type
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (constraint_id) REFERENCES constraints(id) ON DELETE CASCADE,
    FOREIGN KEY (action_type_id) REFERENCES action_types(id) ON DELETE CASCADE
);
```

```go
type Action struct {
    ID           string
    ConstraintID string
    ActionTypeID int
    Trigger      string
    Config       *string // JSON
    CreatedAt    time.Time
}
```

**Config JSON structure (webhook example):**
```json
{
  "url": "https://hooks.slack.com/...",
  "channel": "#alerts",
  "message": "Job exceeded runtime limit"
}
```

**Config JSON structure (kickOffJob example):**
```json
{
  "jobID": "cleanup-job",
  "args": ["--force"]
}
```

#### Jobs Table
Stores job definitions and schedules.

```sql
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    schedule TEXT NOT NULL,     -- Cron expression
    pod_spec TEXT,              -- Kubernetes pod specification (JSON)
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

```go
type Job struct {
    ID        string
    Name      string
    Schedule  string
    PodSpec   string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Job Runs Table
Tracks individual executions of jobs.

```sql
CREATE TABLE job_runs (
    job_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    scheduled_at TIMESTAMP NOT NULL,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    status TEXT NOT NULL,  -- 'pending', 'running', 'completed', 'failed', 'cancelled'
    success BOOLEAN,
    error TEXT,
    trigger TEXT NOT NULL, -- 'scheduled', 'manual', 'retry', 'action'
    PRIMARY KEY (job_id, scheduled_at),
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_job_runs_run_id ON job_runs(run_id);
CREATE INDEX idx_job_runs_job_id ON job_runs(job_id);

-- Note: run_id has unique index for GetJobRunByRunID lookups.
-- job_id has index to optimize FK constraint checks and queries by job.
-- Other potential indexes (status, scheduled_at) will be added later based on UI query patterns.
```

```go
type JobRun struct {
    JobID       string
    RunID       string
    ScheduledAt time.Time
    StartedAt   *time.Time
    CompletedAt *time.Time
    Status      string
    Success     *bool
    Error       *string
    Trigger     string
}
```

**Trigger values:**
- `scheduled` - Run triggered by the scheduler based on cron schedule
- `manual` - Run triggered manually by a user
- `retry` - Run triggered by a retry action
- `action` - Run triggered by another action (e.g., kickOffJob)

#### Constraint Runs Table
Tracks each execution of a constraint check for a job run.

```sql
CREATE TABLE constraint_runs (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    constraint_id TEXT NOT NULL,
    executed_at TIMESTAMP NOT NULL,
    success BOOLEAN NOT NULL,
    violated BOOLEAN NOT NULL,
    in_error BOOLEAN NOT NULL,
    error TEXT,
    details TEXT,                   -- Additional details (JSON)
    FOREIGN KEY (run_id) REFERENCES job_runs(run_id) ON DELETE CASCADE,
    FOREIGN KEY (constraint_id) REFERENCES constraints(id) ON DELETE CASCADE
);

CREATE INDEX idx_constraint_runs_run_id ON constraint_runs(run_id);
CREATE INDEX idx_constraint_runs_constraint_id ON constraint_runs(constraint_id);
```

```go
type ConstraintRun struct {
    ID           string
    RunID        string
    ConstraintID string
    ExecutedAt   time.Time
    Success      bool
    Violated     bool
    InError      bool
    Error        *string
    Details      *string
}
```

**Constraint Run States:**
- **Met** (`success=true, violated=false, in_error=false`): Constraint check passed, condition satisfied
- **Violated** (`success=true, violated=true, in_error=false`): Constraint check completed, but condition violated
- **In Error** (`success=false, in_error=true`): Constraint check failed to execute properly
- Actions configured with `on_met` trigger when violated=false
- Actions configured with `on_violated` trigger when violated=true
- No actions run when in_error=true

**Details JSON structure example:**
```json
{
  "message": "Job exceeded maxAllowedRunTime of 2h",
  "expected": "2h",
  "actual": "2h15m",
  "threshold_exceeded_by": "15m"
}
```

### Statistics Tables

#### Scheduler Stats Table
Tracks scheduler performance metrics.

```sql
CREATE TABLE scheduler_stats (
    stats_period_id TEXT PRIMARY KEY,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    iterations INTEGER NOT NULL DEFAULT 0,
    run_jobs INTEGER NOT NULL DEFAULT 0,
    late_jobs INTEGER NOT NULL DEFAULT 0,
    time_passed_run_time INTEGER NOT NULL DEFAULT 0,
    missed_jobs INTEGER NOT NULL DEFAULT 0,
    time_passed_grace_period INTEGER NOT NULL DEFAULT 0,
    jobs_cancelled INTEGER NOT NULL DEFAULT 0,
    min_inbox_length INTEGER,
    max_inbox_length INTEGER,
    avg_inbox_length REAL,
    empty_inbox_time INTEGER,
    avg_time_in_inbox REAL,
    min_time_in_inbox INTEGER,
    max_time_in_inbox INTEGER
);

-- No additional indexes yet. Will be added based on actual query patterns.
```

```go
type SchedulerStats struct {
    StatsPeriodID         string
    StartTime             time.Time
    EndTime               time.Time
    Iterations            int
    RunJobs               int
    LateJobs              int
    TimePassedRunTime     int
    MissedJobs            int
    TimePassedGracePeriod int
    JobsCancelled         int
    MinInboxLength        *int
    MaxInboxLength        *int
    AvgInboxLength        *float64
    EmptyInboxTime        *int
    AvgTimeInInbox        *float64
    MinTimeInInbox        *int
    MaxTimeInInbox        *int
}
```

#### Orchestrator Stats Table

```sql
CREATE TABLE orchestrator_stats (
    run_id TEXT PRIMARY KEY,
    stats_period_id TEXT NOT NULL,
    runtime INTEGER NOT NULL,
    constraints_checked INTEGER NOT NULL DEFAULT 0,
    actions_taken INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (stats_period_id) REFERENCES scheduler_stats(stats_period_id) ON DELETE CASCADE
);

CREATE INDEX idx_orchestrator_stats_stats_period_id ON orchestrator_stats(stats_period_id);
```

```go
type OrchestratorStats struct {
    RunID              string
    StatsPeriodID      string
    Runtime            int
    ConstraintsChecked int
    ActionsTaken       int
}
```

## Indexing Strategy

The database layer takes a conservative approach to indexing. Indexes are only added when:

1. **High selectivity** - The predicate selects << 50% of table rows
2. **High cardinality** - The indexed column(s) have many distinct values
3. **Proven need** - Query patterns demonstrate the index improves performance

### Current Indexes

#### Unique Indexes
- **job_runs.run_id**: `idx_job_runs_run_id` (UNIQUE) - for GetJobRunByRunID lookups

#### Foreign Key Indexes
All foreign keys have indexes to optimize JOIN operations and foreign key constraint checks:
- **constraints.job_id**: `idx_constraints_job_id` - FK to jobs table
- **actions.constraint_id**: `idx_actions_constraint_id` - FK to constraints table
- **job_runs.job_id**: `idx_job_runs_job_id` - FK to jobs table
- **constraint_runs.run_id**: `idx_constraint_runs_run_id` - FK to job_runs table
- **constraint_runs.constraint_id**: `idx_constraint_runs_constraint_id` - FK to constraints table
- **action_runs.run_id**: `idx_action_runs_run_id` - FK to job_runs table
- **action_runs.constraint_run_id**: `idx_action_runs_constraint_run_id` - FK to constraint_runs table
- **action_runs.action_id**: `idx_action_runs_action_id` - FK to actions table
- **orchestrator_stats.stats_period_id**: `idx_orchestrator_stats_stats_period_id` - FK to scheduler_stats table

**Note**: Primary keys automatically create indexes. Composite primary keys (e.g., job_runs) index all PK columns together.

### Future Indexes

Additional indexes will be added based on:
- UI query patterns (filtering, searching, sorting)
- Constraint violation query patterns
- Performance profiling results

## Memory Management

### In-Memory Jobs Cache

The scheduler loads the entire `jobs` table into memory:
- On startup
- On each index rebuild (every INDEX_REBUILD_INTERVAL)

**Critical requirement**: The in-memory jobs slice must not be leaked or accumulated:

```go
// CORRECT: Replace the old slice
func (s *Scheduler) rebuildIndex() {
    jobs, err := s.db.GetAllJobs()
    if err != nil {
        // handle error
        return
    }

    // Generate new index from jobs
    newIndex := buildScheduledRunIndex(jobs, s.config)

    // Atomically swap index
    s.index.Store(newIndex)

    // Old jobs slice and old index are garbage collected
}

// INCORRECT: Accumulating jobs in a growing slice
func (s *Scheduler) rebuildIndex() {
    jobs, err := s.db.GetAllJobs()
    if err != nil {
        return
    }

    s.allJobs = append(s.allJobs, jobs...) // MEMORY LEAK
}
```

**Memory safety guidelines**:
1. Store jobs temporarily only during index rebuild
2. Don't keep references to old job slices
3. Let garbage collector reclaim old data
4. Consider using `runtime.ReadMemStats()` to monitor memory in tests

## Query Interface

### Jobs Queries

```go
// GetJob retrieves a job by ID
func (db *DB) GetJob(id string) (*Job, error)

// GetAllJobs retrieves all jobs
func (db *DB) GetAllJobs() ([]Job, error)

// CreateJob creates a new job
func (db *DB) CreateJob(job *Job) error

// UpdateJob updates an existing job
func (db *DB) UpdateJob(job *Job) error

// DeleteJob deletes a job by ID
func (db *DB) DeleteJob(id string) error
```

### Job Runs Queries

```go
// GetJobRun retrieves a job run
func (db *DB) GetJobRun(jobID string, scheduledAt time.Time) (*JobRun, error)

// GetJobRunByRunID retrieves a job run by its run ID
func (db *DB) GetJobRunByRunID(runID string) (*JobRun, error)

// GetJobRuns retrieves all runs for a job
func (db *DB) GetJobRuns(jobID string, limit int) ([]JobRun, error)

// CreateJobRun creates a new job run record
func (db *DB) CreateJobRun(run *JobRun) error

// UpdateJobRunStatus updates the status of a job run
func (db *DB) UpdateJobRunStatus(runID string, status string) error

// CompleteJobRun marks a job run as completed
func (db *DB) CompleteJobRun(runID string, success bool, err *string) error
```

### Constraints Queries

```go
// CreateConstraint creates a new constraint configuration
func (db *DB) CreateConstraint(constraint *Constraint) error

// GetConstraint retrieves a constraint by ID
func (db *DB) GetConstraint(id string) (*Constraint, error)

// UpdateConstraint updates an existing constraint
func (db *DB) UpdateConstraint(constraint *Constraint) error

// DeleteConstraint deletes a constraint by ID
func (db *DB) DeleteConstraint(id string) error
```

### Constraints Queries

```go
// CreateConstraint creates a new constraint for a job
func (db *DB) CreateConstraint(constraint *Constraint) error

// GetConstraint retrieves a constraint by ID
func (db *DB) GetConstraint(id string) (*Constraint, error)

// GetConstraintsByJob retrieves all constraints for a job
func (db *DB) GetConstraintsByJob(jobID string) ([]Constraint, error)

// DeleteConstraint deletes a constraint by ID
func (db *DB) DeleteConstraint(id string) error
```

### Actions Queries

```go
// CreateAction creates a new action for a constraint
func (db *DB) CreateAction(action *Action) error

// GetAction retrieves an action by ID
func (db *DB) GetAction(id string) (*Action, error)

// GetActionsByConstraint retrieves all actions for a constraint
func (db *DB) GetActionsByConstraint(constraintID string) ([]Action, error)

// DeleteAction deletes an action by ID
func (db *DB) DeleteAction(id string) error
```

### Constraint Run Queries

```go
// CreateConstraintRun records a constraint check execution
func (db *DB) CreateConstraintRun(constraintRun *ConstraintRun) error

// GetConstraintRuns retrieves all constraint runs for a job run
func (db *DB) GetConstraintRuns(runID string) ([]ConstraintRun, error)

// GetConstraintRunsByConstraint retrieves constraint runs for a specific constraint
func (db *DB) GetConstraintRunsByConstraint(constraintID string, limit int) ([]ConstraintRun, error)
```

### Action Run Queries

```go
// CreateActionRun records an action execution
func (db *DB) CreateActionRun(actionRun *ActionRun) error

// GetActionRuns retrieves all action runs for a job run
func (db *DB) GetActionRuns(runID string) ([]ActionRun, error)

// GetActionRunsByAction retrieves action runs for a specific action
func (db *DB) GetActionRunsByAction(actionID string, limit int) ([]ActionRun, error)
```

### Stats Queries

```go
// CreateSchedulerStats inserts scheduler statistics
func (db *DB) CreateSchedulerStats(stats *SchedulerStats) error

// GetSchedulerStats retrieves scheduler stats for a period
func (db *DB) GetSchedulerStats(startTime, endTime time.Time) ([]SchedulerStats, error)
```

## Database-Specific SQL

Handle dialect differences internally:

```go
// placeholder returns the appropriate placeholder for the database
func (db *DB) placeholder(n int) string {
    switch db.driver {
    case "postgres":
        return fmt.Sprintf("$%d", n)
    case "mysql", "sqlite3":
        return "?"
    default:
        return "?"
    }
}

// insertReturning generates INSERT...RETURNING if supported
func (db *DB) insertReturning(table string) bool {
    return db.driver == "postgres" || db.driver == "sqlite3"
}
```

## Error Handling

```go
// ErrNotFound indicates a resource was not found
var ErrNotFound = errors.New("db: not found")

// ErrDuplicate indicates a duplicate key violation
var ErrDuplicate = errors.New("db: duplicate key")

// ErrForeignKey indicates a foreign key constraint violation
var ErrForeignKey = errors.New("db: foreign key violation")

// IsNotFound checks if error is a not found error
func IsNotFound(err error) bool

// IsDuplicate checks if error is a duplicate key error
func IsDuplicate(err error) bool

// IsForeignKey checks if error is a foreign key error
func IsForeignKey(err error) bool
```

## Usage Example

```go
package main

import (
    "log"

    "github.com/livinlefevreloca/itinerary/internal/db"
    _ "github.com/lib/pq"
)

func main() {
    // Open database connection
    database, err := db.Open("postgres", "postgres://localhost/itinerary?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer database.Close()

    // Create a job
    job := &db.Job{
        ID:       "job-123",
        Name:     "Daily Report",
        Schedule: "0 0 * * *",
        PodSpec:  `{"image": "report-generator:latest"}`,
    }

    if err := database.CreateJob(job); err != nil {
        log.Fatalf("failed to create job: %v", err)
    }

    // Retrieve all jobs
    jobs, err := database.GetAllJobs()
    if err != nil {
        log.Fatalf("failed to get jobs: %v", err)
    }

    log.Printf("Found %d jobs", len(jobs))

    // Use transaction
    err = database.WithTransaction(func(tx *db.Tx) error {
        // Create job run
        run := &db.JobRun{
            JobID:       "job-123",
            RunID:       "run-456",
            ScheduledAt: time.Now(),
            Status:      "pending",
        }
        return tx.CreateJobRun(run)
    })
    if err != nil {
        log.Fatalf("transaction failed: %v", err)
    }
}
```

## Integration with Migrator

The migrator tool will use this database layer:

```go
import (
    "github.com/livinlefevreloca/itinerary/internal/db"
    "github.com/livinlefevreloca/itinerary/tools/migrator"
)

func runMigrations() error {
    database, err := db.Open("postgres", dsn)
    if err != nil {
        return err
    }
    defer database.Close()

    // Migrator uses the same DB connection
    return migrator.RunMigrations(database.DB, "migrations")
}
```

## Testing Support

```go
// NewTestDB creates an in-memory SQLite database for testing
func NewTestDB(t *testing.T) *DB

// ResetDB clears all tables in the database
func (db *DB) ResetDB() error

// SeedTestData populates database with test data
func (db *DB) SeedTestData() error
```

## Non-Goals

1. **ORM features** - No automatic struct mapping, keep SQL explicit
2. **Query builder** - Write SQL directly, no DSL
3. **Caching** - Application layer responsibility
4. **Sharding** - Single database for now
5. **Migrations in this package** - Handled by separate migrator tool
