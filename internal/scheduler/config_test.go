package scheduler

import (
	"testing"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/testutil"
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

// TestDefaultJobStateSyncerConfig verifies that default syncer configuration has positive values and passes validation.
func TestDefaultJobStateSyncerConfig(t *testing.T) {
	config := DefaultJobStateSyncerConfig()

	// Verify defaults are non-zero and positive where required
	if config.MaxBufferedUpdates <= 0 {
		t.Error("MaxBufferedUpdates must be positive")
	}
	if config.ChannelSize <= 0 {
		t.Error("ChannelSize must be positive")
	}
	if config.FlushThreshold <= 0 {
		t.Error("FlushThreshold must be positive")
	}
	if config.FlushInterval <= 0 {
		t.Error("FlushInterval must be positive")
	}

	// Verify defaults pass validation by creating a syncer
	logger := testutil.NewTestLogger()
	_, err := NewJobStateSyncer(config, logger.Logger())
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

// TestValidateSyncerConfig_ZeroMaxBufferedUpdates verifies that validation fails for zero MaxBufferedUpdates.
func TestValidateSyncerConfig_ZeroMaxBufferedUpdates(t *testing.T) {
	config := DefaultJobStateSyncerConfig()
	config.MaxBufferedUpdates = 0

	logger := testutil.NewTestLogger()
	_, err := NewJobStateSyncer(config, logger.Logger())
	if err == nil {
		t.Error("expected error for zero MaxBufferedUpdates")
	}
}
