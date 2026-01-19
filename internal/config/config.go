package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/livinlefevreloca/itinerary/internal/db"
	"github.com/livinlefevreloca/itinerary/internal/scheduler"
	"github.com/livinlefevreloca/itinerary/internal/syncer"
)

// Config represents the application configuration
type Config struct {
	Database  db.Config                 `toml:"database"`
	Scheduler scheduler.SchedulerConfig `toml:"scheduler"`
	Syncer    syncer.Config             `toml:"syncer"`
	HTTP      HTTPConfig                `toml:"http"`
	Metrics   MetricsConfig             `toml:"metrics"`
	Logging   LoggingConfig             `toml:"logging"`
}

// HTTPConfig holds HTTP API server settings
type HTTPConfig struct {
	Enabled bool   `toml:"enabled"`
	Address string `toml:"address"`
	Port    int    `toml:"port"`
}

// MetricsConfig holds metrics/monitoring settings
type MetricsConfig struct {
	Enabled bool   `toml:"enabled"`
	Address string `toml:"address"`
	Port    int    `toml:"port"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Database: db.Config{
			Driver:          "sqlite3",
			DSN:             "itinerary.db",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
			MigrationsDir:   "migrations",
			SkipMigrations:  false,
		},
		Scheduler: scheduler.DefaultSchedulerConfig(),
		Syncer:    syncer.DefaultConfig(),
		HTTP: HTTPConfig{
			Enabled: true,
			Address: "0.0.0.0",
			Port:    8080,
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Address: "0.0.0.0",
			Port:    9090,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// LoadFromFile loads configuration from a TOML file
func LoadFromFile(path string) (*Config, error) {
	// Start with defaults
	config := DefaultConfig()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", path)
	}

	// Parse TOML file
	if _, err := toml.DecodeFile(path, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// LoadConfig loads configuration with the following precedence:
// 1. Default values
// 2. Config file (if specified)
// 3. Environment variables (TODO)
// 4. Command-line flags (handled by caller)
func LoadConfig(configPath string) (*Config, error) {
	// Start with defaults
	config := DefaultConfig()

	// If no config file specified, return defaults
	if configPath == "" {
		return config, nil
	}

	// Load from file if it exists
	fileConfig, err := LoadFromFile(configPath)
	if err != nil {
		return nil, err
	}

	return fileConfig, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Database validation
	if c.Database.Driver == "" {
		return fmt.Errorf("database driver must be specified")
	}
	if c.Database.Driver != "sqlite3" && c.Database.Driver != "postgres" && c.Database.Driver != "mysql" {
		return fmt.Errorf("unsupported database driver: %s (must be sqlite3, postgres, or mysql)", c.Database.Driver)
	}
	if c.Database.DSN == "" {
		return fmt.Errorf("database DSN must be specified")
	}

	// Scheduler validation
	if c.Scheduler.LoopInterval <= 0 {
		return fmt.Errorf("scheduler loop_interval must be positive")
	}
	if c.Scheduler.IndexRebuildInterval <= 0 {
		return fmt.Errorf("scheduler index_rebuild_interval must be positive")
	}
	if c.Scheduler.PreScheduleInterval <= 0 {
		return fmt.Errorf("scheduler pre_schedule_interval must be positive")
	}
	if c.Scheduler.LookaheadWindow <= 0 {
		return fmt.Errorf("scheduler lookahead_window must be positive")
	}
	if c.Scheduler.InboxBufferSize <= 0 {
		return fmt.Errorf("scheduler inbox_buffer_size must be positive")
	}

	// Syncer validation
	if c.Syncer.JobRunChannelSize <= 0 {
		return fmt.Errorf("syncer job_run_channel_size must be positive")
	}
	if c.Syncer.StatsChannelSize <= 0 {
		return fmt.Errorf("syncer stats_channel_size must be positive")
	}

	// HTTP validation
	if c.HTTP.Enabled {
		if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
			return fmt.Errorf("HTTP port must be between 1 and 65535")
		}
	}

	// Metrics validation
	if c.Metrics.Enabled {
		if c.Metrics.Port <= 0 || c.Metrics.Port > 65535 {
			return fmt.Errorf("metrics port must be between 1 and 65535")
		}
	}

	// Logging validation
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	return nil
}
