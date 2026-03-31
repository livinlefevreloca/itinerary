package stats

import (
	"fmt"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/db"
)

// DBAdapter adapts the internal db.DB to the DatabaseWriter interface
type DBAdapter struct {
	db *db.DB
}

// NewDBAdapter creates a new database adapter
func NewDBAdapter(database *db.DB) *DBAdapter {
	return &DBAdapter{db: database}
}

// WriteSchedulerStats writes scheduler statistics to the database
func (a *DBAdapter) WriteSchedulerStats(periodID string, startTime, endTime time.Time, data *SchedulerStatsAccumulator) error {
	// Calculate min/max/avg for inbox length
	minInbox, maxInbox, avgInbox := calculateMinMaxAvgInt(data.InboxLengthSamples)

	// Calculate min/max/avg for message wait times
	minWait, maxWait, avgWait := calculateMinMaxAvgDuration(data.MessageWaitTimes)

	query := `
		INSERT INTO scheduler_stats (
			stats_period_id, start_time, end_time,
			iterations, run_jobs, late_jobs, missed_jobs, jobs_cancelled,
			min_inbox_length, max_inbox_length, avg_inbox_length,
			empty_inbox_time, avg_time_in_inbox, min_time_in_inbox, max_time_in_inbox,
			time_passed_run_time, time_passed_grace_period
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0)
	`

	_, err := a.db.Exec(query,
		periodID,
		startTime,
		endTime,
		data.Iterations,
		data.JobsRun,
		data.LateJobs,
		data.MissedJobs,
		data.JobsCancelled,
		minInbox,
		maxInbox,
		avgInbox,
		data.EmptyInboxTime.Nanoseconds(),
		avgWait.Nanoseconds(),
		minWait.Nanoseconds(),
		maxWait.Nanoseconds(),
	)

	if err != nil {
		return fmt.Errorf("failed to write scheduler stats: %w", err)
	}

	return nil
}

// WriteOrchestratorStats writes orchestrator statistics to the database
func (a *DBAdapter) WriteOrchestratorStats(periodID string, startTime, endTime time.Time, stats map[string]*OrchestratorStatsData) error {
	if len(stats) == 0 {
		return nil
	}

	tx, err := a.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO orchestrator_stats (
			run_id, stats_period_id, runtime, constraints_checked, actions_taken
		) VALUES (?, ?, ?, ?, ?)
	`

	for _, stat := range stats {
		_, err := tx.Exec(query,
			stat.RunID,
			periodID,
			stat.Runtime.Nanoseconds(),
			stat.ConstraintsChecked,
			stat.ActionsTaken,
		)
		if err != nil {
			return fmt.Errorf("failed to write orchestrator stats for run %s: %w", stat.RunID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// WriteSyncerStats writes syncer statistics to the database
func (a *DBAdapter) WriteSyncerStats(periodID string, startTime, endTime time.Time, data *SyncerStatsAccumulator) error {
	// Calculate min/max/avg for writes in flight
	minInFlight, maxInFlight, avgInFlight := calculateMinMaxAvgInt(data.WritesInFlightSamples)

	// Calculate min/max/avg for queued writes
	minQueued, maxQueued, avgQueued := calculateMinMaxAvgInt(data.QueuedWritesSamples)

	// Calculate min/max/avg for inbox length
	minInbox, maxInbox, avgInbox := calculateMinMaxAvgInt(data.InboxLengthSamples)

	// Calculate min/max/avg for time in queue
	minTimeInQueue, maxTimeInQueue, avgTimeInQueue := calculateMinMaxAvgDuration(data.TimeInQueueSamples)

	// Calculate min/max/avg for time in inbox
	minTimeInInbox, maxTimeInInbox, avgTimeInInbox := calculateMinMaxAvgDuration(data.TimeInInboxSamples)

	query := `
		INSERT INTO syncer_stats (
			stats_period_id, start_time, end_time,
			total_writes, writes_succeeded, writes_failed,
			avg_writes_in_flight, max_writes_in_flight, min_writes_in_flight,
			avg_queued_writes, max_queued_writes, min_queued_writes,
			avg_inbox_length, max_inbox_length, min_inbox_length,
			avg_time_in_write_queue, max_time_in_write_queue, min_time_in_write_queue,
			avg_time_in_inbox, max_time_in_inbox, min_time_in_inbox
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := a.db.Exec(query,
		periodID,
		startTime,
		endTime,
		data.TotalWrites,
		data.WritesSucceeded,
		data.WritesFailed,
		avgInFlight,
		maxInFlight,
		minInFlight,
		avgQueued,
		maxQueued,
		minQueued,
		avgInbox,
		maxInbox,
		minInbox,
		avgTimeInQueue.Nanoseconds(),
		maxTimeInQueue.Nanoseconds(),
		minTimeInQueue.Nanoseconds(),
		avgTimeInInbox.Nanoseconds(),
		maxTimeInInbox.Nanoseconds(),
		minTimeInInbox.Nanoseconds(),
	)

	if err != nil {
		return fmt.Errorf("failed to write syncer stats: %w", err)
	}

	return nil
}
