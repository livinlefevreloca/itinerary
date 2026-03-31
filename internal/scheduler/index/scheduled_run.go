package index

import "time"

// ScheduledRun represents a single scheduled execution of a job at a specific time.
type ScheduledRun struct {
	JobID       string
	ScheduledAt time.Time
}
