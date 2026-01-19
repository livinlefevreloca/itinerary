package db

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// DB wraps sql.DB with additional context
type DB struct {
	*sql.DB
	driver string
}

// Tx wraps sql.Tx with additional context
type Tx struct {
	*sql.Tx
	db *DB
}

// Config holds database connection configuration
type Config struct {
	Driver          string        `toml:"driver"`
	DSN             string        `toml:"dsn"`
	MaxOpenConns    int           `toml:"max_open_conns"`
	MaxIdleConns    int           `toml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `toml:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `toml:"conn_max_idle_time"`
	MigrationsDir   string        `toml:"migrations_dir"`
	SkipMigrations  bool          `toml:"skip_migrations"`
}

// Standard errors
var (
	ErrNotFound   = errors.New("db: not found")
	ErrDuplicate  = errors.New("db: duplicate key")
	ErrForeignKey = errors.New("db: foreign key violation")
)

// Open creates a new database connection
func Open(driver, dsn string) (*DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	// Enable foreign key constraints for SQLite
	if driver == "sqlite3" {
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			db.Close()
			return nil, err
		}
	}

	return &DB{
		DB:     db,
		driver: driver,
	}, nil
}

// OpenWithConfig creates a connection with custom configuration
func OpenWithConfig(config Config) (*DB, error) {
	db, err := Open(config.Driver, config.DSN)
	if err != nil {
		return nil, err
	}

	// Apply connection pool settings
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	}
	if config.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	}

	return db, nil
}

// Driver returns the database driver name
func (db *DB) Driver() string {
	return db.driver
}

// Begin starts a new transaction
func (db *DB) Begin() (*Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}

	return &Tx{
		Tx: tx,
		db: db,
	}, nil
}

// WithTransaction executes a function within a transaction
// Automatically commits on success, rolls back on error
func (db *DB) WithTransaction(fn func(*Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Make sure we make a best effort to rollvack on panic
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Error classification functions

// IsNotFound checks if error is a not found error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows)
}

// IsDuplicate checks if error is a duplicate key error
func IsDuplicate(err error) bool {
	if errors.Is(err, ErrDuplicate) {
		return true
	}

	// Check database-specific error messages
	errMsg := err.Error()
	return strings.Contains(errMsg, "UNIQUE constraint failed") ||
		strings.Contains(errMsg, "duplicate key") ||
		strings.Contains(errMsg, "Duplicate entry")
}

// IsForeignKey checks if error is a foreign key error
func IsForeignKey(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, ErrForeignKey) {
		return true
	}

	// Check database-specific error messages
	errMsg := err.Error()
	return strings.Contains(errMsg, "FOREIGN KEY constraint failed") ||
		strings.Contains(errMsg, "foreign key constraint") ||
		strings.Contains(errMsg, "violates foreign key constraint")
}
