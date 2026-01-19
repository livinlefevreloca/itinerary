package migrator

import (
	"database/sql"
	"fmt"
	"strings"
)

// RunMigrations applies all pending migrations from the specified directory.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	// Detect driver
	driver := detectDriver(db)

	// Create schema_migrations table if not exists
	if err := createSchemaTable(db); err != nil {
		return fmt.Errorf("failed to create schema table: %w", err)
	}

	// Acquire lock
	if err := acquireLock(db, driver); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer releaseLock(db, driver)

	// Load all migrations
	migrations, err := LoadMigrations(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Get already applied migrations
	applied, err := GetAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Build set of applied versions
	appliedSet := make(map[int]bool)
	for _, v := range applied {
		appliedSet[v] = true
	}

	// Filter to pending migrations
	var pending []Migration
	for _, m := range migrations {
		if !appliedSet[m.Version] {
			pending = append(pending, m)
		}
	}

	// Validate migration history consistency:
	// If any migrations are already applied, check that we're not trying to
	// apply migrations with lower version numbers (can't go backwards)
	if len(applied) > 0 && len(pending) > 0 {
		maxApplied := 0
		for _, v := range applied {
			if v > maxApplied {
				maxApplied = v
			}
		}

		// Build set of migrations that can't be applied (version < maxApplied)
		cannotApply := make(map[int]bool)
		for _, m := range pending {
			if m.Version < maxApplied {
				cannotApply[m.Version] = true
			}
		}

		// Check if any pending migration depends on a migration that can't be applied
		if len(cannotApply) > 0 {
			for _, m := range pending {
				for _, dep := range m.Dependencies {
					if cannotApply[dep] || (!appliedSet[dep] && dep < maxApplied) {
						return fmt.Errorf("migration %d depends on version %d which has not been applied", m.Version, dep)
					}
				}
			}
			// If no dependency errors found, report the version ordering error
			for v := range cannotApply {
				return fmt.Errorf("cannot apply migration %d: version %d is already applied (migrations must be applied in order)", v, maxApplied)
			}
		}
	}

	// Apply each pending migration
	// Dependencies are validated during application - each migration's dependencies
	// must be applied before we try to apply it
	for _, migration := range pending {
		// Validate dependencies for this migration
		for _, dep := range migration.Dependencies {
			if !appliedSet[dep] {
				return fmt.Errorf("migration %d depends on version %d which has not been applied", migration.Version, dep)
			}
		}

		// Apply the migration
		if err := applyMigration(db, driver, migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}

		// Mark as applied for subsequent dependency checks
		appliedSet[migration.Version] = true
	}

	return nil
}

// GetCurrentVersion returns the highest applied migration version.
// Returns 0 if no migrations have been applied.
func GetCurrentVersion(db *sql.DB) (int, error) {
	var version int
	query := "SELECT COALESCE(MAX(version), 0) FROM schema_migrations"

	err := db.QueryRow(query).Scan(&version)
	if err != nil {
		// If table doesn't exist, return 0
		if strings.Contains(err.Error(), "no such table") ||
		   strings.Contains(err.Error(), "doesn't exist") ||
		   strings.Contains(err.Error(), "does not exist") {
			return 0, nil
		}
		return 0, err
	}

	return version, nil
}

// GetAppliedMigrations returns a slice of all applied migration versions, sorted.
func GetAppliedMigrations(db *sql.DB) ([]int, error) {
	query := "SELECT version FROM schema_migrations ORDER BY version"

	rows, err := db.Query(query)
	if err != nil {
		// If table doesn't exist, return empty slice
		if strings.Contains(err.Error(), "no such table") ||
		   strings.Contains(err.Error(), "doesn't exist") ||
		   strings.Contains(err.Error(), "does not exist") {
			return []int{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return versions, nil
}

// validateDependencies checks that all dependencies for pending migrations are satisfied.
func validateDependencies(migrations []Migration, applied []int) error {
	appliedSet := make(map[int]bool)
	for _, v := range applied {
		appliedSet[v] = true
	}

	for _, m := range migrations {
		for _, dep := range m.Dependencies {
			if !appliedSet[dep] {
				return fmt.Errorf("migration %d depends on version %d which has not been applied", m.Version, dep)
			}
		}
	}

	return nil
}

// createSchemaTable creates the schema_migrations table if it doesn't exist.
func createSchemaTable(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := db.Exec(query)
	return err
}

// applyMigration executes a single migration and records it in schema_migrations.
func applyMigration(db *sql.DB, driver string, migration Migration) error {
	if migration.NoTransaction {
		// Execute without transaction
		if _, err := db.Exec(migration.UpSQL); err != nil {
			return fmt.Errorf("failed to execute SQL: %w", err)
		}

		// Record migration
		recordQuery := "INSERT INTO schema_migrations (version) VALUES (" + placeholder(driver, 1) + ")"
		if _, err := db.Exec(recordQuery, migration.Version); err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}
	} else {
		// Execute in transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Execute migration SQL
		if _, err := tx.Exec(migration.UpSQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute SQL: %w", err)
		}

		// Record migration
		recordQuery := "INSERT INTO schema_migrations (version) VALUES (" + placeholder(driver, 1) + ")"
		if _, err := tx.Exec(recordQuery, migration.Version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration: %w", err)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	return nil
}

// placeholder returns the appropriate SQL placeholder for the given driver.
func placeholder(driver string, n int) string {
	switch driver {
	case "postgres", "postgresql":
		return fmt.Sprintf("$%d", n)
	case "mysql", "sqlite3", "sqlite":
		return "?"
	default:
		return "?"
	}
}

// acquireLock acquires a database-specific advisory lock.
func acquireLock(db *sql.DB, driver string) error {
	switch driver {
	case "postgres", "postgresql":
		_, err := db.Exec("SELECT pg_advisory_lock(123456789)")
		return err
	case "mysql":
		var result int
		err := db.QueryRow("SELECT GET_LOCK('itinerary_migrations', 10)").Scan(&result)
		if err != nil {
			return err
		}
		if result != 1 {
			return fmt.Errorf("failed to acquire MySQL lock")
		}
		return nil
	case "sqlite3", "sqlite":
		// SQLite uses automatic file-level locking
		return nil
	default:
		// Unknown driver, skip locking
		return nil
	}
}

// releaseLock releases the database-specific advisory lock.
func releaseLock(db *sql.DB, driver string) error {
	switch driver {
	case "postgres", "postgresql":
		_, err := db.Exec("SELECT pg_advisory_unlock(123456789)")
		return err
	case "mysql":
		_, err := db.Exec("SELECT RELEASE_LOCK('itinerary_migrations')")
		return err
	case "sqlite3", "sqlite":
		// SQLite uses automatic file-level locking
		return nil
	default:
		// Unknown driver, skip locking
		return nil
	}
}

// detectDriver attempts to detect the database driver being used.
// This is a best-effort heuristic since sql.DB doesn't expose the driver name.
func detectDriver(db *sql.DB) string {
	// Try SQLite-specific query
	var result string
	err := db.QueryRow("SELECT sqlite_version()").Scan(&result)
	if err == nil {
		return "sqlite3"
	}

	// Try PostgreSQL-specific query
	err = db.QueryRow("SELECT version()").Scan(&result)
	if err == nil && strings.Contains(strings.ToLower(result), "postgresql") {
		return "postgres"
	}

	// Try MySQL-specific query
	err = db.QueryRow("SELECT version()").Scan(&result)
	if err == nil && (strings.Contains(strings.ToLower(result), "mysql") ||
	                  strings.Contains(strings.ToLower(result), "mariadb")) {
		return "mysql"
	}

	// Default to SQLite (most common for tests)
	return "sqlite3"
}
