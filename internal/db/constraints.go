package db

import (
	"database/sql"
	"time"
)

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
