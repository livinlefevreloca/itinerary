# Scheduled Run Index Benchmarks

## Test Environment

- **Hardware**: MacBook Pro M2 Max
- **CPU**: Apple M2 Max (16 cores)
- **Memory**: 64 GB
- **OS**: macOS (darwin)
- **Architecture**: arm64
- **Go Version**: go1.23+

## Overview

The Scheduled Run Index uses a sorted slice with atomic pointer swapping for lock-free concurrent reads. This benchmark suite measures performance at scale from 10 to 1,000,000 scheduled runs.

## Build Performance

Building the index involves copying the input slice, sorting by (ScheduledAt, JobID), and storing via atomic pointer.

| Index Size | Time/op | Memory | Allocs |
|------------|---------|--------|--------|
| 10 | 240 ns | 568 B | 6 |
| 100 | 7.0 µs | 4.2 KB | 6 |
| 1,000 | 111 µs | 40 KB | 6 |
| 10,000 | 1.5 ms | 392 KB | 6 |
| 100,000 | 17 ms | 3.9 MB | 6 |
| 1,000,000 | 196 ms | 38 MB | 6 |

Build time is O(n log n), dominated by sorting. Memory usage is ~40 bytes per run. 6 allocations regardless of size.

## Query Performance - Small Window (1 Minute)

Binary search to find start position, then linear scan for matching runs.

| Index Size | Time/op | Memory | Allocs |
|------------|---------|--------|--------|
| 1,000 | 21 ns | 0 B | 0 |
| 10,000 | 30 ns | 0 B | 0 |
| 100,000 | 36 ns | 0 B | 0 |
| 1,000,000 | 42 ns | 0 B | 0 |

Query time increases logarithmically with index size (~20 comparisons for 1M). Zero allocations when no results found.

## Query Performance - Medium Window (1 Hour)

Query for runs in a 1-hour window.

| Index Size | Time/op | Memory | Allocs | Results |
|------------|---------|--------|--------|---------|
| 1,000 | 15 µs | 90 KB | 11 | ~1000 runs |
| 10,000 | 432 µs | 1.8 MB | 19 | ~3600 runs |
| 100,000 | 1.8 ms | 6.8 MB | 24 | ~3600 runs |
| 1,000,000 | 1.1 ms | 6.8 MB | 24 | ~3600 runs |

Time dominated by result collection (O(k) where k = result count). Memory scales with result count, not index size.

## Query Performance - Large Window (1 Day)

| Index Size | Time/op | Memory | Allocs | Results |
|------------|---------|--------|--------|---------|
| 1,000 | 16 µs | 90 KB | 11 | ~1000 runs |
| 10,000 | 446 µs | 1.8 MB | 19 | ~10000 runs |
| 100,000 | 578 µs | 2.4 MB | 20 | ~14400 runs |
| 1,000,000 | 360 µs | 2.4 MB | 20 | ~14400 runs |

Performance depends on result count, not index size.

## Query Performance - Full Scan

Returning all runs in the index.

| Index Size | Time/op | Memory | Allocs |
|------------|---------|--------|--------|
| 1,000 | 15 µs | 90 KB | 11 |
| 10,000 | 460 µs | 1.8 MB | 19 |
| 100,000 | 8.1 ms | 22 MB | 29 |
| 1,000,000 | 43 ms | 215 MB | 39 |

O(n) when returning all results. Memory proportional to result count.

## Concurrent Query Performance

Multiple goroutines querying simultaneously using atomic.Pointer.Load().

| Readers | Index Size | Time/op | Memory | Allocs |
|---------|------------|---------|--------|--------|
| 10 | 100,000 | 6.9 ns | 0 B | 0 |
| 100 | 100,000 | 7.3 ns | 0 B | 0 |
| 10 | 1,000,000 | 8.2 ns | 0 B | 0 |
| 100 | 1,000,000 | 8.7 ns | 0 B | 0 |

Lock-free reads via atomic.Pointer. Zero allocations.

## Realistic Workload

Simulates scheduler behavior: 10,000 jobs with varying schedules (every minute, 5min, 15min, hourly, daily).

- **Time/op**: 64 ns
- **Memory**: 22 B/op
- **Allocs**: 0

## Swap Performance

Atomically replacing the index with new data.

| Index Size | Time/op | Memory | Allocs |
|------------|---------|--------|--------|
| 1,000 | 114 µs | 40 KB | 5 |
| 10,000 | 1.5 ms | 392 KB | 5 |
| 100,000 | 17 ms | 3.9 MB | 5 |
| 1,000,000 | 193 ms | 38 MB | 5 |

Swap includes copy + sort + atomic store. 5 allocations (one less than Build which also creates the struct).

## Swap with Concurrent Reads

Swapping index while 10 readers continuously query.

- **Index Size**: 100,000 runs
- **Time/op**: 43 ms (includes multiple swaps)
- **Memory**: 3.9 MB
- **Allocs**: 5

Readers continue using old index during swap. Old index GC'd when all readers finish.

## Memory Benchmarks

### Build Memory

| Index Size | Bytes/op | Allocs/op |
|------------|----------|-----------|
| 1,000 | 41,112 | 6 |
| 10,000 | 401,561 | 6 |
| 100,000 | 4,006,040 | 6 |
| 1,000,000 | 40,001,700 | 6 |

Memory per run: ~40 bytes (jobID string + time.Time + overhead)

### Query Memory

| Index Size | Bytes/op | Allocs/op |
|------------|----------|-----------|
| 1,000 | 0 | 0 |
| 10,000 | 0 | 0 |
| 100,000 | 0 | 0 |
| 1,000,000 | 0 | 0 |

Zero allocations for queries with no results. Allocations only occur when creating result slice.

## Complexity Analysis

| Operation | Time Complexity | Space Complexity |
|-----------|----------------|------------------|
| Build | O(n log n) | O(n) |
| Query | O(log n + k) | O(k) |
| Len | O(1) | O(1) |
| Swap | O(n log n) | O(n) |

Where:
- n = number of runs in index
- k = number of results returned
