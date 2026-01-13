# Scheduled Run Index Implementation Plan

## Overview
The Scheduled Run Index is a time-ordered data structure that efficiently tracks when jobs need to run. It uses a **sorted slice** with atomic pointer swapping for lock-free bulk rebuilds.

## Key Insight
From cron parser benchmarks:
- Calculating 1 hour of runs for 10,000 schedules: ~5ms
- Matching 10,000 schedules against 1 hour window: ~5ms
- Data comes grouped by job, **requires sorting by time**

**Scale consideration**: 10,000 jobs running every minute = 600,000 runs/hour
- With 2-hour lookahead or jobs running more frequently: could be 1M+ runs
- We'll test up to 1,000,000 runs to be safe
- Sorting adds O(n log n) time: Go's sort is ~50-100ms for 1M items
- Total rebuild time: cron calculation (~5-10ms) + sorting (~50-100ms) = ~60-110ms for 1M runs

Therefore: **Rebuild the entire index periodically rather than incremental updates**

**Implementation choice**: Sorted slice with binary search
- Both BTree and sorted slice have O(log n) lookup
- No incremental updates needed, so BTree provides no advantage
- Sorted slice is simpler, no external dependencies
- Better cache locality for sequential access

## Requirements

### Functional Requirements
1. **Build**: Construct index from pre-sorted scheduled runs
2. **Query**: Find all jobs that should run within a time window [start, end)
3. **Peek**: Get the next job(s) scheduled to run (earliest time)
4. **Swap**: Atomically replace old index with new one (no locks during queries)

### Performance Requirements
- Build from 1,000,000 runs (including sorting): O(n log n), target < 150ms
- Query: O(log n + k) where k is number of results, target < 1ms
- Peek: O(1), target < 1µs
- Memory overhead: < 50 bytes per run
- No lock contention on read path

## Design Philosophy

### Bulk Rebuild Strategy
```
Every scheduler loop iteration (e.g., every 30 seconds):
1. Calculate next N hours of runs for all jobs (using cron parser)
2. Collect results (grouped by job, unsorted)
3. Sort all runs by (ScheduledAt, JobID) - O(n log n)
4. Build new index (just assign sorted slice)
5. Atomically swap index pointer (old index can still be queried)
6. Old index garbage collected when no readers remain
```

### Why This Works
- Cron calculation is fast: O(jobs × minutes_lookahead)
- For 10,000 jobs × 60 minutes = ~5ms from benchmarks
- Sorting is fast: O(n log n), ~50-100ms for 1M items in Go
- Building sorted slice is O(n) copy: extremely fast
- Total rebuild time acceptable: ~60-110ms for worst case (1M runs)
- No lock contention: readers use old index while new one builds
- Simple: no complex update logic, no coordination

### When to Rebuild
- **Periodic**: Every 30-60 seconds (scheduler loop)
- **On schedule change**: Job schedule updated
- **On job add/remove**: New job added or deleted
- **On startup**: Load all jobs

## Data Structures

### ScheduledRun
```go
type ScheduledRun struct {
    JobID       string
    ScheduledAt time.Time
}
```

Note: No methods needed - we build from pre-sorted data.

### ScheduledRunIndex
```go
type ScheduledRunIndex struct {
    runs atomic.Pointer[[]ScheduledRun]
}

// NewScheduledRunIndex builds index from runs (will sort them)
func NewScheduledRunIndex(runs []ScheduledRun) *ScheduledRunIndex

// Query finds all runs in time window [start, end)
func (idx *ScheduledRunIndex) Query(start, end time.Time) []ScheduledRun

// Peek returns the earliest scheduled run
func (idx *ScheduledRunIndex) Peek() (*ScheduledRun, bool)

// Len returns number of scheduled runs
func (idx *ScheduledRunIndex) Len() int

// Swap atomically replaces the index with new sorted runs
func (idx *ScheduledRunIndex) Swap(newRuns []ScheduledRun)
```

## Implementation Details

### Thread Safety via Atomic Swap
```go
type ScheduledRunIndex struct {
    runs atomic.Pointer[[]ScheduledRun]
}

// Readers just load the pointer - no locks!
func (idx *ScheduledRunIndex) Query(start, end time.Time) []ScheduledRun {
    runs := idx.runs.Load()
    if runs == nil || len(*runs) == 0 {
        return nil
    }

    // Binary search + scan, no locks needed
    // Even if writer swaps slice, we still have reference to old slice
    return querySlice(*runs, start, end)
}

// Writers create new slice and swap atomically
func RebuildIndex(idx *ScheduledRunIndex, newRuns []ScheduledRun) {
    idx.runs.Store(&newRuns)
    // Old slice will be GC'd when all readers finish
}
```

### Query Implementation
```go
func (idx *ScheduledRunIndex) Query(start, end time.Time) []ScheduledRun {
    runs := idx.runs.Load()
    if runs == nil || len(*runs) == 0 {
        return nil
    }

    slice := *runs

    // Binary search for start position
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
```

### Peek Implementation
```go
func (idx *ScheduledRunIndex) Peek() (*ScheduledRun, bool) {
    runs := idx.runs.Load()
    if runs == nil || len(*runs) == 0 {
        return nil, false
    }

    // First element is always earliest (slice is sorted)
    return &(*runs)[0], true
}
```

### Len Implementation
```go
func (idx *ScheduledRunIndex) Len() int {
    runs := idx.runs.Load()
    if runs == nil {
        return 0
    }
    return len(*runs)
}
```

### Swap Implementation
```go
func (idx *ScheduledRunIndex) Swap(newRuns []ScheduledRun) {
    idx.runs.Store(&newRuns)
}
```

### Handling Multiple Jobs at Same Time
The index sorts data to ensure stable ordering:
- Sort by (ScheduledAt, JobID) in NewScheduledRunIndex and Swap
- Ensures deterministic iteration order
- Multiple jobs at same time are ordered by JobID

### Generating Runs (Caller Responsibility)
```go
// In scheduler package - generates unsorted runs
func GenerateScheduledRuns(jobs []Job, start, end time.Time) []ScheduledRun {
    runs := []ScheduledRun{}

    for _, job := range jobs {
        schedule, _ := cron.Parse(job.Schedule)
        times := schedule.Between(start, end)

        for _, t := range times {
            runs = append(runs, ScheduledRun{
                JobID:       job.ID,
                ScheduledAt: t,
            })
        }
    }

    // Runs are grouped by job, unsorted by time
    // Index will sort them
    return runs
}

// Sorting happens inside NewScheduledRunIndex and Swap
func sortRuns(runs []ScheduledRun) {
    sort.Slice(runs, func(i, j int) bool {
        if runs[i].ScheduledAt.Equal(runs[j].ScheduledAt) {
            return runs[i].JobID < runs[j].JobID
        }
        return runs[i].ScheduledAt.Before(runs[j].ScheduledAt)
    })
}
```

## No Error Handling Needed

Since there are no mutations, there are no errors:
- Build always succeeds (or panics if out of memory)
- Query always succeeds (may return empty)
- Peek always succeeds (may return false)
- Swap always succeeds

Simple and robust!

## Testing Strategy

### Unit Tests
1. **Build from empty**: Empty input produces queryable index
2. **Build from sorted data**: Correct ordering maintained
3. **Query exact boundaries**: Start/end time edge cases
4. **Query with no results**: Empty time window
5. **Query with all results**: Large time window
6. **Peek**: First element, empty index
7. **Multiple jobs at same time**: Stable ordering
8. **Concurrent reads**: Race detector, multiple goroutines querying
9. **Concurrent rebuild**: Swap while queries in progress

### Benchmark Tests

#### Build Performance
- Build from 10, 100, 1000, 10000, 100000, 1000000 unsorted runs
- Memory allocations
- Should be O(n log n) due to sorting
- Sorting will dominate the cost for large datasets

#### Query Performance
- Various time windows (1 min, 1 hour, 1 day)
- Various result sizes (10, 100, 1000, 10000 results)
- Test with different index sizes (1000, 10000, 100000, 1000000 runs)
- Verify O(log n + k) performance

#### Peek Performance
- Constant time access to first element
- Test with various index sizes (up to 1M runs)
- Should be O(1) regardless of size

#### Realistic Workload
- Simulate scheduler loop:
  - Rebuild every 30 seconds
  - Query every 1 second
  - 10,000 jobs (up to 600,000 runs/hour at 1/min frequency)
- Measure latency percentiles (p50, p95, p99)
- Test with 1,000,000 runs in index (extreme case)

## Integration with Scheduler

### Usage Pattern
```go
type Scheduler struct {
    index     *ScheduledRunIndex
    jobs      []Job
    lookahead time.Duration // e.g., 1 hour
}

// Rebuild index periodically
func (s *Scheduler) rebuildIndex() {
    now := time.Now()
    end := now.Add(s.lookahead)

    // Generate all scheduled runs (fast!)
    runs := GenerateScheduledRuns(s.jobs, now, end)

    // Swap atomically (instant!)
    s.index.Swap(runs)
}

// Main scheduler loop
func (s *Scheduler) loop() {
    rebuildTicker := time.NewTicker(30 * time.Second)
    queryTicker := time.NewTicker(1 * time.Second)

    // Initial build
    s.rebuildIndex()

    for {
        select {
        case <-rebuildTicker.C:
            s.rebuildIndex()

        case <-queryTicker.C:
            now := time.Now()
            window := 30 * time.Second

            // Query jobs to run in next 30 seconds
            runs := s.index.Query(now, now.Add(window))

            for _, run := range runs {
                // Start orchestrator for each job
                s.startOrchestrator(run)
            }
        }
    }
}
```

### Edge Cases Handled
1. **Schedule changes**: Just rebuild index on next cycle
2. **New jobs**: Included in next rebuild
3. **Deleted jobs**: Not included in next rebuild
4. **Clock adjustments**: Rebuild handles naturally
5. **Empty index**: Query returns empty slice
6. **Past times**: Filtered out during generation

### Rebuild Frequency Trade-offs
- **More frequent** (10s): Lower memory, fresher data, more CPU
- **Less frequent** (60s): Higher memory, stale data, less CPU
- **Sweet spot** (30s): Good balance for most workloads

## Performance Goals

### Target Performance
- Build 10,000 runs: < 5ms (sorting + slice operations)
- Build 100,000 runs: < 30ms (sorting dominates)
- Build 1,000,000 runs: < 150ms (sorting ~100ms + operations)
- Query 100 runs: < 1ms (even with 1M runs in index)
- Peek: < 1µs
- Memory per run: ~40 bytes (jobID string + time.Time)
- Total memory for 1M runs: < 100MB

### Expected Characteristics
- Build: O(n log n) - sorting dominates
- Query: O(log n + k) - binary search + linear scan
- Peek: O(1) - array index
- Memory: O(n) - proportional to runs

## Future Optimizations

### If Needed (Only if benchmarks show issues)
1. **Pre-allocated queries**: Reuse result slices
2. **Separate peek cache**: Cache first N items
3. **Parallel rebuild**: Generate runs in parallel per job
4. **Incremental rebuild**: Only recalculate changed jobs
5. **Custom binary search**: Skip sort.Search overhead

### NOT Needed
- Incremental updates (rebuild is fast enough)
- Complex data structures (slice is sufficient)
- Persistence (rebuilt on startup)
- BTree or other structures (no advantage for this use case)

## Package Location
`lib/scheduler/index/` - Part of scheduler package

## Dependencies
- Standard library only
- No external dependencies

## Files Structure
```
lib/scheduler/index/
├── scheduled_run.go          # ScheduledRun type
├── index.go                   # ScheduledRunIndex implementation
├── index_test.go              # Unit tests
├── index_bench_test.go        # Benchmarks
└── generator_test.go          # Test helper for generating runs
```

## Open Questions
1. **Rebuild frequency**: 30s? 60s? Configurable? (Start with 30s)
2. **Lookahead window**: 1 hour? 2 hours? (Start with 1 hour)
3. **Pre-allocate result slice**: Worth it? (Benchmark will answer)

## Success Criteria
- ✅ Build 10,000 runs in < 5ms (including sort)
- ✅ Build 100,000 runs in < 30ms (including sort)
- ✅ Build 1,000,000 runs in < 150ms (including sort)
- ✅ Query < 1ms for typical workload (even with 1M runs)
- ✅ No race conditions (go test -race)
- ✅ Memory usage reasonable (~40 bytes per run, < 100MB for 1M runs)
- ✅ Can swap index while queries in progress
- ✅ Simple implementation (< 200 lines of code)
