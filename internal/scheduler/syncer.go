package scheduler

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Syncer handles all database write operations and buffering
type Syncer struct {
	// Configuration
	config SyncerConfig
	logger *slog.Logger

	// Job run update buffering
	jobRunUpdateBuffer []JobRunUpdate
	jobRunChannel      chan JobRunUpdate
	lastJobRunFlush    time.Time

	// Stats buffering
	statsBuffer    []SchedulerIterationStats
	statsChannel   chan StatsUpdate
	lastStatsFlush time.Time

	// Control
	shutdown chan struct{}
	wg       sync.WaitGroup // Tracks background goroutines (syncers only)
}

// SyncerStats provides current syncer buffer statistics
type SyncerStats struct {
	BufferedJobRunUpdates int
	BufferedStats         int
}

// NewSyncer creates a new syncer with the specified configuration
func NewSyncer(config SyncerConfig, logger *slog.Logger) (*Syncer, error) {
	if err := validateSyncerConfig(config); err != nil {
		return nil, err
	}

	return &Syncer{
		config:             config,
		logger:             logger,
		jobRunUpdateBuffer: make([]JobRunUpdate, 0),
		jobRunChannel:      make(chan JobRunUpdate, config.JobRunChannelSize),
		lastJobRunFlush:    time.Now(),
		statsBuffer:        make([]SchedulerIterationStats, 0),
		statsChannel:       make(chan StatsUpdate, config.StatsChannelSize),
		lastStatsFlush:     time.Now(),
		shutdown:           make(chan struct{}),
	}, nil
}

// BufferJobRunUpdate adds an update to the buffer
// Returns error if buffer exceeds maximum allowed size
func (s *Syncer) BufferJobRunUpdate(update JobRunUpdate) error {
	s.jobRunUpdateBuffer = append(s.jobRunUpdateBuffer, update)

	// Check if buffer exceeded maximum
	if len(s.jobRunUpdateBuffer) > s.config.MaxBufferedJobRunUpdates {
		return fmt.Errorf("job run update buffer exceeded maximum size: %d > %d",
			len(s.jobRunUpdateBuffer), s.config.MaxBufferedJobRunUpdates)
	}

	return nil
}

// FlushJobRunUpdates sends all buffered updates to the syncer channel
// Returns error if channel is full (caller should log and handle)
func (s *Syncer) FlushJobRunUpdates() error {
	if len(s.jobRunUpdateBuffer) == 0 {
		return nil
	}

	for _, update := range s.jobRunUpdateBuffer {
		select {
		case s.jobRunChannel <- update:
			// Successfully sent
		default:
			return fmt.Errorf("job run channel full, %d updates buffered", len(s.jobRunUpdateBuffer))
		}
	}

	// All sent, clear buffer and update timestamp
	s.jobRunUpdateBuffer = make([]JobRunUpdate, 0)
	s.lastJobRunFlush = time.Now()
	return nil
}

// BufferStats adds iteration stats to the buffer
func (s *Syncer) BufferStats(stats SchedulerIterationStats) {
	s.statsBuffer = append(s.statsBuffer, stats)
}

// FlushStats sends all buffered stats to the syncer channel
// Returns error if channel is full (caller should log and handle)
func (s *Syncer) FlushStats() error {
	if len(s.statsBuffer) == 0 {
		return nil
	}

	update := StatsUpdate{
		Stats: s.statsBuffer,
	}

	select {
	case s.statsChannel <- update:
		s.statsBuffer = make([]SchedulerIterationStats, 0)
		s.lastStatsFlush = time.Now()
		return nil
	default:
		return fmt.Errorf("stats channel full, %d stats buffered", len(s.statsBuffer))
	}
}

// GetStats returns current syncer statistics
func (s *Syncer) GetStats() SyncerStats {
	return SyncerStats{
		BufferedJobRunUpdates: len(s.jobRunUpdateBuffer),
		BufferedStats:         len(s.statsBuffer),
	}
}

// Start launches background goroutines for syncing
func (s *Syncer) Start(db interface{}) {
	s.wg.Add(2) // 2 syncer goroutines

	go s.runJobRunSyncer(db)
	go s.runStatsSyncer(db)
}

// runJobRunSyncer writes job run updates to the database
func (s *Syncer) runJobRunSyncer(db interface{}) {
	defer s.wg.Done()

	for update := range s.jobRunChannel {
		err := writeJobRunUpdate(db, update)

		if err != nil {
			s.logger.Error("failed to write job run update",
				"update_id", update.UpdateID,
				"run_id", update.RunID,
				"error", err)
		} else {
			s.logger.Debug("wrote job run update",
				"update_id", update.UpdateID,
				"run_id", update.RunID)
		}
	}

	s.logger.Debug("job run syncer shut down")
}

// runStatsSyncer writes stats updates to the database
func (s *Syncer) runStatsSyncer(db interface{}) {
	defer s.wg.Done()

	for update := range s.statsChannel {
		err := writeStatsUpdate(db, update)
		if err != nil {
			// Just log, stats writes are not critical
			s.logger.Error("failed to write stats update", "error", err)
		}
	}

	s.logger.Debug("stats syncer shut down")
}

// Shutdown performs graceful shutdown ensuring all data is persisted
func (s *Syncer) Shutdown() error {
	s.logger.Info("starting syncer shutdown")

	// Step 1: Signal shutdown (for future use if needed)
	close(s.shutdown)
	s.logger.Debug("shutdown signal sent")

	// Step 2: Final flush of any remaining buffered items
	// This adds final items to channels before we close them
	s.logger.Debug("performing final flush",
		"job_run_updates", len(s.jobRunUpdateBuffer),
		"stats", len(s.statsBuffer))

	if err := s.FlushJobRunUpdates(); err != nil {
		s.logger.Warn("failed to flush job run updates on shutdown", "error", err)
	}

	if err := s.FlushStats(); err != nil {
		s.logger.Warn("failed to flush stats on shutdown", "error", err)
	}

	// Step 3: Close write channels to signal syncers "no more data"
	// The syncers use "for range" which will:
	// - Process all buffered items in the channel
	// - Exit when channel is closed and drained
	// This is Go's standard pattern for signaling completion
	s.logger.Debug("closing write channels")
	close(s.jobRunChannel)
	close(s.statsChannel)

	// Step 4: Wait for syncer goroutines to finish draining and exit
	// This ensures syncers have drained all items from channels
	s.logger.Debug("waiting for syncer goroutines to exit")
	s.wg.Wait()

	s.logger.Info("syncer shutdown complete")
	return nil
}

// Helper functions for database writes

// writeJobRunUpdate writes a job run update to the database
func writeJobRunUpdate(db interface{}, update JobRunUpdate) error {
	// Use type assertion to handle MockDB for testing
	type jobRunWriter interface {
		WriteJobRunUpdate(update interface{}) error
	}

	if writer, ok := db.(jobRunWriter); ok {
		return writer.WriteJobRunUpdate(update)
	}

	// TODO: Actual database write for production
	return nil
}

// writeStatsUpdate writes a stats update to the database
func writeStatsUpdate(db interface{}, update StatsUpdate) error {
	// Use type assertion to handle MockDB for testing
	type statsWriter interface {
		WriteStatsUpdate(update interface{}) error
	}

	if writer, ok := db.(statsWriter); ok {
		return writer.WriteStatsUpdate(update)
	}

	// TODO: Actual database write for production
	return nil
}
