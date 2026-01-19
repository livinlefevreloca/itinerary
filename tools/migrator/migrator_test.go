package migrator

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// =============================================================================
// Test Helpers
// =============================================================================

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

func cleanupTestDB(db *sql.DB) {
	db.Close()
}

func tableExists(t *testing.T, db *sql.DB, tableName string) bool {
	var name string
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
	err := db.QueryRow(query, tableName).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("failed to check if table exists: %v", err)
	}
	return true
}

func getVersion(t *testing.T, db *sql.DB) int {
	version, err := GetCurrentVersion(db)
	if err != nil {
		t.Fatalf("failed to get version: %v", err)
	}
	return version
}

func assertTablesExist(t *testing.T, db *sql.DB, tables ...string) {
	for _, table := range tables {
		if !tableExists(t, db, table) {
			t.Errorf("expected table %s to exist", table)
		}
	}
}

func assertTablesNotExist(t *testing.T, db *sql.DB, tables ...string) {
	for _, table := range tables {
		if tableExists(t, db, table) {
			t.Errorf("expected table %s to not exist", table)
		}
	}
}

// =============================================================================
// Parser Tests
// =============================================================================

func TestParseMigrationFile_Valid(t *testing.T) {
	migration, err := ParseMigrationFile("testdata/migrations/001_create_users.sql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if migration.Version != 1 {
		t.Errorf("expected version 1, got %d", migration.Version)
	}

	if migration.Name != "create_users" {
		t.Errorf("expected name 'create_users', got '%s'", migration.Name)
	}

	if !contains(migration.UpSQL, "CREATE TABLE users") {
		t.Errorf("expected UpSQL to contain 'CREATE TABLE users', got: %s", migration.UpSQL)
	}

	if migration.NoTransaction {
		t.Error("expected NoTransaction to be false")
	}

	if len(migration.Dependencies) != 0 {
		t.Errorf("expected no dependencies, got %v", migration.Dependencies)
	}
}

func TestParseMigrationFile_WithDependencies(t *testing.T) {
	migration, err := ParseMigrationFile("testdata/migrations/003_create_posts.sql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if migration.Version != 3 {
		t.Errorf("expected version 3, got %d", migration.Version)
	}

	if len(migration.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(migration.Dependencies))
	}

	if migration.Dependencies[0] != 1 {
		t.Errorf("expected dependency on version 1, got %d", migration.Dependencies[0])
	}

	if !contains(migration.UpSQL, "CREATE TABLE posts") {
		t.Errorf("expected UpSQL to contain 'CREATE TABLE posts'")
	}
}

func TestParseMigrationFile_MultipleDependencies(t *testing.T) {
	migration, err := ParseMigrationFile("testdata/migrations/005_add_status.sql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(migration.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(migration.Dependencies))
	}

	if migration.Dependencies[0] != 1 || migration.Dependencies[1] != 2 {
		t.Errorf("expected dependencies [1, 2], got %v", migration.Dependencies)
	}
}

func TestParseMigrationFile_NoTransaction(t *testing.T) {
	migration, err := ParseMigrationFile("testdata/migrations/004_create_index.sql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !migration.NoTransaction {
		t.Error("expected NoTransaction to be true")
	}

	if !contains(migration.UpSQL, "CREATE INDEX") {
		t.Error("expected UpSQL to contain 'CREATE INDEX'")
	}
}

func TestParseMigrationFile_MultilineSQL(t *testing.T) {
	// Using 003 which has multi-line SQL
	migration, err := ParseMigrationFile("testdata/migrations/003_create_posts.sql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should preserve line breaks and capture all SQL
	if !contains(migration.UpSQL, "CREATE TABLE posts") {
		t.Error("expected UpSQL to contain full CREATE TABLE statement")
	}

	if !contains(migration.UpSQL, "FOREIGN KEY") {
		t.Error("expected UpSQL to preserve FOREIGN KEY constraint")
	}
}

func TestParseMigrationFile_InvalidFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{"no version", "testdata/invalid/bad_filename.sql"},
		{"short version", "testdata/invalid/1_short.sql"},
		{"non-numeric", "testdata/invalid/abc_invalid.sql"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file if needed
			if tt.filename == "testdata/invalid/1_short.sql" {
				os.WriteFile(tt.filename, []byte("-- +migrate Up\nCREATE TABLE test (id INTEGER);"), 0644)
				defer os.Remove(tt.filename)
			}
			if tt.filename == "testdata/invalid/abc_invalid.sql" {
				os.WriteFile(tt.filename, []byte("-- +migrate Up\nCREATE TABLE test (id INTEGER);"), 0644)
				defer os.Remove(tt.filename)
			}

			_, err := ParseMigrationFile(tt.filename)
			if err == nil {
				t.Error("expected error for invalid filename format")
			}
		})
	}
}

func TestParseMigrationFile_MissingUpSection(t *testing.T) {
	_, err := ParseMigrationFile("testdata/invalid/no_up.sql")
	if err == nil {
		t.Fatal("expected error for missing up section")
	}

	if !contains(err.Error(), "up") {
		t.Errorf("expected error message to mention 'up', got: %v", err)
	}
}

func TestParseMigrationFile_EmptySQL(t *testing.T) {
	// Create temp file with empty SQL
	tmpFile := "testdata/invalid/empty_sql.sql"
	content := "-- +migrate Up\n-- Just comments\n   \n"
	os.WriteFile(tmpFile, []byte(content), 0644)
	defer os.Remove(tmpFile)

	_, err := ParseMigrationFile(tmpFile)
	if err == nil {
		t.Error("expected error for empty SQL section")
	}
}

func TestParseMigrationFile_InvalidDependencySyntax(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			"non-numeric dependency",
			"-- +migrate Up\n-- +migrate Depends: abc\nCREATE TABLE test (id INTEGER);",
		},
		{
			"empty dependency list",
			"-- +migrate Up\n-- +migrate Depends:\nCREATE TABLE test (id INTEGER);",
		},
		{
			"mixed valid and invalid",
			"-- +migrate Up\n-- +migrate Depends: 1 2 abc\nCREATE TABLE test (id INTEGER);",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join("testdata", "invalid", "temp_dep_test.sql")
			os.WriteFile(tmpFile, []byte(tt.content), 0644)
			defer os.Remove(tmpFile)

			_, err := ParseMigrationFile(tmpFile)
			if err == nil {
				t.Error("expected error for invalid dependency syntax")
			}
		})
	}
}

func TestParseMigrationFile_FileNotFound(t *testing.T) {
	_, err := ParseMigrationFile("testdata/nonexistent.sql")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

// =============================================================================
// Loader Tests
// =============================================================================

func TestLoadMigrations_ValidDirectory(t *testing.T) {
	migrations, err := LoadMigrations("testdata/migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(migrations) != 5 {
		t.Errorf("expected 5 migrations, got %d", len(migrations))
	}

	// Check sorted by version
	for i := 0; i < len(migrations)-1; i++ {
		if migrations[i].Version >= migrations[i+1].Version {
			t.Error("migrations not sorted by version")
		}
	}

	// Check dependencies parsed correctly
	var hasDepMigration bool
	for _, m := range migrations {
		if m.Version == 3 && len(m.Dependencies) == 1 {
			hasDepMigration = true
		}
	}
	if !hasDepMigration {
		t.Error("expected migration 3 to have dependencies")
	}

	// Check NoTransaction flag parsed
	var hasNoTxMigration bool
	for _, m := range migrations {
		if m.Version == 4 && m.NoTransaction {
			hasNoTxMigration = true
		}
	}
	if !hasNoTxMigration {
		t.Error("expected migration 4 to have NoTransaction flag")
	}
}

func TestLoadMigrations_EmptyDirectory(t *testing.T) {
	// Create empty temp directory
	tmpDir := "testdata/empty"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	migrations, err := LoadMigrations(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(migrations) != 0 {
		t.Errorf("expected empty slice, got %d migrations", len(migrations))
	}
}

func TestLoadMigrations_DirectoryNotFound(t *testing.T) {
	_, err := LoadMigrations("testdata/nonexistent")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestLoadMigrations_NonSequentialVersions(t *testing.T) {
	// Create temp directory with gap in versions
	tmpDir := "testdata/gap"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "001_first.sql"), []byte("-- +migrate Up\nCREATE TABLE a (id INTEGER);"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "003_third.sql"), []byte("-- +migrate Up\nCREATE TABLE b (id INTEGER);"), 0644)

	_, err := LoadMigrations(tmpDir)
	if err == nil {
		t.Error("expected error for non-sequential versions")
	}

	if !contains(err.Error(), "gap") && !contains(err.Error(), "sequential") {
		t.Errorf("expected error about gap in versions, got: %v", err)
	}
}

func TestLoadMigrations_DuplicateVersions(t *testing.T) {
	// Create temp directory with duplicate versions
	tmpDir := "testdata/duplicate"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "001_first.sql"), []byte("-- +migrate Up\nCREATE TABLE a (id INTEGER);"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "001_duplicate.sql"), []byte("-- +migrate Up\nCREATE TABLE b (id INTEGER);"), 0644)

	_, err := LoadMigrations(tmpDir)
	if err == nil {
		t.Error("expected error for duplicate versions")
	}

	if !contains(err.Error(), "duplicate") {
		t.Errorf("expected error about duplicate version, got: %v", err)
	}
}

func TestLoadMigrations_MixedValidInvalid(t *testing.T) {
	// Create temp directory with valid and invalid files
	tmpDir := "testdata/mixed"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "001_valid.sql"), []byte("-- +migrate Up\nCREATE TABLE a (id INTEGER);"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "002_invalid.sql"), []byte("-- No up marker\nCREATE TABLE b (id INTEGER);"), 0644)

	_, err := LoadMigrations(tmpDir)
	if err == nil {
		t.Error("expected error for invalid migration file")
	}
}

func TestLoadMigrations_IgnoresNonSQLFiles(t *testing.T) {
	// Create temp directory with SQL and non-SQL files
	tmpDir := "testdata/mixed_files"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "001_migration.sql"), []byte("-- +migrate Up\nCREATE TABLE a (id INTEGER);"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Migrations"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".gitkeep"), []byte(""), 0644)

	migrations, err := LoadMigrations(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(migrations) != 1 {
		t.Errorf("expected 1 migration, got %d", len(migrations))
	}
}

func TestLoadMigrations_CircularDependency(t *testing.T) {
	_, err := LoadMigrations("testdata/invalid")
	if err == nil {
		t.Error("expected error for circular dependencies")
	}

	if !contains(err.Error(), "circular") && !contains(err.Error(), "cycle") {
		t.Errorf("expected error about circular dependency, got: %v", err)
	}
}

func TestLoadMigrations_MissingDependency(t *testing.T) {
	// This test uses 008_missing_dep.sql which depends on 999
	tmpDir := "testdata/missing_dep_test"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "001_base.sql"), []byte("-- +migrate Up\nCREATE TABLE a (id INTEGER);"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "002_depends_on_999.sql"), []byte("-- +migrate Up\n-- +migrate Depends: 999\nCREATE TABLE b (id INTEGER);"), 0644)

	_, err := LoadMigrations(tmpDir)
	if err == nil {
		t.Error("expected error for missing dependency")
	}

	if !contains(err.Error(), "999") && !contains(err.Error(), "missing") && !contains(err.Error(), "dependency") {
		t.Errorf("expected error about missing dependency, got: %v", err)
	}
}

// =============================================================================
// Migration Execution Tests
// =============================================================================

func TestRunMigrations_FreshDatabase(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	err := RunMigrations(db, "testdata/migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check schema_migrations table exists
	if !tableExists(t, db, "schema_migrations") {
		t.Error("expected schema_migrations table to exist")
	}

	// Check all migrations applied
	version := getVersion(t, db)
	if version != 5 {
		t.Errorf("expected version 5, got %d", version)
	}

	// Check tables exist
	assertTablesExist(t, db, "users", "posts")
}

func TestRunMigrations_PartiallyMigrated(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	// Apply only first migration manually
	_, err := db.Exec("CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)")
	if err != nil {
		t.Fatalf("failed to create schema_migrations: %v", err)
	}

	_, err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)")
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	_, err = db.Exec("INSERT INTO schema_migrations (version) VALUES (1)")
	if err != nil {
		t.Fatalf("failed to insert version: %v", err)
	}

	// Now run all migrations
	err = RunMigrations(db, "testdata/migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should skip migration 1 and apply 2-5
	version := getVersion(t, db)
	if version != 5 {
		t.Errorf("expected version 5, got %d", version)
	}

	// All tables should exist
	assertTablesExist(t, db, "users", "posts")
}

func TestRunMigrations_AlreadyUpToDate(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	// Run migrations once
	err := RunMigrations(db, "testdata/migrations")
	if err != nil {
		t.Fatalf("unexpected error on first run: %v", err)
	}

	version1 := getVersion(t, db)

	// Run again
	err = RunMigrations(db, "testdata/migrations")
	if err != nil {
		t.Fatalf("unexpected error on second run: %v", err)
	}

	version2 := getVersion(t, db)

	if version1 != version2 {
		t.Errorf("version changed from %d to %d", version1, version2)
	}
}

func TestRunMigrations_FailedMigration(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	// Create temp directory with failing migration
	tmpDir := "testdata/failing"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "001_good.sql"), []byte("-- +migrate Up\nCREATE TABLE a (id INTEGER);"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "002_bad.sql"), []byte("-- +migrate Up\nINVALID SQL HERE;"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "003_good.sql"), []byte("-- +migrate Up\nCREATE TABLE b (id INTEGER);"), 0644)

	err := RunMigrations(db, tmpDir)
	if err == nil {
		t.Fatal("expected error for failed migration")
	}

	// Migration 1 should be applied
	version := getVersion(t, db)
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}

	// Migration 2 should NOT be recorded
	var count int
	db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 2").Scan(&count)
	if count != 0 {
		t.Error("migration 2 should not be recorded")
	}

	// Migration 3 should not be attempted
	db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 3").Scan(&count)
	if count != 0 {
		t.Error("migration 3 should not be attempted")
	}
}

func TestRunMigrations_TransactionRollback(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	// Create migration that creates table then fails
	tmpDir := "testdata/rollback_test"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	content := "-- +migrate Up\nCREATE TABLE test (id INTEGER);\nINVALID SQL;"
	os.WriteFile(filepath.Join(tmpDir, "001_rollback.sql"), []byte(content), 0644)

	err := RunMigrations(db, tmpDir)
	if err == nil {
		t.Fatal("expected error")
	}

	// Table should NOT exist (transaction rolled back)
	assertTablesNotExist(t, db, "test")

	// No entry in schema_migrations
	version := getVersion(t, db)
	if version != 0 {
		t.Errorf("expected version 0, got %d", version)
	}
}

func TestRunMigrations_MultipleStatements(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	// Create migration with multiple statements
	tmpDir := "testdata/multi_stmt"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	content := `-- +migrate Up
CREATE TABLE a (id INTEGER);
CREATE TABLE b (id INTEGER);
CREATE INDEX idx_a ON a(id);
CREATE INDEX idx_b ON b(id);`
	os.WriteFile(filepath.Join(tmpDir, "001_multi.sql"), []byte(content), 0644)

	err := RunMigrations(db, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both tables should exist
	assertTablesExist(t, db, "a", "b")
}

func TestRunMigrations_NoTransaction(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	// First apply migrations 1 and 2 to create users table with email
	tmpDir := "testdata/notx_test"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "001_users.sql"), []byte("-- +migrate Up\nCREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "002_email.sql"), []byte("-- +migrate Up\nALTER TABLE users ADD COLUMN email TEXT;"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "003_index.sql"), []byte("-- +migrate Up notransaction\nCREATE INDEX idx_users_email ON users(email);"), 0644)

	err := RunMigrations(db, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Index should be created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_users_email'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to check index: %v", err)
	}
	if count != 1 {
		t.Error("expected index to be created")
	}

	// Migration should be recorded
	version := getVersion(t, db)
	if version != 3 {
		t.Errorf("expected version 3, got %d", version)
	}
}

// =============================================================================
// Dependency Tests
// =============================================================================

func TestRunMigrations_WithDependencies(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	err := RunMigrations(db, "testdata/migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All migrations should apply successfully
	version := getVersion(t, db)
	if version != 5 {
		t.Errorf("expected version 5, got %d", version)
	}

	// Posts table should exist (depends on users)
	assertTablesExist(t, db, "users", "posts")
}

func TestRunMigrations_DependencyNotApplied(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	// Manually set up database with only migration 2 applied
	_, err := db.Exec("CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)")
	if err != nil {
		t.Fatalf("failed to create schema_migrations: %v", err)
	}

	_, err = db.Exec("INSERT INTO schema_migrations (version) VALUES (2)")
	if err != nil {
		t.Fatalf("failed to insert version: %v", err)
	}

	// Create temp directory with migration 3 that depends on 1
	tmpDir := "testdata/dep_not_applied"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "001_base.sql"), []byte("-- +migrate Up\nCREATE TABLE users (id INTEGER);"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "002_other.sql"), []byte("-- +migrate Up\nCREATE TABLE other (id INTEGER);"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "003_depends.sql"), []byte("-- +migrate Up\n-- +migrate Depends: 001\nCREATE TABLE depends (user_id INTEGER);"), 0644)

	err = RunMigrations(db, tmpDir)
	if err == nil {
		t.Fatal("expected error for unmet dependency")
	}

	if !contains(err.Error(), "depend") {
		t.Errorf("expected error about dependency, got: %v", err)
	}

	// Version should still be 2
	version := getVersion(t, db)
	if version != 2 {
		t.Errorf("expected version 2, got %d", version)
	}
}

func TestRunMigrations_MultipleDependencies(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	err := RunMigrations(db, "testdata/migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Migration 5 depends on 1 and 2, should work
	version := getVersion(t, db)
	if version != 5 {
		t.Errorf("expected version 5, got %d", version)
	}
}

func TestValidateDependencies_Success(t *testing.T) {
	migrations := []Migration{
		{Version: 1, Dependencies: []int{}},
		{Version: 2, Dependencies: []int{1}},
		{Version: 3, Dependencies: []int{1, 2}},
	}

	applied := []int{1, 2}

	err := validateDependencies(migrations, applied)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateDependencies_MissingDep(t *testing.T) {
	migrations := []Migration{
		{Version: 1, Dependencies: []int{}},
		{Version: 2, Dependencies: []int{5}}, // 5 not applied
	}

	applied := []int{1}

	err := validateDependencies(migrations, applied)
	if err == nil {
		t.Error("expected error for missing dependency")
	}

	if !contains(err.Error(), "5") {
		t.Errorf("expected error to mention version 5, got: %v", err)
	}
}

func TestValidateDependencies_CircularDep(t *testing.T) {
	// Circular deps are caught during loading, so test that here
	tmpDir := "testdata/circular_test"
	os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "001_a.sql"), []byte("-- +migrate Up\n-- +migrate Depends: 002\nCREATE TABLE a (id INTEGER);"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "002_b.sql"), []byte("-- +migrate Up\n-- +migrate Depends: 001\nCREATE TABLE b (id INTEGER);"), 0644)

	_, err := LoadMigrations(tmpDir)
	if err == nil {
		t.Error("expected error for circular dependency")
	}
}

// =============================================================================
// Version Tracking Tests
// =============================================================================

func TestGetCurrentVersion_FreshDatabase(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	version, err := GetCurrentVersion(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if version != 0 {
		t.Errorf("expected version 0 for fresh database, got %d", version)
	}
}

func TestGetCurrentVersion_AfterMigrations(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	err := RunMigrations(db, "testdata/migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	version, err := GetCurrentVersion(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if version != 5 {
		t.Errorf("expected version 5, got %d", version)
	}
}

func TestGetCurrentVersion_EmptyTable(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	// Create schema_migrations but don't insert any rows
	_, err := db.Exec("CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	version, err := GetCurrentVersion(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if version != 0 {
		t.Errorf("expected version 0 for empty table, got %d", version)
	}
}

func TestGetAppliedMigrations(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	err := RunMigrations(db, "testdata/migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	applied, err := GetAppliedMigrations(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(applied) != 5 {
		t.Errorf("expected 5 applied migrations, got %d", len(applied))
	}

	// Check all versions present
	expectedVersions := map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true}
	for _, v := range applied {
		if !expectedVersions[v] {
			t.Errorf("unexpected version %d in applied migrations", v)
		}
		delete(expectedVersions, v)
	}

	if len(expectedVersions) > 0 {
		t.Errorf("missing versions in applied migrations: %v", expectedVersions)
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestRunMigrations_ConcurrentAttempts(t *testing.T) {
	// This test is harder to implement reliably in SQLite
	// Would work better with PostgreSQL advisory locks
	// Skipping for now but structure is here
	t.Skip("Concurrent test requires PostgreSQL advisory locks")
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestFullMigrationCycle(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	// Run migrations
	err := RunMigrations(db, "testdata/migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all tables exist
	assertTablesExist(t, db, "users", "posts")

	// Check version
	version := getVersion(t, db)
	if version != 5 {
		t.Errorf("expected version 5, got %d", version)
	}

	// Run again (should be no-op)
	err = RunMigrations(db, "testdata/migrations")
	if err != nil {
		t.Fatalf("unexpected error on second run: %v", err)
	}

	// Version unchanged
	version2 := getVersion(t, db)
	if version2 != version {
		t.Errorf("version changed from %d to %d", version, version2)
	}
}

func TestMigrationIdempotency(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(db)

	// Run three times
	for i := 0; i < 3; i++ {
		err := RunMigrations(db, "testdata/migrations")
		if err != nil {
			t.Fatalf("unexpected error on run %d: %v", i+1, err)
		}
	}

	// Should still be at version 5
	version := getVersion(t, db)
	if version != 5 {
		t.Errorf("expected version 5, got %d", version)
	}

	// Count total migrations applied (should be 5, not 15)
	var count int
	db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if count != 5 {
		t.Errorf("expected 5 migration records, got %d", count)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) >= len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
