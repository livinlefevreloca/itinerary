# Application Startup

The Itinerary application follows a strict startup sequence to ensure the database schema is always up-to-date before the scheduler begins processing jobs.

## Startup Sequence

```
1. Parse command-line flags
2. Open database connection
3. Run migrations (if not skipped)
4. Log schema version
5. Initialize scheduler (TODO)
6. Start HTTP API server (TODO)
7. Wait for shutdown signal
```

## Running the Application

### Basic Usage

```bash
# Start with default settings (SQLite database)
./itinerary

# Use configuration file
./itinerary --config=config.toml

# Override specific settings
./itinerary --config=config.toml --db-dsn="postgres://localhost/itinerary"
```

### Configuration

Itinerary uses a TOML configuration file. See `CONFIGURATION.md` for complete documentation.

**Quick start:**

1. Copy the example config:
   ```bash
   cp config.toml.example config.toml
   ```

2. Edit `config.toml` for your environment

3. Run the application:
   ```bash
   ./itinerary --config=config.toml
   ```

### Command-Line Flags

Command-line flags override configuration file settings.

| Flag | Description |
|------|-------------|
| `--config` | Path to configuration file (TOML) |
| `--db-driver` | Database driver (overrides config) |
| `--db-dsn` | Database connection string (overrides config) |
| `--migrations-dir` | Path to migrations directory (overrides config) |
| `--skip-migrations` | Skip running migrations on startup (overrides config) |

See `CONFIGURATION.md` for all configuration options.

## Database Migrations

### Automatic Execution

Migrations run automatically on every startup:

1. **Advisory Lock** - Acquires a database lock to prevent concurrent migrations
2. **Load Migrations** - Reads all `.sql` files from the migrations directory
3. **Check Applied** - Queries `schema_migrations` table for already-applied versions
4. **Apply Pending** - Runs each pending migration in order within a transaction
5. **Record Success** - Inserts migration version into `schema_migrations` table

### Migration Safety

- **Fail-fast**: Application won't start if migrations fail
- **Transactional**: Each migration runs in a transaction (automatic rollback on error)
- **Idempotent**: Safe to run multiple times (already-applied migrations are skipped)
- **Concurrent-safe**: Advisory locks prevent multiple instances from migrating simultaneously

### Schema Version

Check the current database schema version:

```bash
# The application logs the version on startup
2026/01/19 14:19:54 Database schema version: 2
```

Or query directly:

```bash
sqlite3 itinerary.db "SELECT MAX(version) FROM schema_migrations;"
```

## First-Time Setup

On first run with a new database:

```bash
$ ./itinerary
2026/01/19 14:19:54 Starting Itinerary Job Scheduler
2026/01/19 14:19:54 Connecting to database (sqlite3)...
2026/01/19 14:19:54 Running migrations from migrations...
2026/01/19 14:19:54 Database schema version: 2
2026/01/19 14:19:54 Itinerary is running. Press Ctrl+C to stop.
```

The application will:
1. Create the database file (if using SQLite)
2. Create the `schema_migrations` table
3. Run all migrations in order
4. Seed dimension tables with built-in constraint and action types
5. Start the scheduler

## Graceful Shutdown

The application listens for `SIGINT` (Ctrl+C) and `SIGTERM`:

```
^C
2026/01/19 14:19:56 Shutting down gracefully...
```

Shutdown sequence (when implemented):
1. Stop accepting new job submissions
2. Allow running jobs to complete (with timeout)
3. Close database connections
4. Exit cleanly

## Development Tips

### Testing Migrations

```bash
# Start fresh
rm itinerary.db
./itinerary

# Verify schema
sqlite3 itinerary.db "SELECT name FROM sqlite_master WHERE type='table';"

# Check dimension tables
sqlite3 itinerary.db "SELECT * FROM constraint_types;"
sqlite3 itinerary.db "SELECT * FROM action_types;"
```

### In-Memory Database (Testing)

```bash
# Use in-memory database (won't persist)
./itinerary --db-dsn=":memory:"
```

### Skipping Migrations

Only skip migrations when:
- Database schema is already up-to-date
- Running tests that don't need database
- Debugging non-database issues

**Never skip migrations in production** - schema mismatches will cause runtime errors.

## Troubleshooting

### Migration Fails

If a migration fails, the application won't start:

```
2026/01/19 14:19:54 Failed to run migrations: migration 3 failed: ...
```

**Solution**: Fix the migration SQL and restart. Previous migrations remain applied.

### Lock Timeout

If another instance is running migrations:

```
2026/01/19 14:19:54 Failed to acquire lock: another migration in progress
```

**Solution**: Wait for the other migration to complete, or stop the other instance.

### Schema Mismatch

If the database has a higher version than the migration files:

```
2026/01/19 14:19:54 Warning: database version (5) is newer than latest migration (3)
```

**Solution**: Ensure you have the latest migration files, or roll back the database.

## Production Deployment

### Rolling Updates

When deploying new versions:

1. New version starts
2. Runs new migrations (with advisory lock)
3. Old version continues running (migrations already applied)
4. Old version is shut down
5. New version takes over

The advisory lock ensures only one instance runs migrations at a time.

### Database Backups

**Always backup before deploying new migrations**:

```bash
# SQLite
cp itinerary.db itinerary.db.backup

# PostgreSQL
pg_dump itinerary > backup.sql

# MySQL
mysqldump itinerary > backup.sql
```

### Monitoring

Monitor these startup logs:
- Database connection success
- Migration execution time
- Final schema version
- Any migration errors

Failed migrations should trigger alerts - the application won't serve requests without an up-to-date schema.
