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
		INSERT INTO jobs (id, name, schedule, pod_spec, constraints, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query, job.ID, job.Name, job.Schedule, job.PodSpec, job.Constraints, job.CreatedAt, job.UpdatedAt)
	return err
}

// CreateJob creates a new job within a transaction
func (tx *Tx) CreateJob(job *Job) error {
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now

	query := `
		INSERT INTO jobs (id, name, schedule, pod_spec, constraints, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := tx.Exec(query, job.ID, job.Name, job.Schedule, job.PodSpec, job.Constraints, job.CreatedAt, job.UpdatedAt)
	return err
}

// GetJob retrieves a job by ID
func (db *DB) GetJob(id string) (*Job, error) {
	job := &Job{}

	query := `
		SELECT id, name, schedule, pod_spec, constraints, created_at, updated_at
		FROM jobs
		WHERE id = ?
	`

	err := db.QueryRow(query, id).Scan(
		&job.ID,
		&job.Name,
		&job.Schedule,
		&job.PodSpec,
		&job.Constraints,
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
		SELECT id, name, schedule, pod_spec, constraints, created_at, updated_at
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
			&job.Constraints,
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
		SET name = ?, schedule = ?, pod_spec = ?, constraints = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := db.Exec(query, job.Name, job.Schedule, job.PodSpec, job.Constraints, job.UpdatedAt, job.ID)
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

// =============================================================================
// Constraint Type Operations (Read-only dimension table)
// =============================================================================

// GetConstraintType retrieves a constraint type by ID
func (db *DB) GetConstraintType(id int) (*ConstraintType, error) {
	ct := &ConstraintType{}

	query := `SELECT id, name FROM constraint_types WHERE id = ?`

	err := db.QueryRow(query, id).Scan(&ct.ID, &ct.Name)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	return ct, nil
}

// GetAllConstraintTypes retrieves all constraint types
func (db *DB) GetAllConstraintTypes() ([]ConstraintType, error) {
	query := `SELECT id, name FROM constraint_types ORDER BY id`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []ConstraintType
	for rows.Next() {
		var ct ConstraintType
		if err := rows.Scan(&ct.ID, &ct.Name); err != nil {
			return nil, err
		}
		types = append(types, ct)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if types == nil {
		types = []ConstraintType{}
	}

	return types, nil
}

// =============================================================================
// Action Type Operations (Read-only dimension table)
// =============================================================================

// GetActionType retrieves an action type by ID
func (db *DB) GetActionType(id int) (*ActionType, error) {
	at := &ActionType{}

	query := `SELECT id, name FROM action_types WHERE id = ?`

	err := db.QueryRow(query, id).Scan(&at.ID, &at.Name)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	return at, nil
}

// GetAllActionTypes retrieves all action types
func (db *DB) GetAllActionTypes() ([]ActionType, error) {
	query := `SELECT id, name FROM action_types ORDER BY id`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []ActionType
	for rows.Next() {
		var at ActionType
		if err := rows.Scan(&at.ID, &at.Name); err != nil {
			return nil, err
		}
		types = append(types, at)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if types == nil {
		types = []ActionType{}
	}

	return types, nil
}

// =============================================================================
// Job Action Operations
// =============================================================================

// CreateJobAction creates a new job action
func (db *DB) CreateJobAction(jobAction *JobAction) error {
	query := `
		INSERT INTO job_actions (id, job_id, action_type_id, trigger, constraint_type_id, config)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		jobAction.ID,
		jobAction.JobID,
		jobAction.ActionTypeID,
		jobAction.Trigger,
		jobAction.ConstraintTypeID,
		jobAction.Config,
	)
	return err
}

// GetJobActions retrieves all actions for a job
func (db *DB) GetJobActions(jobID string) ([]JobAction, error) {
	query := `
		SELECT id, job_id, action_type_id, trigger, constraint_type_id, config
		FROM job_actions
		WHERE job_id = ?
		ORDER BY id
	`

	rows, err := db.Query(query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobActions []JobAction
	for rows.Next() {
		var ja JobAction
		err := rows.Scan(
			&ja.ID,
			&ja.JobID,
			&ja.ActionTypeID,
			&ja.Trigger,
			&ja.ConstraintTypeID,
			&ja.Config,
		)
		if err != nil {
			return nil, err
		}
		jobActions = append(jobActions, ja)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if jobActions == nil {
		jobActions = []JobAction{}
	}

	return jobActions, nil
}

// DeleteJobAction removes a job action by ID
func (db *DB) DeleteJobAction(id string) error {
	query := `DELETE FROM job_actions WHERE id = ?`

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

// =============================================================================
// Constraint Violation Operations
// =============================================================================

// CreateConstraintViolation records a constraint violation
func (db *DB) CreateConstraintViolation(violation *ConstraintViolation) error {
	query := `
		INSERT INTO constraint_violations (id, run_id, constraint_type_id, violation_time, action_taken, details)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		violation.ID,
		violation.RunID,
		violation.ConstraintTypeID,
		violation.ViolationTime,
		violation.ActionTaken,
		violation.Details,
	)
	return err
}

// GetConstraintViolations retrieves all violations for a run
func (db *DB) GetConstraintViolations(runID string) ([]ConstraintViolation, error) {
	query := `
		SELECT id, run_id, constraint_type_id, violation_time, action_taken, details
		FROM constraint_violations
		WHERE run_id = ?
		ORDER BY violation_time DESC
	`

	rows, err := db.Query(query, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var violations []ConstraintViolation
	for rows.Next() {
		var v ConstraintViolation
		err := rows.Scan(
			&v.ID,
			&v.RunID,
			&v.ConstraintTypeID,
			&v.ViolationTime,
			&v.ActionTaken,
			&v.Details,
		)
		if err != nil {
			return nil, err
		}
		violations = append(violations, v)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if violations == nil {
		violations = []ConstraintViolation{}
	}

	return violations, nil
}

// GetConstraintViolationsByType retrieves violations for a specific constraint type
func (db *DB) GetConstraintViolationsByType(constraintTypeID int, limit int) ([]ConstraintViolation, error) {
	query := `
		SELECT id, run_id, constraint_type_id, violation_time, action_taken, details
		FROM constraint_violations
		WHERE constraint_type_id = ?
		ORDER BY violation_time DESC
		LIMIT ?
	`

	rows, err := db.Query(query, constraintTypeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var violations []ConstraintViolation
	for rows.Next() {
		var v ConstraintViolation
		err := rows.Scan(
			&v.ID,
			&v.RunID,
			&v.ConstraintTypeID,
			&v.ViolationTime,
			&v.ActionTaken,
			&v.Details,
		)
		if err != nil {
			return nil, err
		}
		violations = append(violations, v)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if violations == nil {
		violations = []ConstraintViolation{}
	}

	return violations, nil
}
