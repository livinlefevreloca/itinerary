# Itinerary Command-Line Application

This directory contains the main Itinerary application entry point.

## Quick Start

```bash
# Build the application
go build ./cmd/itinerary

# Run with defaults
./itinerary

# Run with config file
./itinerary --config=config.toml
```

## Documentation

- **[STARTUP.md](STARTUP.md)** - Application startup sequence and usage
- **[CONFIGURATION.md](CONFIGURATION.md)** - Complete configuration reference

## Directory Structure

```
cmd/
├── itinerary/          # Application entry point
│   └── main.go         # Main function
├── CONFIGURATION.md    # Configuration documentation
├── STARTUP.md          # Startup and usage guide
└── README.md           # This file
```

## Configuration Files

Configuration files should be placed in the project root or specified via `--config` flag:

```bash
# Project root
config.toml              # Default config file
config.toml.example      # Example/template config

# Environment-specific
config/
├── dev.toml
├── staging.toml
└── prod.toml
```

## Building

### Development Build

```bash
go build ./cmd/itinerary
./itinerary --config=config.toml
```

### Production Build

```bash
# With optimizations
go build -ldflags="-s -w" -o itinerary ./cmd/itinerary

# With version info
VERSION=$(git describe --tags --always --dirty)
go build -ldflags="-s -w -X main.version=$VERSION" -o itinerary ./cmd/itinerary
```

### Cross-Compilation

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o itinerary-linux-amd64 ./cmd/itinerary

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o itinerary-linux-arm64 ./cmd/itinerary

# macOS ARM64 (M1/M2)
GOOS=darwin GOARCH=arm64 go build -o itinerary-darwin-arm64 ./cmd/itinerary
```

## Dependencies

The application requires:
- Go 1.21 or later
- Database driver for your chosen database:
  - SQLite: `github.com/mattn/go-sqlite3` (included)
  - PostgreSQL: `github.com/lib/pq` (install separately if needed)
  - MySQL: `github.com/go-sql-driver/mysql` (install separately if needed)

## Features

- **Automatic database migrations** on startup
- **TOML configuration** with command-line overrides
- **Multiple database support** (SQLite, PostgreSQL, MySQL)
- **Connection pooling** with configurable limits
- **Graceful shutdown** with signal handling
- **Configuration validation** before startup

## Next Steps

After building and running the application:

1. Check the logs to verify configuration and database connection
2. Verify migrations completed successfully
3. Access HTTP API at configured port (default: 8080)
4. Access metrics at configured port (default: 9090)

See [STARTUP.md](STARTUP.md) for detailed startup information and troubleshooting.
