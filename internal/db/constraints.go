package db

import (
	"database/sql"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/model"
)

// =============================================================================
// Constraint Type Operations (Read-only dimension table)
// =============================================================================

// GetConstraintType retrieves a constraint type by ID
func (db *DB) GetConstraintType(id int) (*model.ConstraintType, error) {
	ct := &model.ConstraintType{}

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
func (db *DB) GetAllConstraintTypes() ([]model.ConstraintType, error) {
	query := `SELECT id, name FROM constraint_types ORDER BY id`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []model.ConstraintType
	for rows.Next() {
		var ct model.ConstraintType
		if err := rows.Scan(&ct.ID, &ct.Name); err != nil {
			return nil, err
		}
		types = append(types, ct)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if types == nil {
		types = []model.ConstraintType{}
	}

	return types, nil
}

// =============================================================================
// Constraint Operations
// =============================================================================

// CreateConstraint creates a new constraint for a job
func (db *DB) CreateConstraint(constraint *model.ConstraintConfig) error {
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
func (db *DB) GetConstraint(id string) (*model.ConstraintConfig, error) {
	constraint := &model.ConstraintConfig{}

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
func (db *DB) GetConstraintsByJob(jobID string) ([]model.ConstraintConfig, error) {
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

	var constraints []model.ConstraintConfig
	for rows.Next() {
		var c model.ConstraintConfig
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
		constraints = []model.ConstraintConfig{}
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
func (db *DB) CreateConstraintRun(constraintRun *model.ConstraintRun) error {
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
func (db *DB) GetConstraintRuns(runID string) ([]model.ConstraintRun, error) {
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

	var constraintRuns []model.ConstraintRun
	for rows.Next() {
		var cr model.ConstraintRun
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
		constraintRuns = []model.ConstraintRun{}
	}

	return constraintRuns, nil
}

// GetConstraintRunsByConstraint retrieves constraint runs for a specific constraint
func (db *DB) GetConstraintRunsByConstraint(constraintID string, limit int) ([]model.ConstraintRun, error) {
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

	var constraintRuns []model.ConstraintRun
	for rows.Next() {
		var cr model.ConstraintRun
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
		constraintRuns = []model.ConstraintRun{}
	}

	return constraintRuns, nil
}
