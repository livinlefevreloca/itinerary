package db

// initTestSchema creates all tables in the test database
func initTestSchema(db *DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS constraint_types (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE
		);

		CREATE TABLE IF NOT EXISTS action_types (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE
		);

		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			schedule TEXT NOT NULL,
			pod_spec TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS constraints (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			constraint_type_id INTEGER NOT NULL,
			config TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,
			FOREIGN KEY (constraint_type_id) REFERENCES constraint_types(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS actions (
			id TEXT PRIMARY KEY,
			constraint_id TEXT NOT NULL,
			action_type_id INTEGER NOT NULL,
			trigger TEXT NOT NULL,
			config TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (constraint_id) REFERENCES constraints(id) ON DELETE CASCADE,
			FOREIGN KEY (action_type_id) REFERENCES action_types(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS job_runs (
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

		CREATE UNIQUE INDEX IF NOT EXISTS idx_job_runs_run_id ON job_runs(run_id);
		CREATE INDEX IF NOT EXISTS idx_job_runs_job_id ON job_runs(job_id);
		CREATE INDEX IF NOT EXISTS idx_constraints_job_id ON constraints(job_id);
		CREATE INDEX IF NOT EXISTS idx_actions_constraint_id ON actions(constraint_id);

		CREATE TABLE IF NOT EXISTS constraint_runs (
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

		CREATE INDEX IF NOT EXISTS idx_constraint_runs_run_id ON constraint_runs(run_id);
		CREATE INDEX IF NOT EXISTS idx_constraint_runs_constraint_id ON constraint_runs(constraint_id);

		CREATE TABLE IF NOT EXISTS action_runs (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			constraint_run_id TEXT,
			action_id TEXT NOT NULL,
			executed_at TIMESTAMP NOT NULL,
			success BOOLEAN NOT NULL,
			error TEXT,
			details TEXT,
			FOREIGN KEY (run_id) REFERENCES job_runs(run_id) ON DELETE CASCADE,
			FOREIGN KEY (constraint_run_id) REFERENCES constraint_runs(id) ON DELETE CASCADE,
			FOREIGN KEY (action_id) REFERENCES actions(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_action_runs_run_id ON action_runs(run_id);
		CREATE INDEX IF NOT EXISTS idx_action_runs_constraint_run_id ON action_runs(constraint_run_id);
		CREATE INDEX IF NOT EXISTS idx_action_runs_action_id ON action_runs(action_id);

		CREATE TABLE IF NOT EXISTS scheduler_stats (
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

		CREATE TABLE IF NOT EXISTS orchestrator_stats (
			run_id TEXT PRIMARY KEY,
			stats_period_id TEXT NOT NULL,
			runtime INTEGER NOT NULL,
			constraints_checked INTEGER NOT NULL DEFAULT 0,
			actions_taken INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (stats_period_id) REFERENCES scheduler_stats(stats_period_id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_orchestrator_stats_stats_period_id ON orchestrator_stats(stats_period_id);

		-- Seed dimension tables with built-in types
		INSERT OR IGNORE INTO constraint_types (id, name) VALUES
			(1, 'maxConcurrentRuns'),
			(2, 'catchUp'),
			(3, 'preRunHook'),
			(4, 'postRunHook'),
			(5, 'catchUpWindow'),
			(6, 'maxExpectedRunTime'),
			(7, 'maxAllowedRunTime'),
			(8, 'requirePreviousSuccess');

		INSERT OR IGNORE INTO action_types (id, name) VALUES
			(1, 'retry'),
			(2, 'kickOffJob'),
			(3, 'webhook'),
			(4, 'killAllInstances'),
			(5, 'killLatestInstance'),
			(6, 'skipNextInstance');
	`

	_, err := db.Exec(schema)
	return err
}
