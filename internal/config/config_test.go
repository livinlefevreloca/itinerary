package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Database defaults
	if cfg.Database.Driver != "sqlite3" {
		t.Errorf("expected driver sqlite3, got %s", cfg.Database.Driver)
	}
	if cfg.Database.DSN != "itinerary.db" {
		t.Errorf("expected DSN itinerary.db, got %s", cfg.Database.DSN)
	}
	if cfg.Database.MaxOpenConns != 25 {
		t.Errorf("expected max_open_conns 25, got %d", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MigrationsDir != "migrations" {
		t.Errorf("expected migrations_dir migrations, got %s", cfg.Database.MigrationsDir)
	}

	// Scheduler defaults
	if cfg.Scheduler.LoopInterval != 1*time.Second {
		t.Errorf("expected loop_interval 1s, got %v", cfg.Scheduler.LoopInterval)
	}
	if cfg.Scheduler.InboxBufferSize != 10000 {
		t.Errorf("expected inbox_buffer_size 10000, got %d", cfg.Scheduler.InboxBufferSize)
	}

	// HTTP defaults
	if !cfg.HTTP.Enabled {
		t.Error("expected HTTP enabled by default")
	}
	if cfg.HTTP.Port != 8080 {
		t.Errorf("expected HTTP port 8080, got %d", cfg.HTTP.Port)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[database]
driver = "postgres"
dsn = "postgres://localhost/test"
max_open_conns = 50

[scheduler]
loop_interval = "2s"
inbox_buffer_size = 5000

[http]
enabled = false
port = 9000
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Check overridden values
	if cfg.Database.Driver != "postgres" {
		t.Errorf("expected driver postgres, got %s", cfg.Database.Driver)
	}
	if cfg.Database.MaxOpenConns != 50 {
		t.Errorf("expected max_open_conns 50, got %d", cfg.Database.MaxOpenConns)
	}
	if cfg.Scheduler.LoopInterval != 2*time.Second {
		t.Errorf("expected loop_interval 2s, got %v", cfg.Scheduler.LoopInterval)
	}
	if cfg.Scheduler.InboxBufferSize != 5000 {
		t.Errorf("expected inbox_buffer_size 5000, got %d", cfg.Scheduler.InboxBufferSize)
	}
	if cfg.HTTP.Enabled {
		t.Error("expected HTTP disabled")
	}
	if cfg.HTTP.Port != 9000 {
		t.Errorf("expected HTTP port 9000, got %d", cfg.HTTP.Port)
	}

	// Check default values still present
	if cfg.Database.MaxIdleConns != 5 {
		t.Errorf("expected max_idle_conns default 5, got %d", cfg.Database.MaxIdleConns)
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/config.toml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadConfig_NoFile(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("expected no error for empty config path, got %v", err)
	}

	// Should return defaults
	if cfg.Database.Driver != "sqlite3" {
		t.Errorf("expected default driver, got %s", cfg.Database.Driver)
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidate_InvalidDriver(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Database.Driver = "invalid"

	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid driver")
	}
}

func TestValidate_EmptyDriver(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Database.Driver = ""

	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty driver")
	}
}

func TestValidate_EmptyDSN(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Database.DSN = ""

	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty DSN")
	}
}

func TestValidate_InvalidLoopInterval(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Scheduler.LoopInterval = 0

	if err := cfg.Validate(); err == nil {
		t.Error("expected error for zero loop interval")
	}
}

func TestValidate_InvalidHTTPPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HTTP.Port = 99999

	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid HTTP port")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Logging.Level = "invalid"

	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid log level")
	}
}
