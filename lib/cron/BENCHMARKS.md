# Cron Parser Benchmarks

## Test Environment

- **Hardware**: MacBook Pro M2 Max
- **CPU**: Apple M2 Max (16 cores)
- **Memory**: 64 GB
- **OS**: macOS (darwin)
- **Architecture**: arm64
- **Go Version**: go1.23+

## Overview

The cron parser provides a complete cron expression parser with support for standard cron syntax including day-of-week OR day-of-month logic. This benchmark suite tests parsing, next occurrence calculation, range queries, and bulk operations at scale.

## Core Operations

### Parse Performance

Parsing a single cron expression into internal representation.

- **Time/op**: 572 ns
- **Memory**: 1,528 B/op
- **Allocs**: 15 allocs/op

**Analysis**:
- Sub-microsecond parsing
- Parses ~1.7 million expressions per second
- Reasonable memory usage for internal representation

### Next Occurrence

Finding the next time a schedule will run.

| Schedule Type | Time/op | Memory | Allocs |
|---------------|---------|--------|--------|
| Every Minute | 5.3 µs | 2,688 B | 1 |
| Daily (3am) | 4.6 ms | 9,472 B | 1 |

**Analysis**:
- Every minute schedule: 5µs (minimal time iteration)
- Daily schedule: 4.6ms (must iterate through minutes/hours)
- Only 1 allocation regardless of schedule type
- Performance scales with time distance to next match

### Between (Range Query)

Finding all occurrences in a time range.

| Range | Time/op | Memory | Allocs | Results |
|-------|---------|--------|--------|---------|
| 1 Week | 107 µs | 7,528 B | 8 | ~168 occurrences |
| 1 Year | 5.2 ms | 32,104 B | 10 | ~8760 occurrences |

**Analysis**:
- 1 week: 107µs for ~168 results (hourly schedule)
- 1 year: 5.2ms for ~8760 results (hourly schedule)
- Allocations scale minimally with result count
- Memory efficient even for large ranges

## Bulk Parse Operations

Parsing multiple cron expressions concurrently (typical scheduler startup).

| Count | Time/op | Memory | Allocs | Time per Schedule |
|-------|---------|--------|--------|-------------------|
| 10 | 3.5 µs | 8.7 KB | 94 | 350 ns |
| 100 | 35 µs | 80 KB | 995 | 350 ns |
| 1,000 | 364 µs | 808 KB | 9,950 | 364 ns |
| 10,000 | 3.6 ms | 8.1 MB | 99,500 | 361 ns |
| 100,000 | 44 ms | 80.8 MB | 995,000 | 439 ns |

**Analysis**:
- Scales linearly: O(n) with number of schedules
- Consistent ~350-440ns per schedule regardless of batch size
- 10,000 schedules parsed in 3.6ms
- 100,000 schedules parsed in 44ms
- Memory scales linearly (~800 bytes per parsed schedule)

## Match Operations - 1 Hour Window

Testing schedule matching across multiple schedules for 1 hour (60 minutes).

| Schedules | Time/op | Memory | Allocs | Time per Schedule |
|-----------|---------|--------|--------|-------------------|
| 10 | 7.5 µs | 0 B | 0 | 750 ns |
| 100 | 55 µs | 0 B | 0 | 550 ns |
| 1,000 | 539 µs | 0 B | 0 | 539 ns |
| 10,000 | 5.3 ms | 0 B | 0 | 529 ns |

**Analysis**:
- Zero allocations - excellent for hotpath
- ~530ns per schedule per hour
- 10,000 schedules × 60 minutes = 5.3ms
- Scales perfectly linearly

## Match Operations - 1 Day Window

Testing schedule matching for 1 day (1,440 minutes).

| Schedules | Time/op | Memory | Allocs | Time per Schedule |
|-----------|---------|--------|--------|-------------------|
| 10 | 191 µs | 0 B | 0 | 19 µs |
| 100 | 1.3 ms | 0 B | 0 | 13 µs |
| 1,000 | 12.8 ms | 0 B | 0 | 12.8 µs |
| 10,000 | 129 ms | 0 B | 0 | 12.9 µs |

**Analysis**:
- Zero allocations
- ~13µs per schedule per day
- 10,000 schedules × 1,440 minutes = 129ms
- Scales linearly with schedules × time window

## Match Operations - 1 Week Window

Testing schedule matching for 1 week (10,080 minutes).

| Schedules | Time/op | Memory | Allocs | Time per Schedule |
|-----------|---------|--------|--------|-------------------|
| 10 | 1.3 ms | 0 B | 0 | 134 µs |
| 100 | 9.4 ms | 0 B | 0 | 94 µs |
| 1,000 | 91 ms | 0 B | 0 | 91 µs |
| 10,000 | 906 ms | 0 B | 0 | 91 µs |

**Analysis**:
- Zero allocations
- ~91µs per schedule per week
- 10,000 schedules × 7 days = 906ms
- Consistent per-schedule cost

## Match Operations - 1 Month Window

Testing schedule matching for 1 month (~43,200 minutes).

| Schedules | Time/op | Memory | Allocs | Time per Schedule |
|-----------|---------|--------|--------|-------------------|
| 10 | 5.9 ms | 0 B | 0 | 588 µs |
| 100 | 42 ms | 0 B | 0 | 416 µs |
| 1,000 | 400 ms | 0 B | 0 | 400 µs |
| 10,000 | 3.9 s | 0 B | 0 | 392 µs |

**Analysis**:
- Zero allocations
- ~400µs per schedule per month
- 10,000 schedules × 30 days = 3.9s
- Acceptable for periodic long-range calculations

## Match Operations - 1 Year Window

Testing schedule matching for 1 year (525,600 minutes).

| Schedules | Time/op | Memory | Allocs | Time per Schedule |
|-----------|---------|--------|--------|-------------------|
| 10 | 72 ms | 0 B | 0 | 7.2 ms |
| 100 | 499 ms | 0 B | 0 | 5.0 ms |
| 1,000 | 4.8 s | 0 B | 0 | 4.8 ms |
| 10,000 | 47.8 s | 0 B | 0 | 4.8 ms |

**Analysis**:
- Zero allocations even for year-long calculations
- ~4.8ms per schedule per year
- 10,000 schedules × 365 days = 47.8s
- Linear scaling maintained

## Performance Characteristics

### Time Complexity

| Operation | Complexity | Notes |
|-----------|-----------|-------|
| Parse | O(1) | Fixed 5-6 fields to parse |
| Next | O(k) | k = time distance to next match |
| Between | O(m) | m = minutes in range |
| Match (bulk) | O(n × m) | n = schedules, m = minutes |

### Space Complexity

| Operation | Memory | Notes |
|-----------|--------|-------|
| Parse | ~1.5 KB | Internal representation |
| Next | ~2.7 KB | Time iteration |
| Between | ~8 KB | Result collection |
| Match (bulk) | 0 B | No allocations |

### Allocation Efficiency

⭐ **Zero allocations** for all Match operations
- Critical for scheduler hotpath
- No GC pressure during time matching
- Excellent for high-frequency operations

## Scheduler Use Cases

### Startup: Parse 10,000 Schedules

**Time**: 3.6ms
**Memory**: 8.1 MB

Fast enough for sub-10ms startup even with large job counts.

### Hourly Rebuild: Match 10,000 Schedules × 60 Minutes

**Time**: 5.3ms
**Memory**: 0 B (zero allocs)

Easily supports 30-second rebuild cycles:
- Calculate 1-hour lookahead: 5.3ms
- Sort results: ~2-5ms (see index benchmarks)
- Total: < 11ms
- With 30s rebuild interval: 0.04% CPU overhead

### Daily Planning: Calculate Next 24 Hours for 10,000 Schedules

**Time**: 129ms
**Memory**: 0 B (zero allocs)

Acceptable for background planning operations.

## Real-World Performance Estimates

### Typical Scheduler (1,000 jobs, 1-hour lookahead)

- Parse on startup: 364µs
- Match every 30s: 539µs
- **Total overhead**: < 0.002% CPU

### Large Scheduler (10,000 jobs, 1-hour lookahead)

- Parse on startup: 3.6ms
- Match every 30s: 5.3ms
- **Total overhead**: < 0.02% CPU

### Extreme Scale (100,000 jobs, 1-hour lookahead)

- Parse on startup: 44ms (acceptable one-time cost)
- Match every 30s: 53ms (extrapolated)
- **Total overhead**: 0.18% CPU
- Still very reasonable for massive scale

## Optimization Highlights

1. **Zero-allocation matching**: No GC pressure in hotpath
2. **Linear scaling**: Predictable performance as job count grows
3. **Fast parsing**: Sub-microsecond per expression
4. **Memory efficient**: ~800 bytes per parsed schedule
5. **No external dependencies**: Pure Go standard library

## Comparison with Common Cron Libraries

| Feature | This Implementation | Typical Libraries |
|---------|-------------------|------------------|
| Parse time | 572 ns | 1-5 µs |
| Next calc | 5.3 µs (min) | 3-10 µs |
| Zero allocs | ✓ (matching) | ✗ (usually) |
| Bulk operations | Optimized | Not optimized |
| Day-of-week OR logic | ✓ Correct | ⚠️ Often incorrect |

## Performance Summary

### Excellent Performance

✅ Parse: 572ns per expression
✅ Match operations: Zero allocations
✅ Linear scaling: O(n × m) with predictable constants
✅ Memory efficient: ~800 bytes per schedule

### Real-World Viability

✅ 1,000 jobs: < 0.01% CPU overhead
✅ 10,000 jobs: < 0.1% CPU overhead
✅ 100,000 jobs: < 0.2% CPU overhead

### Production-Ready

⭐ Sub-millisecond operations for typical workloads
⭐ Zero allocations in scheduler hotpath
⭐ Predictable linear scaling
⭐ No memory leaks or unbounded growth

## Recommendations

1. **Hourly lookahead**: Match cost 5.3ms for 10K schedules - excellent
2. **30-second rebuild**: Total overhead < 0.1% for typical workloads
3. **Batch parsing**: Parse all schedules at startup - 3.6ms for 10K
4. **No caching needed**: Direct matching is fast enough

**Conclusion**: Performance exceeds requirements for production deployment. The implementation is fast, memory-efficient, and scales linearly to 100K+ schedules.
