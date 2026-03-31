package stats

import (
	"fmt"
	"sync/atomic"
	"time"
)

// MockStatsDatabaseWriter simulates database operations for stats testing.
// Implements DatabaseWriter.
type MockStatsDatabaseWriter struct {
	schedulerStats []*SchedulerStatsData
	orchestrator   []*OrchestratorStatsData
	syncerStats    []*SyncerStatsData
	failCount      int32
	writeLatency   time.Duration
	writeCalls     int32
}

func NewMockStatsDatabaseWriter() *MockStatsDatabaseWriter {
	return &MockStatsDatabaseWriter{
		schedulerStats: make([]*SchedulerStatsData, 0),
		orchestrator:   make([]*OrchestratorStatsData, 0),
		syncerStats:    make([]*SyncerStatsData, 0),
	}
}

func (m *MockStatsDatabaseWriter) WriteSchedulerStats(periodID string, startTime, endTime time.Time, data *SchedulerStatsAccumulator) error {
	atomic.AddInt32(&m.writeCalls, 1)

	if m.writeLatency > 0 {
		time.Sleep(m.writeLatency)
	}

	failCount := atomic.LoadInt32(&m.failCount)
	if failCount > 0 {
		atomic.AddInt32(&m.failCount, -1)
		return fmt.Errorf("simulated database failure")
	}

	m.schedulerStats = append(m.schedulerStats, &SchedulerStatsData{
		Iterations:    data.Iterations,
		JobsRun:       data.JobsRun,
		LateJobs:      data.LateJobs,
		MissedJobs:    data.MissedJobs,
		JobsCancelled: data.JobsCancelled,
	})
	return nil
}

func (m *MockStatsDatabaseWriter) WriteOrchestratorStats(periodID string, startTime, endTime time.Time, statsMap map[string]*OrchestratorStatsData) error {
	atomic.AddInt32(&m.writeCalls, 1)

	if m.writeLatency > 0 {
		time.Sleep(m.writeLatency)
	}

	failCount := atomic.LoadInt32(&m.failCount)
	if failCount > 0 {
		atomic.AddInt32(&m.failCount, -1)
		return fmt.Errorf("simulated database failure")
	}

	for _, stat := range statsMap {
		m.orchestrator = append(m.orchestrator, stat)
	}
	return nil
}

func (m *MockStatsDatabaseWriter) WriteSyncerStats(periodID string, startTime, endTime time.Time, data *SyncerStatsAccumulator) error {
	atomic.AddInt32(&m.writeCalls, 1)

	if m.writeLatency > 0 {
		time.Sleep(m.writeLatency)
	}

	failCount := atomic.LoadInt32(&m.failCount)
	if failCount > 0 {
		atomic.AddInt32(&m.failCount, -1)
		return fmt.Errorf("simulated database failure")
	}

	m.syncerStats = append(m.syncerStats, &SyncerStatsData{
		TotalWrites:     data.TotalWrites,
		WritesSucceeded: data.WritesSucceeded,
		WritesFailed:    data.WritesFailed,
	})
	return nil
}

func (m *MockStatsDatabaseWriter) GetSchedulerStatsCount() int {
	return len(m.schedulerStats)
}

func (m *MockStatsDatabaseWriter) GetOrchestratorStatsCount() int {
	return len(m.orchestrator)
}

func (m *MockStatsDatabaseWriter) GetSyncerStatsCount() int {
	return len(m.syncerStats)
}

func (m *MockStatsDatabaseWriter) GetWriteCalls() int {
	return int(atomic.LoadInt32(&m.writeCalls))
}

func (m *MockStatsDatabaseWriter) SetFailCount(count int) {
	atomic.StoreInt32(&m.failCount, int32(count))
}
