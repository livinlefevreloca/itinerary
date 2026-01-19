package main

import (
	"flag"
	"log"
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
	dbDriver := flag.String("db-driver", "", "Database driver (sqlite3, postgres, mysql) [overrides config]")
	dbDSN := flag.String("db-dsn", "", "Database connection string [overrides config]")
	migrationsDir := flag.String("migrations-dir", "", "Path to migrations directory [overrides config]")
	skipMigrations := flag.Bool("skip-migrations", false, "Skip running migrations on startup [overrides config]")
	flag.Parse()

	log.Println("Starting Itinerary Job Scheduler")

	// Load configuration
	log.Println("Loading configuration...")
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Apply command-line flag overrides
	if *dbDriver != "" {
		cfg.Database.Driver = *dbDriver
	}
	if *dbDSN != "" {
		cfg.Database.DSN = *dbDSN
	}
	if *migrationsDir != "" {
		cfg.Database.MigrationsDir = *migrationsDir
	}
	if *skipMigrations {
		cfg.Database.SkipMigrations = true
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Log configuration summary
	log.Printf("Database: %s (%s)", cfg.Database.Driver, cfg.Database.DSN)
	log.Printf("Migrations: %s", cfg.Database.MigrationsDir)
	if cfg.HTTP.Enabled {
		log.Printf("HTTP API: %s:%d", cfg.HTTP.Address, cfg.HTTP.Port)
	}
	if cfg.Metrics.Enabled {
		log.Printf("Metrics: %s:%d", cfg.Metrics.Address, cfg.Metrics.Port)
	}

	// Open database connection with pool settings
	log.Printf("Connecting to database (%s)...", cfg.Database.Driver)
	database, err := db.OpenWithConfig(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run migrations
	if !cfg.Database.SkipMigrations {
		log.Printf("Running migrations from %s...", cfg.Database.MigrationsDir)
		if err := migrator.RunMigrations(database.DB, cfg.Database.MigrationsDir); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}

		// Get current schema version
		version, err := migrator.GetCurrentVersion(database.DB)
		if err != nil {
			log.Fatalf("Failed to get schema version: %v", err)
		}
		log.Printf("Database schema version: %d", version)
	} else {
		log.Println("Skipping migrations (configured to skip)")
	}

	// TODO: Initialize scheduler with cfg.Scheduler
	// TODO: Start HTTP server for API with cfg.HTTP
	// TODO: Start metrics server with cfg.Metrics

	log.Println("Itinerary is running. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")
	// TODO: Stop scheduler
	// TODO: Stop HTTP server
}
