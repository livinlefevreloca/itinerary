package scheduler

import (
	"testing"
	"time"
)

func TestDefaultSchedulerConfig(t *testing.T) {
	config := DefaultSchedulerConfig()

	// Verify defaults are non-zero and positive where required
	if config.PreScheduleInterval <= 0 {
		t.Error("PreScheduleInterval must be positive")
	}
	if config.IndexRebuildInterval <= 0 {
		t.Error("IndexRebuildInterval must be positive")
	}
	if config.LookaheadWindow <= 0 {
		t.Error("LookaheadWindow must be positive")
	}
	if config.GracePeriod <= 0 {
		t.Error("GracePeriod must be positive")
	}
	if config.LoopInterval <= 0 {
		t.Error("LoopInterval must be positive")
	}
	if config.InboxBufferSize <= 0 {
		t.Error("InboxBufferSize must be positive")
	}
	if config.InboxSendTimeout <= 0 {
		t.Error("InboxSendTimeout must be positive")
	}
	if config.OrchestratorHeartbeatInterval <= 0 {
		t.Error("OrchestratorHeartbeatInterval must be positive")
	}
	if config.MaxMissedHeartbeats <= 0 {
		t.Error("MaxMissedHeartbeats must be positive")
	}

	// Verify defaults pass validation
	err := validateConfig(config)
	if err != nil {
		t.Errorf("default config should pass validation, got error: %v", err)
	}
}

func TestDefaultSyncerConfig(t *testing.T) {
	config := DefaultSyncerConfig()

	// Verify defaults are non-zero and positive where required
	if config.MaxBufferedJobRunUpdates <= 0 {
		t.Error("MaxBufferedJobRunUpdates must be positive")
	}
	if config.JobRunChannelSize <= 0 {
		t.Error("JobRunChannelSize must be positive")
	}
	if config.StatsChannelSize <= 0 {
		t.Error("StatsChannelSize must be positive")
	}
	if config.JobRunFlushThreshold <= 0 {
		t.Error("JobRunFlushThreshold must be positive")
	}
	if config.JobRunFlushInterval <= 0 {
		t.Error("JobRunFlushInterval must be positive")
	}
	if config.StatsFlushThreshold <= 0 {
		t.Error("StatsFlushThreshold must be positive")
	}
	if config.StatsFlushInterval <= 0 {
		t.Error("StatsFlushInterval must be positive")
	}

	// Verify defaults pass validation
	err := validateSyncerConfig(config)
	if err != nil {
		t.Errorf("default syncer config should pass validation, got error: %v", err)
	}
}

func TestValidateConfig_IndexRebuildInterval_TooLarge(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.IndexRebuildInterval = 15 * time.Minute
	config.LookaheadWindow = 10 * time.Minute

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error when IndexRebuildInterval >= LookaheadWindow")
	}
}

func TestValidateConfig_NegativePreScheduleInterval(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.PreScheduleInterval = -1 * time.Second

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for negative PreScheduleInterval")
	}
}

func TestValidateConfig_NegativeLookaheadWindow(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.LookaheadWindow = -1 * time.Minute

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for negative LookaheadWindow")
	}
}

func TestValidateConfig_NegativeGracePeriod(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.GracePeriod = -1 * time.Second

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for negative GracePeriod")
	}
}

func TestValidateConfig_ZeroInboxBufferSize(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.InboxBufferSize = 0

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for zero InboxBufferSize")
	}
}

func TestValidateConfig_NegativeOrchestratorHeartbeatInterval(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.OrchestratorHeartbeatInterval = -1 * time.Second

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for negative OrchestratorHeartbeatInterval")
	}
}

func TestValidateConfig_ZeroMaxMissedHeartbeats(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.MaxMissedHeartbeats = 0

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for zero MaxMissedHeartbeats")
	}
}

func TestValidateSyncerConfig_ZeroMaxBufferedJobRunUpdates(t *testing.T) {
	config := DefaultSyncerConfig()
	config.MaxBufferedJobRunUpdates = 0

	err := validateSyncerConfig(config)
	if err == nil {
		t.Error("expected error for zero MaxBufferedJobRunUpdates")
	}
}
