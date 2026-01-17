package index

import (
	"fmt"
	"time"
)

// generateUnsortedRuns generates unsorted runs (grouped by job, realistic input).
// This simulates what the scheduler would produce when iterating through jobs.
func generateUnsortedRuns(count int, startTime time.Time, interval time.Duration) []ScheduledRun {
	runs := make([]ScheduledRun, 0, count)

	// Simulate multiple jobs, each generating their runs in sequence
	// This creates data grouped by job (unsorted by time)
	jobCount := 10
	runsPerJob := count / jobCount
	remainder := count % jobCount

	if runsPerJob == 0 {
		// For small counts, just create one run per job
		jobCount = count
		runsPerJob = 1
		remainder = 0
	}

	runIndex := 0
	for j := 0; j < jobCount && runIndex < count; j++ {
		jobID := generateJobID(j)
		// Give this job its base runs plus one extra if we have remainder
		jobRuns := runsPerJob
		if j < remainder {
			jobRuns++
		}

		for i := 0; i < jobRuns && runIndex < count; i++ {
			runs = append(runs, ScheduledRun{
				JobID:       jobID,
				ScheduledAt: startTime.Add(time.Duration(i) * interval),
			})
			runIndex++
		}
	}

	return runs
}

// generateRealisticRuns generates runs with specific patterns.
// Returns unsorted data (grouped by job, like real scheduler).
func generateRealisticRuns(jobCount int, startTime, endTime time.Time, intervals []time.Duration) []ScheduledRun {
	runs := []ScheduledRun{}

	for j := 0; j < jobCount; j++ {
		jobID := generateJobID(j)
		interval := intervals[j%len(intervals)]

		for t := startTime; t.Before(endTime); t = t.Add(interval) {
			runs = append(runs, ScheduledRun{
				JobID:       jobID,
				ScheduledAt: t,
			})
		}
	}

	return runs
}

// generateCoincidentRuns generates multiple runs at the exact same time.
func generateCoincidentRuns(count int, sameTime time.Time) []ScheduledRun {
	runs := make([]ScheduledRun, count)
	for i := 0; i < count; i++ {
		runs[i] = ScheduledRun{
			JobID:       generateJobID(i),
			ScheduledAt: sameTime,
		}
	}
	return runs
}

// generateJobID generates a job ID with realistic format.
func generateJobID(index int) string {
	return fmt.Sprintf("job-%04d", index)
}

// generateLargeJobID generates a very long job ID for edge case testing.
func generateLargeJobID(index int, length int) string {
	base := fmt.Sprintf("job-%04d-", index)
	for len(base) < length {
		base += "x"
	}
	return base[:length]
}
