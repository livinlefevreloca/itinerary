package scheduler

import (
	"fmt"
	"time"
)

// SchedulerConfig defines configuration for the scheduler's main loop and orchestrator management
type SchedulerConfig struct {
	// How far ahead to launch orchestrators before job start time
	PreScheduleInterval time.Duration

	// How often the index builder queries DB and rebuilds index
	IndexRebuildInterval time.Duration

	// How far ahead to calculate scheduled runs
	LookaheadWindow time.Duration

	// How far back to include runs (prevents missing near-past jobs)
	GracePeriod time.Duration

	// Main loop iteration interval
	LoopInterval time.Duration

	// Inbox buffer size
	InboxBufferSize int

	// Timeout for sending to inbox
	InboxSendTimeout time.Duration

	// Orchestrator heartbeat configuration
	OrchestratorHeartbeatInterval   time.Duration
	MaxMissedOrchestratorHeartbeats int
}

// SyncerConfig defines configuration for the syncer's database write buffering
type SyncerConfig struct {
	// Maximum buffered job run updates before stopping
	MaxBufferedJobRunUpdates int

	// Channel buffer sizes
	JobRunChannelSize int
	StatsChannelSize  int

	// Job run flushing - dual mechanism (size OR time triggers flush)
	JobRunFlushThreshold int
	JobRunFlushInterval  time.Duration

	// Stats flushing - dual mechanism (size OR time triggers flush)
	StatsFlushThreshold int
	StatsFlushInterval  time.Duration
}

// DefaultSchedulerConfig returns OLTP-friendly scheduler configuration defaults
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		PreScheduleInterval:             10 * time.Second,
		IndexRebuildInterval:            1 * time.Minute,
		LookaheadWindow:                 10 * time.Minute,
		GracePeriod:                     30 * time.Second,
		LoopInterval:                    1 * time.Second,
		InboxBufferSize:                 10000,
		InboxSendTimeout:                5 * time.Second,
		OrchestratorHeartbeatInterval:   10 * time.Second,
		MaxMissedOrchestratorHeartbeats: 3,
	}
}

// DefaultSyncerConfig returns OLTP-friendly syncer configuration defaults
func DefaultSyncerConfig() SyncerConfig {
	return SyncerConfig{
		MaxBufferedJobRunUpdates: 10000,
		JobRunChannelSize:        200, // OLTP-friendly: smaller batches, more frequent flushes
		StatsChannelSize:         100,
		JobRunFlushThreshold:     100, // OLTP-friendly: half of channel size, reduces lock contention
		JobRunFlushInterval:      1 * time.Second,
		StatsFlushThreshold:      30, // 30 iterations (~30 seconds at 1s loop interval)
		StatsFlushInterval:       30 * time.Second,
	}
}

// validateConfig validates scheduler configuration and returns error if invalid
func validateConfig(config SchedulerConfig) error {
	if config.PreScheduleInterval <= 0 {
		return fmt.Errorf("PreScheduleInterval must be positive, got %v", config.PreScheduleInterval)
	}

	if config.IndexRebuildInterval <= 0 {
		return fmt.Errorf("IndexRebuildInterval must be positive, got %v", config.IndexRebuildInterval)
	}

	if config.LookaheadWindow <= 0 {
		return fmt.Errorf("LookaheadWindow must be positive, got %v", config.LookaheadWindow)
	}

	if config.IndexRebuildInterval >= config.LookaheadWindow {
		return fmt.Errorf("IndexRebuildInterval (%v) must be less than LookaheadWindow (%v)",
			config.IndexRebuildInterval, config.LookaheadWindow)
	}

	if config.GracePeriod <= 0 {
		return fmt.Errorf("GracePeriod must be positive, got %v", config.GracePeriod)
	}

	if config.LoopInterval <= 0 {
		return fmt.Errorf("LoopInterval must be positive, got %v", config.LoopInterval)
	}

	if config.InboxBufferSize <= 0 {
		return fmt.Errorf("InboxBufferSize must be positive, got %d", config.InboxBufferSize)
	}

	if config.InboxSendTimeout <= 0 {
		return fmt.Errorf("InboxSendTimeout must be positive, got %v", config.InboxSendTimeout)
	}

	if config.OrchestratorHeartbeatInterval <= 0 {
		return fmt.Errorf("OrchestratorHeartbeatInterval must be positive, got %v", config.OrchestratorHeartbeatInterval)
	}

	if config.MaxMissedOrchestratorHeartbeats <= 0 {
		return fmt.Errorf("MaxMissedOrchestratorHeartbeats must be positive, got %d", config.MaxMissedOrchestratorHeartbeats)
	}

	return nil
}

// validateSyncerConfig validates syncer configuration and returns error if invalid
func validateSyncerConfig(config SyncerConfig) error {
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
