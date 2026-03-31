package db

import (
	"database/sql"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/model"
)

// =============================================================================
// Action Type Operations (Read-only dimension table)
// =============================================================================

// GetActionType retrieves an action type by ID
func (db *DB) GetActionType(id int) (*model.ActionType, error) {
	at := &model.ActionType{}

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
func (db *DB) GetAllActionTypes() ([]model.ActionType, error) {
	query := `SELECT id, name FROM action_types ORDER BY id`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []model.ActionType
	for rows.Next() {
		var at model.ActionType
		if err := rows.Scan(&at.ID, &at.Name); err != nil {
			return nil, err
		}
		types = append(types, at)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if types == nil {
		types = []model.ActionType{}
	}

	return types, nil
}

// =============================================================================
// Action Operations
// =============================================================================

// CreateAction creates a new action for a constraint
func (db *DB) CreateAction(action *model.ActionConfig) error {
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
func (db *DB) GetAction(id string) (*model.ActionConfig, error) {
	action := &model.ActionConfig{}

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
func (db *DB) GetActionsByConstraint(constraintID string) ([]model.ActionConfig, error) {
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

	var actions []model.ActionConfig
	for rows.Next() {
		var a model.ActionConfig
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
		actions = []model.ActionConfig{}
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
// Action Run Operations
// =============================================================================

// CreateActionRun records an action execution
func (db *DB) CreateActionRun(actionRun *model.ActionRun) error {
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
func (db *DB) GetActionRuns(runID string) ([]model.ActionRun, error) {
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

	var actionRuns []model.ActionRun
	for rows.Next() {
		var ar model.ActionRun
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
		actionRuns = []model.ActionRun{}
	}

	return actionRuns, nil
}

// GetActionRunsByAction retrieves action runs for a specific action
func (db *DB) GetActionRunsByAction(actionID string, limit int) ([]model.ActionRun, error) {
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

	var actionRuns []model.ActionRun
	for rows.Next() {
		var ar model.ActionRun
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
		actionRuns = []model.ActionRun{}
	}

	return actionRuns, nil
}
