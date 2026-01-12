# Scheduled Run Index Test Suite

## Overview
Comprehensive test suite for the Scheduled Run Index, covering both BTree and sorted slice implementations. Focus on correctness, concurrency safety, and performance at scale (up to 1M runs).

## Test Categories

### 1. Unit Tests - Basic Functionality

#### Build Tests
```go
TestBuild_Empty
- Build index from empty slice
- Both implementations should handle gracefully
- Query should return empty, Peek should return false
- Len should return 0

TestBuild_SingleRun
- Build index with one scheduled run
- Query should find it
- Peek should return it
- Len should return 1

TestBuild_MultipleRuns_SameTime
- Build with multiple jobs at exact same time
- Should maintain stable ordering (by JobID)
- Query should return all in sorted order

TestBuild_MultipleRuns_DifferentTimes
- Build with jobs at different times (pre-sorted)
- Verify correct ordering in queries

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

#### Peek Tests
```go
TestPeek_EmptyIndex
- Peek on empty index
- Should return nil, false

TestPeek_SingleRun
- Peek with one run
- Should return that run

TestPeek_MultipleRuns
- Peek with many runs
- Should return earliest run
- If multiple at same time, should be deterministic (first by JobID)

TestPeek_AfterQuery
- Peek should still work after queries
- Should return earliest remaining run
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

### 4. Comparison Tests (BTree vs Slice)

```go
TestComparison_ResultEquivalence
- Build same data in both implementations
- Query same windows
- Results should be identical (order and content)

TestComparison_AllOperations
- Run all unit tests on both implementations
- Should behave identically

TestComparison_PeekEquivalence
- Peek should return same result from both
```

### 5. Atomic Swap Tests

```go
TestSwap_Basic
- Create index, swap with new data
- Queries on new index should see new data

TestSwap_OldIndexStillValid
- Get reference to old index
- Swap in new index
- Old reference should still be queryable (GC later)

TestSwap_MultipleTimes
- Swap 10 times rapidly
- Final index should have latest data

TestSwap_Nil
- Swap with nil should work (empty index)
```

## Benchmark Suite

### Build Benchmarks

```go
BenchmarkBuild_BTree_10
BenchmarkBuild_BTree_100
BenchmarkBuild_BTree_1000
BenchmarkBuild_BTree_10000
BenchmarkBuild_BTree_100000
BenchmarkBuild_BTree_1000000

BenchmarkBuild_Slice_10
BenchmarkBuild_Slice_100
BenchmarkBuild_Slice_1000
BenchmarkBuild_Slice_10000
BenchmarkBuild_Slice_100000
BenchmarkBuild_Slice_1000000

// Each benchmark:
// - Measures build time
// - Reports memory allocations
// - Uses pre-generated sorted runs
```

### Query Benchmarks - Small Window (1 minute)

```go
BenchmarkQuery_BTree_1000Runs_1MinWindow
BenchmarkQuery_BTree_10000Runs_1MinWindow
BenchmarkQuery_BTree_100000Runs_1MinWindow
BenchmarkQuery_BTree_1000000Runs_1MinWindow

BenchmarkQuery_Slice_1000Runs_1MinWindow
BenchmarkQuery_Slice_10000Runs_1MinWindow
BenchmarkQuery_Slice_100000Runs_1MinWindow
BenchmarkQuery_Slice_1000000Runs_1MinWindow

// Window returns ~10 results
// Measures query latency
```

### Query Benchmarks - Medium Window (1 hour)

```go
BenchmarkQuery_BTree_1000Runs_1HourWindow
BenchmarkQuery_BTree_10000Runs_1HourWindow
BenchmarkQuery_BTree_100000Runs_1HourWindow
BenchmarkQuery_BTree_1000000Runs_1HourWindow

BenchmarkQuery_Slice_1000Runs_1HourWindow
BenchmarkQuery_Slice_10000Runs_1HourWindow
BenchmarkQuery_Slice_100000Runs_1HourWindow
BenchmarkQuery_Slice_1000000Runs_1HourWindow

// Window returns ~100 results
// Tests typical scheduler query
```

### Query Benchmarks - Large Window (1 day)

```go
BenchmarkQuery_BTree_1000Runs_1DayWindow
BenchmarkQuery_BTree_10000Runs_1DayWindow
BenchmarkQuery_BTree_100000Runs_1DayWindow
BenchmarkQuery_BTree_1000000Runs_1DayWindow

BenchmarkQuery_Slice_1000Runs_1DayWindow
BenchmarkQuery_Slice_10000Runs_1DayWindow
BenchmarkQuery_Slice_100000Runs_1DayWindow
BenchmarkQuery_Slice_1000000Runs_1DayWindow

// Window returns ~1000 results
// Tests large query performance
```

### Query Benchmarks - Full Scan

```go
BenchmarkQuery_BTree_FullScan_1000
BenchmarkQuery_BTree_FullScan_10000
BenchmarkQuery_BTree_FullScan_100000
BenchmarkQuery_BTree_FullScan_1000000

BenchmarkQuery_Slice_FullScan_1000
BenchmarkQuery_Slice_FullScan_10000
BenchmarkQuery_Slice_FullScan_100000
BenchmarkQuery_Slice_FullScan_1000000

// Query returns all runs
// Worst case performance
```

### Peek Benchmarks

```go
BenchmarkPeek_BTree_1000
BenchmarkPeek_BTree_10000
BenchmarkPeek_BTree_100000
BenchmarkPeek_BTree_1000000

BenchmarkPeek_Slice_1000
BenchmarkPeek_Slice_10000
BenchmarkPeek_Slice_100000
BenchmarkPeek_Slice_1000000

// Should be O(1) for both
// Verify no degradation with size
```

### Concurrent Benchmarks

```go
BenchmarkConcurrentQuery_BTree_10Readers_100000Runs
BenchmarkConcurrentQuery_BTree_100Readers_100000Runs

BenchmarkConcurrentQuery_Slice_10Readers_100000Runs
BenchmarkConcurrentQuery_Slice_100Readers_100000Runs

// Multiple goroutines querying simultaneously
// Measures throughput and contention
```

### Realistic Workload Benchmarks

```go
BenchmarkRealisticWorkload_BTree
BenchmarkRealisticWorkload_Slice

// Simulates scheduler loop:
// - 100,000 runs in index
// - Rebuild every 30 seconds (in goroutine)
// - Query every 1 second (in goroutine)
// - Run for benchmark duration
// - Measure query latency distribution (p50, p95, p99)
```

### Memory Benchmarks

```go
BenchmarkMemory_BTree_1000
BenchmarkMemory_BTree_10000
BenchmarkMemory_BTree_100000
BenchmarkMemory_BTree_1000000

BenchmarkMemory_Slice_1000
BenchmarkMemory_Slice_10000
BenchmarkMemory_Slice_100000
BenchmarkMemory_Slice_1000000

// Uses benchmem flag
// Reports bytes per operation
// Reports allocations per operation
```

## Test Helpers

### Data Generation
```go
// Generate sorted runs for testing
func generateSortedRuns(count int, startTime time.Time, interval time.Duration) []ScheduledRun

// Generate runs with specific pattern (e.g., 10 jobs each running every minute)
func generateRealisticRuns(jobCount int, startTime, endTime time.Time, schedules []string) []ScheduledRun

// Generate runs with multiple jobs at same time
func generateCoincidentRuns(count int, sameTime time.Time) []ScheduledRun
```

### Assertion Helpers
```go
// Assert query results match expected
func assertQueryResults(t *testing.T, expected, actual []ScheduledRun)

// Assert runs are sorted correctly
func assertSorted(t *testing.T, runs []ScheduledRun)

// Assert no duplicates
func assertNoDuplicates(t *testing.T, runs []ScheduledRun)
```

## Test Data Patterns

### Pattern 1: Uniform Distribution
- Runs evenly spaced over time
- Every minute for 1 hour = 60 runs
- Every minute for 1 day = 1440 runs

### Pattern 2: Clustered
- Many runs at specific times (e.g., top of every hour)
- Simulates batch job patterns

### Pattern 3: Sparse
- Few runs spread over large time range
- Tests binary search efficiency

### Pattern 4: Realistic
- Mix of different job frequencies
- Some every minute, some hourly, some daily
- Simulates actual scheduler workload

## Success Criteria

### Unit Tests
- ✅ All unit tests pass
- ✅ No races with `go test -race`
- ✅ 100% coverage of public API
- ✅ Both implementations behave identically

### Benchmarks - Build Performance
- ✅ BTree builds 1M runs in < 500ms
- ✅ Slice builds 1M runs in < 500ms
- ✅ Memory usage < 50 bytes per run overhead

### Benchmarks - Query Performance
- ✅ Query with 1M runs, 1-hour window: < 1ms
- ✅ Peek with 1M runs: < 1µs
- ✅ Performance doesn't degrade with index size

### Benchmarks - Comparison
- ✅ Clear winner identified for scheduler use case
- ✅ Or: both acceptable, document trade-offs

### Benchmarks - Concurrency
- ✅ No lock contention on query path
- ✅ Swap doesn't block queries
- ✅ Can handle 100+ concurrent readers

## Running the Tests

```bash
# All unit tests
./test.sh --test

# All benchmarks
./test.sh --bench --all

# Specific benchmark
./test.sh --bench --pattern "BenchmarkBuild_BTree"

# Build benchmarks with memory stats
./test.sh --bench --pattern "BenchmarkBuild" --mem

# Query benchmarks
./test.sh --bench --pattern "BenchmarkQuery" --mem

# Race detection
./test.sh --test --race

# Specific test pattern
./test.sh --test Concurrency
```

## Performance Expectations

Based on our design and the cron parser benchmarks:

### Build Performance
| Runs | BTree (est) | Slice (est) |
|------|-------------|-------------|
| 10 | < 1µs | < 1µs |
| 100 | < 10µs | < 10µs |
| 1,000 | < 100µs | < 50µs |
| 10,000 | < 1ms | < 500µs |
| 100,000 | < 10ms | < 5ms |
| 1,000,000 | < 100ms | < 50ms |

### Query Performance (1-hour window, ~100 results)
| Index Size | BTree (est) | Slice (est) |
|------------|-------------|-------------|
| 1,000 | < 10µs | < 5µs |
| 10,000 | < 20µs | < 10µs |
| 100,000 | < 50µs | < 50µs |
| 1,000,000 | < 100µs | < 500µs |

**Hypothesis**: Slice may be faster for small indexes and small queries. BTree should win for large indexes (1M+ runs).

## Notes
- All times in UTC (from cron parser)
- Test data should be deterministic (fixed seed)
- Benchmarks run with `-benchmem` to track allocations
- Focus on realistic patterns (not just sequential data)
- Test with actual job IDs (realistic string lengths: 10-50 chars)
