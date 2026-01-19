package db

import (
	"database/sql"
	"time"
)

// CreateJobRun creates a new job run record
func (db *DB) CreateJobRun(run *JobRun) error {
	query := `
		INSERT INTO job_runs (job_id, run_id, scheduled_at, started_at, completed_at, status, success, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		run.JobID,
		run.RunID,
		run.ScheduledAt,
		run.StartedAt,
		run.CompletedAt,
		run.Status,
		run.Success,
		run.Error,
	)

	return err
}

// GetJobRun retrieves a job run by job ID and scheduled time
func (db *DB) GetJobRun(jobID string, scheduledAt time.Time) (*JobRun, error) {
	run := &JobRun{}

	query := `
		SELECT job_id, run_id, scheduled_at, started_at, completed_at, status, success, error
		FROM job_runs
		WHERE job_id = ? AND scheduled_at = ?
	`

	err := db.QueryRow(query, jobID, scheduledAt).Scan(
		&run.JobID,
		&run.RunID,
		&run.ScheduledAt,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Status,
		&run.Success,
		&run.Error,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	return run, nil
}

// GetJobRunByRunID retrieves a job run by its run ID
func (db *DB) GetJobRunByRunID(runID string) (*JobRun, error) {
	run := &JobRun{}

	query := `
		SELECT job_id, run_id, scheduled_at, started_at, completed_at, status, success, error
		FROM job_runs
		WHERE run_id = ?
	`

	err := db.QueryRow(query, runID).Scan(
		&run.JobID,
		&run.RunID,
		&run.ScheduledAt,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Status,
		&run.Success,
		&run.Error,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	return run, nil
}

// GetJobRuns retrieves all runs for a job
func (db *DB) GetJobRuns(jobID string, limit int) ([]JobRun, error) {
	query := `
		SELECT job_id, run_id, scheduled_at, started_at, completed_at, status, success, error
		FROM job_runs
		WHERE job_id = ?
		ORDER BY scheduled_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []JobRun
	for rows.Next() {
		var run JobRun
		err := rows.Scan(
			&run.JobID,
			&run.RunID,
			&run.ScheduledAt,
			&run.StartedAt,
			&run.CompletedAt,
			&run.Status,
			&run.Success,
			&run.Error,
		)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Return empty slice instead of nil
	if runs == nil {
		runs = []JobRun{}
	}

	return runs, nil
}

// UpdateJobRunStatus updates the status of a job run
func (db *DB) UpdateJobRunStatus(runID string, status string) error {
	query := `
		UPDATE job_runs
		SET status = ?
		WHERE run_id = ?
	`

	result, err := db.Exec(query, status, runID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// CompleteJobRun marks a job run as completed
func (db *DB) CompleteJobRun(runID string, success bool, errorMsg *string) error {
	now := time.Now()
	status := "completed"
	if !success {
		status = "failed"
	}

	query := `
		UPDATE job_runs
		SET status = ?, completed_at = ?, success = ?, error = ?
		WHERE run_id = ?
	`

	result, err := db.Exec(query, status, now, success, errorMsg, runID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
