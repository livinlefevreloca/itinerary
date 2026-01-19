package syncer

import "time"

// JobRunUpdate represents a database update for a job run
type JobRunUpdate struct {
	UpdateID    string // UUID for idempotent database writes
	RunID       string // Deterministic format: "jobID:unixTimestamp"
	JobID       string
	ScheduledAt time.Time
	CompletedAt time.Time
	Status      string
	Success     bool
	Error       error
}

// Stats provides current syncer statistics
type Stats struct {
	BufferedJobRunUpdates int
	BufferedStats         int
}

// StatsUpdate represents a batch of stats to be written to the database
// The Stats field contains stats specific to the component sending them
type StatsUpdate struct {
	Stats []interface{}
}
