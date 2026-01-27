# Database Migrator Tool

## Overview

A simple, self-contained database migration tool that manages schema versioning and applies SQL migrations in order. Built using only the Go standard library `database/sql` package.

## Goals

1. **Simple and explicit** - SQL-based migrations, no magic
2. **Safe** - Transaction support, prevents concurrent migrations
3. **Multi-database** - Support PostgreSQL, MySQL, and SQLite
4. **Zero dependencies** - Use only stdlib (except database drivers)
5. **Minimal** - Just what we need, nothing more

## Component Location

```
tools/
└── migrator/
    ├── migrator.go         # Core migration logic
    ├── migrator_test.go    # Tests
    ├── parser.go           # Migration file parser
    └── testdata/           # Test fixtures
```

## Migration File Format

Migrations are SQL files with a specific naming convention and format:

### Naming Convention

```
<version>_<description>.sql
```

Examples:
- `001_initial_schema.sql`
- `002_add_job_stats.sql`
- `003_add_requirements_table.sql`

Version must be a 3-digit zero-padded integer. Files are applied in numerical order.

### File Structure

Each migration file contains SQL to apply the migration:

```sql
-- +migrate Up
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    schedule TEXT NOT NULL,
    pod_spec TEXT
);

CREATE INDEX idx_jobs_name ON jobs(name);
```

**Rules**:
- Lines starting with `-- +migrate Up` begin the migration
- Everything after the marker until EOF is part of the migration
- Empty lines and comments are preserved
- SQL statements can span multiple lines

### Non-Transactional Migrations

Some DDL operations cannot be run within a transaction (e.g., creating indexes concurrently in PostgreSQL). Mark these migrations with the `notransaction` flag:

```sql
-- +migrate Up notransaction
CREATE INDEX CONCURRENTLY idx_jobs_schedule ON jobs(schedule);
```

When this flag is present, the migration runs outside of a transaction. Use with caution - if the migration fails, there's no automatic rollback.

### Migration Dependencies

Migrations can declare dependencies on other migrations using the `depends` directive. This prevents conflicts when multiple branches are merged:

```sql
-- +migrate Up
-- +migrate Depends: 003 005

ALTER TABLE jobs ADD COLUMN priority INTEGER DEFAULT 0;
```

**Dependency Rules**:
- Dependencies must be declared before any SQL
- List version numbers separated by spaces
- All dependencies must be applied before this migration runs
- Creates a directed acyclic graph (DAG) of migrations
- Prevents divergent branches from applying conflicting migrations

**Example scenario**:
- Branch A creates migration 004 that adds column `priority`
- Branch B creates migration 004 that adds column `status`
- When both merge, one becomes 004 and one becomes 005
- Migration 005 declares `-- +migrate Depends: 003` to ensure sequential application

## Managing Dimension Tables

Dimension tables (lookup/reference tables) require special consideration in migrations because existing data may reference their IDs. The Itinerary schema uses dimension tables for constraint types and action types.

### Key Principle: Never Delete or Renumber

Once a dimension table row is created and its ID is used in production data:
- **NEVER delete the row** - it may be referenced by historical records
- **NEVER change the ID** - all foreign keys would become invalid
- **NEVER renumber sequences** - this breaks referential integrity

### Adding New Types

To add new constraint or action types, use database-specific "insert if not exists" syntax:

**SQLite**:
```sql
-- +migrate Up
INSERT OR IGNORE INTO constraint_types (id, name) VALUES
    (9, 'minHealthyInstances'),
    (10, 'requiredResources');

INSERT OR IGNORE INTO action_types (id, name) VALUES
    (7, 'sendEmail'),
    (8, 'createTicket');
```

**PostgreSQL**:
```sql
-- +migrate Up
INSERT INTO constraint_types (id, name) VALUES
    (9, 'minHealthyInstances'),
    (10, 'requiredResources')
ON CONFLICT (id) DO NOTHING;

INSERT INTO action_types (id, name) VALUES
    (7, 'sendEmail'),
    (8, 'createTicket')
ON CONFLICT (id) DO NOTHING;
```

**MySQL**:
```sql
-- +migrate Up
INSERT IGNORE INTO constraint_types (id, name) VALUES
    (9, 'minHealthyInstances'),
    (10, 'requiredResources');

INSERT IGNORE INTO action_types (id, name) VALUES
    (7, 'sendEmail'),
    (8, 'createTicket');
```

### Initial Seed Data

The first migration that creates dimension tables should seed them with built-in types:

```sql
-- +migrate Up

-- Create dimension tables
CREATE TABLE constraint_types (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE action_types (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

-- Seed with built-in types
INSERT INTO constraint_types (id, name) VALUES
    (1, 'maxConcurrentRuns'),
    (2, 'catchUp'),
    (3, 'preRunHook'),
    (4, 'postRunHook'),
    (5, 'catchUpWindow'),
    (6, 'maxExpectedRunTime'),
    (7, 'maxAllowedRunTime'),
    (8, 'requirePreviousSuccess');

INSERT INTO action_types (id, name) VALUES
    (1, 'retry'),
    (2, 'kickOffJob'),
    (3, 'webhook'),
    (4, 'killAllInstances'),
    (5, 'killLatestInstance'),
    (6, 'skipNextInstance');
```

### ID Assignment Strategy

**Manual ID assignment (recommended)**:
- Explicitly assign IDs in migrations (as shown above)
- Prevents ID collisions across branches
- Makes IDs deterministic and documentable
- Easier to reason about in code

**Coordinate new IDs across team**:
- Document reserved ID ranges in comments
- Check existing migrations before assigning new IDs
- Consider grouping related types (e.g., 100-199 for webhook types)

### Updating Existing Types

If you need to change a type's name (rare):

```sql
-- +migrate Up
UPDATE constraint_types
SET name = 'maxConcurrentExecutions'
WHERE id = 1;
```

**Warning**: Changing names may break application code that relies on the name string.

### Deprecating Types

Never delete dimension table rows. Instead, add a `deprecated` flag:

```sql
-- +migrate Up
-- Add deprecated column if it doesn't exist
ALTER TABLE constraint_types ADD COLUMN deprecated BOOLEAN DEFAULT FALSE;
ALTER TABLE action_types ADD COLUMN deprecated BOOLEAN DEFAULT FALSE;

-- Mark types as deprecated
UPDATE constraint_types SET deprecated = TRUE WHERE id = 5;
UPDATE action_types SET deprecated = TRUE WHERE id = 3;
```

Application code can then filter out deprecated types while maintaining historical data integrity.

## Schema Version Tracking

The tool maintains a `schema_migrations` table to track applied migrations:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

This table is automatically created on first run.

## Core API

### Primary Function

```go
// RunMigrations applies all pending migrations from the specified directory
func RunMigrations(db *sql.DB, migrationsDir string) error
```

**Behavior**:
1. Creates `schema_migrations` table if it doesn't exist
2. Acquires advisory lock to prevent concurrent migrations
3. Reads all migration files from directory
4. Determines current schema version
5. Applies each pending migration in order within a transaction
6. Records successful migrations in `schema_migrations`
7. Returns error if any migration fails (previous migrations remain applied)

### Supporting Functions

```go
// GetCurrentVersion returns the current schema version
func GetCurrentVersion(db *sql.DB) (int, error)

// GetAppliedMigrations returns all applied migration versions
func GetAppliedMigrations(db *sql.DB) ([]int, error)
```

### Migration File Parser

```go
// Migration represents a parsed migration file
type Migration struct {
    Version       int
    Name          string
    UpSQL         string
    NoTransaction bool   // true if migration should run outside transaction
    Dependencies  []int  // version numbers this migration depends on
}

// ParseMigrationFile parses a migration file and returns a Migration
func ParseMigrationFile(path string) (*Migration, error)

// LoadMigrations loads all migration files from a directory
func LoadMigrations(dir string) ([]Migration, error)
```

## Implementation Details

### Preventing Concurrent Migrations

Use database advisory locks to prevent multiple instances from running migrations simultaneously:

**PostgreSQL**:
```sql
SELECT pg_advisory_lock(123456789);
-- run migrations
SELECT pg_advisory_unlock(123456789);
```

**MySQL**:
```sql
SELECT GET_LOCK('itinerary_migrations', 10);
-- run migrations
SELECT RELEASE_LOCK('itinerary_migrations');
```

**SQLite**:
- Relies on file-level locking (automatic)
- Single writer at a time is enforced by SQLite itself

### Transaction Handling

By default, each migration runs in its own transaction:

```go
func applyMigration(db *sql.DB, migration Migration) error {
    if migration.NoTransaction {
        // Run without transaction
        if _, err := db.Exec(migration.UpSQL); err != nil {
            return fmt.Errorf("migration %d failed: %w", migration.Version, err)
        }

        // Record migration
        if _, err := db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", migration.Version); err != nil {
            return err
        }

        return nil
    }

    // Run within transaction
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback() // rolled back if not committed

    // Execute migration SQL
    if _, err := tx.Exec(migration.UpSQL); err != nil {
        return fmt.Errorf("migration %d failed: %w", migration.Version, err)
    }

    // Record migration
    if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", migration.Version); err != nil {
        return err
    }

    return tx.Commit()
}
```

**Note**:
- Transactional migrations are rolled back automatically on failure
- Non-transactional migrations have no rollback - they must be fixed by rolling forward with a new migration
- Use non-transactional migrations only when necessary (e.g., `CREATE INDEX CONCURRENTLY`)

### Dependency Resolution

Before applying migrations, validate all dependencies are met:

```go
func validateDependencies(migrations []Migration, applied []int) error {
    appliedSet := make(map[int]bool)
    for _, v := range applied {
        appliedSet[v] = true
    }

    for _, m := range migrations {
        for _, dep := range m.Dependencies {
            if !appliedSet[dep] {
                return fmt.Errorf("migration %d depends on %d which is not applied", m.Version, dep)
            }
        }
    }

    return nil
}
```

Dependencies are validated before any migrations are applied. If validation fails, no migrations run.

### Migration File Discovery

```go
func LoadMigrations(dir string) ([]Migration, error) {
    // 1. Read all .sql files from directory
    // 2. Parse filename to extract version and name
    // 3. Parse file content to extract up SQL, flags, and dependencies
    // 4. Sort by version
    // 5. Validate no gaps in version numbers
    // 6. Validate dependencies form valid DAG
    // 7. Return sorted slice
}
```

**Validation rules**:
- All files must follow naming convention `NNN_description.sql`
- Version numbers must be sequential (no gaps)
- Each file must have an up section
- Duplicate version numbers are an error
- Dependencies must form a valid directed acyclic graph (no cycles)
- All dependencies must reference existing migrations

### Error Handling

The tool should provide clear errors for:

1. **Migration file errors**:
   - Invalid filename format
   - Missing up section
   - Failed to read file
   - Non-sequential version numbers
   - Invalid dependency syntax
   - Circular dependencies
   - Dependencies reference non-existent migrations

2. **Database errors**:
   - Cannot acquire lock (another migration in progress)
   - Migration SQL execution failure
   - Transaction commit failure

3. **State errors**:
   - Dependency not yet applied
   - Current database version > latest migration file
   - Missing migration files for versions in database

All errors include context about which migration failed and why.

## Database Compatibility

### SQL Dialect Differences

The migration tool itself is database-agnostic, but migration SQL must be written for the target database. Common differences:

| Feature | PostgreSQL | MySQL | SQLite |
|---------|-----------|-------|--------|
| Boolean | `BOOLEAN` | `TINYINT(1)` | `INTEGER` |
| Timestamp | `TIMESTAMP` | `TIMESTAMP` | `DATETIME` |
| Auto-increment | `SERIAL` | `AUTO_INCREMENT` | `AUTOINCREMENT` |
| Text | `TEXT` | `TEXT` | `TEXT` |
| JSON | `JSONB` | `JSON` | `TEXT` |

For this project, we'll initially support **PostgreSQL** as the primary database. SQLite support can be added for development/testing.

### Advisory Lock Implementation

```go
func acquireLock(db *sql.DB) error {
    // Detect database type from driver name
    driverName := db.Driver()

    switch driverName {
    case "postgres":
        _, err := db.Exec("SELECT pg_advisory_lock(123456789)")
        return err
    case "mysql":
        var result int
        err := db.QueryRow("SELECT GET_LOCK('itinerary_migrations', 10)").Scan(&result)
        if result != 1 {
            return errors.New("failed to acquire migration lock")
        }
        return err
    case "sqlite3":
        // SQLite handles locking automatically
        return nil
    default:
        return fmt.Errorf("unsupported database driver: %s", driverName)
    }
}
```

## Usage Example

```go
package main

import (
    "database/sql"
    "log"

    "github.com/livinlefevreloca/itinerary/tools/migrator"
    _ "github.com/lib/pq"
)

func main() {
    database, err := sql.Open("postgres", "postgres://localhost/itinerary?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer database.Close()

    // Run all pending migrations
    if err := migrator.RunMigrations(database, "migrations"); err != nil {
        log.Fatalf("migration failed: %v", err)
    }

    log.Println("migrations completed successfully")
}
```

## Non-Goals

Things we explicitly don't need:

1. **CLI tool** - Migrations run at application startup
2. **Down migrations** - Roll forward only, fix issues with new migrations
3. **Migration generation** - Write SQL files manually
4. **Checksums** - Trust version numbers and dependencies
5. **Repeatable migrations** - One-time migrations only
6. **Dry-run mode** - Test on dev database
