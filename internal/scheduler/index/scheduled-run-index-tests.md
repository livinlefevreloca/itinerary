# Scheduled Run Index Test Suite

## Overview
Comprehensive test suite for the Scheduled Run Index (sorted slice implementation). Focus on correctness, concurrency safety, and performance at scale (up to 1M runs).

## Test Categories

### 1. Unit Tests - Basic Functionality

#### Build Tests
```go
TestBuild_Empty
- Build index from empty slice
- Query should return empty
- Len should return 0

TestBuild_SingleRun
- Build index with one scheduled run
- Query should find it
- Len should return 1

TestBuild_MultipleRuns_SameTime
- Build with multiple jobs at exact same time
- Should maintain stable ordering (by JobID)
- Query should return all in sorted order

TestBuild_MultipleRuns_DifferentTimes
- Build with jobs at different times (unsorted input)
- Index should sort them automatically
- Verify correct ordering in queries

TestBuild_UnsortedData
- Build with deliberately unsorted runs
- Verify index sorts them correctly by (ScheduledAt, JobID)

TestBuild_LargeDataset
- Build with 10,000 runs
- Verify index is queryable
- Spot check ordering
```

#### Query Tests
```go
TestQuery_ExactBoundaries
- Query with start exactly matching first run time
- Query with end exactly matching last run time
- Verify start is inclusive, end is exclusive

TestQuery_BeforeAllRuns
- Query window entirely before first scheduled run
- Should return empty slice

TestQuery_AfterAllRuns
- Query window entirely after last scheduled run
- Should return empty slice

TestQuery_EmptyWindow
- Query with start == end
- Should return empty slice

TestQuery_SingleResult
- Query that matches exactly one run
- Verify correct run returned

TestQuery_MultipleResults
- Query matching 10, 100, 1000 runs
- Verify all correct runs returned
- Verify ordering (time, then JobID)

TestQuery_PartialOverlap
- Query window partially overlaps runs
- Only runs in [start, end) should be returned

TestQuery_AllRuns
- Query with window covering entire dataset
- Should return all runs in order

TestQuery_NanosecondPrecision
- Query with nanosecond-level time differences
- Verify precise boundary handling
```

#### Len Tests
```go
TestLen_EmptyIndex
- Len should return 0

TestLen_AfterBuild
- Len should return number of runs built

TestLen_Consistency
- Len should match number of items from Query(all time)
```

### 2. Unit Tests - Edge Cases

```go
TestEdgeCase_FarFutureTime
- Runs scheduled far in future (year 2100)
- Should handle correctly

TestEdgeCase_PastTime
- Runs scheduled in the past
- Should handle correctly (though scheduler filters these)

TestEdgeCase_TimeZones
- All times should be in UTC (cron parser handles this)
- Verify no timezone issues

TestEdgeCase_ZeroTime
- Query with time.Time{} zero value
- Should handle gracefully

TestEdgeCase_MaxTime
- Query up to very far future
- Should handle without overflow

TestEdgeCase_DuplicateJobIDs_DifferentTimes
- Same job scheduled at multiple times
- All should be tracked separately

TestEdgeCase_VeryLongJobID
- Job ID with 1000 characters
- Should work (just uses more memory)

TestEdgeCase_SpecialCharactersInJobID
- Unicode, emoji, etc. in job IDs
- Should work (string comparison)
```

### 3. Concurrency Tests

```go
TestConcurrency_MultipleReaders
- 10 goroutines querying simultaneously
- Run with -race flag
- All should get consistent results

TestConcurrency_ReadDuringSwap
- Goroutines querying while Swap happens
- Some see old index, some see new - both valid
- No races, no panics

TestConcurrency_MultipleSwaps
- Multiple Swaps in rapid succession
- Last swap should win
- Readers should see either old or new, never corrupted

TestConcurrency_HeavyQueryLoad
- 100 goroutines querying different windows
- Measure throughput
- Verify no contention issues

TestConcurrency_RealisticWorkload
- Simulate scheduler loop:
  - 1 goroutine swapping every 30s
  - 10 goroutines querying every 1s
- Run for 10 seconds
- Verify correctness and no races
```

### 4. Atomic Swap Tests

```go
TestSwap_Basic
- Create index, swap with new data
- Queries on new index should see new data

TestSwap_OldIndexStillValid
- Get reference to old index via Query during swap
- Old reference should still be queryable (GC later)

TestSwap_MultipleTimes
- Swap 10 times rapidly
- Final index should have latest data

TestSwap_Empty
- Swap with empty slice should work
- Queries return empty
```

## Benchmark Suite

### Build Benchmarks

```go
BenchmarkBuild_10
BenchmarkBuild_100
BenchmarkBuild_1000
BenchmarkBuild_10000
BenchmarkBuild_100000
BenchmarkBuild_1000000

// Each benchmark:
// - Measures build time (including sorting + atomic swap)
// - Reports memory allocations
// - Uses pre-generated unsorted runs (realistic input)
// - Should be O(n log n) due to sorting
// - Sorting dominates cost for large datasets
```

### Query Benchmarks - Small Window (1 minute, ~10 results)

```go
BenchmarkQuery_1000Runs_1MinWindow
BenchmarkQuery_10000Runs_1MinWindow
BenchmarkQuery_100000Runs_1MinWindow
BenchmarkQuery_1000000Runs_1MinWindow

// Binary search + scan for ~10 results
// Measures query latency
// Should scale with O(log n + k)
```

### Query Benchmarks - Medium Window (1 hour, ~100 results)

```go
BenchmarkQuery_1000Runs_1HourWindow
BenchmarkQuery_10000Runs_1HourWindow
BenchmarkQuery_100000Runs_1HourWindow
BenchmarkQuery_1000000Runs_1HourWindow

// Tests typical scheduler query
// Binary search + scan for ~100 results
```

### Query Benchmarks - Large Window (1 day, ~1000 results)

```go
BenchmarkQuery_1000Runs_1DayWindow
BenchmarkQuery_10000Runs_1DayWindow
BenchmarkQuery_100000Runs_1DayWindow
BenchmarkQuery_1000000Runs_1DayWindow

// Tests large query performance
// Binary search + scan for ~1000 results
```

### Query Benchmarks - Full Scan

```go
BenchmarkQuery_FullScan_1000
BenchmarkQuery_FullScan_10000
BenchmarkQuery_FullScan_100000
BenchmarkQuery_FullScan_1000000

// Query returns all runs
// Worst case: O(n) scan
// Should still be fast (sequential access)
```

### Concurrent Benchmarks

```go
BenchmarkConcurrentQuery_10Readers_100000Runs
BenchmarkConcurrentQuery_100Readers_100000Runs
BenchmarkConcurrentQuery_10Readers_1000000Runs
BenchmarkConcurrentQuery_100Readers_1000000Runs

// Multiple goroutines querying simultaneously
// Measures throughput
// No lock contention expected (atomic.Pointer.Load)
```

### Realistic Workload Benchmarks

```go
BenchmarkRealisticWorkload_100000Runs
BenchmarkRealisticWorkload_1000000Runs

// Simulates scheduler loop:
// - N runs in index
// - Rebuild every 30 seconds (in goroutine)
// - Query every 1 second (in goroutine)
// - Run for benchmark duration
// - Measure query latency distribution (p50, p95, p99)
```

### Memory Benchmarks

```go
BenchmarkMemory_Build_1000
BenchmarkMemory_Build_10000
BenchmarkMemory_Build_100000
BenchmarkMemory_Build_1000000

BenchmarkMemory_Query_1000
BenchmarkMemory_Query_10000
BenchmarkMemory_Query_100000
BenchmarkMemory_Query_1000000

// Uses benchmem flag
// Reports bytes per operation
// Reports allocations per operation
// Verify ~40 bytes per run overhead
```

## Test Helpers

### Data Generation
```go
// Generate unsorted runs for testing (realistic: grouped by job)
func generateUnsortedRuns(count int, startTime time.Time, interval time.Duration) []ScheduledRun

// Generate runs with specific pattern (e.g., 10 jobs each running every minute)
// Returns unsorted data (grouped by job, like real scheduler)
func generateRealisticRuns(jobCount int, startTime, endTime time.Time, schedules []string) []ScheduledRun

// Generate runs with multiple jobs at same time
func generateCoincidentRuns(count int, sameTime time.Time) []ScheduledRun

// Generate job IDs with realistic lengths
func generateJobID(index int) string // Returns "job-0001" format
```

### Assertion Helpers
```go
// Assert query results match expected
func assertQueryResults(t *testing.T, expected, actual []ScheduledRun)

// Assert runs are sorted correctly
func assertSorted(t *testing.T, runs []ScheduledRun)

// Assert no duplicates
func assertNoDuplicates(t *testing.T, runs []ScheduledRun)

// Assert times are within range
func assertTimesInRange(t *testing.T, runs []ScheduledRun, start, end time.Time)
```

## Test Data Patterns

### Pattern 1: Uniform Distribution
- Runs evenly spaced over time
- Every minute for 1 hour = 60 runs
- Every minute for 1 day = 1440 runs
- Used for most benchmarks

### Pattern 2: Clustered
- Many runs at specific times (e.g., top of every hour)
- Simulates batch job patterns
- Tests stable ordering when times are equal

### Pattern 3: Sparse
- Few runs spread over large time range
- Tests binary search efficiency
- Edge case for query performance

### Pattern 4: Realistic
- Mix of different job frequencies
- Some every minute, some hourly, some daily
- Simulates actual scheduler workload
- Used for realistic workload benchmarks

## Success Criteria

### Unit Tests
- ✅ All unit tests pass
- ✅ No races with `go test -race`
- ✅ 100% coverage of public API
- ✅ Edge cases handled correctly

### Benchmarks - Build Performance
- ✅ Build 1,000 runs in < 1ms (including sort)
- ✅ Build 10,000 runs in < 5ms (including sort)
- ✅ Build 100,000 runs in < 30ms (including sort)
- ✅ Build 1,000,000 runs in < 150ms (including sort)
- ✅ Memory usage ~40 bytes per run overhead

### Benchmarks - Query Performance
- ✅ Query with 1M runs, 1-hour window: < 1ms
- ✅ Query scales with O(log n + k)
- ✅ Len with 1M runs: < 1µs
- ✅ Performance doesn't degrade unexpectedly

### Benchmarks - Concurrency
- ✅ No lock contention on query path
- ✅ Swap doesn't block queries
- ✅ Can handle 100+ concurrent readers
- ✅ Realistic workload shows acceptable latencies

## Running the Tests

```bash
# All unit tests
cd internal/scheduler/index
go test ./...

# With race detector
go test -race ./...

# All benchmarks
go test -bench=. -benchmem ./...

# Build benchmarks only
go test -bench=BenchmarkBuild -benchmem ./...

# Query benchmarks only
go test -bench=BenchmarkQuery -benchmem ./...

# Concurrent benchmarks
go test -bench=BenchmarkConcurrent -benchmem ./...

# Specific size
go test -bench=1000000 -benchmem ./...

# Realistic workload
go test -bench=Realistic -benchmem -benchtime=30s ./...
```

## Performance Expectations

Based on our design:

### Build Performance (Slice Copy + Atomic Swap)
| Runs | Expected Time | Expected Memory |
|------|---------------|-----------------|
| 10 | < 1µs | < 1 KB |
| 100 | < 10µs | < 10 KB |
| 1,000 | < 100µs | < 100 KB |
| 10,000 | < 1ms | < 1 MB |
| 100,000 | < 10ms | < 10 MB |
| 1,000,000 | < 100ms | < 100 MB |

### Query Performance (Binary Search + Linear Scan)
Window returns ~100 results:

| Index Size | Expected Time |
|------------|---------------|
| 1,000 | < 10µs |
| 10,000 | < 20µs |
| 100,000 | < 50µs |
| 1,000,000 | < 100µs |

**Reasoning**: Binary search is ~10-20 comparisons, scan of 100 results is very fast (sequential)

### Peek Performance (Array Access)
| Index Size | Expected Time |
|------------|---------------|
| Any | < 1µs |

**Reasoning**: Just accessing index 0, O(1)

## Notes
- All times in UTC (from cron parser)
- Test data should be deterministic (fixed seed)
- Benchmarks run with `-benchmem` to track allocations
- Focus on realistic patterns (not just sequential data)
- Test with actual job IDs (realistic string lengths: 10-50 chars)
- Verify atomic.Pointer provides lock-free reads (no contention)
