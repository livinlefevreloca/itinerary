package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/livinlefevreloca/itinerary/internal/config"
	"github.com/livinlefevreloca/itinerary/internal/db"
	"github.com/livinlefevreloca/itinerary/tools/migrator"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "", "Path to configuration file (TOML)")
	flag.Parse()

	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("starting itinerary job scheduler")

	// Load configuration
	slog.Info("loading configuration", "config_file", *configFile)
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// Log configuration summary
	slog.Info("database configuration",
		"driver", cfg.Database.Driver,
		"dsn", cfg.Database.DSN,
		"migrations_dir", cfg.Database.MigrationsDir)

	if cfg.HTTP.Enabled {
		slog.Info("http api enabled",
			"address", cfg.HTTP.Address,
			"port", cfg.HTTP.Port)
	}

	if cfg.Metrics.Enabled {
		slog.Info("metrics enabled",
			"address", cfg.Metrics.Address,
			"port", cfg.Metrics.Port)
	}

	// Open database connection with pool settings
	slog.Info("connecting to database", "driver", cfg.Database.Driver)
	database, err := db.OpenWithConfig(cfg.Database)
	if err != nil {
		slog.Error("failed to connect to database", "error", err, "driver", cfg.Database.Driver)
		os.Exit(1)
	}
	defer database.Close()

	// Run migrations
	if !cfg.Database.SkipMigrations {
		slog.Info("running migrations", "migrations_dir", cfg.Database.MigrationsDir)
		if err := migrator.RunMigrations(database.DB, cfg.Database.MigrationsDir); err != nil {
			slog.Error("failed to run migrations", "error", err, "migrations_dir", cfg.Database.MigrationsDir)
			os.Exit(1)
		}

		// Get current schema version
		version, err := migrator.GetCurrentVersion(database.DB)
		if err != nil {
			slog.Error("failed to get schema version", "error", err)
			os.Exit(1)
		}
		slog.Info("database schema ready", "version", version)
	} else {
		slog.Info("skipping migrations", "reason", "configured to skip")
	}

	// TODO: Initialize scheduler with cfg.Scheduler
	// TODO: Start HTTP server for API with cfg.HTTP
	// TODO: Start metrics server with cfg.Metrics

	slog.Info("itinerary is running")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	slog.Info("shutting down gracefully")
	// TODO: Stop scheduler
	// TODO: Stop HTTP server
}
