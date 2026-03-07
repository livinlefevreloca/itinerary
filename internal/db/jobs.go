package db

import (
	"database/sql"
	"time"
)

// =============================================================================
// Job Operations
// =============================================================================

// CreateJob creates a new job
func (db *DB) CreateJob(job *Job) error {
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now

	query := `
		INSERT INTO jobs (id, name, schedule, pod_spec, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query, job.ID, job.Name, job.Schedule, job.PodSpec, job.CreatedAt, job.UpdatedAt)
	return err
}

// GetJob retrieves a job by ID
func (db *DB) GetJob(id string) (*Job, error) {
	job := &Job{}

	query := `
		SELECT id, name, schedule, pod_spec, created_at, updated_at
		FROM jobs
		WHERE id = ?
	`

	err := db.QueryRow(query, id).Scan(
		&job.ID,
		&job.Name,
		&job.Schedule,
		&job.PodSpec,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	return job, nil
}

// GetAllJobs retrieves all jobs
func (db *DB) GetAllJobs() ([]Job, error) {
	query := `
		SELECT id, name, schedule, pod_spec, created_at, updated_at
		FROM jobs
		ORDER BY created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		err := rows.Scan(
			&job.ID,
			&job.Name,
			&job.Schedule,
			&job.PodSpec,
			&job.CreatedAt,
			&job.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Return empty slice instead of nil
	if jobs == nil {
		jobs = []Job{}
	}

	return jobs, nil
}

// UpdateJob updates an existing job
func (db *DB) UpdateJob(job *Job) error {
	job.UpdatedAt = time.Now()

	query := `
		UPDATE jobs
		SET name = ?, schedule = ?, pod_spec = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := db.Exec(query, job.Name, job.Schedule, job.PodSpec, job.UpdatedAt, job.ID)
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

// DeleteJob deletes a job by ID
func (db *DB) DeleteJob(id string) error {
	query := `DELETE FROM jobs WHERE id = ?`

	result, err := db.Exec(query, id)
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
