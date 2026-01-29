package stats

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/inbox"
)

// DatabaseWriter defines the interface for writing stats to the database
type DatabaseWriter interface {
	WriteSchedulerStats(periodID string, startTime, endTime time.Time, data *SchedulerStatsAccumulator) error
	WriteOrchestratorStats(periodID string, startTime, endTime time.Time, stats map[string]*OrchestratorStatsData) error
	WriteSyncerStats(periodID string, startTime, endTime time.Time, data *SyncerStatsAccumulator) error
}

// StatsCollector collects and persists statistics from all components
type StatsCollector struct {
	db     DatabaseWriter
	inbox  *inbox.Inbox[StatsMessage]
	config Config
	logger *slog.Logger

	// Current stats period tracking
	currentPeriod   string
	periodStartTime time.Time

	// Accumulators for current period
	schedulerStats    *SchedulerStatsAccumulator
	orchestratorStats map[string]*OrchestratorStatsData // keyed by run_id
	syncerStats       *SyncerStatsAccumulator

	// Flush tracking
	messageCount int
	flushTicker  *time.Ticker

	// Shutdown coordination
	done chan struct{}
	wg   sync.WaitGroup
	mu   sync.RWMutex // Protects accumulators for testing
}

// NewStatsCollector creates a new stats collector
func NewStatsCollector(config Config, db DatabaseWriter, logger *slog.Logger) *StatsCollector {
	now := time.Now()
	periodID := generatePeriodID(now, config.PeriodDuration)

	sc := &StatsCollector{
		db:                db,
		inbox:             inbox.New[StatsMessage](config.InboxBufferSize, config.InboxSendTimeout, logger),
		config:            config,
		logger:            logger,
		currentPeriod:     periodID,
		periodStartTime:   now,
		schedulerStats:    &SchedulerStatsAccumulator{},
		orchestratorStats: make(map[string]*OrchestratorStatsData),
		syncerStats:       &SyncerStatsAccumulator{},
		done:              make(chan struct{}),
	}

	return sc
}

// Start begins the stats collection loop
func (sc *StatsCollector) Start() {
	sc.wg.Add(1)
	go sc.run()
}

// Stop gracefully shuts down the stats collector
func (sc *StatsCollector) Stop() error {
	// Signal shutdown
	select {
	case <-sc.done:
		// Already stopped
		return nil
	default:
		close(sc.done)
	}

	// Wait for goroutine to finish
	sc.wg.Wait()

	return nil
}

// Send sends a stats message to the collector (non-blocking with timeout)
func (sc *StatsCollector) Send(msg StatsMessage) bool {
	return sc.inbox.Send(msg)
}

// run is the main loop that processes stats messages
func (sc *StatsCollector) run() {
	defer sc.wg.Done()

	sc.flushTicker = time.NewTicker(sc.config.FlushInterval)
	defer sc.flushTicker.Stop()

	for {
		select {
		case <-sc.done:
			// Shutdown: process remaining messages and flush
			sc.drainInbox()
			if err := sc.flush(); err != nil {
				sc.logger.Error("failed to flush stats on shutdown", "error", err)
			}
			return

		case <-sc.flushTicker.C:
			// Periodic flush
			if err := sc.flush(); err != nil {
				sc.logger.Error("failed to flush stats", "error", err)
			}
			// Check if we need to start a new period
			if sc.shouldStartNewPeriod() {
				sc.startNewPeriod()
			}

		case msg, ok := <-sc.inbox.Chan():
			if !ok {
				// Inbox closed, flush and exit
				if err := sc.flush(); err != nil {
					sc.logger.Error("failed to flush stats on inbox close", "error", err)
				}
				return
			}

			sc.processMessage(msg)
			sc.messageCount++

			// Check if we've hit the flush threshold
			if sc.messageCount >= sc.config.FlushThreshold {
				if err := sc.flush(); err != nil {
					sc.logger.Error("failed to flush stats", "error", err)
				}
				sc.messageCount = 0
			}

			// Check if we need to start a new period
			if sc.shouldStartNewPeriod() {
				sc.startNewPeriod()
			}
		}
	}
}

// drainInbox processes all remaining messages in the inbox
func (sc *StatsCollector) drainInbox() {
	for {
		select {
		case msg, ok := <-sc.inbox.Chan():
			if !ok {
				return
			}
			sc.processMessage(msg)
		default:
			return
		}
	}
}

// processMessage routes a stats message to the appropriate accumulator
func (sc *StatsCollector) processMessage(msg StatsMessage) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	switch msg.Source {
	case StatsSourceScheduler:
		if data, ok := msg.Data.(*SchedulerStatsData); ok {
			sc.schedulerStats.Add(data)
		} else {
			sc.logger.Warn("invalid scheduler stats data type")
		}

	case StatsSourceOrchestrator:
		if data, ok := msg.Data.(*OrchestratorStatsData); ok {
			// Store or update orchestrator stats by run_id
			sc.orchestratorStats[data.RunID] = data
		} else {
			sc.logger.Warn("invalid orchestrator stats data type")
		}

	case StatsSourceSyncer:
		if data, ok := msg.Data.(*SyncerStatsData); ok {
			sc.syncerStats.Add(data)
		} else {
			sc.logger.Warn("invalid syncer stats data type")
		}

	default:
		sc.logger.Warn("unknown stats source", "source", msg.Source)
	}
}

// flush writes accumulated stats to the database
func (sc *StatsCollector) flush() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Don't flush if we have no data
	if sc.schedulerStats.Iterations == 0 &&
		len(sc.orchestratorStats) == 0 &&
		sc.syncerStats.TotalWrites == 0 {
		return nil
	}

	endTime := time.Now()

	var flushErrors []error

	// Flush scheduler stats
	if sc.schedulerStats.Iterations > 0 {
		if err := sc.db.WriteSchedulerStats(
			sc.currentPeriod,
			sc.periodStartTime,
			endTime,
			sc.schedulerStats,
		); err != nil {
			flushErrors = append(flushErrors, fmt.Errorf("scheduler stats: %w", err))
		}
	}

	// Flush orchestrator stats
	if len(sc.orchestratorStats) > 0 {
		if err := sc.db.WriteOrchestratorStats(
			sc.currentPeriod,
			sc.periodStartTime,
			endTime,
			sc.orchestratorStats,
		); err != nil {
			flushErrors = append(flushErrors, fmt.Errorf("orchestrator stats: %w", err))
		}
	}

	// Flush syncer stats
	if sc.syncerStats.TotalWrites > 0 {
		if err := sc.db.WriteSyncerStats(
			sc.currentPeriod,
			sc.periodStartTime,
			endTime,
			sc.syncerStats,
		); err != nil {
			flushErrors = append(flushErrors, fmt.Errorf("syncer stats: %w", err))
		}
	}

	// Reset accumulators
	sc.schedulerStats.Reset()
	sc.orchestratorStats = make(map[string]*OrchestratorStatsData)
	sc.syncerStats.Reset()

	if len(flushErrors) > 0 {
		return fmt.Errorf("flush errors: %v", flushErrors)
	}

	return nil
}

// shouldStartNewPeriod checks if the current period has expired
func (sc *StatsCollector) shouldStartNewPeriod() bool {
	expectedPeriodID := generatePeriodID(time.Now(), sc.config.PeriodDuration)
	return expectedPeriodID != sc.currentPeriod
}

// startNewPeriod begins a new stats period
func (sc *StatsCollector) startNewPeriod() {
	now := time.Now()
	sc.currentPeriod = generatePeriodID(now, sc.config.PeriodDuration)
	sc.periodStartTime = now

	sc.logger.Info("started new stats period", "period_id", sc.currentPeriod)
}

// generatePeriodID creates a period ID based on time and duration
func generatePeriodID(t time.Time, duration time.Duration) string {
	periodNum := t.Unix() / int64(duration.Seconds())
	return fmt.Sprintf("period-%d", periodNum)
}

// GetSchedulerIterations returns the current scheduler iteration count (for testing)
func (sc *StatsCollector) GetSchedulerIterations() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.schedulerStats.Iterations
}

// calculateMinMaxAvgInt calculates min, max, and average for integer slice
func calculateMinMaxAvgInt(values []int) (min int, max int, avg int) {
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

// calculateMinMaxAvgDuration calculates min, max, and average for duration slice
func calculateMinMaxAvgDuration(values []time.Duration) (min time.Duration, max time.Duration, avg time.Duration) {
	if len(values) == 0 {
		return 0, 0, 0
	}

	min = values[0]
	max = values[0]
	sum := time.Duration(0)

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
