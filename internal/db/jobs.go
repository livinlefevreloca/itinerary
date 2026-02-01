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

// CreateJob creates a new job within a transaction
func (tx *Tx) CreateJob(job *Job) error {
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now

	query := `
		INSERT INTO jobs (id, name, schedule, pod_spec, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := tx.Exec(query, job.ID, job.Name, job.Schedule, job.PodSpec, job.CreatedAt, job.UpdatedAt)
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
// Constraint Operations
// =============================================================================

// CreateConstraint creates a new constraint for a job
func (db *DB) CreateConstraint(constraint *Constraint) error {
	now := time.Now()
	constraint.CreatedAt = now

	query := `
		INSERT INTO constraints (id, job_id, constraint_type_id, config, created_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		constraint.ID,
		constraint.JobID,
		constraint.ConstraintTypeID,
		constraint.Config,
		constraint.CreatedAt,
	)
	return err
}

// GetConstraint retrieves a constraint by ID
func (db *DB) GetConstraint(id string) (*Constraint, error) {
	constraint := &Constraint{}

	query := `SELECT id, job_id, constraint_type_id, config, created_at FROM constraints WHERE id = ?`

	err := db.QueryRow(query, id).Scan(
		&constraint.ID,
		&constraint.JobID,
		&constraint.ConstraintTypeID,
		&constraint.Config,
		&constraint.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	return constraint, nil
}

// GetConstraintsByJob retrieves all constraints for a job
func (db *DB) GetConstraintsByJob(jobID string) ([]Constraint, error) {
	query := `
		SELECT id, job_id, constraint_type_id, config, created_at
		FROM constraints
		WHERE job_id = ?
		ORDER BY created_at
	`

	rows, err := db.Query(query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []Constraint
	for rows.Next() {
		var c Constraint
		err := rows.Scan(&c.ID, &c.JobID, &c.ConstraintTypeID, &c.Config, &c.CreatedAt)
		if err != nil {
			return nil, err
		}
		constraints = append(constraints, c)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if constraints == nil {
		constraints = []Constraint{}
	}

	return constraints, nil
}

// DeleteConstraint removes a constraint by ID
func (db *DB) DeleteConstraint(id string) error {
	query := `DELETE FROM constraints WHERE id = ?`

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
// Action Operations
// =============================================================================

// CreateAction creates a new action for a constraint
func (db *DB) CreateAction(action *Action) error {
	now := time.Now()
	action.CreatedAt = now

	query := `
		INSERT INTO actions (id, constraint_id, action_type_id, trigger, config, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		action.ID,
		action.ConstraintID,
		action.ActionTypeID,
		action.Trigger,
		action.Config,
		action.CreatedAt,
	)
	return err
}

// GetAction retrieves an action by ID
func (db *DB) GetAction(id string) (*Action, error) {
	action := &Action{}

	query := `SELECT id, constraint_id, action_type_id, trigger, config, created_at FROM actions WHERE id = ?`

	err := db.QueryRow(query, id).Scan(
		&action.ID,
		&action.ConstraintID,
		&action.ActionTypeID,
		&action.Trigger,
		&action.Config,
		&action.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	return action, nil
}

// GetActionsByConstraint retrieves all actions for a constraint
func (db *DB) GetActionsByConstraint(constraintID string) ([]Action, error) {
	query := `
		SELECT id, constraint_id, action_type_id, trigger, config, created_at
		FROM actions
		WHERE constraint_id = ?
		ORDER BY created_at
	`

	rows, err := db.Query(query, constraintID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []Action
	for rows.Next() {
		var a Action
		err := rows.Scan(&a.ID, &a.ConstraintID, &a.ActionTypeID, &a.Trigger, &a.Config, &a.CreatedAt)
		if err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if actions == nil {
		actions = []Action{}
	}

	return actions, nil
}

// DeleteAction removes an action by ID
func (db *DB) DeleteAction(id string) error {
	query := `DELETE FROM actions WHERE id = ?`

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
// Constraint Run Operations
// =============================================================================

// CreateConstraintRun records a constraint check execution
func (db *DB) CreateConstraintRun(constraintRun *ConstraintRun) error {
	query := `
		INSERT INTO constraint_runs (id, run_id, constraint_id, executed_at, success, violated, in_error, error, details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		constraintRun.ID,
		constraintRun.RunID,
		constraintRun.ConstraintID,
		constraintRun.ExecutedAt,
		constraintRun.Success,
		constraintRun.Violated,
		constraintRun.InError,
		constraintRun.Error,
		constraintRun.Details,
	)
	return err
}

// GetConstraintRuns retrieves all constraint runs for a job run
func (db *DB) GetConstraintRuns(runID string) ([]ConstraintRun, error) {
	query := `
		SELECT id, run_id, constraint_id, executed_at, success, violated, in_error, error, details
		FROM constraint_runs
		WHERE run_id = ?
		ORDER BY executed_at DESC
	`

	rows, err := db.Query(query, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraintRuns []ConstraintRun
	for rows.Next() {
		var cr ConstraintRun
		err := rows.Scan(
			&cr.ID,
			&cr.RunID,
			&cr.ConstraintID,
			&cr.ExecutedAt,
			&cr.Success,
			&cr.Violated,
			&cr.InError,
			&cr.Error,
			&cr.Details,
		)
		if err != nil {
			return nil, err
		}
		constraintRuns = append(constraintRuns, cr)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if constraintRuns == nil {
		constraintRuns = []ConstraintRun{}
	}

	return constraintRuns, nil
}

// GetConstraintRunsByConstraint retrieves constraint runs for a specific constraint
func (db *DB) GetConstraintRunsByConstraint(constraintID string, limit int) ([]ConstraintRun, error) {
	query := `
		SELECT id, run_id, constraint_id, executed_at, success, violated, in_error, error, details
		FROM constraint_runs
		WHERE constraint_id = ?
		ORDER BY executed_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, constraintID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraintRuns []ConstraintRun
	for rows.Next() {
		var cr ConstraintRun
		err := rows.Scan(
			&cr.ID,
			&cr.RunID,
			&cr.ConstraintID,
			&cr.ExecutedAt,
			&cr.Success,
			&cr.Violated,
			&cr.InError,
			&cr.Error,
			&cr.Details,
		)
		if err != nil {
			return nil, err
		}
		constraintRuns = append(constraintRuns, cr)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if constraintRuns == nil {
		constraintRuns = []ConstraintRun{}
	}

	return constraintRuns, nil
}
// =============================================================================
// Action Run Operations
// =============================================================================

// CreateActionRun records an action execution
func (db *DB) CreateActionRun(actionRun *ActionRun) error {
	query := `
		INSERT INTO action_runs (id, run_id, constraint_run_id, action_id, executed_at, success, error, details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		actionRun.ID,
		actionRun.RunID,
		actionRun.ConstraintRunID,
		actionRun.ActionID,
		actionRun.ExecutedAt,
		actionRun.Success,
		actionRun.Error,
		actionRun.Details,
	)
	return err
}

// GetActionRuns retrieves all action runs for a run
func (db *DB) GetActionRuns(runID string) ([]ActionRun, error) {
	query := `
		SELECT id, run_id, constraint_run_id, action_id, executed_at, success, error, details
		FROM action_runs
		WHERE run_id = ?
		ORDER BY executed_at DESC
	`

	rows, err := db.Query(query, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actionRuns []ActionRun
	for rows.Next() {
		var ar ActionRun
		err := rows.Scan(
			&ar.ID,
			&ar.RunID,
			&ar.ConstraintRunID,
			&ar.ActionID,
			&ar.ExecutedAt,
			&ar.Success,
			&ar.Error,
			&ar.Details,
		)
		if err != nil {
			return nil, err
		}
		actionRuns = append(actionRuns, ar)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if actionRuns == nil {
		actionRuns = []ActionRun{}
	}

	return actionRuns, nil
}

// GetActionRunsByAction retrieves action runs for a specific action
func (db *DB) GetActionRunsByAction(actionID string, limit int) ([]ActionRun, error) {
	query := `
		SELECT id, run_id, constraint_run_id, action_id, executed_at, success, error, details
		FROM action_runs
		WHERE action_id = ?
		ORDER BY executed_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, actionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actionRuns []ActionRun
	for rows.Next() {
		var ar ActionRun
		err := rows.Scan(
			&ar.ID,
			&ar.RunID,
			&ar.ConstraintRunID,
			&ar.ActionID,
			&ar.ExecutedAt,
			&ar.Success,
			&ar.Error,
			&ar.Details,
		)
		if err != nil {
			return nil, err
		}
		actionRuns = append(actionRuns, ar)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if actionRuns == nil {
		actionRuns = []ActionRun{}
	}

	return actionRuns, nil
}

