# Scheduled Run Index Implementation Plan

## Overview
The Scheduled Run Index is a time-ordered data structure that efficiently tracks when jobs need to run. Unlike a traditional mutable index, this is designed for **bulk rebuilding** from pre-sorted data, since calculating all scheduled runs from cron expressions is extremely fast.

## Key Insight
From cron parser benchmarks:
- Calculating 1 hour of runs for 10,000 schedules: ~5ms
- Matching 10,000 schedules against 1 hour window: ~5ms
- Data naturally comes out in sorted order when we iterate through schedules

**Scale consideration**: 10,000 jobs running every minute = 600,000 runs/hour
- With 2-hour lookahead or jobs running more frequently: could be 1M+ runs
- We'll test up to 1,000,000 runs to be safe
- Even at this scale, rebuild should be fast enough

Therefore: **Rebuild the entire index periodically rather than incremental updates**

## Requirements

### Functional Requirements
1. **Build**: Construct index from pre-sorted scheduled runs
2. **Query**: Find all jobs that should run within a time window [start, end)
3. **Peek**: Get the next job(s) scheduled to run (earliest time)
4. **Swap**: Atomically replace old index with new one (no locks during queries)

### Performance Requirements
- Build from 1,000,000 sorted runs: O(n), target < 500ms
- Query: O(log n + k) where k is number of results, target < 1ms
- Peek: O(1), target < 1µs
- Memory overhead: < 50 bytes per run
- No lock contention on read path

## Design Philosophy

### Bulk Rebuild Strategy
```
Every scheduler loop iteration (e.g., every 30 seconds):
1. Calculate next N hours of runs for all jobs (using cron parser)
2. Collect results in sorted order
3. Build new index from sorted data
4. Atomically swap index pointer (old index can still be queried)
5. Old index garbage collected when no readers remain
```

### Why This Works
- Cron calculation is fast: O(jobs × minutes_lookahead)
- For 10,000 jobs × 60 minutes = ~5ms from benchmarks
- Bulk build is faster than incremental updates: O(n) vs O(n log n)
- No lock contention: readers use old index while new one builds
- Simple: no complex update logic, no coordination

### When to Rebuild
- **Periodic**: Every 30-60 seconds (scheduler loop)
- **On schedule change**: Job schedule updated
- **On job add/remove**: New job added or deleted
- **On startup**: Load all jobs

## Design Options

### Option A: Google BTree with Bulk Build (Chosen)
Use `github.com/google/btree` with bottom-up bulk construction.

**Pros:**
- Bulk build from sorted data is O(n) and very fast
- Efficient range queries O(log n + k)
- Good cache locality
- Battle-tested in production

**Cons:**
- External dependency
- Need to implement bulk build (BTree doesn't have built-in bulk build)

**Bulk Build Strategy:**
- BTree degree d means each node has [d, 2d-1] items
- Pack sorted data into leaf nodes (fill to 2d-1 items each)
- Build internal nodes bottom-up
- Result: minimal height, well-balanced tree

### Option B: Simple Sorted Slice (Comparison)
Just maintain a sorted slice of (time, jobID) pairs.

**Pros:**
- No external dependencies
- Build is just slice assignment: O(n)
- Excellent cache locality
- Very simple implementation

**Cons:**
- Query requires binary search + linear scan
- Still O(log n + k) but worse constants

**Decision**: Implement both, benchmark to decide. Sorted slice may be sufficient!

## Data Structures

### ScheduledRun
```go
type ScheduledRun struct {
    JobID       string
    ScheduledAt time.Time
}
```

Note: No Less() method needed - we build from pre-sorted data.

### ScheduledRunIndex (BTree Implementation)
```go
type ScheduledRunIndex struct {
    tree atomic.Pointer[btree.BTree]
}

// NewScheduledRunIndex builds index from pre-sorted runs
func NewScheduledRunIndex(degree int, sortedRuns []ScheduledRun) *ScheduledRunIndex

// Query finds all runs in time window [start, end)
func (idx *ScheduledRunIndex) Query(start, end time.Time) []ScheduledRun

// Peek returns the earliest scheduled run
func (idx *ScheduledRunIndex) Peek() (*ScheduledRun, bool)

// Len returns number of scheduled runs
func (idx *ScheduledRunIndex) Len() int

// Swap atomically replaces the index with a new one
func (idx *ScheduledRunIndex) Swap(newTree *btree.BTree)
```

### SortedSliceIndex (Comparison Implementation)
```go
type SortedSliceIndex struct {
    runs atomic.Pointer[[]ScheduledRun]
}

// NewSortedSliceIndex builds index from pre-sorted runs
func NewSortedSliceIndex(sortedRuns []ScheduledRun) *SortedSliceIndex

// Same query interface as BTree version
func (idx *SortedSliceIndex) Query(start, end time.Time) []ScheduledRun
func (idx *SortedSliceIndex) Peek() (*ScheduledRun, bool)
func (idx *SortedSliceIndex) Len() int
func (idx *SortedSliceIndex) Swap(newRuns []ScheduledRun)
```

## Implementation Details

### Thread Safety via Atomic Swap
```go
type ScheduledRunIndex struct {
    tree atomic.Pointer[btree.BTree]
}

// Readers just load the pointer - no locks!
func (idx *ScheduledRunIndex) Query(start, end time.Time) []ScheduledRun {
    tree := idx.tree.Load()
    // Query tree without any locks
    // Even if writer swaps tree, we still have reference to old tree
    return queryTree(tree, start, end)
}

// Writers create new tree and swap atomically
func RebuildIndex(idx *ScheduledRunIndex, newRuns []ScheduledRun) {
    newTree := bulkBuildBTree(newRuns, degree)
    idx.tree.Store(newTree)
    // Old tree will be GC'd when all readers finish
}
```

### BTree Bulk Build Algorithm
```go
func bulkBuildBTree(sortedRuns []ScheduledRun, degree int) *btree.BTree {
    if len(sortedRuns) == 0 {
        return btree.New(degree)
    }

    // Simple approach for MVP: just insert in order
    // BTree is fairly efficient with sorted inserts
    tree := btree.New(degree)
    for _, run := range sortedRuns {
        tree.ReplaceOrInsert(&run)
    }
    return tree

    // Future optimization: true bottom-up construction
    // Pack leaves, build internal nodes, very fast
}
```

### Sorted Slice Query Implementation
```go
func (idx *SortedSliceIndex) Query(start, end time.Time) []ScheduledRun {
    runs := idx.runs.Load()
    if runs == nil || len(*runs) == 0 {
        return nil
    }

    // Binary search for start
    startIdx := sort.Search(len(*runs), func(i int) bool {
        return !(*runs)[i].ScheduledAt.Before(start)
    })

    // Collect all runs until end
    results := []ScheduledRun{}
    for i := startIdx; i < len(*runs); i++ {
        if (*runs)[i].ScheduledAt.Before(end) {
            results = append(results, (*runs)[i])
        } else {
            break
        }
    }
    return results
}
```

### Handling Multiple Jobs at Same Time
Since data comes pre-sorted from scheduler, we need stable ordering:
- Sort by (time, jobID) when building runs list
- Ensures deterministic iteration order
- No special handling needed in index

### Generating Sorted Runs
```go
// In scheduler package
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

    // Sort by time, then JobID for stability
    sort.Slice(runs, func(i, j int) bool {
        if runs[i].ScheduledAt.Equal(runs[j].ScheduledAt) {
            return runs[i].JobID < runs[j].JobID
        }
        return runs[i].ScheduledAt.Before(runs[j].ScheduledAt)
    })

    return runs
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
2. **Build from sorted data**: Correct tree structure
3. **Query exact boundaries**: Start/end time edge cases
4. **Query with no results**: Empty time window
5. **Query with all results**: Large time window
6. **Peek**: First element, empty index
7. **Multiple jobs at same time**: Stable ordering
8. **Concurrent reads**: Race detector, multiple goroutines querying
9. **Concurrent rebuild**: Swap while queries in progress

### Benchmark Tests

#### Build Performance
- Build from 10, 100, 1000, 10000, 100000, 1000000 sorted runs
- Memory allocations
- Compare BTree vs Slice

#### Query Performance
- Various time windows (1 min, 1 hour, 1 day)
- Various result sizes (10, 100, 1000, 10000 results)
- Test with different index sizes (1000, 10000, 100000, 1000000 runs)
- Compare BTree vs Slice

#### Peek Performance
- Constant time access to first element
- Test with various index sizes (up to 1M runs)
- Compare BTree vs Slice

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

    // Build new index (fast!)
    newTree := bulkBuildBTree(runs, 32)

    // Swap atomically (instant!)
    s.index.tree.Store(newTree)
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

### Target Performance (Both Implementations)
- Build 10,000 runs: < 5ms
- Build 100,000 runs: < 50ms
- Build 1,000,000 runs: < 500ms
- Query 100 runs: < 1ms (even with 1M runs in index)
- Peek: < 1µs
- Memory per run: ~40 bytes (jobID string + time.Time)
- Total memory for 1M runs: < 100MB

### Comparison Goals
- BTree may be faster for large queries (> 1000 results)
- BTree should scale better to 1M runs (O(log n) vs O(n) for binary search)
- Slice may be faster for small queries (< 100 results) and small indexes
- Both should have similar memory usage
- Benchmark will tell us which to use!

## Future Optimizations

### If Needed (Only if benchmarks show issues)
1. **True bulk build**: Bottom-up BTree construction
2. **Pre-allocated queries**: Reuse result slices
3. **Separate peek cache**: Cache first N items
4. **Parallel rebuild**: Generate runs in parallel per job
5. **Incremental rebuild**: Only recalculate changed jobs

### NOT Needed
- Incremental updates (rebuild is fast enough)
- Complex thread safety (atomic swap is sufficient)
- Persistence (rebuilt on startup)

## Package Location
`lib/scheduler/index/` - Part of scheduler package

## Dependencies
- `github.com/google/btree` - BTree implementation
- Standard library only otherwise

## Files Structure
```
lib/scheduler/index/
├── scheduled_run.go          # ScheduledRun type
├── btree_index.go            # BTree implementation
├── slice_index.go            # Sorted slice implementation
├── index_test.go             # Unit tests
├── index_bench_test.go       # Benchmarks
└── generator_test.go         # Test helper for generating runs
```

## Open Questions
1. **Rebuild frequency**: 30s? 60s? Configurable? (Start with 30s)
2. **Lookahead window**: 1 hour? 2 hours? (Start with 1 hour)
3. **BTree degree**: 32? 64? (Benchmark will answer)
4. **BTree vs Slice**: Which is better? (Benchmark will answer!)

## Success Criteria
- ✅ Build 10,000 runs in < 5ms
- ✅ Build 100,000 runs in < 50ms
- ✅ Build 1,000,000 runs in < 500ms
- ✅ Query < 1ms for typical workload (even with 1M runs)
- ✅ No race conditions (go test -race)
- ✅ Memory usage reasonable (~40 bytes per run, < 100MB for 1M runs)
- ✅ BTree vs Slice comparison complete at all scales (10 to 1M)
- ✅ Can swap index while queries in progress
