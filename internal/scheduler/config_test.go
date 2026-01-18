package scheduler

import (
	"testing"
	"time"
)

// =============================================================================
// Default Configuration Tests
// =============================================================================

// TestDefaultSchedulerConfig verifies that default scheduler configuration has positive values and passes validation.
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
	if config.MaxMissedOrchestratorHeartbeats <= 0 {
		t.Error("MaxMissedOrchestratorHeartbeats must be positive")
	}

	// Verify IndexRebuildInterval < LookaheadWindow (required constraint)
	if config.IndexRebuildInterval >= config.LookaheadWindow {
		t.Error("IndexRebuildInterval must be less than LookaheadWindow")
	}

	// Verify defaults pass validation
	err := validateConfig(config)
	if err != nil {
		t.Errorf("default config should pass validation, got error: %v", err)
	}
}

// TestDefaultSyncerConfig verifies that default syncer configuration has positive values and passes validation.
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

// =============================================================================
// Configuration Validation Tests
// =============================================================================

// TestValidateConfig_IndexRebuildInterval_TooLarge verifies that validation fails when IndexRebuildInterval >= LookaheadWindow.
func TestValidateConfig_IndexRebuildInterval_TooLarge(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.IndexRebuildInterval = 15 * time.Minute
	config.LookaheadWindow = 10 * time.Minute

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error when IndexRebuildInterval >= LookaheadWindow")
	}
}

// TestValidateConfig_NegativePreScheduleInterval verifies that validation fails for negative PreScheduleInterval.
func TestValidateConfig_NegativePreScheduleInterval(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.PreScheduleInterval = -1 * time.Second

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for negative PreScheduleInterval")
	}
}

// TestValidateConfig_NegativeLookaheadWindow verifies that validation fails for negative LookaheadWindow.
func TestValidateConfig_NegativeLookaheadWindow(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.LookaheadWindow = -1 * time.Minute

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for negative LookaheadWindow")
	}
}

// TestValidateConfig_NegativeGracePeriod verifies that validation fails for negative GracePeriod.
func TestValidateConfig_NegativeGracePeriod(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.GracePeriod = -1 * time.Second

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for negative GracePeriod")
	}
}

// TestValidateConfig_ZeroInboxBufferSize verifies that validation fails for zero InboxBufferSize.
func TestValidateConfig_ZeroInboxBufferSize(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.InboxBufferSize = 0

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for zero InboxBufferSize")
	}
}

// TestValidateConfig_NegativeOrchestratorHeartbeatInterval verifies that validation fails for negative OrchestratorHeartbeatInterval.
func TestValidateConfig_NegativeOrchestratorHeartbeatInterval(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.OrchestratorHeartbeatInterval = -1 * time.Second

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for negative OrchestratorHeartbeatInterval")
	}
}

// TestValidateConfig_ZeroMaxMissedOrchestratorHeartbeats verifies that validation fails for zero MaxMissedOrchestratorHeartbeats.
func TestValidateConfig_ZeroMaxMissedOrchestratorHeartbeats(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.MaxMissedOrchestratorHeartbeats = 0

	err := validateConfig(config)
	if err == nil {
		t.Error("expected error for zero MaxMissedOrchestratorHeartbeats")
	}
}

// TestValidateSyncerConfig_ZeroMaxBufferedJobRunUpdates verifies that validation fails for zero MaxBufferedJobRunUpdates.
func TestValidateSyncerConfig_ZeroMaxBufferedJobRunUpdates(t *testing.T) {
	config := DefaultSyncerConfig()
	config.MaxBufferedJobRunUpdates = 0

	err := validateSyncerConfig(config)
	if err == nil {
		t.Error("expected error for zero MaxBufferedJobRunUpdates")
	}
}
