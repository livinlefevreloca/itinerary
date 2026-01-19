package stats

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/db"
	"github.com/livinlefevreloca/itinerary/internal/inbox"
)

// DatabaseWriter interface for database operations
type DatabaseWriter interface {
	WriteSchedulerStats(periodID string, startTime, endTime time.Time, data *SchedulerStatsAccumulator) error
	WriteOrchestratorStats(periodID string, startTime, endTime time.Time, stats map[string]*OrchestratorStatsData) error
	WriteSyncerStats(periodID string, startTime, endTime time.Time, data *SyncerStatsAccumulator) error
}

// DBAdapter adapts db.DB to implement DatabaseWriter interface
type DBAdapter struct {
	db interface {
		CreateSchedulerStats(stats *db.SchedulerStats) error
		CreateOrchestratorStats(stats *db.OrchestratorStats) error
		CreateSyncerStats(stats *db.SyncerStats) error
	}
}

// NewDBAdapter creates a new database adapter
func NewDBAdapter(database *db.DB) *DBAdapter {
	return &DBAdapter{db: database}
}

// Helper functions to convert values to pointers
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}

// WriteSchedulerStats implements DatabaseWriter for db.DB
func (a *DBAdapter) WriteSchedulerStats(periodID string, startTime, endTime time.Time, data *SchedulerStatsAccumulator) error {
	minInbox, maxInbox, avgInbox := calculateMinMaxAvgInt(data.InboxLengthSamples)
	minWait, maxWait, avgWait := calculateMinMaxAvgDuration(data.MessageWaitTimes)

	stats := &db.SchedulerStats{
		StatsPeriodID:          periodID,
		StartTime:              startTime,
		EndTime:                endTime,
		Iterations:             data.Iterations,
		RunJobs:                data.JobsRun,
		LateJobs:               data.LateJobs,
		TimePassedRunTime:      0, // Not tracked in accumulator
		MissedJobs:             data.MissedJobs,
		TimePassedGracePeriod:  0, // Not tracked in accumulator
		JobsCancelled:          data.JobsCancelled,
		MinInboxLength:         intPtr(minInbox),
		MaxInboxLength:         intPtr(maxInbox),
		AvgInboxLength:         float64Ptr(float64(avgInbox)),
		EmptyInboxTime:         intPtr(int(data.EmptyInboxTime.Microseconds())),
		AvgTimeInInbox:         float64Ptr(float64(avgWait.Microseconds())),
		MinTimeInInbox:         intPtr(int(minWait.Microseconds())),
		MaxTimeInInbox:         intPtr(int(maxWait.Microseconds())),
	}

	return a.db.CreateSchedulerStats(stats)
}

// WriteOrchestratorStats implements DatabaseWriter for db.DB
func (a *DBAdapter) WriteOrchestratorStats(periodID string, startTime, endTime time.Time, stats map[string]*OrchestratorStatsData) error {
	for _, stat := range stats {
		orchStats := &db.OrchestratorStats{
			RunID:              stat.RunID,
			StatsPeriodID:      periodID,
			Runtime:            int(stat.Runtime.Microseconds()),
			ConstraintsChecked: stat.ConstraintsChecked,
			ActionsTaken:       stat.ActionsTaken,
		}
		if err := a.db.CreateOrchestratorStats(orchStats); err != nil {
			return err
		}
	}
	return nil
}

// WriteSyncerStats implements DatabaseWriter for db.DB
func (a *DBAdapter) WriteSyncerStats(periodID string, startTime, endTime time.Time, data *SyncerStatsAccumulator) error {
	minInFlight, maxInFlight, avgInFlight := calculateMinMaxAvgInt(data.WritesInFlightSamples)
	minQueued, maxQueued, avgQueued := calculateMinMaxAvgInt(data.QueuedWritesSamples)
	minInbox, maxInbox, avgInbox := calculateMinMaxAvgInt(data.InboxLengthSamples)
	minQueueTime, maxQueueTime, avgQueueTime := calculateMinMaxAvgDuration(data.TimeInQueueSamples)
	minInboxTime, maxInboxTime, avgInboxTime := calculateMinMaxAvgDuration(data.TimeInInboxSamples)

	stats := &db.SyncerStats{
		StatsPeriodID:         periodID,
		StartTime:             startTime,
		EndTime:               endTime,
		TotalWrites:           data.TotalWrites,
		WritesSucceeded:       data.WritesSucceeded,
		WritesFailed:          data.WritesFailed,
		AvgWritesInFlight:     float64Ptr(float64(avgInFlight)),
		MaxWritesInFlight:     intPtr(maxInFlight),
		MinWritesInFlight:     intPtr(minInFlight),
		AvgQueuedWrites:       float64Ptr(float64(avgQueued)),
		MaxQueuedWrites:       intPtr(maxQueued),
		MinQueuedWrites:       intPtr(minQueued),
		AvgInboxLength:        float64Ptr(float64(avgInbox)),
		MaxInboxLength:        intPtr(maxInbox),
		MinInboxLength:        intPtr(minInbox),
		AvgTimeInWriteQueue:   float64Ptr(float64(avgQueueTime.Microseconds())),
		MaxTimeInWriteQueue:   intPtr(int(maxQueueTime.Microseconds())),
		MinTimeInWriteQueue:   intPtr(int(minQueueTime.Microseconds())),
		AvgTimeInInbox:        float64Ptr(float64(avgInboxTime.Microseconds())),
		MaxTimeInInbox:        intPtr(int(maxInboxTime.Microseconds())),
		MinTimeInInbox:        intPtr(int(minInboxTime.Microseconds())),
	}

	return a.db.CreateSyncerStats(stats)
}

// StatsCollector centralizes all statistics collection and database writing
type StatsCollector struct {
	db     DatabaseWriter
	inbox  *inbox.Inbox[StatsMessage]
	config Config
	logger *slog.Logger

	// Mutex protects all mutable fields below
	mu sync.Mutex

	// Current stats period tracking
	currentPeriod   string
	periodStartTime time.Time

	// Accumulators for current period
	schedulerStats    *SchedulerStatsAccumulator
	orchestratorStats map[string]*OrchestratorStatsData
	syncerStats       *SyncerStatsAccumulator
	webhookStats      *WebhookStatsAccumulator

	// Flush timer
	flushTicker *time.Ticker

	// Message counter for threshold-based flushing
	messageCount int

	// Shutdown coordination
	done     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewStatsCollector creates a new stats collector
func NewStatsCollector(config Config, db DatabaseWriter, logger *slog.Logger) *StatsCollector {
	return &StatsCollector{
		db:                db,
		inbox:             inbox.New[StatsMessage](config.InboxBufferSize, config.InboxSendTimeout, logger),
		config:            config,
		logger:            logger,
		currentPeriod:     generatePeriodID(time.Now()),
		periodStartTime:   time.Now(),
		schedulerStats:    &SchedulerStatsAccumulator{},
		orchestratorStats: make(map[string]*OrchestratorStatsData),
		syncerStats:       &SyncerStatsAccumulator{},
		webhookStats:      &WebhookStatsAccumulator{},
		done:              make(chan struct{}),
	}
}

// Start begins the stats collection loop
func (sc *StatsCollector) Start() {
	sc.logger.Info("starting stats collector")

	sc.mu.Lock()
	sc.flushTicker = time.NewTicker(sc.config.FlushInterval)
	sc.mu.Unlock()

	sc.wg.Add(1)
	go sc.run()
}

// Stop gracefully shuts down the stats collector
func (sc *StatsCollector) Stop() error {
	var stopErr error
	sc.stopOnce.Do(func() {
		sc.logger.Info("stopping stats collector")

		// Signal shutdown
		close(sc.done)

		// Close flush ticker
		sc.mu.Lock()
		if sc.flushTicker != nil {
			sc.flushTicker.Stop()
		}
		sc.mu.Unlock()

		// Close inbox to stop receiving new messages
		sc.inbox.Close()

		// Wait for run loop to exit
		sc.wg.Wait()

		// Final flush of any pending stats
		if err := sc.flush(); err != nil {
			sc.logger.Error("final flush failed", "error", err)
			stopErr = err
			return
		}

		sc.logger.Info("stats collector stopped")
	})
	return stopErr
}

// Send sends a stats message to the collector (non-blocking with timeout)
func (sc *StatsCollector) Send(msg StatsMessage) bool {
	return sc.inbox.Send(msg)
}

// GetSchedulerIterations returns the current scheduler iterations count (for testing)
func (sc *StatsCollector) GetSchedulerIterations() int {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.schedulerStats.Iterations
}

// run is the main stats collection loop
func (sc *StatsCollector) run() {
	defer sc.wg.Done()

	for {
		select {
		case <-sc.done:
			sc.logger.Debug("shutdown signal received")
			return

		case <-sc.flushTicker.C:
			sc.logger.Debug("flush timer triggered")
			if err := sc.flush(); err != nil {
				sc.logger.Error("flush failed", "error", err)
			}

		default:
			// Try to receive message with short timeout to check other signals
			msg, ok := sc.inbox.TryReceive()
			if !ok {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			// Process the message
			sc.processMessage(msg)

			// Increment message count and check thresholds (protected by lock)
			sc.mu.Lock()
			sc.messageCount++
			shouldFlushThreshold := sc.messageCount >= sc.config.FlushThreshold
			shouldFlushPeriod := time.Since(sc.periodStartTime) >= sc.config.PeriodDuration
			sc.mu.Unlock()

			// Check if we need to flush based on threshold
			if shouldFlushThreshold {
				if err := sc.flush(); err != nil {
					sc.logger.Error("threshold flush failed", "error", err)
				}
			}

			// Check if period has expired
			if shouldFlushPeriod {
				if err := sc.flush(); err != nil {
					sc.logger.Error("period flush failed", "error", err)
				}
				sc.startNewPeriod()
			}
		}
	}
}

// processMessage routes a message to the appropriate accumulator
func (sc *StatsCollector) processMessage(msg StatsMessage) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	switch msg.Source {
	case StatsSourceScheduler:
		data, ok := msg.Data.(*SchedulerStatsData)
		if !ok {
			sc.logger.Error("invalid scheduler stats data type")
			return
		}
		sc.schedulerStats.Add(data)

	case StatsSourceOrchestrator:
		data, ok := msg.Data.(*OrchestratorStatsData)
		if !ok {
			sc.logger.Error("invalid orchestrator stats data type")
			return
		}
		// Store/update orchestrator stats by run ID
		sc.orchestratorStats[data.RunID] = data

	case StatsSourceSyncer:
		data, ok := msg.Data.(*SyncerStatsData)
		if !ok {
			sc.logger.Error("invalid syncer stats data type")
			return
		}
		sc.syncerStats.Add(data)

	case StatsSourceWebhookHandler:
		// Future implementation
		sc.logger.Debug("webhook stats not yet implemented")

	default:
		sc.logger.Error("unknown stats source", "source", msg.Source)
	}
}

// flush writes current stats to database and resets accumulators
func (sc *StatsCollector) flush() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Don't flush if no data collected
	if sc.messageCount == 0 {
		return nil
	}

	sc.logger.Debug("flushing stats to database",
		"period", sc.currentPeriod,
		"messages", sc.messageCount)

	periodEnd := time.Now()

	// Write scheduler stats
	if sc.schedulerStats.Iterations > 0 {
		if err := sc.db.WriteSchedulerStats(sc.currentPeriod, sc.periodStartTime, periodEnd, sc.schedulerStats); err != nil {
			return fmt.Errorf("write scheduler stats failed: %w", err)
		}
	}

	// Write orchestrator stats
	if len(sc.orchestratorStats) > 0 {
		if err := sc.db.WriteOrchestratorStats(sc.currentPeriod, sc.periodStartTime, periodEnd, sc.orchestratorStats); err != nil {
			return fmt.Errorf("write orchestrator stats failed: %w", err)
		}
	}

	// Write syncer stats
	if sc.syncerStats.TotalWrites > 0 {
		if err := sc.db.WriteSyncerStats(sc.currentPeriod, sc.periodStartTime, periodEnd, sc.syncerStats); err != nil {
			return fmt.Errorf("write syncer stats failed: %w", err)
		}
	}

	// Reset accumulators and counter
	sc.schedulerStats.Reset()
	sc.orchestratorStats = make(map[string]*OrchestratorStatsData)
	sc.syncerStats.Reset()
	sc.messageCount = 0

	sc.logger.Debug("flush complete")
	return nil
}

// startNewPeriod starts a new stats period
func (sc *StatsCollector) startNewPeriod() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.currentPeriod = generatePeriodID(time.Now())
	sc.periodStartTime = time.Now()
	sc.logger.Debug("started new period", "period", sc.currentPeriod)
}

// generatePeriodID generates a unique period ID based on timestamp
func generatePeriodID(t time.Time) string {
	return fmt.Sprintf("period-%d", t.Unix())
}

// Helper functions for min/max/avg calculations

func calculateMinMaxAvgInt(values []int) (min, max, avg int) {
	if len(values) == 0 {
		return 0, 0, 0
	}

	min = values[0]
	max = values[0]
	sum := 0

	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}

	avg = sum / len(values)
	return min, max, avg
}

func calculateMinMaxAvgDuration(values []time.Duration) (min, max, avg time.Duration) {
	if len(values) == 0 {
		return 0, 0, 0
	}

	min = values[0]
	max = values[0]
	var sum time.Duration

	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}

	avg = sum / time.Duration(len(values))
	return min, max, avg
}
