package index

import (
	"sort"
	"sync/atomic"
	"time"
)

// ScheduledRunIndex is a time-ordered index of scheduled job runs.
// It uses an atomic pointer for lock-free concurrent reads.
type ScheduledRunIndex struct {
	runs atomic.Pointer[[]ScheduledRun]
}

// NewScheduledRunIndex creates a new index from the given runs.
// The runs are sorted by (ScheduledAt, JobID) before being stored.
// The input slice is copied and sorted, so the caller can safely reuse it.
func NewScheduledRunIndex(runs []ScheduledRun) *ScheduledRunIndex {
	idx := &ScheduledRunIndex{}

	// Make a copy and sort
	sorted := make([]ScheduledRun, len(runs))
	copy(sorted, runs)
	sortRuns(sorted)

	// Store sorted slice
	idx.runs.Store(&sorted)

	return idx
}

// Query returns all runs in the time window [start, end).
// Start is inclusive, end is exclusive.
// Returns runs sorted by (ScheduledAt, JobID).
func (idx *ScheduledRunIndex) Query(start, end time.Time) []ScheduledRun {
	runs := idx.runs.Load()
	if runs == nil || len(*runs) == 0 {
		return nil
	}

	slice := *runs

	// Binary search for first run >= start
	startIdx := sort.Search(len(slice), func(i int) bool {
		return !slice[i].ScheduledAt.Before(start)
	})

	// Collect all runs until end
	results := []ScheduledRun{}
	for i := startIdx; i < len(slice); i++ {
		if slice[i].ScheduledAt.Before(end) {
			results = append(results, slice[i])
		} else {
			break
		}
	}

	return results
}

// Len returns the number of scheduled runs in the index.
func (idx *ScheduledRunIndex) Len() int {
	runs := idx.runs.Load()
	if runs == nil {
		return 0
	}
	return len(*runs)
}

// Swap atomically replaces the index with new runs.
// The new runs are sorted by (ScheduledAt, JobID) before being stored.
// The input slice is copied and sorted, so the caller can safely reuse it.
func (idx *ScheduledRunIndex) Swap(newRuns []ScheduledRun) {
	// Make a copy and sort
	sorted := make([]ScheduledRun, len(newRuns))
	copy(sorted, newRuns)
	sortRuns(sorted)

	// Atomically swap
	idx.runs.Store(&sorted)
}

// sortRuns sorts runs by (ScheduledAt, JobID).
// When times are equal, runs are ordered by JobID for deterministic iteration.
func sortRuns(runs []ScheduledRun) {
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].ScheduledAt.Equal(runs[j].ScheduledAt) {
			return runs[i].JobID < runs[j].JobID
		}
		return runs[i].ScheduledAt.Before(runs[j].ScheduledAt)
	})
}
