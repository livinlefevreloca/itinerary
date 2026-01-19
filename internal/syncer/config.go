package syncer

import (
	"fmt"
	"time"
)

// Config defines configuration for the syncer's database write buffering
type Config struct {
	// Maximum buffered job run updates before stopping
	MaxBufferedJobRunUpdates int `toml:"max_buffered_job_run_updates"`

	// Channel buffer sizes
	JobRunChannelSize int `toml:"job_run_channel_size"`
	StatsChannelSize  int `toml:"stats_channel_size"`

	// Job run flushing - dual mechanism (size OR time triggers flush)
	JobRunFlushThreshold int           `toml:"job_run_flush_threshold"`
	JobRunFlushInterval  time.Duration `toml:"job_run_flush_interval"`

	// Stats flushing - dual mechanism (size OR time triggers flush)
	StatsFlushThreshold int           `toml:"stats_flush_threshold"`
	StatsFlushInterval  time.Duration `toml:"stats_flush_interval"`
}

// DefaultConfig returns OLTP-friendly syncer configuration defaults
func DefaultConfig() Config {
	return Config{
		MaxBufferedJobRunUpdates: 10000,
		JobRunChannelSize:        200, // OLTP-friendly: smaller batches, more frequent flushes
		StatsChannelSize:         100,
		JobRunFlushThreshold:     100, // OLTP-friendly: half of channel size, reduces lock contention
		JobRunFlushInterval:      1 * time.Second,
		StatsFlushThreshold:      30, // 30 iterations (~30 seconds at 1s loop interval)
		StatsFlushInterval:       30 * time.Second,
	}
}

// validateConfig validates syncer configuration and returns error if invalid
func validateConfig(config Config) error {
	if config.MaxBufferedJobRunUpdates <= 0 {
		return fmt.Errorf("MaxBufferedJobRunUpdates must be positive, got %d", config.MaxBufferedJobRunUpdates)
	}

	if config.JobRunChannelSize <= 0 {
		return fmt.Errorf("JobRunChannelSize must be positive, got %d", config.JobRunChannelSize)
	}

	if config.StatsChannelSize <= 0 {
		return fmt.Errorf("StatsChannelSize must be positive, got %d", config.StatsChannelSize)
	}

	if config.JobRunFlushThreshold <= 0 {
		return fmt.Errorf("JobRunFlushThreshold must be positive, got %d", config.JobRunFlushThreshold)
	}

	if config.JobRunFlushInterval <= 0 {
		return fmt.Errorf("JobRunFlushInterval must be positive, got %v", config.JobRunFlushInterval)
	}

	if config.StatsFlushThreshold <= 0 {
		return fmt.Errorf("StatsFlushThreshold must be positive, got %d", config.StatsFlushThreshold)
	}

	if config.StatsFlushInterval <= 0 {
		return fmt.Errorf("StatsFlushInterval must be positive, got %v", config.StatsFlushInterval)
	}

	return nil
}
