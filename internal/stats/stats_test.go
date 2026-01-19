package stats

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/livinlefevreloca/itinerary/internal/db"
	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

// =============================================================================
// Test Helpers
// =============================================================================

// MockDB simulates database operations for testing
type MockDB struct {
	mu             sync.Mutex
	schedulerStats []*SchedulerStatsData
	orchestrator   []*OrchestratorStatsData
	syncerStats    []*SyncerStatsData
	failCount      int32 // Fail this many times before succeeding
	writeLatency   time.Duration
	writeCalls     int32
}

func NewMockDB() *MockDB {
	return &MockDB{
		schedulerStats: make([]*SchedulerStatsData, 0),
		orchestrator:   make([]*OrchestratorStatsData, 0),
		syncerStats:    make([]*SyncerStatsData, 0),
	}
}

func (m *MockDB) WriteSchedulerStats(periodID string, startTime, endTime time.Time, data *SchedulerStatsAccumulator) error {
	atomic.AddInt32(&m.writeCalls, 1)

	if m.writeLatency > 0 {
		time.Sleep(m.writeLatency)
	}

	failCount := atomic.LoadInt32(&m.failCount)
	if failCount > 0 {
		atomic.AddInt32(&m.failCount, -1)
		return fmt.Errorf("simulated database failure")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Store for verification
	m.schedulerStats = append(m.schedulerStats, &SchedulerStatsData{
		Iterations:     data.Iterations,
		JobsRun:        data.JobsRun,
		LateJobs:       data.LateJobs,
		MissedJobs:     data.MissedJobs,
		JobsCancelled:  data.JobsCancelled,
	})
	return nil
}

func (m *MockDB) WriteOrchestratorStats(periodID string, startTime, endTime time.Time, stats map[string]*OrchestratorStatsData) error {
	atomic.AddInt32(&m.writeCalls, 1)

	if m.writeLatency > 0 {
		time.Sleep(m.writeLatency)
	}

	failCount := atomic.LoadInt32(&m.failCount)
	if failCount > 0 {
		atomic.AddInt32(&m.failCount, -1)
		return fmt.Errorf("simulated database failure")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, stat := range stats {
		m.orchestrator = append(m.orchestrator, stat)
	}
	return nil
}

func (m *MockDB) WriteSyncerStats(periodID string, startTime, endTime time.Time, data *SyncerStatsAccumulator) error {
	atomic.AddInt32(&m.writeCalls, 1)

	if m.writeLatency > 0 {
		time.Sleep(m.writeLatency)
	}

	failCount := atomic.LoadInt32(&m.failCount)
	if failCount > 0 {
		atomic.AddInt32(&m.failCount, -1)
		return fmt.Errorf("simulated database failure")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.syncerStats = append(m.syncerStats, &SyncerStatsData{
		TotalWrites:     data.TotalWrites,
		WritesSucceeded: data.WritesSucceeded,
		WritesFailed:    data.WritesFailed,
	})
	return nil
}

func (m *MockDB) GetSchedulerStatsCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.schedulerStats)
}

func (m *MockDB) GetOrchestratorStatsCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.orchestrator)
}

func (m *MockDB) GetSyncerStatsCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.syncerStats)
}

func (m *MockDB) GetWriteCalls() int {
	return int(atomic.LoadInt32(&m.writeCalls))
}

func (m *MockDB) SetFailCount(count int) {
	atomic.StoreInt32(&m.failCount, int32(count))
}

// setupTestDB creates an in-memory SQLite database with schema
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()

	database, err := db.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Enable foreign keys (disabled by default in SQLite)
	if _, err := database.Exec("PRAGMA foreign_keys = ON"); err != nil {
		database.Close()
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	// Create minimal schema needed for stats integration tests
	schema := `
		CREATE TABLE jobs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			schedule TEXT NOT NULL,
			pod_spec TEXT,
			constraints TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE job_runs (
			run_id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			scheduled_at TIMESTAMP NOT NULL,
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			status TEXT NOT NULL,
			success BOOLEAN,
			error TEXT,
			FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
		);

		CREATE TABLE scheduler_stats (
			stats_period_id TEXT PRIMARY KEY,
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP NOT NULL,
			iterations INTEGER NOT NULL DEFAULT 0,
			run_jobs INTEGER NOT NULL DEFAULT 0,
			late_jobs INTEGER NOT NULL DEFAULT 0,
			time_passed_run_time INTEGER NOT NULL DEFAULT 0,
			missed_jobs INTEGER NOT NULL DEFAULT 0,
			time_passed_grace_period INTEGER NOT NULL DEFAULT 0,
			jobs_cancelled INTEGER NOT NULL DEFAULT 0,
			min_inbox_length INTEGER NOT NULL DEFAULT 0,
			max_inbox_length INTEGER NOT NULL DEFAULT 0,
			avg_inbox_length INTEGER NOT NULL DEFAULT 0,
			empty_inbox_time INTEGER NOT NULL DEFAULT 0,
			avg_time_in_inbox INTEGER NOT NULL DEFAULT 0,
			min_time_in_inbox INTEGER NOT NULL DEFAULT 0,
			max_time_in_inbox INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE orchestrator_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			stats_period_id TEXT NOT NULL,
			runtime INTEGER NOT NULL,
			constraints_checked INTEGER NOT NULL DEFAULT 0,
			actions_taken INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (run_id) REFERENCES job_runs(run_id) ON DELETE CASCADE
		);

		CREATE TABLE syncer_stats (
			stats_period_id TEXT PRIMARY KEY,
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP NOT NULL,
			total_writes INTEGER NOT NULL DEFAULT 0,
			writes_succeeded INTEGER NOT NULL DEFAULT 0,
			writes_failed INTEGER NOT NULL DEFAULT 0,
			avg_writes_in_flight INTEGER NOT NULL DEFAULT 0,
			max_writes_in_flight INTEGER NOT NULL DEFAULT 0,
			min_writes_in_flight INTEGER NOT NULL DEFAULT 0,
			avg_queued_writes INTEGER NOT NULL DEFAULT 0,
			max_queued_writes INTEGER NOT NULL DEFAULT 0,
			min_queued_writes INTEGER NOT NULL DEFAULT 0,
			avg_inbox_length INTEGER NOT NULL DEFAULT 0,
			max_inbox_length INTEGER NOT NULL DEFAULT 0,
			min_inbox_length INTEGER NOT NULL DEFAULT 0,
			avg_time_in_write_queue INTEGER NOT NULL DEFAULT 0,
			max_time_in_write_queue INTEGER NOT NULL DEFAULT 0,
			min_time_in_write_queue INTEGER NOT NULL DEFAULT 0,
			avg_time_in_inbox INTEGER NOT NULL DEFAULT 0,
			max_time_in_inbox INTEGER NOT NULL DEFAULT 0,
			min_time_in_inbox INTEGER NOT NULL DEFAULT 0
		);
	`

	if _, err := database.Exec(schema); err != nil {
		database.Close()
		t.Fatalf("failed to create schema: %v", err)
	}

	return database
}

// =============================================================================
// Unit Tests
// =============================================================================

// TestNewStatsCollector verifies stats collector initialization
func TestNewStatsCollector(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	if collector == nil {
		t.Fatal("NewStatsCollector returned nil")
	}

	if collector.config.InboxBufferSize != config.InboxBufferSize {
		t.Errorf("config not stored correctly")
	}

	if collector.inbox == nil {
		t.Error("inbox not initialized")
	}

	if collector.schedulerStats == nil {
		t.Error("schedulerStats accumulator not initialized")
	}

	if collector.orchestratorStats == nil {
		t.Error("orchestratorStats map not initialized")
	}

	if collector.syncerStats == nil {
		t.Error("syncerStats accumulator not initialized")
	}

	if collector.currentPeriod == "" {
		t.Error("current period ID not set")
	}

	if collector.periodStartTime.IsZero() {
		t.Error("period start time not set")
	}
}

// TestDefaultConfig verifies default configuration values
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.InboxBufferSize <= 0 {
		t.Error("InboxBufferSize must be positive")
	}

	if config.InboxSendTimeout <= 0 {
		t.Error("InboxSendTimeout must be positive")
	}

	if config.PeriodDuration <= 0 {
		t.Error("PeriodDuration must be positive")
	}

	if config.FlushInterval <= 0 {
		t.Error("FlushInterval must be positive")
	}

	if config.FlushThreshold <= 0 {
		t.Error("FlushThreshold must be positive")
	}
}

// TestSend_Success verifies successful message sending
func TestSend_Success(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	config.InboxBufferSize = 10
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	msg := StatsMessage{
		Source:    StatsSourceScheduler,
		Timestamp: time.Now(),
		Data: &SchedulerStatsData{
			Iterations: 1,
			JobsRun:    5,
		},
	}

	success := collector.Send(msg)
	if !success {
		t.Error("Send() should return true for successful send")
	}

	// Verify message is in inbox
	if collector.inbox.Len() != 1 {
		t.Errorf("inbox should contain 1 message, got %d", collector.inbox.Len())
	}
}

// TestSend_Timeout verifies timeout behavior when inbox is full
func TestSend_Timeout(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	config.InboxBufferSize = 1
	config.InboxSendTimeout = 10 * time.Millisecond
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	// Fill the inbox
	msg := StatsMessage{
		Source:    StatsSourceScheduler,
		Timestamp: time.Now(),
		Data:      &SchedulerStatsData{Iterations: 1},
	}
	collector.Send(msg)

	// Try to send another (should timeout)
	success := collector.Send(msg)
	if success {
		t.Error("Send() should return false when inbox is full")
	}
}

// TestMessageRouting_Scheduler verifies scheduler stats routing
func TestMessageRouting_Scheduler(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	msg := StatsMessage{
		Source:    StatsSourceScheduler,
		Timestamp: time.Now(),
		Data: &SchedulerStatsData{
			Iterations: 3,
			JobsRun:    5,
		},
	}

	// Process message directly (not through inbox)
	collector.processMessage(msg)

	if collector.schedulerStats.Iterations != 3 {
		t.Errorf("expected 3 iterations, got %d", collector.schedulerStats.Iterations)
	}

	if collector.schedulerStats.JobsRun != 5 {
		t.Errorf("expected 5 jobs run, got %d", collector.schedulerStats.JobsRun)
	}

	// Verify other accumulators not affected
	if collector.syncerStats.TotalWrites != 0 {
		t.Error("syncer stats should not be affected")
	}
}

// TestMessageRouting_Orchestrator verifies orchestrator stats routing
func TestMessageRouting_Orchestrator(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	msg := StatsMessage{
		Source:    StatsSourceOrchestrator,
		Timestamp: time.Now(),
		Data: &OrchestratorStatsData{
			RunID:              "run-123",
			Runtime:            100 * time.Millisecond,
			ConstraintsChecked: 5,
			ActionsTaken:       2,
		},
	}

	collector.processMessage(msg)

	if len(collector.orchestratorStats) != 1 {
		t.Errorf("expected 1 orchestrator entry, got %d", len(collector.orchestratorStats))
	}

	stats, exists := collector.orchestratorStats["run-123"]
	if !exists {
		t.Fatal("orchestrator stats for run-123 not found")
	}

	if stats.ConstraintsChecked != 5 {
		t.Errorf("expected 5 constraints checked, got %d", stats.ConstraintsChecked)
	}

	// Send another message for same run_id (should update, not duplicate)
	msg.Data.(*OrchestratorStatsData).ActionsTaken = 3
	collector.processMessage(msg)

	if len(collector.orchestratorStats) != 1 {
		t.Errorf("expected still 1 orchestrator entry, got %d", len(collector.orchestratorStats))
	}
}

// TestMessageRouting_Syncer verifies syncer stats routing
func TestMessageRouting_Syncer(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	msg := StatsMessage{
		Source:    StatsSourceSyncer,
		Timestamp: time.Now(),
		Data: &SyncerStatsData{
			TotalWrites:     10,
			WritesSucceeded: 8,
			WritesFailed:    2,
		},
	}

	collector.processMessage(msg)

	if collector.syncerStats.TotalWrites != 10 {
		t.Errorf("expected 10 total writes, got %d", collector.syncerStats.TotalWrites)
	}

	if collector.syncerStats.WritesSucceeded != 8 {
		t.Errorf("expected 8 succeeded, got %d", collector.syncerStats.WritesSucceeded)
	}

	// Verify other accumulators not affected
	if collector.schedulerStats.Iterations != 0 {
		t.Error("scheduler stats should not be affected")
	}
}

// TestSchedulerStatsAccumulator_Add verifies accumulator correctly adds data
func TestSchedulerStatsAccumulator_Add(t *testing.T) {
	acc := &SchedulerStatsAccumulator{}

	// Add first batch
	acc.Add(&SchedulerStatsData{
		Iterations:  3,
		JobsRun:     5,
		InboxLength: 10,
	})

	// Add second batch
	acc.Add(&SchedulerStatsData{
		Iterations:  2,
		JobsRun:     3,
		InboxLength: 15,
	})

	if acc.Iterations != 5 {
		t.Errorf("expected 5 iterations, got %d", acc.Iterations)
	}

	if acc.JobsRun != 8 {
		t.Errorf("expected 8 jobs run, got %d", acc.JobsRun)
	}

	if len(acc.InboxLengthSamples) != 2 {
		t.Errorf("expected 2 inbox samples, got %d", len(acc.InboxLengthSamples))
	}
}

// TestSchedulerStatsAccumulator_Calculations verifies min/max/avg calculations
func TestSchedulerStatsAccumulator_Calculations(t *testing.T) {
	acc := &SchedulerStatsAccumulator{
		InboxLengthSamples: []int{5, 10, 15, 20, 10},
		MessageWaitTimes:   []time.Duration{1 * time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond, 4 * time.Millisecond, 5 * time.Millisecond},
	}

	min, max, avg := calculateMinMaxAvgInt(acc.InboxLengthSamples)
	if min != 5 {
		t.Errorf("expected min inbox length 5, got %d", min)
	}
	if max != 20 {
		t.Errorf("expected max inbox length 20, got %d", max)
	}
	if avg != 12 {
		t.Errorf("expected avg inbox length 12, got %d", avg)
	}

	minWait, maxWait, avgWait := calculateMinMaxAvgDuration(acc.MessageWaitTimes)
	if minWait != 1*time.Millisecond {
		t.Errorf("expected min wait 1ms, got %v", minWait)
	}
	if maxWait != 5*time.Millisecond {
		t.Errorf("expected max wait 5ms, got %v", maxWait)
	}
	if avgWait != 3*time.Millisecond {
		t.Errorf("expected avg wait 3ms, got %v", avgWait)
	}
}

// TestSyncerStatsAccumulator_Add verifies syncer accumulator correctly adds data
func TestSyncerStatsAccumulator_Add(t *testing.T) {
	acc := &SyncerStatsAccumulator{}

	// Add first batch
	acc.Add(&SyncerStatsData{
		TotalWrites:     10,
		WritesSucceeded: 8,
		WritesFailed:    2,
	})

	// Add second batch
	acc.Add(&SyncerStatsData{
		TotalWrites:     5,
		WritesSucceeded: 5,
		WritesFailed:    0,
	})

	if acc.TotalWrites != 15 {
		t.Errorf("expected 15 total writes, got %d", acc.TotalWrites)
	}

	if acc.WritesSucceeded != 13 {
		t.Errorf("expected 13 succeeded, got %d", acc.WritesSucceeded)
	}

	if acc.WritesFailed != 2 {
		t.Errorf("expected 2 failed, got %d", acc.WritesFailed)
	}
}

// TestCalculateMinMaxAvg_Integers verifies calculation for integers
func TestCalculateMinMaxAvg_Integers(t *testing.T) {
	tests := []struct {
		name   string
		values []int
		min    int
		max    int
		avg    int
	}{
		{"empty", []int{}, 0, 0, 0},
		{"single", []int{5}, 5, 5, 5},
		{"multiple", []int{1, 5, 3, 9, 2}, 1, 9, 4},
		{"all same", []int{5, 5, 5}, 5, 5, 5},
		{"negative", []int{-5, -2, -8}, -8, -2, -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, max, avg := calculateMinMaxAvgInt(tt.values)
			if min != tt.min {
				t.Errorf("min: expected %d, got %d", tt.min, min)
			}
			if max != tt.max {
				t.Errorf("max: expected %d, got %d", tt.max, max)
			}
			if avg != tt.avg {
				t.Errorf("avg: expected %d, got %d", tt.avg, avg)
			}
		})
	}
}

// TestCalculateMinMaxAvg_Durations verifies calculation for durations
func TestCalculateMinMaxAvg_Durations(t *testing.T) {
	values := []time.Duration{1 * time.Millisecond, 5 * time.Millisecond, 3 * time.Millisecond, 9 * time.Millisecond, 2 * time.Millisecond}

	min, max, avg := calculateMinMaxAvgDuration(values)

	if min != 1*time.Millisecond {
		t.Errorf("min: expected 1ms, got %v", min)
	}
	if max != 9*time.Millisecond {
		t.Errorf("max: expected 9ms, got %v", max)
	}
	if avg != 4*time.Millisecond {
		t.Errorf("avg: expected 4ms, got %v", avg)
	}
}

// TestOrchestratorStats_MultipleRuns verifies multiple runs tracked separately
func TestOrchestratorStats_MultipleRuns(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	// Send stats for run-1
	msg1 := StatsMessage{
		Source:    StatsSourceOrchestrator,
		Timestamp: time.Now(),
		Data: &OrchestratorStatsData{
			RunID:              "run-1",
			Runtime:            100 * time.Millisecond,
			ConstraintsChecked: 5,
			ActionsTaken:       2,
		},
	}
	collector.processMessage(msg1)

	// Send stats for run-2
	msg2 := StatsMessage{
		Source:    StatsSourceOrchestrator,
		Timestamp: time.Now(),
		Data: &OrchestratorStatsData{
			RunID:              "run-2",
			Runtime:            200 * time.Millisecond,
			ConstraintsChecked: 3,
			ActionsTaken:       1,
		},
	}
	collector.processMessage(msg2)

	// Update stats for run-1
	msg1.Data.(*OrchestratorStatsData).ActionsTaken = 3
	collector.processMessage(msg1)

	if len(collector.orchestratorStats) != 2 {
		t.Errorf("expected 2 orchestrator entries, got %d", len(collector.orchestratorStats))
	}

	stats1 := collector.orchestratorStats["run-1"]
	if stats1.ActionsTaken != 3 {
		t.Errorf("run-1: expected 3 actions, got %d", stats1.ActionsTaken)
	}

	stats2 := collector.orchestratorStats["run-2"]
	if stats2.ActionsTaken != 1 {
		t.Errorf("run-2: expected 1 action, got %d", stats2.ActionsTaken)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestDatabaseWrite_SchedulerStats verifies scheduler stats written to database
func TestDatabaseWrite_SchedulerStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	database := setupTestDB(t)
	defer database.Close()

	dbAdapter := NewDBAdapter(database)
	collector := NewStatsCollector(config, dbAdapter, logger.Logger())

	msg := StatsMessage{
		Source:    StatsSourceScheduler,
		Timestamp: time.Now(),
		Data: &SchedulerStatsData{
			Iterations:    5,
			JobsRun:       10,
			LateJobs:      2,
			MissedJobs:    1,
			JobsCancelled: 0,
			InboxLength:   15,
		},
	}

	collector.processMessage(msg)
	collector.messageCount++ // Normally incremented in run() loop

	// Trigger flush
	err := collector.flush()
	if err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	// Query database
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM scheduler_stats").Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 row in scheduler_stats, got %d", count)
	}
}

// TestDatabaseWrite_OrchestratorStats verifies orchestrator stats written to database
func TestDatabaseWrite_OrchestratorStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	database := setupTestDB(t)
	defer database.Close()

	// Create job_run entries (foreign key requirement)
	job := &db.Job{
		ID:       "test-job",
		Name:     "Test Job",
		Schedule: "* * * * *",
	}
	if err := database.CreateJob(job); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	for i := 1; i <= 3; i++ {
		runID := fmt.Sprintf("run-%d", i)
		scheduledAt := time.Now().Add(time.Duration(i) * time.Minute)
		jobRun := &db.JobRun{
			JobID:       "test-job",
			RunID:       runID,
			ScheduledAt: scheduledAt,
			Status:      "completed",
		}
		if err := database.CreateJobRun(jobRun); err != nil {
			t.Fatalf("failed to create job run %s: %v", runID, err)
		}
	}

	dbAdapter := NewDBAdapter(database)
	collector := NewStatsCollector(config, dbAdapter, logger.Logger())

	// Send stats for 3 different runs
	for i := 1; i <= 3; i++ {
		msg := StatsMessage{
			Source:    StatsSourceOrchestrator,
			Timestamp: time.Now(),
			Data: &OrchestratorStatsData{
				RunID:              fmt.Sprintf("run-%d", i),
				Runtime:            time.Duration(i*100) * time.Millisecond,
				ConstraintsChecked: i * 2,
				ActionsTaken:       i,
			},
		}
		collector.processMessage(msg)
		collector.messageCount++ // Normally incremented in run() loop
	}

	// Trigger flush
	err := collector.flush()
	if err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	// Query database
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM orchestrator_stats").Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 rows in orchestrator_stats, got %d", count)
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestConcurrentSends verifies multiple goroutines can send concurrently
func TestConcurrentSends(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	config.InboxBufferSize = 10000
	config.FlushThreshold = 10000 // Prevent flushing during test
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	// Start the collector
	collector.Start()
	defer collector.Stop()

	const numGoroutines = 10
	const messagesPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := StatsMessage{
					Source:    StatsSourceScheduler,
					Timestamp: time.Now(),
					Data: &SchedulerStatsData{
						Iterations: 1,
					},
				}
				collector.Send(msg)
			}
		}(i)
	}

	wg.Wait()

	// Give time for processing
	time.Sleep(500 * time.Millisecond)

	// Should have received 1000 messages
	iterations := collector.GetSchedulerIterations()
	if iterations != numGoroutines*messagesPerGoroutine {
		t.Errorf("expected %d iterations, got %d", numGoroutines*messagesPerGoroutine, iterations)
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestEmptyAccumulatorFlush verifies flush works with no stats collected
func TestEmptyAccumulatorFlush(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	// Trigger flush without sending any messages
	err := collector.flush()
	if err != nil {
		t.Errorf("flush should not error with empty accumulator: %v", err)
	}

	// Verify no database writes occurred
	if mockDB.GetWriteCalls() > 0 {
		t.Error("no database writes should occur for empty accumulator")
	}
}

// TestSingleSampleCalculations verifies calculations work with single sample
func TestSingleSampleCalculations(t *testing.T) {
	values := []int{42}
	min, max, avg := calculateMinMaxAvgInt(values)

	if min != 42 || max != 42 || avg != 42 {
		t.Errorf("single sample: expected min=max=avg=42, got min=%d, max=%d, avg=%d", min, max, avg)
	}
}

// TestZeroValues verifies handling of zero values
func TestZeroValues(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	msg := StatsMessage{
		Source:    StatsSourceScheduler,
		Timestamp: time.Now(),
		Data: &SchedulerStatsData{
			Iterations:    0,
			JobsRun:       0,
			InboxLength:   0,
		},
	}

	collector.processMessage(msg)

	if collector.schedulerStats.Iterations != 0 {
		t.Error("zero values should be stored correctly")
	}
}

// =============================================================================
// Shutdown Tests
// =============================================================================

// TestGracefulShutdown_EmptyInbox verifies clean shutdown
func TestGracefulShutdown_EmptyInbox(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	collector.Start()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop should complete cleanly
	err := collector.Stop()
	if err != nil {
		t.Errorf("Stop() should not error: %v", err)
	}
}

// TestGracefulShutdown_PendingMessages verifies shutdown flushes pending stats
func TestGracefulShutdown_PendingMessages(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	config.FlushThreshold = 100 // High threshold so we don't auto-flush
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	collector.Start()

	// Send some messages (below flush threshold)
	for i := 0; i < 10; i++ {
		msg := StatsMessage{
			Source:    StatsSourceScheduler,
			Timestamp: time.Now(),
			Data:      &SchedulerStatsData{Iterations: 1},
		}
		collector.Send(msg)
	}

	time.Sleep(100 * time.Millisecond)

	// Stop should flush pending messages
	err := collector.Stop()
	if err != nil {
		t.Errorf("Stop() should not error: %v", err)
	}

	// Verify flush occurred
	if mockDB.GetSchedulerStatsCount() == 0 {
		t.Error("pending stats should have been flushed on shutdown")
	}
}

// TestStopIdempotency verifies Stop() can be called multiple times
func TestStopIdempotency(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := DefaultConfig()
	mockDB := NewMockDB()

	collector := NewStatsCollector(config, mockDB, logger.Logger())

	collector.Start()
	time.Sleep(50 * time.Millisecond)

	// Call Stop() twice
	err1 := collector.Stop()
	err2 := collector.Stop()

	if err1 != nil {
		t.Errorf("first Stop() should not error: %v", err1)
	}

	if err2 != nil {
		t.Errorf("second Stop() should not error: %v", err2)
	}
}
