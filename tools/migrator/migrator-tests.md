# Database Migrations Test Suite

## Overview

Comprehensive test suite for the database migration component. Tests cover parsing, file loading, migration application, dependency validation, error handling, and edge cases.

## Test Organization

Tests are organized by functionality:

1. **Parser Tests** - Migration file parsing (including flags and dependencies)
2. **Loader Tests** - Loading migrations from directory
3. **Migration Execution Tests** - Applying migrations (transactional and non-transactional)
4. **Dependency Tests** - Dependency validation and resolution
5. **Version Tracking Tests** - Schema version queries
6. **Concurrency Tests** - Preventing concurrent migrations
7. **Error Handling Tests** - Various failure scenarios

## Test Infrastructure

### Test Fixtures

Create test migration files in `testdata/migrations/`:

**testdata/migrations/001_create_users.sql**:
```sql
-- +migrate Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);
```

**testdata/migrations/002_add_email.sql**:
```sql
-- +migrate Up
ALTER TABLE users ADD COLUMN email TEXT;
```

**testdata/migrations/003_create_posts.sql**:
```sql
-- +migrate Up
-- +migrate Depends: 001
CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
```

**testdata/migrations/004_create_index.sql**:
```sql
-- +migrate Up notransaction
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
```

**testdata/migrations/005_add_status.sql**:
```sql
-- +migrate Up
-- +migrate Depends: 001 002
ALTER TABLE users ADD COLUMN status TEXT DEFAULT 'active';
```

### Invalid Migration Files (for error testing)

**testdata/invalid/no_up.sql**:
```sql
-- Just a comment, no Up marker
CREATE TABLE test (id INTEGER);
```

**testdata/invalid/bad_filename.sql**:
```sql
-- +migrate Up
CREATE TABLE test (id INTEGER);
```

**testdata/invalid/circular_dep_a.sql** (version 006):
```sql
-- +migrate Up
-- +migrate Depends: 007
CREATE TABLE a (id INTEGER);
```

**testdata/invalid/circular_dep_b.sql** (version 007):
```sql
-- +migrate Up
-- +migrate Depends: 006
CREATE TABLE b (id INTEGER);
```

**testdata/invalid/missing_dep.sql** (version 008):
```sql
-- +migrate Up
-- +migrate Depends: 999
CREATE TABLE test (id INTEGER);
```

### Test Database Helper

```go
func setupTestDB(t *testing.T) *sql.DB {
    // Create in-memory SQLite database for testing
    db, err := sql.Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatalf("failed to open test db: %v", err)
    }
    return db
}

func cleanupTestDB(db *sql.DB) {
    db.Close()
}
```

## Parser Tests

### TestParseMigrationFile_Valid
**Purpose**: Verify parsing of a valid migration file

**Setup**:
- Use `001_create_users.sql` fixture

**Actions**:
- Call `ParseMigrationFile("testdata/migrations/001_create_users.sql")`

**Assertions**:
- No error returned
- `migration.Version == 1`
- `migration.Name == "create_users"`
- `migration.UpSQL` contains "CREATE TABLE users"
- `migration.NoTransaction == false`
- `migration.Dependencies == []`
- SQL strings are trimmed (no leading/trailing whitespace)

### TestParseMigrationFile_WithDependencies
**Purpose**: Verify parsing migrations with dependencies

**Setup**:
- Use `003_create_posts.sql` fixture (depends on 001)

**Actions**:
- Parse the file

**Assertions**:
- No error returned
- `migration.Dependencies == []int{1}`
- SQL captured correctly

### TestParseMigrationFile_MultipleDependencies
**Purpose**: Verify parsing migrations with multiple dependencies

**Setup**:
- Use `005_add_status.sql` fixture (depends on 001, 002)

**Actions**:
- Parse the file

**Assertions**:
- No error returned
- `migration.Dependencies == []int{1, 2}`
- Dependencies in correct order

### TestParseMigrationFile_NoTransaction
**Purpose**: Verify parsing migrations with notransaction flag

**Setup**:
- Use `004_create_index.sql` fixture

**Actions**:
- Parse the file

**Assertions**:
- No error returned
- `migration.NoTransaction == true`
- SQL captured correctly

### TestParseMigrationFile_MultilineSQL
**Purpose**: Verify parsing handles multi-line SQL statements

**Setup**:
- Create migration file with complex multi-line SQL

**Actions**:
- Parse the file

**Assertions**:
- Up SQL captured completely
- Line breaks preserved within SQL
- Comments within SQL preserved

### TestParseMigrationFile_InvalidFilename
**Purpose**: Verify error on invalid filename format

**Test Cases**:
- `missing_version.sql` - no version number
- `abc_invalid.sql` - non-numeric version
- `1_short_version.sql` - version not zero-padded to 3 digits
- `001-wrong-separator.sql` - wrong separator

**Assertions**:
- Error returned with clear message
- Error indicates filename format issue

### TestParseMigrationFile_MissingUpSection
**Purpose**: Verify error when up section missing

**Setup**:
- Use `testdata/invalid/no_up.sql`

**Actions**:
- Parse the file

**Assertions**:
- Error returned
- Error message indicates missing "up" section

### TestParseMigrationFile_EmptySQL
**Purpose**: Verify error when up SQL is empty

**Test Cases**:
- Up section exists but contains only whitespace/comments

**Assertions**:
- Error returned indicating empty SQL section

### TestParseMigrationFile_InvalidDependencySyntax
**Purpose**: Verify error on malformed dependency declarations

**Test Cases**:
- `-- +migrate Depends: abc` - non-numeric dependency
- `-- +migrate Depends:` - empty dependency list
- `-- +migrate Depends: 1 2 abc` - mix of valid and invalid

**Assertions**:
- Error returned
- Error indicates invalid dependency syntax

### TestParseMigrationFile_FileNotFound
**Purpose**: Verify error when file doesn't exist

**Actions**:
- Parse non-existent file

**Assertions**:
- Error returned
- Error indicates file not found

## Loader Tests

### TestLoadMigrations_ValidDirectory
**Purpose**: Verify loading all migrations from a valid directory

**Setup**:
- Use `testdata/migrations/` with 5 migration files

**Actions**:
- Call `LoadMigrations("testdata/migrations")`

**Assertions**:
- No error returned
- Returns 5 migrations
- Migrations sorted by version (001-005)
- All migrations parsed correctly
- Dependencies parsed correctly
- NoTransaction flags parsed correctly

### TestLoadMigrations_EmptyDirectory
**Purpose**: Verify handling of directory with no migration files

**Setup**:
- Create empty test directory

**Actions**:
- Load migrations from empty directory

**Assertions**:
- No error returned
- Returns empty slice

### TestLoadMigrations_DirectoryNotFound
**Purpose**: Verify error when directory doesn't exist

**Actions**:
- Load from non-existent directory

**Assertions**:
- Error returned
- Error indicates directory not found

### TestLoadMigrations_NonSequentialVersions
**Purpose**: Verify error when version numbers have gaps

**Setup**:
- Directory with: 001_first.sql, 003_third.sql (missing 002)

**Actions**:
- Load migrations

**Assertions**:
- Error returned
- Error indicates gap in version sequence (missing version 2)

### TestLoadMigrations_DuplicateVersions
**Purpose**: Verify error when multiple files have same version

**Setup**:
- Directory with: 001_first.sql, 001_duplicate.sql

**Actions**:
- Load migrations

**Assertions**:
- Error returned
- Error indicates duplicate version number

### TestLoadMigrations_MixedValidInvalid
**Purpose**: Verify error when directory contains invalid migration files

**Setup**:
- Directory with valid and invalid migration files

**Actions**:
- Load migrations

**Assertions**:
- Error returned
- Error indicates which file(s) are invalid

### TestLoadMigrations_IgnoresNonSQLFiles
**Purpose**: Verify non-.sql files are ignored

**Setup**:
- Directory with: 001_first.sql, README.md, .gitkeep

**Actions**:
- Load migrations

**Assertions**:
- No error returned
- Only .sql file loaded
- Non-.sql files ignored

### TestLoadMigrations_CircularDependency
**Purpose**: Verify error when migrations have circular dependencies

**Setup**:
- Use `testdata/invalid/` with circular_dep_a.sql and circular_dep_b.sql

**Actions**:
- Load migrations

**Assertions**:
- Error returned
- Error indicates circular dependency detected
- Error specifies which migrations form the cycle

### TestLoadMigrations_MissingDependency
**Purpose**: Verify error when dependency references non-existent migration

**Setup**:
- Use `testdata/invalid/missing_dep.sql` (depends on 999)

**Actions**:
- Load migrations

**Assertions**:
- Error returned
- Error indicates missing dependency
- Error specifies which version is missing

## Migration Execution Tests

### TestRunMigrations_FreshDatabase
**Purpose**: Verify running migrations on fresh database

**Setup**:
- Fresh test database (no schema_migrations table)
- 3 test migrations

**Actions**:
- Run migrations

**Assertions**:
- No error returned
- `schema_migrations` table created
- All 3 migrations applied
- Tables from migrations exist
- `schema_migrations` has 3 rows (versions 1, 2, 3)
- `applied_at` timestamps set

### TestRunMigrations_PartiallyMigrated
**Purpose**: Verify running migrations on partially migrated database

**Setup**:
- Database with version 1 already applied
- 3 migrations available

**Actions**:
- Run migrations

**Assertions**:
- No error returned
- Only migrations 2 and 3 applied
- Migration 1 skipped
- Final version is 3

### TestRunMigrations_AlreadyUpToDate
**Purpose**: Verify no-op when database already at latest version

**Setup**:
- Database with all 3 migrations applied

**Actions**:
- Run migrations again

**Assertions**:
- No error returned
- No migrations applied
- Version remains at 3

### TestRunMigrations_FailedMigration
**Purpose**: Verify behavior when a migration fails

**Setup**:
- Fresh database
- 3 migrations, where migration 2 has invalid SQL

**Actions**:
- Run migrations

**Assertions**:
- Error returned
- Error indicates migration 2 failed
- Migration 1 applied successfully (committed)
- Migration 2 NOT recorded in schema_migrations
- Migration 3 not attempted
- Database in consistent state (no partial migration 2)

### TestRunMigrations_TransactionRollback
**Purpose**: Verify transaction rolls back on error

**Setup**:
- Fresh database
- Migration that creates table, then fails

**Actions**:
- Run migration

**Assertions**:
- Error returned
- Table NOT created (transaction rolled back)
- No entry in schema_migrations

### TestRunMigrations_MultipleStatements
**Purpose**: Verify migration with multiple SQL statements

**Setup**:
- Migration that creates multiple tables and indexes

**Actions**:
- Run migration

**Assertions**:
- All statements executed
- All tables/indexes created

### TestRunMigrations_NoTransaction
**Purpose**: Verify non-transactional migrations run outside of transactions

**Setup**:
- Use `004_create_index.sql` fixture (marked with notransaction)
- Database with users table already created

**Actions**:
- Run migrations including the notransaction one

**Assertions**:
- No error returned
- Index created successfully
- Migration recorded in schema_migrations
- Verify migration ran outside transaction (database-specific check if possible)

## Dependency Tests

### TestRunMigrations_WithDependencies
**Purpose**: Verify migrations with dependencies apply correctly

**Setup**:
- Fresh database
- Migrations 001, 002, 003 (where 003 depends on 001)

**Actions**:
- Run migrations

**Assertions**:
- All migrations apply successfully
- Migration 003 only runs after 001 is applied
- Final version is 3

### TestRunMigrations_DependencyNotApplied
**Purpose**: Verify error when dependency not yet applied

**Setup**:
- Database with only migration 002 applied
- Try to apply migration 003 which depends on 001

**Actions**:
- Run migrations

**Assertions**:
- Error returned
- Error indicates migration 003 depends on 001 which is not applied
- Migration 003 NOT applied
- Database remains at version 2

### TestRunMigrations_MultipleDependencies
**Purpose**: Verify migrations with multiple dependencies

**Setup**:
- Fresh database
- Use migration 005 which depends on 001 and 002

**Actions**:
- Run all migrations

**Assertions**:
- All dependencies met
- Migration 005 applies successfully
- All migrations in correct order

### TestValidateDependencies_Success
**Purpose**: Verify dependency validation succeeds for valid DAG

**Setup**:
- Migrations with valid dependency chain

**Actions**:
- Call `validateDependencies(migrations, []int{1, 2})`

**Assertions**:
- No error returned
- All dependencies satisfied

### TestValidateDependencies_MissingDep
**Purpose**: Verify error when dependency missing

**Setup**:
- Migration depends on version 5
- Only versions 1-3 applied

**Actions**:
- Validate dependencies

**Assertions**:
- Error returned
- Error specifies which dependency is missing

### TestValidateDependencies_CircularDep
**Purpose**: Verify detection of circular dependencies

**Setup**:
- Use circular dependency test fixtures

**Actions**:
- Load migrations

**Assertions**:
- Error returned during load
- Error indicates circular dependency
- Specifies which migrations form the cycle

## Version Tracking Tests

### TestGetCurrentVersion_FreshDatabase
**Purpose**: Verify getting version from fresh database

**Setup**:
- Database without schema_migrations table

**Actions**:
- Call `GetCurrentVersion()`

**Assertions**:
- Returns version 0
- No error

### TestGetCurrentVersion_AfterMigrations
**Purpose**: Verify getting version after migrations

**Setup**:
- Apply 3 migrations

**Actions**:
- Call `GetCurrentVersion()`

**Assertions**:
- Returns version 3
- No error

### TestGetCurrentVersion_EmptyTable
**Purpose**: Verify version when schema_migrations exists but is empty

**Setup**:
- Create schema_migrations table
- Don't insert any rows

**Actions**:
- Call `GetCurrentVersion()`

**Assertions**:
- Returns version 0
- No error

## Concurrency Tests

### TestRunMigrations_ConcurrentAttempts
**Purpose**: Verify only one migration runs when multiple attempted simultaneously

**Setup**:
- Fresh database
- 5 test migrations

**Actions**:
- Start 3 goroutines that all call `RunMigrations()` simultaneously
- Wait for all to complete

**Assertions**:
- Exactly one succeeds
- Other two return error indicating lock acquisition failed
- Database ends up at version 5
- No duplicate migrations applied

### TestRunMigrations_LockTimeout
**Purpose**: Verify timeout when lock held too long

**Setup**:
- Acquire migration lock manually
- Hold it

**Actions**:
- Call `RunMigrations()` in another goroutine

**Assertions**:
- Returns error after timeout
- Error indicates lock acquisition failed

## Error Handling Tests

### TestRunMigrations_DatabaseConnectionClosed
**Purpose**: Verify error when database connection is closed

**Setup**:
- Close database connection

**Actions**:
- Run migrations

**Assertions**:
- Error returned
- Error indicates database connection problem

### TestRunMigrations_InvalidMigrationsDir
**Purpose**: Verify error with invalid migrations directory

**Actions**:
- Run migrations with non-existent directory

**Assertions**:
- Error returned
- Error message clear

### TestRunMigrations_CorruptedSchemaTable
**Purpose**: Verify handling of corrupted schema_migrations table

**Setup**:
- Manually create schema_migrations with invalid data

**Actions**:
- Run migrations

**Assertions**:
- Error returned or gracefully handled

## Integration Tests

### TestFullMigrationCycle
**Purpose**: End-to-end test of complete migration lifecycle

**Actions**:
1. Start with fresh database
2. Run migrations (0 → 3)
3. Verify all tables created
4. Rollback one migration (3 → 2)
5. Verify table removed
6. Run migrations again (2 → 3)
7. Verify table recreated
8. Rollback to 0
9. Verify all tables removed

**Assertions**:
- All steps succeed
- Database state correct at each step
- Version tracking accurate throughout

### TestMigrationIdempotency
**Purpose**: Verify migrations can be run multiple times safely

**Actions**:
1. Run migrations
2. Run migrations again
3. Run migrations again

**Assertions**:
- All runs succeed
- Only first run applies migrations
- Subsequent runs are no-ops
- Version remains consistent

## Test Utilities

### Helper Functions

```go
// tableExists checks if a table exists in the database
func tableExists(t *testing.T, db *sql.DB, tableName string) bool

// getVersion queries the current schema version
func getVersion(t *testing.T, db *sql.DB) int

// assertTablesExist verifies multiple tables exist
func assertTablesExist(t *testing.T, db *sql.DB, tables ...string)

// assertTablesNotExist verifies tables don't exist
func assertTablesNotExist(t *testing.T, db *sql.DB, tables ...string)

// createInvalidMigration creates a migration file with invalid SQL
func createInvalidMigration(t *testing.T, dir string, version int) string
```

## Test Coverage Goals

- **Line coverage**: > 90%
- **Branch coverage**: > 85%
- **Error paths**: All error returns tested
- **Edge cases**: All documented edge cases covered

## Performance Tests (Future)

Not required initially, but could add:

- Benchmark migration parsing performance
- Benchmark applying 100+ migrations
- Memory usage during large migrations
