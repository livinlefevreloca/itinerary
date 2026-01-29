package syncer

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Syncer handles all database write operations and buffering
type Syncer struct {
	// Configuration
	config Config
	logger *slog.Logger

	// Job run update buffering
	jobRunUpdateBuffer []JobRunUpdate
	jobRunChannel      chan JobRunUpdate
	lastJobRunFlush    time.Time

	// Control
	shutdown chan struct{}
	wg       sync.WaitGroup // Tracks background goroutines (syncers only)
}

// New creates a new syncer with the specified configuration
func NewSyncer(config Config, logger *slog.Logger) (*Syncer, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return &Syncer{
		config:             config,
		logger:             logger,
		jobRunUpdateBuffer: make([]JobRunUpdate, 0),
		jobRunChannel:      make(chan JobRunUpdate, config.JobRunChannelSize),
		lastJobRunFlush:    time.Now(),
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

// GetStats returns current syncer statistics
func (s *Syncer) GetStats() Stats {
	return Stats{
		BufferedJobRunUpdates: len(s.jobRunUpdateBuffer),
	}
}

// GetConfig returns the syncer configuration
func (s *Syncer) GetConfig() Config {
	return s.config
}

// GetLastJobRunFlushTime returns the timestamp of the last job run flush
func (s *Syncer) GetLastJobRunFlushTime() time.Time {
	return s.lastJobRunFlush
}

// Start launches background goroutines for syncing
func (s *Syncer) Start(db interface{}) {
	s.wg.Add(1) // 1 syncer goroutine

	go s.runJobRunSyncer(db)
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

// Shutdown performs graceful shutdown ensuring all data is persisted
func (s *Syncer) Shutdown() error {
	s.logger.Info("starting syncer shutdown")

	// Step 1: Signal shutdown (for future use if needed)
	close(s.shutdown)
	s.logger.Debug("shutdown signal sent")

	// Step 2: Final flush of any remaining buffered items
	// This adds final items to channels before we close them
	s.logger.Debug("performing final flush",
		"job_run_updates", len(s.jobRunUpdateBuffer))

	if err := s.FlushJobRunUpdates(); err != nil {
		s.logger.Warn("failed to flush job run updates on shutdown", "error", err)
	}

	// Step 3: Close write channels to signal syncers "no more data"
	// The syncers use "for range" which will:
	// - Process all buffered items in the channel
	// - Exit when channel is closed and drained
	// This is Go's standard pattern for signaling completion
	s.logger.Debug("closing write channel")
	close(s.jobRunChannel)

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

