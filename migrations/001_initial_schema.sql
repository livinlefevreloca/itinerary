-- +migrate Up

-- =============================================================================
-- DIMENSION TABLES (Reference Data)
-- =============================================================================

-- Constraint types dimension table
-- IDs are manually assigned and never reused or deleted
CREATE TABLE constraint_types (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

-- Action types dimension table
-- IDs are manually assigned and never reused or deleted
CREATE TABLE action_types (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

-- Seed constraint types with built-in types (IDs 1-8 reserved)
INSERT INTO constraint_types (id, name) VALUES
    (1, 'maxConcurrentRuns'),
    (2, 'catchUp'),
    (3, 'preRunHook'),
    (4, 'postRunHook'),
    (5, 'catchUpWindow'),
    (6, 'maxExpectedRunTime'),
    (7, 'maxAllowedRunTime'),
    (8, 'requirePreviousSuccess');

-- Seed action types with built-in types (IDs 1-6 reserved)
INSERT INTO action_types (id, name) VALUES
    (1, 'retry'),
    (2, 'kickOffJob'),
    (3, 'webhook'),
    (4, 'killAllInstances'),
    (5, 'killLatestInstance'),
    (6, 'skipNextInstance');

-- =============================================================================
-- JOB CONFIGURATION TABLES
-- =============================================================================

-- Jobs table stores job definitions
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    schedule TEXT NOT NULL,
    pod_spec TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Constraints table stores constraint instances attached to jobs
CREATE TABLE constraints (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    constraint_type_id INTEGER NOT NULL,
    config TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,
    FOREIGN KEY (constraint_type_id) REFERENCES constraint_types(id) ON DELETE CASCADE
);

CREATE INDEX idx_constraints_job_id ON constraints(job_id);

-- Actions table stores action instances attached to constraints
CREATE TABLE actions (
    id TEXT PRIMARY KEY,
    constraint_id TEXT NOT NULL,
    action_type_id INTEGER NOT NULL,
    trigger TEXT NOT NULL,
    config TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (constraint_id) REFERENCES constraints(id) ON DELETE CASCADE,
    FOREIGN KEY (action_type_id) REFERENCES action_types(id) ON DELETE CASCADE
);

CREATE INDEX idx_actions_constraint_id ON actions(constraint_id);

-- =============================================================================
-- JOB EXECUTION TABLES
-- =============================================================================

-- Job runs table tracks individual job executions
CREATE TABLE job_runs (
    job_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    scheduled_at TIMESTAMP NOT NULL,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    status TEXT NOT NULL,
    success BOOLEAN,
    error TEXT,
    trigger TEXT NOT NULL,
    PRIMARY KEY (job_id, scheduled_at),
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_job_runs_run_id ON job_runs(run_id);
CREATE INDEX idx_job_runs_job_id ON job_runs(job_id);

-- Constraint runs table tracks constraint evaluations
CREATE TABLE constraint_runs (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    constraint_id TEXT NOT NULL,
    executed_at TIMESTAMP NOT NULL,
    success BOOLEAN NOT NULL,
    violated BOOLEAN NOT NULL,
    in_error BOOLEAN NOT NULL,
    error TEXT,
    details TEXT,
    FOREIGN KEY (run_id) REFERENCES job_runs(run_id) ON DELETE CASCADE,
    FOREIGN KEY (constraint_id) REFERENCES constraints(id) ON DELETE CASCADE
);

CREATE INDEX idx_constraint_runs_run_id ON constraint_runs(run_id);
CREATE INDEX idx_constraint_runs_constraint_id ON constraint_runs(constraint_id);

-- Action runs table tracks action executions (future)
CREATE TABLE action_runs (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    constraint_run_id TEXT,
    action_id TEXT NOT NULL,
    executed_at TIMESTAMP NOT NULL,
    success BOOLEAN NOT NULL,
    error TEXT,
    details TEXT,
    FOREIGN KEY (run_id) REFERENCES job_runs(run_id) ON DELETE CASCADE,
    FOREIGN KEY (constraint_run_id) REFERENCES constraint_runs(id) ON DELETE SET NULL,
    FOREIGN KEY (action_id) REFERENCES actions(id) ON DELETE CASCADE
);

CREATE INDEX idx_action_runs_run_id ON action_runs(run_id);
CREATE INDEX idx_action_runs_constraint_run_id ON action_runs(constraint_run_id);
CREATE INDEX idx_action_runs_action_id ON action_runs(action_id);

-- =============================================================================
-- STATISTICS TABLES
-- =============================================================================

-- Scheduler stats table tracks per-period scheduler metrics
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

-- Orchestrator stats table tracks per-run orchestrator metrics
CREATE TABLE orchestrator_stats (
    run_id TEXT PRIMARY KEY,
    stats_period_id TEXT NOT NULL,
    runtime INTEGER NOT NULL,
    constraints_checked INTEGER NOT NULL DEFAULT 0,
    actions_taken INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (stats_period_id) REFERENCES scheduler_stats(stats_period_id) ON DELETE CASCADE
);

CREATE INDEX idx_orchestrator_stats_stats_period_id ON orchestrator_stats(stats_period_id);

-- Syncer stats table tracks per-period Job State Syncer metrics
CREATE TABLE syncer_stats (
    stats_period_id TEXT PRIMARY KEY,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    total_writes INTEGER NOT NULL DEFAULT 0,
    writes_succeeded INTEGER NOT NULL DEFAULT 0,
    writes_failed INTEGER NOT NULL DEFAULT 0,
    avg_writes_in_flight REAL,
    max_writes_in_flight INTEGER,
    min_writes_in_flight INTEGER,
    avg_queued_writes REAL,
    max_queued_writes INTEGER,
    min_queued_writes INTEGER,
    avg_inbox_length REAL,
    max_inbox_length INTEGER,
    min_inbox_length INTEGER,
    avg_time_in_write_queue REAL,
    max_time_in_write_queue INTEGER,
    min_time_in_write_queue INTEGER,
    avg_time_in_inbox REAL,
    max_time_in_inbox INTEGER,
    min_time_in_inbox INTEGER
);

-- Stats collector stats table tracks per-period Stats Collector self-monitoring
CREATE TABLE stats_collector_stats (
    stats_period_id TEXT PRIMARY KEY,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    messages_received INTEGER NOT NULL DEFAULT 0,
    messages_processed INTEGER NOT NULL DEFAULT 0,
    scheduler_messages INTEGER NOT NULL DEFAULT 0,
    orchestrator_messages INTEGER NOT NULL DEFAULT 0,
    syncer_messages INTEGER NOT NULL DEFAULT 0,
    webhook_messages INTEGER NOT NULL DEFAULT 0,
    periods_completed INTEGER NOT NULL DEFAULT 0,
    database_flushes INTEGER NOT NULL DEFAULT 0,
    flush_errors INTEGER NOT NULL DEFAULT 0,
    avg_inbox_length REAL,
    max_inbox_length INTEGER,
    min_inbox_length INTEGER,
    avg_processing_time REAL,
    max_processing_time INTEGER,
    min_processing_time INTEGER
);

-- =============================================================================
-- FUTURE TABLES (Webhook handling)
-- =============================================================================

-- Webhook deliveries table tracks individual webhook delivery attempts
CREATE TABLE webhook_deliveries (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    webhook_type TEXT NOT NULL,
    trigger TEXT NOT NULL,
    url TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    status_code INTEGER,
    success BOOLEAN NOT NULL,
    error TEXT,
    request_duration INTEGER,
    created_at TIMESTAMP NOT NULL,
    delivered_at TIMESTAMP,
    FOREIGN KEY (run_id) REFERENCES job_runs(run_id) ON DELETE CASCADE
);

CREATE INDEX idx_webhook_deliveries_run_id ON webhook_deliveries(run_id);
CREATE INDEX idx_webhook_deliveries_created_at ON webhook_deliveries(created_at);

-- Webhook handler stats table tracks per-period webhook handler metrics
CREATE TABLE webhook_handler_stats (
    stats_period_id TEXT PRIMARY KEY,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    webhooks_sent INTEGER NOT NULL DEFAULT 0,
    webhooks_succeeded INTEGER NOT NULL DEFAULT 0,
    webhooks_failed INTEGER NOT NULL DEFAULT 0,
    total_retries INTEGER NOT NULL DEFAULT 0,
    avg_delivery_time REAL,
    max_delivery_time INTEGER,
    min_delivery_time INTEGER,
    avg_inbox_length REAL,
    max_inbox_length INTEGER,
    min_inbox_length INTEGER
);
