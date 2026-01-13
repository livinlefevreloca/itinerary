package index

import (
	"sync"
	"testing"
	"time"
)

// Build Benchmarks

func BenchmarkBuild_10(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(10, now, time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewScheduledRunIndex(runs)
	}
}

func BenchmarkBuild_100(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100, now, time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewScheduledRunIndex(runs)
	}
}

func BenchmarkBuild_1000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000, now, time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewScheduledRunIndex(runs)
	}
}

func BenchmarkBuild_10000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(10000, now, time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewScheduledRunIndex(runs)
	}
}

func BenchmarkBuild_100000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100000, now, time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewScheduledRunIndex(runs)
	}
}

func BenchmarkBuild_1000000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000000, now, time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewScheduledRunIndex(runs)
	}
}

// Query Benchmarks - Small Window (1 minute)

func BenchmarkQuery_1000Runs_1MinWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(500 * time.Second)
	end := start.Add(time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_10000Runs_1MinWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(10000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(5000 * time.Second)
	end := start.Add(time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_100000Runs_1MinWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(50000 * time.Second)
	end := start.Add(time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_1000000Runs_1MinWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(500000 * time.Second)
	end := start.Add(time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

// Query Benchmarks - Medium Window (1 hour)

func BenchmarkQuery_1000Runs_1HourWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now
	end := start.Add(time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_10000Runs_1HourWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(10000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now
	end := start.Add(time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_100000Runs_1HourWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now
	end := start.Add(time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_1000000Runs_1HourWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now
	end := start.Add(time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

// Query Benchmarks - Large Window (1 day)

func BenchmarkQuery_1000Runs_1DayWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000, now, time.Minute)
	idx := NewScheduledRunIndex(runs)

	start := now
	end := start.Add(24 * time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_10000Runs_1DayWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(10000, now, time.Minute)
	idx := NewScheduledRunIndex(runs)

	start := now
	end := start.Add(24 * time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_100000Runs_1DayWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100000, now, time.Minute)
	idx := NewScheduledRunIndex(runs)

	start := now
	end := start.Add(24 * time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_1000000Runs_1DayWindow(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000000, now, time.Minute)
	idx := NewScheduledRunIndex(runs)

	start := now
	end := start.Add(24 * time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

// Query Benchmarks - Full Scan

func BenchmarkQuery_FullScan_1000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour * 2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_FullScan_10000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(10000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour * 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_FullScan_100000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour * 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkQuery_FullScan_1000000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour * 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

// Peek Benchmarks

func BenchmarkPeek_1000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.Peek()
	}
}

func BenchmarkPeek_10000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(10000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.Peek()
	}
}

func BenchmarkPeek_100000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.Peek()
	}
}

func BenchmarkPeek_1000000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.Peek()
	}
}

// Concurrent Benchmarks

func BenchmarkConcurrentQuery_10Readers_100000Runs(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(50000 * time.Second)
	end := start.Add(time.Minute)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = idx.Query(start, end)
		}
	})
}

func BenchmarkConcurrentQuery_100Readers_100000Runs(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(50000 * time.Second)
	end := start.Add(time.Minute)

	b.SetParallelism(100)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = idx.Query(start, end)
		}
	})
}

func BenchmarkConcurrentQuery_10Readers_1000000Runs(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(500000 * time.Second)
	end := start.Add(time.Minute)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = idx.Query(start, end)
		}
	})
}

func BenchmarkConcurrentQuery_100Readers_1000000Runs(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(500000 * time.Second)
	end := start.Add(time.Minute)

	b.SetParallelism(100)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = idx.Query(start, end)
		}
	})
}

// Realistic Workload Benchmark

func BenchmarkRealisticWorkload(b *testing.B) {
	// Simulate: 10,000 jobs, various schedules
	now := time.Now()
	intervals := []time.Duration{
		time.Minute,      // Every minute
		5 * time.Minute,  // Every 5 minutes
		15 * time.Minute, // Every 15 minutes
		time.Hour,        // Hourly
		24 * time.Hour,   // Daily
	}

	runs := generateRealisticRuns(10000, now, now.Add(2*time.Hour), intervals)
	idx := NewScheduledRunIndex(runs)

	queryStart := now
	queryEnd := now.Add(30 * time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Typical scheduler query: next 30 seconds
		_ = idx.Query(queryStart, queryEnd)

		// Advance time
		queryStart = queryStart.Add(time.Second)
		queryEnd = queryEnd.Add(time.Second)
	}
}

// Swap Benchmarks

func BenchmarkSwap_1000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	newRuns := generateUnsortedRuns(1000, now.Add(time.Hour), time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Swap(newRuns)
	}
}

func BenchmarkSwap_10000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(10000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	newRuns := generateUnsortedRuns(10000, now.Add(time.Hour), time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Swap(newRuns)
	}
}

func BenchmarkSwap_100000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	newRuns := generateUnsortedRuns(100000, now.Add(time.Hour), time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Swap(newRuns)
	}
}

func BenchmarkSwap_1000000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	newRuns := generateUnsortedRuns(1000000, now.Add(time.Hour), time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Swap(newRuns)
	}
}

// Swap with Concurrent Reads

func BenchmarkSwapWithReads_100000Runs(b *testing.B) {
	now := time.Now()
	runs1 := generateUnsortedRuns(100000, now, time.Second)
	runs2 := generateUnsortedRuns(100000, now.Add(time.Hour), time.Second)
	idx := NewScheduledRunIndex(runs1)

	queryStart := now.Add(50000 * time.Second)
	queryEnd := queryStart.Add(time.Minute)

	// Start readers
	var wg sync.WaitGroup
	done := make(chan bool)

	// 10 concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					_ = idx.Query(queryStart, queryEnd)
				}
			}
		}()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			idx.Swap(runs2)
		} else {
			idx.Swap(runs1)
		}
	}
	b.StopTimer()

	close(done)
	wg.Wait()
}

// Memory Benchmarks

func BenchmarkMemory_Build_1000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000, now, time.Second)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewScheduledRunIndex(runs)
	}
}

func BenchmarkMemory_Build_10000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(10000, now, time.Second)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewScheduledRunIndex(runs)
	}
}

func BenchmarkMemory_Build_100000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100000, now, time.Second)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewScheduledRunIndex(runs)
	}
}

func BenchmarkMemory_Build_1000000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000000, now, time.Second)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewScheduledRunIndex(runs)
	}
}

func BenchmarkMemory_Query_1000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(500 * time.Second)
	end := start.Add(time.Minute)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkMemory_Query_10000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(10000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(5000 * time.Second)
	end := start.Add(time.Minute)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkMemory_Query_100000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(100000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(50000 * time.Second)
	end := start.Add(time.Minute)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}

func BenchmarkMemory_Query_1000000(b *testing.B) {
	now := time.Now()
	runs := generateUnsortedRuns(1000000, now, time.Second)
	idx := NewScheduledRunIndex(runs)

	start := now.Add(500000 * time.Second)
	end := start.Add(time.Minute)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(start, end)
	}
}
