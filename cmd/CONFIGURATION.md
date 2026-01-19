# Configuration

Itinerary can be configured using a combination of configuration files, environment variables, and command-line flags.

## Configuration Precedence

Settings are applied in the following order (later sources override earlier ones):

1. **Built-in defaults**
2. **Configuration file** (TOML format)
3. **Environment variables** (TODO)
4. **Command-line flags**

## Configuration File

### Location

By default, Itinerary looks for configuration files in this order:

1. Path specified via `--config` flag
2. `config.toml` in the current directory
3. Falls back to defaults if no config file is found

### Format

Configuration files use TOML format. See `config.toml.example` for a complete example.

```toml
[database]
driver = "sqlite3"
dsn = "itinerary.db"
max_open_conns = 25
max_idle_conns = 5
conn_max_lifetime = "5m"
conn_max_idle_time = "5m"
migrations_dir = "migrations"
skip_migrations = false

[scheduler]
tick_interval = "100ms"
index_rebuild_interval = "5m"
grace_period = "10s"

[http]
enabled = true
address = "0.0.0.0"
port = 8080

[metrics]
enabled = true
address = "0.0.0.0"
port = 9090

[logging]
level = "info"
format = "text"
```

## Configuration Sections

### Database

Database connection and migration settings.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `driver` | string | `sqlite3` | Database driver: `sqlite3`, `postgres`, or `mysql` |
| `dsn` | string | `itinerary.db` | Database connection string |
| `max_open_conns` | int | `25` | Maximum open connections |
| `max_idle_conns` | int | `5` | Maximum idle connections |
| `conn_max_lifetime` | duration | `5m` | Maximum connection lifetime |
| `conn_max_idle_time` | duration | `5m` | Maximum connection idle time |
| `migrations_dir` | string | `migrations` | Path to migrations directory |
| `skip_migrations` | bool | `false` | Skip running migrations on startup |

**Connection String Examples:**

```toml
# SQLite
dsn = "itinerary.db"              # File-based
dsn = ":memory:"                  # In-memory (testing)

# PostgreSQL
dsn = "postgres://user:pass@localhost/itinerary?sslmode=disable"
dsn = "host=localhost port=5432 user=itinerary password=secret dbname=itinerary sslmode=disable"

# MySQL
dsn = "user:password@tcp(localhost:3306)/itinerary?parseTime=true"
```

### Scheduler

Scheduler timing and behavior settings.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `tick_interval` | duration | `100ms` | How often the scheduler checks for jobs to run |
| `index_rebuild_interval` | duration | `5m` | How often to rebuild the index from the database |
| `grace_period` | duration | `10s` | Grace period for late jobs before marking them as missed |

### HTTP

HTTP API server settings.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | bool | `true` | Enable HTTP API server |
| `address` | string | `0.0.0.0` | Address to bind to (`0.0.0.0` for all interfaces) |
| `port` | int | `8080` | Port for HTTP API |

### Metrics

Metrics/monitoring endpoint settings.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | bool | `true` | Enable metrics endpoint |
| `address` | string | `0.0.0.0` | Address to bind to |
| `port` | int | `9090` | Port for metrics endpoint |

### Logging

Logging configuration.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `level` | string | `info` | Log level: `debug`, `info`, `warn`, or `error` |
| `format` | string | `text` | Log format: `text` or `json` |

## Command-Line Flags

Command-line flags override configuration file settings.

```bash
./itinerary [flags]
```

| Flag | Description |
|------|-------------|
| `--config` | Path to configuration file (TOML) |
| `--db-driver` | Database driver (overrides config) |
| `--db-dsn` | Database connection string (overrides config) |
| `--migrations-dir` | Path to migrations directory (overrides config) |
| `--skip-migrations` | Skip running migrations on startup (overrides config) |

## Usage Examples

### Using Config File

```bash
# Use config.toml in current directory
./itinerary

# Use specific config file
./itinerary --config=/etc/itinerary/prod.toml
```

### Override Specific Settings

```bash
# Use config file but override database
./itinerary --config=config.toml --db-dsn=postgres://localhost/prod

# Skip migrations for debugging
./itinerary --config=config.toml --skip-migrations
```

### Defaults Only (No Config File)

```bash
# Run with built-in defaults
./itinerary

# Override specific defaults
./itinerary --db-driver=postgres --db-dsn="postgres://localhost/itinerary"
```

## Environment-Specific Configurations

Create separate config files for different environments:

```bash
config/
├── dev.toml        # Development settings
├── staging.toml    # Staging settings
└── prod.toml       # Production settings
```

Then specify which to use:

```bash
# Development
./itinerary --config=config/dev.toml

# Production
./itinerary --config=config/prod.toml
```

### Example: Development Config

```toml
# config/dev.toml
[database]
driver = "sqlite3"
dsn = "dev.db"

[scheduler]
tick_interval = "1s"  # Slower for easier debugging

[logging]
level = "debug"       # Verbose logging
format = "text"
```

### Example: Production Config

```toml
# config/prod.toml
[database]
driver = "postgres"
dsn = "postgres://itinerary:${DB_PASSWORD}@db.prod.internal/itinerary?sslmode=require"
max_open_conns = 50
max_idle_conns = 10
conn_max_lifetime = "15m"

[scheduler]
tick_interval = "100ms"
index_rebuild_interval = "10m"

[http]
enabled = true
address = "0.0.0.0"
port = 8080

[metrics]
enabled = true
address = "0.0.0.0"
port = 9090

[logging]
level = "info"
format = "json"       # Structured logging for production
```

## Validation

The application validates all configuration settings on startup. Invalid settings cause the application to fail immediately with a clear error message:

```bash
$ ./itinerary --config=bad.toml
2026/01/19 14:24:16 Starting Itinerary Job Scheduler
2026/01/19 14:24:16 Loading configuration...
2026/01/19 14:24:16 Invalid configuration: unsupported database driver: oracle (must be sqlite3, postgres, or mysql)
```

Common validation errors:
- Invalid database driver
- Empty connection string
- Invalid port numbers
- Unknown log level
- Negative or zero duration values

## Security Considerations

### Sensitive Data

**Never commit sensitive data to version control:**
- Database passwords
- API keys
- Connection strings with credentials

**Best practices:**
1. Use environment variables for secrets (TODO: implement)
2. Use separate config files for production (not in git)
3. Use restrictive file permissions: `chmod 600 config.toml`

### File Permissions

Recommended permissions for config files:

```bash
# Owner read/write only
chmod 600 config.toml

# Owner read only (production)
chmod 400 /etc/itinerary/prod.toml
```

## Troubleshooting

### Config File Not Found

If you specify a config file that doesn't exist, the application will fail:

```bash
$ ./itinerary --config=missing.toml
2026/01/19 14:24:16 Failed to load configuration: config file does not exist: missing.toml
```

### Invalid TOML Syntax

Parse errors will show the specific issue:

```bash
$ ./itinerary --config=bad.toml
2026/01/19 14:24:16 Failed to load configuration: failed to parse config file: Near line 5 (last key parsed 'database.dsn'): expected value but found "}" instead
```

### Config Verification

To verify your configuration is valid, run with `--help` first - the app will load and validate the config before showing help:

```bash
# This validates config.toml before showing help
./itinerary --config=config.toml --help
```

Or check the startup logs for the configuration summary:

```bash
$ ./itinerary --config=config.toml
2026/01/19 14:24:16 Starting Itinerary Job Scheduler
2026/01/19 14:24:16 Loading configuration...
2026/01/19 14:24:16 Database: sqlite3 (itinerary.db)
2026/01/19 14:24:16 Migrations: migrations
2026/01/19 14:24:16 HTTP API: 0.0.0.0:8080
2026/01/19 14:24:16 Metrics: 0.0.0.0:9090
```
