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
// CreateOrchestratorStats records orchestrator statistics
func (db *DB) CreateOrchestratorStats(stats *OrchestratorStats) error {
	query := `
		INSERT INTO orchestrator_stats (
			run_id, stats_period_id, runtime, constraints_checked, actions_taken
		) VALUES (?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		stats.RunID,
		stats.StatsPeriodID,
		stats.Runtime,
		stats.ConstraintsChecked,
		stats.ActionsTaken,
	)
	return err
}

// CreateSyncerStats records syncer statistics for a period
func (db *DB) CreateSyncerStats(stats *SyncerStats) error {
	query := `
		INSERT INTO syncer_stats (
			stats_period_id, start_time, end_time, total_writes, writes_succeeded, writes_failed,
			avg_writes_in_flight, max_writes_in_flight, min_writes_in_flight,
			avg_queued_writes, max_queued_writes, min_queued_writes,
			avg_inbox_length, max_inbox_length, min_inbox_length,
			avg_time_in_write_queue, max_time_in_write_queue, min_time_in_write_queue,
			avg_time_in_inbox, max_time_in_inbox, min_time_in_inbox
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		stats.StatsPeriodID,
		stats.StartTime,
		stats.EndTime,
		stats.TotalWrites,
		stats.WritesSucceeded,
		stats.WritesFailed,
		stats.AvgWritesInFlight,
		stats.MaxWritesInFlight,
		stats.MinWritesInFlight,
		stats.AvgQueuedWrites,
		stats.MaxQueuedWrites,
		stats.MinQueuedWrites,
		stats.AvgInboxLength,
		stats.MaxInboxLength,
		stats.MinInboxLength,
		stats.AvgTimeInWriteQueue,
		stats.MaxTimeInWriteQueue,
		stats.MinTimeInWriteQueue,
		stats.AvgTimeInInbox,
		stats.MaxTimeInInbox,
		stats.MinTimeInInbox,
	)
	return err
}

// CreateStatsCollectorStats records stats collector statistics for a period
func (db *DB) CreateStatsCollectorStats(stats *StatsCollectorStats) error {
	query := `
		INSERT INTO stats_collector_stats (
			stats_period_id, start_time, end_time, messages_received, messages_processed,
			scheduler_messages, orchestrator_messages, syncer_messages, webhook_messages,
			periods_completed, database_flushes, flush_errors,
			avg_inbox_length, max_inbox_length, min_inbox_length,
			avg_processing_time, max_processing_time, min_processing_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		stats.StatsPeriodID,
		stats.StartTime,
		stats.EndTime,
		stats.MessagesReceived,
		stats.MessagesProcessed,
		stats.SchedulerMessages,
		stats.OrchestratorMessages,
		stats.SyncerMessages,
		stats.WebhookMessages,
		stats.PeriodsCompleted,
		stats.DatabaseFlushes,
		stats.FlushErrors,
		stats.AvgInboxLength,
		stats.MaxInboxLength,
		stats.MinInboxLength,
		stats.AvgProcessingTime,
		stats.MaxProcessingTime,
		stats.MinProcessingTime,
	)
	return err
}

// CreateWebhookDelivery records a webhook delivery attempt (future)
func (db *DB) CreateWebhookDelivery(delivery *WebhookDelivery) error {
	query := `
		INSERT INTO webhook_deliveries (
			id, run_id, webhook_type, trigger, url, attempt_count,
			status_code, success, error, request_duration, created_at, delivered_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		delivery.ID,
		delivery.RunID,
		delivery.WebhookType,
		delivery.Trigger,
		delivery.URL,
		delivery.AttemptCount,
		delivery.StatusCode,
		delivery.Success,
		delivery.Error,
		delivery.RequestDuration,
		delivery.CreatedAt,
		delivery.DeliveredAt,
	)
	return err
}

// CreateWebhookHandlerStats records webhook handler statistics for a period (future)
func (db *DB) CreateWebhookHandlerStats(stats *WebhookHandlerStats) error {
	query := `
		INSERT INTO webhook_handler_stats (
			stats_period_id, start_time, end_time, webhooks_sent, webhooks_succeeded, webhooks_failed,
			total_retries, avg_delivery_time, max_delivery_time, min_delivery_time,
			avg_inbox_length, max_inbox_length, min_inbox_length
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		stats.StatsPeriodID,
		stats.StartTime,
		stats.EndTime,
		stats.WebhooksSent,
		stats.WebhooksSucceeded,
		stats.WebhooksFailed,
		stats.TotalRetries,
		stats.AvgDeliveryTime,
		stats.MaxDeliveryTime,
		stats.MinDeliveryTime,
		stats.AvgInboxLength,
		stats.MaxInboxLength,
		stats.MinInboxLength,
	)
	return err
}

// GetSyncerStats retrieves syncer statistics for a time range
func (db *DB) GetSyncerStats(startTime, endTime time.Time) ([]SyncerStats, error) {
	query := `
		SELECT stats_period_id, start_time, end_time, total_writes, writes_succeeded, writes_failed,
			avg_writes_in_flight, max_writes_in_flight, min_writes_in_flight,
			avg_queued_writes, max_queued_writes, min_queued_writes,
			avg_inbox_length, max_inbox_length, min_inbox_length,
			avg_time_in_write_queue, max_time_in_write_queue, min_time_in_write_queue,
			avg_time_in_inbox, max_time_in_inbox, min_time_in_inbox
		FROM syncer_stats
		WHERE start_time >= ? AND end_time <= ?
		ORDER BY start_time ASC
	`

	rows, err := db.Query(query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []SyncerStats
	for rows.Next() {
		var s SyncerStats
		err := rows.Scan(
			&s.StatsPeriodID,
			&s.StartTime,
			&s.EndTime,
			&s.TotalWrites,
			&s.WritesSucceeded,
			&s.WritesFailed,
			&s.AvgWritesInFlight,
			&s.MaxWritesInFlight,
			&s.MinWritesInFlight,
			&s.AvgQueuedWrites,
			&s.MaxQueuedWrites,
			&s.MinQueuedWrites,
			&s.AvgInboxLength,
			&s.MaxInboxLength,
			&s.MinInboxLength,
			&s.AvgTimeInWriteQueue,
			&s.MaxTimeInWriteQueue,
			&s.MinTimeInWriteQueue,
			&s.AvgTimeInInbox,
			&s.MaxTimeInInbox,
			&s.MinTimeInInbox,
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
		stats = []SyncerStats{}
	}

	return stats, nil
}

// GetStatsCollectorStats retrieves stats collector statistics for a time range
func (db *DB) GetStatsCollectorStats(startTime, endTime time.Time) ([]StatsCollectorStats, error) {
	query := `
		SELECT stats_period_id, start_time, end_time, messages_received, messages_processed,
			scheduler_messages, orchestrator_messages, syncer_messages, webhook_messages,
			periods_completed, database_flushes, flush_errors,
			avg_inbox_length, max_inbox_length, min_inbox_length,
			avg_processing_time, max_processing_time, min_processing_time
		FROM stats_collector_stats
		WHERE start_time >= ? AND end_time <= ?
		ORDER BY start_time ASC
	`

	rows, err := db.Query(query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []StatsCollectorStats
	for rows.Next() {
		var s StatsCollectorStats
		err := rows.Scan(
			&s.StatsPeriodID,
			&s.StartTime,
			&s.EndTime,
			&s.MessagesReceived,
			&s.MessagesProcessed,
			&s.SchedulerMessages,
			&s.OrchestratorMessages,
			&s.SyncerMessages,
			&s.WebhookMessages,
			&s.PeriodsCompleted,
			&s.DatabaseFlushes,
			&s.FlushErrors,
			&s.AvgInboxLength,
			&s.MaxInboxLength,
			&s.MinInboxLength,
			&s.AvgProcessingTime,
			&s.MaxProcessingTime,
			&s.MinProcessingTime,
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
		stats = []StatsCollectorStats{}
	}

	return stats, nil
}
