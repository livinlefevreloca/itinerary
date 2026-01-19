package scheduler

import (
	"fmt"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/syncer"
)

// SchedulerConfig defines configuration for the scheduler's main loop and orchestrator management
type SchedulerConfig struct {
	// How far ahead to launch orchestrators before job start time
	PreScheduleInterval time.Duration `toml:"pre_schedule_interval"`

	// How often the index builder queries DB and rebuilds index
	IndexRebuildInterval time.Duration `toml:"index_rebuild_interval"`

	// How far ahead to calculate scheduled runs
	LookaheadWindow time.Duration `toml:"lookahead_window"`

	// How far back to include runs (prevents missing near-past jobs)
	GracePeriod time.Duration `toml:"grace_period"`

	// Main loop iteration interval
	LoopInterval time.Duration `toml:"loop_interval"`

	// Inbox buffer size
	InboxBufferSize int `toml:"inbox_buffer_size"`

	// Timeout for sending to inbox
	InboxSendTimeout time.Duration `toml:"inbox_send_timeout"`

	// Orchestrator heartbeat configuration
	OrchestratorHeartbeatInterval   time.Duration `toml:"orchestrator_heartbeat_interval"`
	MaxMissedOrchestratorHeartbeats int           `toml:"max_missed_orchestrator_heartbeats"`
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
func DefaultSyncerConfig() syncer.Config {
	return syncer.DefaultConfig()
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

