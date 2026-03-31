package cron

import (
	"time"
)

// CronSchedule represents a parsed cron expression
type CronSchedule struct {
	// Each field stores all valid values for that field
	minutes     []int // 0-59
	hours       []int // 0-23
	daysOfMonth []int // 1-31
	months      []int // 1-12
	daysOfWeek  []int // 0-6 (0=Sunday)

	// Store original expression for debugging
	original string
}

// Parse parses a cron expression and validates all constraints
// Returns error if:
// - Format is invalid (not 5 fields)
// - Any field contains invalid syntax
// - Impossible dates are specified (e.g., Feb 31st)
func Parse(expr string) (*CronSchedule, error) {
	return parse(expr)
}

// Next calculates the next N occurrences of this schedule after the given time
// "After" means strictly after - if 'after' is exactly at a scheduled time, that time is NOT included
// The time is evaluated in the provided timezone
// Returns empty slice if no occurrences found (should not happen for valid schedules)
func (cs *CronSchedule) Next(after time.Time, count int) []time.Time {
	results := make([]time.Time, 0, count)

	// Start checking from the next minute after 'after'
	// Truncate to minute boundary, then advance by 1 minute
	current := after.Truncate(time.Minute).Add(time.Minute)

	for len(results) < count {
		if cs.matches(current) {
			results = append(results, current)
		}
		current = current.Add(time.Minute)
	}

	return results
}

// Between calculates all occurrences within the given time window [start, end)
// Start is inclusive, end is exclusive
// Returns all matching times in chronological order
func (cs *CronSchedule) Between(start, end time.Time) []time.Time {
	results := []time.Time{}

	// Start at start time truncated to minute
	current := start.Truncate(time.Minute)

	for current.Before(end) {
		if cs.matches(current) {
			results = append(results, current)
		}
		current = current.Add(time.Minute)
	}

	return results
}
