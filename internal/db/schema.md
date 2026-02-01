# Database Schema Documentation

## Overview

The Itinerary database schema is designed to support job scheduling, constraint checking, and action triggering. It tracks job definitions, executions, constraint evaluations, and actions taken in response to constraint results.

## Entity Relationship Diagram

```
┌──────────────────┐
│ constraint_types │ (dimension table)
│──────────────────│
│ PK id            │
│    name          │
└────────┬─────────┘
         │
         │ Referenced by
         │
┌────────▼──────────┐          ┌───────────────┐          ┌───────────┐
│ constraints       │◄─────────┤ jobs          │          │action_    │
│───────────────────│ Many:1   │───────────────│          │types      │
│ PK id             │          │ PK id         │          │───────────│
│ FK job_id         │          │    name       │          │ PK id     │
│ FK constraint_    │          │    schedule   │          │    name   │
│    type_id        │          │    pod_spec   │          │           │
│    config         │          │    created_at │          └─────┬─────┘
│    created_at     │          │    updated_at │                │
└────────┬──────────┘          └────────┬──────┘                │
         │                              │                        │
         │                              │ 1:Many                 │ Referenced by
         │                              │                        │
         │                     ┌────────▼───────┐               │
         │         ┌───────────┤ job_runs       │               │
         │         │           │────────────────│               │
         │         │           │ PK (job_id,    │               │
         │         │           │     scheduled_ │               │
         │         │           │     at)        │               │
         │         │           │ UQ run_id      │               │
         │         │           │ FK job_id      │               │
         │         │           │    started_at  │               │
         │         │           │    completed_at│               │
         │         │           │    status      │               │
         │         │           │    success     │               │
         │         │           │    error       │               │
         │         │           │    trigger     │               │
         │         │           └───────┬────────┘               │
         │         │                   │                        │
         │         │ 1:Many            │ 1:Many                 │
         │         │                   │                        │
┌────────▼─────────▼──┐       ┌────────▼────────┐              │
│ actions             │       │constraint_runs  │              │
│─────────────────────│       │─────────────────│              │
│ PK id               │       │ PK id           │              │
│ FK constraint_id    │       │ FK run_id       │              │
│ FK action_type_id   │◄──┐   │ FK constraint_id│              │
│    trigger          │   │   │    executed_at  │              │
│    config           │   │   │    success      │              │
│    created_at       │   │   │    violated     │              │
└─────────────────────┘   │   │    in_error     │              │
                          │   │    error        │              │
                          │   │    details      │              │
                          │   └───────┬─────────┘              │
                          │           │                        │
                          │           │ 1:Many                 │
                          │           │                        │
                          │   ┌───────▼──────────┐             │
                          └───┤ action_runs      │◄────────────┘
                              │──────────────────│  Many:1
                              │ PK id            │
                              │ FK run_id        │
                              │ FK constraint_   │
                              │    run_id        │
                              │ FK action_id     │
                              │    executed_at   │
                              │    success       │
                              │    error         │
                              │    details       │
                              └──────────────────┘
```

## Data Model Concepts

### Core Entities

1. **Job** - The central entity representing a scheduled task
2. **Constraint** - A specific test/check attached to a job (instance of a constraint type)
3. **Action** - An operation that can be triggered when a constraint is met or violated
4. **Job Run** - A single execution of a job
5. **Constraint Run** - A single execution of a constraint check
6. **Action Run** - A single execution of an action

### Dimension Tables

- **ConstraintType** - Pre-defined types of constraints (maxConcurrentRuns, catchUp, etc.)
- **ActionType** - Pre-defined types of actions (retry, kickOffJob, webhook, etc.)

### Key Relationships

- A **Job** can have multiple **Constraints** (1:Many)
- A **Constraint** belongs to one **Job** and has one **ConstraintType** (Many:1, Many:1)
- A **Constraint** can have multiple **Actions** (1:Many)
- An **Action** belongs to one **Constraint** and has one **ActionType** (Many:1, Many:1)
- A **Job** can have multiple **JobRuns** (1:Many)
- A **JobRun** can have multiple **ConstraintRuns** (1:Many)
- A **JobRun** can have multiple **ActionRuns** (1:Many)
- A **ConstraintRun** can trigger multiple **ActionRuns** (1:Many)

## Tables

### constraint_types (Dimension Table)

Pre-seeded table with built-in constraint types.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | INTEGER | PRIMARY KEY | Unique constraint type identifier |
| name | TEXT | NOT NULL, UNIQUE | Constraint type name |

**Pre-seeded values:**
- 1: maxConcurrentRuns
- 2: catchUp
- 3: preRunHook
- 4: postRunHook
- 5: catchUpWindow
- 6: maxExpectedRunTime
- 7: maxAllowedRunTime
- 8: requirePreviousSuccess

### action_types (Dimension Table)

Pre-seeded table with built-in action types.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | INTEGER | PRIMARY KEY | Unique action type identifier |
| name | TEXT | NOT NULL, UNIQUE | Action type name |

**Pre-seeded values:**
- 1: retry
- 2: kickOffJob
- 3: webhook
- 4: killAllInstances
- 5: killLatestInstance
- 6: skipNextInstance

### jobs

Stores job definitions and schedules.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | TEXT | PRIMARY KEY | Unique job identifier |
| name | TEXT | NOT NULL | Human-readable job name |
| schedule | TEXT | NOT NULL | Cron expression defining schedule |
| pod_spec | TEXT | | Kubernetes pod specification (JSON) |
| created_at | TIMESTAMP | NOT NULL | Timestamp when job was created |
| updated_at | TIMESTAMP | NOT NULL | Timestamp when job was last updated |

**Indexes:**
- PRIMARY KEY on id

### constraints

Stores constraint configurations for jobs. Each record is a specific instance of a constraint type attached to a job.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | TEXT | PRIMARY KEY | Unique constraint identifier |
| job_id | TEXT | NOT NULL, FK→jobs(id) | Job this constraint belongs to |
| constraint_type_id | INTEGER | NOT NULL, FK→constraint_types(id) | Type of constraint |
| config | TEXT | | JSON configuration specific to constraint type |
| created_at | TIMESTAMP | NOT NULL | Timestamp when constraint was created |

**Indexes:**
- PRIMARY KEY on id
- INDEX on job_id

**Foreign Keys:**
- job_id REFERENCES jobs(id) ON DELETE CASCADE
- constraint_type_id REFERENCES constraint_types(id) ON DELETE CASCADE

**Example config JSON:**
```json
{
  "value": 5,
  "units": "instances"
}
```

### actions

Stores action definitions that can be triggered when constraints are met or violated. Actions belong to a specific constraint.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | TEXT | PRIMARY KEY | Unique action identifier |
| constraint_id | TEXT | NOT NULL, FK→constraints(id) | Constraint this action belongs to |
| action_type_id | INTEGER | NOT NULL, FK→action_types(id) | Type of action |
| trigger | TEXT | NOT NULL | When to trigger: 'on_met', 'on_violated' |
| config | TEXT | | JSON configuration specific to action type |
| created_at | TIMESTAMP | NOT NULL | Timestamp when action was created |

**Indexes:**
- PRIMARY KEY on id
- INDEX on constraint_id

**Foreign Keys:**
- constraint_id REFERENCES constraints(id) ON DELETE CASCADE
- action_type_id REFERENCES action_types(id) ON DELETE CASCADE

**Example config JSON (webhook):**
```json
{
  "url": "https://hooks.slack.com/...",
  "channel": "#alerts",
  "message": "Job exceeded runtime limit"
}
```

**Example config JSON (kickOffJob):**
```json
{
  "jobID": "cleanup-job",
  "args": ["--force"]
}
```

### job_runs

Tracks individual executions of jobs.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| job_id | TEXT | NOT NULL, FK→jobs(id) | Job being executed |
| run_id | TEXT | NOT NULL, UNIQUE | Unique run identifier |
| scheduled_at | TIMESTAMP | NOT NULL | When this run was scheduled |
| started_at | TIMESTAMP | | When execution started |
| completed_at | TIMESTAMP | | When execution completed |
| status | TEXT | NOT NULL | 'pending', 'running', 'completed', 'failed', 'cancelled' |
| success | BOOLEAN | | Whether execution succeeded (NULL if not completed) |
| error | TEXT | | Error message if execution failed |
| trigger | TEXT | NOT NULL | How run was triggered: 'scheduled', 'manual', 'retry', 'action' |

**Indexes:**
- PRIMARY KEY on (job_id, scheduled_at)
- UNIQUE INDEX on run_id
- INDEX on job_id

**Foreign Keys:**
- job_id REFERENCES jobs(id) ON DELETE CASCADE

### constraint_runs

Tracks each execution of a constraint check. Records whether the constraint was satisfied (success), violated, or errored.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | TEXT | PRIMARY KEY | Unique constraint run identifier |
| run_id | TEXT | NOT NULL, FK→job_runs(run_id) | Job run this check belongs to |
| constraint_id | TEXT | NOT NULL, FK→constraints(id) | Constraint being checked |
| executed_at | TIMESTAMP | NOT NULL | When constraint was checked |
| success | BOOLEAN | NOT NULL | Whether check completed successfully |
| violated | BOOLEAN | NOT NULL | Whether constraint was violated |
| in_error | BOOLEAN | NOT NULL | Whether check encountered an error |
| error | TEXT | | Error message if check failed |
| details | TEXT | | JSON with check-specific details |

**Indexes:**
- PRIMARY KEY on id
- INDEX on run_id
- INDEX on constraint_id

**Foreign Keys:**
- run_id REFERENCES job_runs(run_id) ON DELETE CASCADE
- constraint_id REFERENCES constraints(id) ON DELETE CASCADE

**Constraint States:**
- **Met** (`success=true, violated=false, in_error=false`): Constraint check passed, condition satisfied
- **Violated** (`success=true, violated=true, in_error=false`): Constraint check passed, but condition violated
- **In Error** (`success=false, in_error=true`): Constraint check failed to execute
- Actions trigger on 'on_met' or 'on_violated' only; no actions run when in_error

**Example details JSON:**
```json
{
  "expected": "2h",
  "actual": "2h15m",
  "threshold_exceeded_by": "15m"
}
```

### action_runs

Tracks each execution of an action. Actions run in isolation - if one action fails, it doesn't affect other actions.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | TEXT | PRIMARY KEY | Unique action run identifier |
| run_id | TEXT | NOT NULL, FK→job_runs(run_id) | Job run this action belongs to |
| constraint_run_id | TEXT | FK→constraint_runs(id) | Constraint run that triggered this action (NULL for non-constraint triggers) |
| action_id | TEXT | NOT NULL, FK→actions(id) | Action being executed |
| executed_at | TIMESTAMP | NOT NULL | When action was executed |
| success | BOOLEAN | NOT NULL | Whether action succeeded |
| error | TEXT | | Error message if action failed |
| details | TEXT | | JSON with action-specific details |

**Indexes:**
- PRIMARY KEY on id
- INDEX on run_id
- INDEX on constraint_run_id
- INDEX on action_id

**Foreign Keys:**
- run_id REFERENCES job_runs(run_id) ON DELETE CASCADE
- constraint_run_id REFERENCES constraint_runs(id) ON DELETE CASCADE
- action_id REFERENCES actions(id) ON DELETE CASCADE

**Example details JSON (webhook):**
```json
{
  "status_code": 200,
  "response_time_ms": 145,
  "response_body": "ok"
}
```

**Example details JSON (retry):**
```json
{
  "retry_count": 1,
  "new_run_id": "run-abc-retry-1"
}
```

## Statistics Tables

### scheduler_stats

Tracks scheduler performance metrics for a time period.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| stats_period_id | TEXT | PRIMARY KEY | Unique period identifier |
| start_time | TIMESTAMP | NOT NULL | Period start time |
| end_time | TIMESTAMP | NOT NULL | Period end time |
| iterations | INTEGER | NOT NULL | Number of scheduler iterations |
| run_jobs | INTEGER | NOT NULL | Number of jobs run |
| late_jobs | INTEGER | NOT NULL | Number of late jobs |
| time_passed_run_time | INTEGER | NOT NULL | Jobs that exceeded expected runtime |
| missed_jobs | INTEGER | NOT NULL | Jobs that missed their window |
| time_passed_grace_period | INTEGER | NOT NULL | Jobs that exceeded grace period |
| jobs_cancelled | INTEGER | NOT NULL | Jobs that were cancelled |
| min_inbox_length | INTEGER | | Minimum inbox queue length |
| max_inbox_length | INTEGER | | Maximum inbox queue length |
| avg_inbox_length | REAL | | Average inbox queue length |
| empty_inbox_time | INTEGER | | Time inbox was empty (ms) |
| avg_time_in_inbox | REAL | | Average time messages spent in inbox (ms) |
| min_time_in_inbox | INTEGER | | Minimum time in inbox (ms) |
| max_time_in_inbox | INTEGER | | Maximum time in inbox (ms) |

### orchestrator_stats

Tracks orchestrator performance for each job run.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| run_id | TEXT | PRIMARY KEY, FK→job_runs(run_id) | Job run identifier |
| stats_period_id | TEXT | NOT NULL, FK→scheduler_stats(stats_period_id) | Associated stats period |
| runtime | INTEGER | NOT NULL | Orchestrator runtime (ms) |
| constraints_checked | INTEGER | NOT NULL | Number of constraints checked |
| actions_taken | INTEGER | NOT NULL | Number of actions executed |

### syncer_stats

Tracks database syncer performance metrics.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| stats_period_id | TEXT | PRIMARY KEY | Unique period identifier |
| start_time | TIMESTAMP | NOT NULL | Period start time |
| end_time | TIMESTAMP | NOT NULL | Period end time |
| total_writes | INTEGER | NOT NULL | Total write operations attempted |
| writes_succeeded | INTEGER | NOT NULL | Successful write operations |
| writes_failed | INTEGER | NOT NULL | Failed write operations |
| avg_writes_in_flight | REAL | | Average concurrent writes |
| max_writes_in_flight | INTEGER | | Maximum concurrent writes |
| min_writes_in_flight | INTEGER | | Minimum concurrent writes |
| avg_queued_writes | REAL | | Average queued writes |
| max_queued_writes | INTEGER | | Maximum queued writes |
| min_queued_writes | INTEGER | | Minimum queued writes |
| avg_inbox_length | REAL | | Average inbox queue length |
| max_inbox_length | INTEGER | | Maximum inbox queue length |
| min_inbox_length | INTEGER | | Minimum inbox queue length |
| avg_time_in_write_queue | REAL | | Average time in write queue (ms) |
| max_time_in_write_queue | INTEGER | | Maximum time in write queue (ms) |
| min_time_in_write_queue | INTEGER | | Minimum time in write queue (ms) |
| avg_time_in_inbox | REAL | | Average time in inbox (ms) |
| max_time_in_inbox | INTEGER | | Maximum time in inbox (ms) |
| min_time_in_inbox | INTEGER | | Minimum time in inbox (ms) |

### stats_collector_stats

Tracks stats collector component performance.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| stats_period_id | TEXT | PRIMARY KEY | Unique period identifier |
| start_time | TIMESTAMP | NOT NULL | Period start time |
| end_time | TIMESTAMP | NOT NULL | Period end time |
| messages_received | INTEGER | NOT NULL | Total messages received |
| messages_processed | INTEGER | NOT NULL | Messages successfully processed |
| scheduler_messages | INTEGER | NOT NULL | Messages from scheduler |
| orchestrator_messages | INTEGER | NOT NULL | Messages from orchestrator |
| syncer_messages | INTEGER | NOT NULL | Messages from syncer |
| webhook_messages | INTEGER | NOT NULL | Messages from webhook handler |
| periods_completed | INTEGER | NOT NULL | Number of stat periods completed |
| database_flushes | INTEGER | NOT NULL | Number of DB flush operations |
| flush_errors | INTEGER | NOT NULL | Number of failed flushes |
| avg_inbox_length | REAL | | Average inbox queue length |
| max_inbox_length | INTEGER | | Maximum inbox queue length |
| min_inbox_length | INTEGER | | Minimum inbox queue length |
| avg_processing_time | REAL | | Average message processing time (μs) |
| max_processing_time | INTEGER | | Maximum processing time (μs) |
| min_processing_time | INTEGER | | Minimum processing time (μs) |

## Schema Evolution Notes

### Changes from Previous Version

1. **Removed `constraints` JSON field from `jobs` table** - Constraints are now first-class entities in their own table
2. **Renamed `job_actions` table to `actions`** - Actions now belong to constraints, not jobs
3. **Removed `job_id` from actions** - Actions are scoped to constraints
4. **Removed `constraint_type_id` from actions** - This information is on the constraint
5. **Added `constraints` table** - New table to represent constraint instances
6. **Added `constraint_runs` table** - Tracks each constraint check execution
7. **Removed `constraint_violations` table** - Violations are now indicated by a flag on constraint_runs
8. **Added `trigger` field to `job_runs`** - Tracks how the run was initiated
9. **Updated `action_runs` schema** - Now references constraint_run_id and action_id instead of action_type_id and constraint_violation_id
10. **Removed webhook tables** - WebhookDelivery and WebhookHandlerStats tables removed (may be added back later)

### Migration Considerations

When migrating from the old schema:
- Extract constraint configurations from jobs.constraints JSON into separate constraint records
- Migrate job_actions to actions, linking them to constraints instead of jobs
- Convert constraint_violations to constraint_runs with appropriate flags
- Add default trigger value ('scheduled') to existing job_runs
- Update action_runs foreign keys

## Query Patterns

### Common Operations

#### Get all constraints for a job with their actions
```sql
SELECT c.*, a.*
FROM constraints c
LEFT JOIN actions a ON a.constraint_id = c.id
WHERE c.job_id = ?
ORDER BY c.created_at, a.created_at;
```

#### Get constraint run history with violation status
```sql
SELECT cr.*, c.constraint_type_id, ct.name
FROM constraint_runs cr
JOIN constraints c ON c.id = cr.constraint_id
JOIN constraint_types ct ON ct.id = c.constraint_type_id
WHERE cr.run_id = ?
ORDER BY cr.executed_at;
```

#### Get all actions triggered by a specific constraint run
```sql
SELECT ar.*, a.action_type_id, at.name
FROM action_runs ar
JOIN actions a ON a.id = ar.action_id
JOIN action_types at ON at.id = a.action_type_id
WHERE ar.constraint_run_id = ?
ORDER BY ar.executed_at;
```

#### Find violated constraints for a job
```sql
SELECT cr.*, jr.job_id
FROM constraint_runs cr
JOIN job_runs jr ON jr.run_id = cr.run_id
WHERE jr.job_id = ? AND cr.violated = true
ORDER BY cr.executed_at DESC
LIMIT 10;
```
