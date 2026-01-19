# Database Migrations

This directory contains SQL migration files for the Itinerary database schema.

## Migration Files

- `001_initial_schema.sql` - Creates all tables and seeds dimension tables with built-in constraint/action types
- `002_add_new_constraint_types.sql` - Example of adding new constraint and action types

## Running Migrations

### Automatic on Startup

Migrations run automatically when the application starts. The application will fail to start if migrations fail, ensuring the database schema is always up-to-date.

```bash
# Default (runs migrations from ./migrations directory)
./itinerary

# Custom migrations directory
./itinerary --migrations-dir=/path/to/migrations

# Skip migrations (not recommended for production)
./itinerary --skip-migrations
```

### Programmatic Usage

You can also run migrations programmatically:

```go
import (
    "github.com/livinlefevreloca/itinerary/internal/db"
    "github.com/livinlefevreloca/itinerary/tools/migrator"
)

database, err := db.Open("sqlite3", "itinerary.db")
if err != nil {
    log.Fatal(err)
}

err = migrator.RunMigrations(database.DB, "migrations")
if err != nil {
    log.Fatal(err)
}
```

### Migration Safety

- Migrations acquire an advisory lock to prevent concurrent execution
- Each migration runs in a transaction (unless marked `notransaction`)
- Failed migrations are rolled back automatically
- Application won't start if migrations fail
- Migration state is tracked in `schema_migrations` table

## Creating New Migrations

### Regular Schema Changes

```bash
# Create new migration file with next version number
touch migrations/003_add_job_tags.sql
```

```sql
-- +migrate Up
ALTER TABLE jobs ADD COLUMN tags TEXT;
CREATE INDEX idx_jobs_tags ON jobs(tags);
```

### Adding New Constraint/Action Types

When adding new types to dimension tables, use database-specific upsert syntax:

**SQLite**:
```sql
-- +migrate Up
INSERT OR IGNORE INTO constraint_types (id, name) VALUES
    (11, 'requireHealthCheck'),
    (12, 'maxRetries');
```

**PostgreSQL**:
```sql
-- +migrate Up
INSERT INTO constraint_types (id, name) VALUES
    (11, 'requireHealthCheck'),
    (12, 'maxRetries')
ON CONFLICT (id) DO NOTHING;
```

**MySQL**:
```sql
-- +migrate Up
INSERT IGNORE INTO constraint_types (id, name) VALUES
    (11, 'requireHealthCheck'),
    (12, 'maxRetries');
```

### ID Assignment

**Constraint Types**: IDs 1-8 are reserved for built-in types. Use 9+ for new types.

**Action Types**: IDs 1-6 are reserved for built-in types. Use 7+ for new types.

**Rules**:
- Always assign IDs explicitly (never use AUTO_INCREMENT)
- Never reuse or change existing IDs
- Coordinate with team before assigning new IDs
- Document new IDs in this README

## Current Type IDs

### Constraint Types

| ID | Name | Description |
|----|------|-------------|
| 1 | maxConcurrentRuns | Maximum concurrent executions |
| 2 | catchUp | Whether to run missed executions |
| 3 | preRunHook | Webhook to call before execution |
| 4 | postRunHook | Webhook to call after execution |
| 5 | catchUpWindow | Time window for catch-up |
| 6 | maxExpectedRunTime | Expected completion time |
| 7 | maxAllowedRunTime | Hard timeout |
| 8 | requirePreviousSuccess | Dependency on previous run |
| 9 | minHealthyInstances | Minimum healthy instances (example) |
| 10 | requiredResources | Required resources (example) |

### Action Types

| ID | Name | Description |
|----|------|-------------|
| 1 | retry | Retry the failed job |
| 2 | kickOffJob | Start another job |
| 3 | webhook | Trigger a webhook |
| 4 | killAllInstances | Kill all running instances |
| 5 | killLatestInstance | Kill most recent instance |
| 6 | skipNextInstance | Skip next scheduled run |
| 7 | sendEmail | Send email notification (example) |
| 8 | createTicket | Create support ticket (example) |

## Important: Dimension Table Rules

1. **Never delete dimension table rows** - Historical data may reference them
2. **Never change IDs** - All foreign keys would break
3. **Never renumber IDs** - Immutable once assigned
4. **Always use upsert syntax** - Prevents errors on re-runs
5. **Coordinate ID assignment** - Check this README before picking IDs

## Migration Dependencies

If multiple feature branches create conflicting migrations, use the `Depends` directive:

```sql
-- +migrate Up
-- +migrate Depends: 005 007

ALTER TABLE jobs ADD COLUMN new_field TEXT;
```

This ensures migration 008 won't run until both 005 and 007 are applied.
