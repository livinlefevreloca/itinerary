-- +migrate Up

-- Create dimension tables for constraint and action types
CREATE TABLE constraint_types (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE action_types (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

-- Seed dimension tables with built-in types
-- IDs 1-8 reserved for built-in constraint types
INSERT INTO constraint_types (id, name) VALUES
    (1, 'maxConcurrentRuns'),
    (2, 'catchUp'),
    (3, 'preRunHook'),
    (4, 'postRunHook'),
    (5, 'catchUpWindow'),
    (6, 'maxExpectedRunTime'),
    (7, 'maxAllowedRunTime'),
    (8, 'requirePreviousSuccess');

-- IDs 1-6 reserved for built-in action types
INSERT INTO action_types (id, name) VALUES
    (1, 'retry'),
    (2, 'kickOffJob'),
    (3, 'webhook'),
    (4, 'killAllInstances'),
    (5, 'killLatestInstance'),
    (6, 'skipNextInstance');

-- Create jobs table
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    schedule TEXT NOT NULL,
    pod_spec TEXT,
    constraints TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create job_actions table
CREATE TABLE job_actions (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    action_type_id INTEGER NOT NULL,
    trigger TEXT NOT NULL,
    constraint_type_id INTEGER,
    config TEXT,
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,
    FOREIGN KEY (action_type_id) REFERENCES action_types(id) ON DELETE CASCADE,
    FOREIGN KEY (constraint_type_id) REFERENCES constraint_types(id) ON DELETE CASCADE
);

CREATE INDEX idx_job_actions_job_id ON job_actions(job_id);
CREATE INDEX idx_job_actions_action_type_id ON job_actions(action_type_id);
CREATE INDEX idx_job_actions_constraint_type_id ON job_actions(constraint_type_id);

-- Create job_runs table
CREATE TABLE job_runs (
    job_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    scheduled_at TIMESTAMP NOT NULL,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    status TEXT NOT NULL,
    success BOOLEAN,
    error TEXT,
    PRIMARY KEY (job_id, scheduled_at),
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_job_runs_run_id ON job_runs(run_id);
CREATE INDEX idx_job_runs_job_id ON job_runs(job_id);

-- Create constraint_violations table
CREATE TABLE constraint_violations (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    constraint_type_id INTEGER NOT NULL,
    violation_time TIMESTAMP NOT NULL,
    action_taken TEXT,
    details TEXT,
    FOREIGN KEY (run_id) REFERENCES job_runs(run_id) ON DELETE CASCADE,
    FOREIGN KEY (constraint_type_id) REFERENCES constraint_types(id) ON DELETE CASCADE
);

CREATE INDEX idx_constraint_violations_run_id ON constraint_violations(run_id);
CREATE INDEX idx_constraint_violations_constraint_type_id ON constraint_violations(constraint_type_id);

-- Create scheduler_stats table
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

-- Create orchestrator_stats table
CREATE TABLE orchestrator_stats (
    run_id TEXT PRIMARY KEY,
    stats_period_id TEXT NOT NULL,
    runtime INTEGER NOT NULL,
    constraints_checked INTEGER NOT NULL DEFAULT 0,
    actions_taken INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (stats_period_id) REFERENCES scheduler_stats(stats_period_id) ON DELETE CASCADE
);

CREATE INDEX idx_orchestrator_stats_stats_period_id ON orchestrator_stats(stats_period_id);
