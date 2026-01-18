package scheduler

import (
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/cron"
	"github.com/livinlefevreloca/itinerary/internal/scheduler/index"
)

// Scheduler is the main scheduler component that coordinates all job orchestrators
type Scheduler struct {
	// Configuration
	config SchedulerConfig
	logger *slog.Logger

	// State
	index               *index.ScheduledRunIndex
	activeOrchestrators map[string]*OrchestratorState // runID → state

	// Stats (accessed only by main loop)
	indexBuildCount        int
	lastIndexBuildDuration time.Duration
	missedHeartbeatCount   int

	// Communication
	inbox  *Inbox
	syncer *Syncer

	// Control
	shutdown         chan struct{}
	rebuildIndexChan chan struct{}
}

// NewScheduler creates a new scheduler instance with validated configuration
func NewScheduler(config SchedulerConfig, syncerConfig SyncerConfig, db interface{}, logger *slog.Logger) (*Scheduler, error) {
	// 1. Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	// 2. Create inbox
	inbox := NewInbox(config.InboxBufferSize, config.InboxSendTimeout, logger)

	// 3. Create syncer
	syncer, err := NewSyncer(syncerConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create syncer: %w", err)
	}

	// 4. Initialize scheduler with empty index
	s := &Scheduler{
		config:              config,
		logger:              logger,
		index:               index.NewScheduledRunIndex([]index.ScheduledRun{}),
		activeOrchestrators: make(map[string]*OrchestratorState),
		inbox:               inbox,
		syncer:              syncer,
		shutdown:            make(chan struct{}),
		rebuildIndexChan:    make(chan struct{}, 1),
	}

	// 5. Build initial index (synchronously on startup)
	if err := s.performIndexBuild(db); err != nil {
		return nil, fmt.Errorf("failed to build initial index: %w", err)
	}

	return s, nil
}

// Start starts the scheduler's background goroutines and main loop
func (s *Scheduler) Start(db interface{}) {
	s.logger.Info("starting scheduler")

	// Start syncer background goroutines
	s.syncer.Start(db)

	// Start index builder
	go s.runIndexBuilder(db)

	// Start main loop
	s.run()
}

// Shutdown sends a shutdown signal to the scheduler
func (s *Scheduler) Shutdown() {
	close(s.shutdown)
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	ticker := time.NewTicker(s.config.LoopInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdown:
			s.handleShutdown()
			return

		case <-ticker.C:
			s.iteration()
		}
	}
}

// iteration performs a single iteration of the scheduler loop
func (s *Scheduler) iteration() {
	start := time.Now()

	// Step 1: Schedule new orchestrators
	if err := s.scheduleOrchestrators(start); err != nil {
		s.logger.Error("scheduler stopping due to buffer overflow", "error", err)
		close(s.shutdown)
		return
	}

	// Step 2: Process ALL inbox messages
	messagesProcessed := s.processInbox()

	// Step 3: Check for missed heartbeats and mark orphaned
	s.checkHeartbeats(start)

	// Step 4: Clean up completed orchestrators
	s.cleanupOrchestrators(start)

	// Step 5: Record iteration statistics
	s.recordIterationStats(start, messagesProcessed)

	// Step 6: Flush buffers based on time and size thresholds
	s.flushBuffers(start)

	// Step 7: Update inbox depth stats
	s.inbox.UpdateDepthStats()
}

// scheduleOrchestrators queries the index and launches orchestrators for jobs that need to run
func (s *Scheduler) scheduleOrchestrators(now time.Time) error {
	// Query for jobs to schedule in the next PRE_SCHEDULE_INTERVAL
	start := now
	end := now.Add(s.config.PreScheduleInterval)

	runs := s.index.Query(start, end)

	for _, run := range runs {
		// Generate deterministic runID from jobID and scheduled time
		// Format: "jobID:unixTimestamp" (e.g., "job123:1704067200")
		runID := generateRunID(run.JobID, run.ScheduledAt)

		if _, exists := s.activeOrchestrators[runID]; exists {
			continue
		}

		// Create orchestrator state
		cancelChan := make(chan struct{})
		configUpdateChan := make(chan *Job, 1)

		state := &OrchestratorState{
			RunID:         runID,
			JobID:         run.JobID,
			ScheduledAt:   run.ScheduledAt,
			Status:        OrchestratorPreRun,
			CancelChan:    cancelChan,
			ConfigUpdate:  configUpdateChan,
			LastHeartbeat: now, // Initialize to now to prevent immediate orphaning
		}

		s.activeOrchestrators[runID] = state

		// Launch orchestrator goroutine
		go s.runOrchestrator(run.JobID, run.ScheduledAt, runID, cancelChan, configUpdateChan)

		// Buffer job run update
		if err := s.syncer.BufferJobRunUpdate(JobRunUpdate{
			UpdateID:    generateUpdateID(),
			RunID:       runID,
			JobID:       run.JobID,
			ScheduledAt: run.ScheduledAt,
			Status:      OrchestratorPreRun.String(),
		}); err != nil {
			return err
		}
	}

	return nil
}

// processInbox drains all available messages from the inbox
func (s *Scheduler) processInbox() int {
	messagesProcessed := 0

	// Process all available messages (non-blocking)
	for {
		msg, ok := s.inbox.TryReceive()
		if !ok {
			break
		}

		s.handleMessage(msg)
		messagesProcessed++
	}

	return messagesProcessed
}

// handleMessage dispatches messages to appropriate handlers
func (s *Scheduler) handleMessage(msg InboxMessage) {
	s.logger.Debug("handling message", "type", msg.Type.String())

	switch msg.Type {
	case MsgOrchestratorStateChange:
		s.handleOrchestratorStateChange(msg)
	case MsgOrchestratorComplete:
		s.handleOrchestratorComplete(msg)
	case MsgOrchestratorFailed:
		s.handleOrchestratorFailed(msg)
	case MsgOrchestratorHeartbeat:
		s.handleOrchestratorHeartbeat(msg)
	case MsgCancelRun:
		s.handleCancelRun(msg)
	case MsgUpdateRunConfig:
		s.handleUpdateRunConfig(msg)
	case MsgGetOrchestratorState:
		s.handleGetOrchestratorState(msg)
	case MsgGetAllActiveRuns:
		s.handleGetAllActiveRuns(msg)
	case MsgGetStats:
		s.handleGetStats(msg)
	case MsgShutdown:
		close(s.shutdown)
	default:
		s.logger.Warn("unknown message type", "type", msg.Type)
	}
}

// handleOrchestratorHeartbeat updates heartbeat tracking for an orchestrator
func (s *Scheduler) handleOrchestratorHeartbeat(msg InboxMessage) {
	data := msg.Data.(OrchestratorHeartbeatMsg)

	state, exists := s.activeOrchestrators[data.RunID]
	if !exists {
		return
	}

	// Update heartbeat tracking
	state.LastHeartbeat = data.Timestamp
	state.MissedHeartbeats = 0 // Reset counter on successful heartbeat
}

// handleOrchestratorStateChange updates orchestrator status and buffers database update
func (s *Scheduler) handleOrchestratorStateChange(msg InboxMessage) {
	data := msg.Data.(OrchestratorStateChangeMsg)

	state, exists := s.activeOrchestrators[data.RunID]
	if !exists {
		return
	}

	state.Status = data.NewStatus

	// Buffer update (ignore error - will be caught in scheduleOrchestrators)
	s.syncer.BufferJobRunUpdate(JobRunUpdate{
		UpdateID: generateUpdateID(),
		RunID:    data.RunID,
		Status:   data.NewStatus.String(),
	})
}

// handleOrchestratorComplete marks orchestrator as completed and buffers database update
func (s *Scheduler) handleOrchestratorComplete(msg InboxMessage) {
	data := msg.Data.(OrchestratorCompleteMsg)

	state, exists := s.activeOrchestrators[data.RunID]
	if !exists {
		return
	}

	state.Status = OrchestratorCompleted
	state.CompletedAt = data.CompletedAt

	// Buffer update (ignore error - will be caught in scheduleOrchestrators)
	s.syncer.BufferJobRunUpdate(JobRunUpdate{
		UpdateID:    generateUpdateID(),
		RunID:       data.RunID,
		CompletedAt: data.CompletedAt,
		Status:      OrchestratorCompleted.String(),
		Success:     data.Success,
		Error:       data.Error,
	})
}

// handleOrchestratorFailed marks orchestrator as failed and buffers database update
func (s *Scheduler) handleOrchestratorFailed(msg InboxMessage) {
	data := msg.Data.(OrchestratorCompleteMsg)

	state, exists := s.activeOrchestrators[data.RunID]
	if !exists {
		return
	}

	state.Status = OrchestratorFailed
	state.CompletedAt = data.CompletedAt

	// Buffer update (ignore error - will be caught in scheduleOrchestrators)
	s.syncer.BufferJobRunUpdate(JobRunUpdate{
		UpdateID:    generateUpdateID(),
		RunID:       data.RunID,
		CompletedAt: data.CompletedAt,
		Status:      OrchestratorFailed.String(),
		Success:     false,
		Error:       data.Error,
	})
}

// handleCancelRun cancels a running orchestrator
func (s *Scheduler) handleCancelRun(msg InboxMessage) {
	data := msg.Data.(CancelRunMsg)

	state, exists := s.activeOrchestrators[data.RunID]
	if !exists {
		s.logger.Warn("attempted to cancel non-existent run", "run_id", data.RunID)
		return
	}

	// Signal cancellation to orchestrator
	close(state.CancelChan)
	state.Status = OrchestratorCancelled

	s.logger.Info("cancelled run", "run_id", data.RunID)
}

// handleUpdateRunConfig updates the configuration for an orchestrator in PreRun state
func (s *Scheduler) handleUpdateRunConfig(msg InboxMessage) {
	data := msg.Data.(UpdateRunConfigMsg)

	state, exists := s.activeOrchestrators[data.RunID]
	if !exists {
		s.logger.Warn("attempted to update config for non-existent run",
			"run_id", data.RunID)
		return
	}

	// Can only update config in PreRun state
	if state.Status != OrchestratorPreRun {
		s.logger.Warn("attempted to update config for run not in PreRun state",
			"run_id", data.RunID,
			"status", state.Status.String())
		return
	}

	// Send new config to orchestrator
	select {
	case state.ConfigUpdate <- data.NewConfig:
		state.JobConfig = data.NewConfig
		s.logger.Info("updated run config",
			"run_id", data.RunID,
			"job_id", data.NewConfig.ID)
	default:
		s.logger.Warn("failed to send config update to orchestrator",
			"run_id", data.RunID)
	}
}

// handleGetOrchestratorState returns the state of a specific orchestrator
func (s *Scheduler) handleGetOrchestratorState(msg InboxMessage) {
	data := msg.Data.(GetOrchestratorStateMsg)

	state, found := s.activeOrchestrators[data.RunID]

	response := OrchestratorStateResponse{
		State: state,
		Found: found,
	}

	if msg.ResponseChan != nil {
		msg.ResponseChan <- response
	}
}

// handleGetAllActiveRuns returns all active orchestrator states
func (s *Scheduler) handleGetAllActiveRuns(msg InboxMessage) {
	runs := make([]*OrchestratorState, 0, len(s.activeOrchestrators))
	for _, state := range s.activeOrchestrators {
		runs = append(runs, state)
	}

	response := AllActiveRunsResponse{
		Runs: runs,
	}

	if msg.ResponseChan != nil {
		msg.ResponseChan <- response
	}
}

// handleGetStats returns current scheduler statistics
func (s *Scheduler) handleGetStats(msg InboxMessage) {
	response := StatsResponse{
		SchedulerStats: SchedulerStats{
			ActiveOrchestratorCount: len(s.activeOrchestrators),
			IndexSize:               s.index.Len(),
		},
		InboxStats:  s.inbox.Stats(),
		SyncerStats: s.syncer.GetStats(),
	}

	if msg.ResponseChan != nil {
		msg.ResponseChan <- response
	}
}

// checkHeartbeats checks for orchestrators with missed heartbeats and marks them as orphaned
func (s *Scheduler) checkHeartbeats(now time.Time) {
	for runID, state := range s.activeOrchestrators {
		// Only check heartbeats for non-terminal states
		// Skip terminal states: Completed, Failed, Cancelled, Orphaned
		if state.Status == OrchestratorCompleted ||
			state.Status == OrchestratorFailed ||
			state.Status == OrchestratorCancelled ||
			state.Status == OrchestratorOrphaned {
			continue
		}

		// Check if heartbeat is overdue
		timeSinceLastHeartbeat := now.Sub(state.LastHeartbeat)
		expectedInterval := s.config.OrchestratorHeartbeatInterval

		if timeSinceLastHeartbeat > expectedInterval {
			// Increment missed heartbeat counters
			state.MissedHeartbeats++
			s.missedHeartbeatCount++

			s.logger.Debug("missed heartbeat",
				"run_id", runID,
				"job_id", state.JobID,
				"missed_count", state.MissedHeartbeats,
				"time_since_last", timeSinceLastHeartbeat)

			// Check if we should mark as orphaned
			if state.MissedHeartbeats >= s.config.MaxMissedOrchestratorHeartbeats {
				s.logger.Error("marking JobRun as orphaned",
					"run_id", runID,
					"job_id", state.JobID,
					"missed_count", state.MissedHeartbeats)

				state.Status = OrchestratorOrphaned
				state.CompletedAt = now

				// Buffer update
				s.syncer.BufferJobRunUpdate(JobRunUpdate{
					UpdateID:    generateUpdateID(),
					RunID:       runID,
					JobID:       state.JobID,
					CompletedAt: now,
					Status:      OrchestratorOrphaned.String(),
					Success:     false,
					Error:       fmt.Errorf("JobRun orphaned after %d missed heartbeats", state.MissedHeartbeats),
				})
			}
		}
	}
}

// cleanupOrchestrators removes terminal state orchestrators after grace period
func (s *Scheduler) cleanupOrchestrators(now time.Time) {
	toDelete := []string{}

	for runID, state := range s.activeOrchestrators {
		// Only clean up terminal state orchestrators
		if state.Status != OrchestratorCompleted &&
			state.Status != OrchestratorFailed &&
			state.Status != OrchestratorCancelled &&
			state.Status != OrchestratorOrphaned {
			continue
		}

		// Check if grace period has expired
		if now.After(state.ScheduledAt.Add(s.config.GracePeriod)) {
			toDelete = append(toDelete, runID)
		}
	}

	// Delete from map
	for _, runID := range toDelete {
		delete(s.activeOrchestrators, runID)
	}

	if len(toDelete) > 0 {
		s.logger.Debug("cleaned up orchestrators", "count", len(toDelete))
	}
}

// recordIterationStats buffers iteration statistics for persistence
func (s *Scheduler) recordIterationStats(start time.Time, messagesProcessed int) {
	stats := SchedulerIterationStats{
		Timestamp:               start,
		IterationDuration:       time.Since(start),
		ActiveOrchestratorCount: len(s.activeOrchestrators),
		IndexSize:               s.index.Len(),
		InboxDepth:              s.inbox.Stats().CurrentDepth,
		MessagesProcessed:       messagesProcessed,
		IndexBuildCount:         s.indexBuildCount,
		LastIndexBuildDuration:  s.lastIndexBuildDuration,
		MissedHeartbeatCount:    s.missedHeartbeatCount,
	}

	s.syncer.BufferStats(stats)
}

// flushBuffers checks time and size thresholds and flushes buffers when needed
func (s *Scheduler) flushBuffers(now time.Time) {
	syncerConfig := s.syncer.config
	syncerStats := s.syncer.GetStats()

	// Check job run update flush thresholds (time OR size)
	timeSinceJobRunFlush := now.Sub(s.syncer.lastJobRunFlush)
	shouldFlushJobRuns := timeSinceJobRunFlush >= syncerConfig.JobRunFlushInterval ||
		syncerStats.BufferedJobRunUpdates >= syncerConfig.JobRunFlushThreshold

	if shouldFlushJobRuns && syncerStats.BufferedJobRunUpdates > 0 {
		if err := s.syncer.FlushJobRunUpdates(); err != nil {
			s.logger.Error("failed to flush job run updates",
				"error", err,
				"buffered_count", syncerStats.BufferedJobRunUpdates,
				"time_since_last_flush", timeSinceJobRunFlush)
		}
	}

	// Check stats flush thresholds (time OR size)
	timeSinceStatsFlush := now.Sub(s.syncer.lastStatsFlush)
	shouldFlushStats := timeSinceStatsFlush >= syncerConfig.StatsFlushInterval ||
		syncerStats.BufferedStats >= syncerConfig.StatsFlushThreshold

	if shouldFlushStats && syncerStats.BufferedStats > 0 {
		if err := s.syncer.FlushStats(); err != nil {
			s.logger.Error("failed to flush stats",
				"error", err,
				"buffered_count", syncerStats.BufferedStats,
				"time_since_last_flush", timeSinceStatsFlush)
		}
	}
}

// runIndexBuilder periodically rebuilds the scheduled run index
func (s *Scheduler) runIndexBuilder(db interface{}) {
	ticker := time.NewTicker(s.config.IndexRebuildInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdown:
			return

		case <-ticker.C:
			_ = s.performIndexBuild(db)

		case <-s.rebuildIndexChan:
			// Explicit rebuild requested (e.g., schedule change)
			_ = s.performIndexBuild(db)
		}
	}
}

// performIndexBuild queries jobs and builds the scheduled run index
// Returns error if database query fails
func (s *Scheduler) performIndexBuild(db interface{}) error {
	buildStart := time.Now()
	start := buildStart.Add(-s.config.GracePeriod)
	end := buildStart.Add(s.config.LookaheadWindow)

	s.logger.Info("starting index build",
		"start", start,
		"end", end)

	// 1. Query database for all job definitions
	jobs, err := queryJobDefinitions(db)
	if err != nil {
		s.logger.Error("failed to query job definitions",
			"error", err)
		return fmt.Errorf("failed to query jobs on startup: %w", err)
	}

	// 2. Generate scheduled runs
	runs := []index.ScheduledRun{}
	for _, job := range jobs {
		schedule, err := cron.Parse(job.Schedule)
		if err != nil {
			s.logger.Error("failed to parse cron schedule",
				"job_id", job.ID,
				"schedule", job.Schedule,
				"error", err)
			continue
		}

		times := schedule.Between(start, end)
		for _, t := range times {
			runs = append(runs, index.ScheduledRun{
				JobID:       job.ID,
				ScheduledAt: t,
			})
		}
	}

	// 3. Atomically swap index (lock-free!)
	s.index.Swap(runs)

	// 4. Record stats
	duration := time.Since(buildStart)
	s.indexBuildCount++
	s.lastIndexBuildDuration = duration

	s.logger.Info("index build complete",
		"run_count", len(runs),
		"job_count", len(jobs),
		"duration", duration,
		"total_builds", s.indexBuildCount)

	return nil
}

// queryJobDefinitions queries job definitions from the database
func queryJobDefinitions(db interface{}) ([]*Job, error) {
	// Use type assertion to handle MockDB for testing
	// The interface uses interface{} to avoid import cycles with testutil
	type jobQuerier interface {
		QueryJobDefinitions() (interface{}, error)
	}

	if querier, ok := db.(jobQuerier); ok {
		jobsInterface, err := querier.QueryJobDefinitions()
		if err != nil {
			return nil, err
		}

		// Convert jobs to scheduler.Job using reflection
		// This works with any struct that has ID and Schedule fields
		val := reflect.ValueOf(jobsInterface)
		if val.Kind() != reflect.Slice {
			return []*Job{}, fmt.Errorf("expected slice from QueryJobDefinitions, got %T", jobsInterface)
		}

		result := make([]*Job, val.Len())
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}

			// Extract ID and Schedule fields
			idField := elem.FieldByName("ID")
			scheduleField := elem.FieldByName("Schedule")

			if !idField.IsValid() || !scheduleField.IsValid() {
				return nil, fmt.Errorf("job struct missing ID or Schedule field")
			}

			result[i] = &Job{
				ID:       idField.String(),
				Schedule: scheduleField.String(),
			}
		}
		return result, nil
	}

	// TODO: Actual database query for production
	return []*Job{}, nil
}

// handleShutdown performs graceful shutdown of the scheduler
func (s *Scheduler) handleShutdown() error {
	s.logger.Info("shutting down scheduler")

	// Cancel all active orchestrators
	for runID, state := range s.activeOrchestrators {
		if state.Status == OrchestratorPreRun ||
			state.Status == OrchestratorPending ||
			state.Status == OrchestratorRunning {
			close(state.CancelChan)
			s.logger.Debug("cancelled orchestrator", "run_id", runID)
		}
	}

	// Shutdown syncer (flushes and closes channels)
	if err := s.syncer.Shutdown(); err != nil {
		s.logger.Error("error shutting down syncer", "error", err)
	}

	// Close index builder channel
	close(s.rebuildIndexChan)

	s.logger.Info("scheduler shutdown complete")
	return nil
}
