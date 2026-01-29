package scheduler

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// DatabaseWriter defines the interface for writing to the database
type DatabaseWriter interface {
	WriteJobRunUpdate(update JobRunUpdate) error
}

// JobStateSyncer handles buffered writes of job run state updates to the database
type JobStateSyncer struct {
	// Configuration
	config JobStateSyncerConfig
	logger *slog.Logger
	db     DatabaseWriter

	// Job run update buffering
	jobRunUpdateBuffer []JobRunUpdate
	jobRunChannel      chan JobRunUpdate
	lastJobRunFlush    time.Time

	// Control
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// NewJobStateSyncer creates a new job state syncer
func NewJobStateSyncer(config JobStateSyncerConfig, logger *slog.Logger) (*JobStateSyncer, error) {
	if err := validateJobStateSyncerConfig(config); err != nil {
		return nil, err
	}

	return &JobStateSyncer{
		config:             config,
		logger:             logger,
		db:                 nil, // Set via Start()
		jobRunUpdateBuffer: make([]JobRunUpdate, 0),
		jobRunChannel:      make(chan JobRunUpdate, config.ChannelSize),
		lastJobRunFlush:    time.Now(),
		shutdown:           make(chan struct{}),
	}, nil
}

// BufferJobRunUpdate adds an update to the buffer
// Returns error if buffer exceeds maximum allowed size
func (s *JobStateSyncer) BufferJobRunUpdate(update JobRunUpdate) error {
	s.jobRunUpdateBuffer = append(s.jobRunUpdateBuffer, update)

	// Check if buffer exceeded maximum
	if len(s.jobRunUpdateBuffer) > s.config.MaxBufferedUpdates {
		return fmt.Errorf("job run update buffer exceeded maximum size: %d > %d",
			len(s.jobRunUpdateBuffer), s.config.MaxBufferedUpdates)
	}

	return nil
}

// FlushJobRunUpdates sends all buffered updates to the syncer channel
// Returns error if channel is full (caller should log and handle)
func (s *JobStateSyncer) FlushJobRunUpdates() error {
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

// GetStats returns current job state syncer statistics
func (s *JobStateSyncer) GetStats() JobStateSyncerStats {
	return JobStateSyncerStats{
		BufferedJobRunUpdates: len(s.jobRunUpdateBuffer),
	}
}

// GetLastFlushTime returns the timestamp of the last flush
func (s *JobStateSyncer) GetLastFlushTime() time.Time {
	return s.lastJobRunFlush
}

// Start launches the background writer goroutine
func (s *JobStateSyncer) Start(db DatabaseWriter) {
	s.db = db
	s.wg.Add(1)
	go s.runWriter()
}

// runWriter writes job run updates to the database
func (s *JobStateSyncer) runWriter() {
	defer s.wg.Done()

	for update := range s.jobRunChannel {
		err := s.writeJobRunUpdate(update)

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

	s.logger.Debug("job state syncer shut down")
}

// writeJobRunUpdate writes a job run update to the database
func (s *JobStateSyncer) writeJobRunUpdate(update JobRunUpdate) error {
	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return s.db.WriteJobRunUpdate(update)
}

// Shutdown performs graceful shutdown ensuring all data is persisted
func (s *JobStateSyncer) Shutdown() error {
	s.logger.Info("starting job state syncer shutdown")

	// Signal shutdown
	close(s.shutdown)
	s.logger.Debug("shutdown signal sent")

	// Final flush of any remaining buffered items
	s.logger.Debug("performing final flush",
		"job_run_updates", len(s.jobRunUpdateBuffer))

	if err := s.FlushJobRunUpdates(); err != nil {
		s.logger.Warn("failed to flush job run updates on shutdown", "error", err)
	}

	// Close write channel to signal writer "no more data"
	s.logger.Debug("closing write channel")
	close(s.jobRunChannel)

	// Wait for writer goroutine to finish draining and exit
	s.logger.Debug("waiting for writer goroutine to exit")
	s.wg.Wait()

	s.logger.Info("job state syncer shutdown complete")
	return nil
}
