package db

import "time"

// CreateSchedulerStats inserts scheduler statistics
func (db *DB) CreateSchedulerStats(stats *SchedulerStats) error {
	query := `
		INSERT INTO scheduler_stats (
			stats_period_id, start_time, end_time, iterations, run_jobs, late_jobs,
			time_passed_run_time, missed_jobs, time_passed_grace_period, jobs_cancelled,
			min_inbox_length, max_inbox_length, avg_inbox_length, empty_inbox_time,
			avg_time_in_inbox, min_time_in_inbox, max_time_in_inbox
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		stats.StatsPeriodID,
		stats.StartTime,
		stats.EndTime,
		stats.Iterations,
		stats.RunJobs,
		stats.LateJobs,
		stats.TimePassedRunTime,
		stats.MissedJobs,
		stats.TimePassedGracePeriod,
		stats.JobsCancelled,
		stats.MinInboxLength,
		stats.MaxInboxLength,
		stats.AvgInboxLength,
		stats.EmptyInboxTime,
		stats.AvgTimeInInbox,
		stats.MinTimeInInbox,
		stats.MaxTimeInInbox,
	)

	return err
}

// GetSchedulerStats retrieves scheduler stats for a period
func (db *DB) GetSchedulerStats(startTime, endTime time.Time) ([]SchedulerStats, error) {
	query := `
		SELECT
			stats_period_id, start_time, end_time, iterations, run_jobs, late_jobs,
			time_passed_run_time, missed_jobs, time_passed_grace_period, jobs_cancelled,
			min_inbox_length, max_inbox_length, avg_inbox_length, empty_inbox_time,
			avg_time_in_inbox, min_time_in_inbox, max_time_in_inbox
		FROM scheduler_stats
		WHERE (start_time >= ? AND start_time < ?) OR (end_time > ? AND end_time <= ?)
		ORDER BY start_time
	`

	rows, err := db.Query(query, startTime, endTime, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []SchedulerStats
	for rows.Next() {
		var s SchedulerStats
		err := rows.Scan(
			&s.StatsPeriodID,
			&s.StartTime,
			&s.EndTime,
			&s.Iterations,
			&s.RunJobs,
			&s.LateJobs,
			&s.TimePassedRunTime,
			&s.MissedJobs,
			&s.TimePassedGracePeriod,
			&s.JobsCancelled,
			&s.MinInboxLength,
			&s.MaxInboxLength,
			&s.AvgInboxLength,
			&s.EmptyInboxTime,
			&s.AvgTimeInInbox,
			&s.MinTimeInInbox,
			&s.MaxTimeInInbox,
		)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if stats == nil {
		stats = []SchedulerStats{}
	}

	return stats, nil
}
