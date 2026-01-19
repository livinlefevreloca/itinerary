-- +migrate Up

-- Create action_runs table to track action executions
CREATE TABLE action_runs (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    action_type_id INTEGER NOT NULL,
    trigger TEXT NOT NULL,
    constraint_violation_id TEXT,
    executed_at TIMESTAMP NOT NULL,
    success BOOLEAN NOT NULL,
    error TEXT,
    details TEXT,
    FOREIGN KEY (run_id) REFERENCES job_runs(run_id) ON DELETE CASCADE,
    FOREIGN KEY (action_type_id) REFERENCES action_types(id) ON DELETE CASCADE,
    FOREIGN KEY (constraint_violation_id) REFERENCES constraint_violations(id) ON DELETE SET NULL
);

CREATE INDEX idx_action_runs_run_id ON action_runs(run_id);
CREATE INDEX idx_action_runs_action_type_id ON action_runs(action_type_id);
CREATE INDEX idx_action_runs_constraint_violation_id ON action_runs(constraint_violation_id);

-- Remove action_taken column from constraint_violations
-- SQLite doesn't support DROP COLUMN, so we need to recreate the table
CREATE TABLE constraint_violations_new (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    constraint_type_id INTEGER NOT NULL,
    violation_time TIMESTAMP NOT NULL,
    details TEXT,
    FOREIGN KEY (run_id) REFERENCES job_runs(run_id) ON DELETE CASCADE,
    FOREIGN KEY (constraint_type_id) REFERENCES constraint_types(id) ON DELETE CASCADE
);

-- Copy data from old table
INSERT INTO constraint_violations_new (id, run_id, constraint_type_id, violation_time, details)
SELECT id, run_id, constraint_type_id, violation_time, details FROM constraint_violations;

-- Drop old table and rename new one
DROP TABLE constraint_violations;
ALTER TABLE constraint_violations_new RENAME TO constraint_violations;

-- Recreate indexes
CREATE INDEX idx_constraint_violations_run_id ON constraint_violations(run_id);
CREATE INDEX idx_constraint_violations_constraint_type_id ON constraint_violations(constraint_type_id);

-- Create syncer_stats table
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

CREATE INDEX idx_syncer_stats_start_time ON syncer_stats(start_time);
CREATE INDEX idx_syncer_stats_end_time ON syncer_stats(end_time);

-- Create stats_collector_stats table
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

CREATE INDEX idx_stats_collector_stats_start_time ON stats_collector_stats(start_time);
CREATE INDEX idx_stats_collector_stats_end_time ON stats_collector_stats(end_time);

-- Create webhook_deliveries table (future)
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

-- Create webhook_handler_stats table (future)
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

CREATE INDEX idx_webhook_handler_stats_start_time ON webhook_handler_stats(start_time);
CREATE INDEX idx_webhook_handler_stats_end_time ON webhook_handler_stats(end_time);

-- Add indexes for scheduler_stats time columns
CREATE INDEX idx_scheduler_stats_start_time ON scheduler_stats(start_time);
CREATE INDEX idx_scheduler_stats_end_time ON scheduler_stats(end_time);
