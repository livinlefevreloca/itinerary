package index

import (
	"sync"
	"testing"
	"time"
)

// assertSorted verifies runs are sorted by (ScheduledAt, JobID).
func assertSorted(t *testing.T, runs []ScheduledRun) {
	t.Helper()
	for i := 1; i < len(runs); i++ {
		prev := runs[i-1]
		curr := runs[i]

		if curr.ScheduledAt.Before(prev.ScheduledAt) {
			t.Errorf("runs not sorted by time: runs[%d].ScheduledAt=%v > runs[%d].ScheduledAt=%v",
				i-1, prev.ScheduledAt, i, curr.ScheduledAt)
		}

		if curr.ScheduledAt.Equal(prev.ScheduledAt) && curr.JobID < prev.JobID {
			t.Errorf("runs with same time not sorted by JobID: runs[%d].JobID=%s > runs[%d].JobID=%s",
				i-1, prev.JobID, i, curr.JobID)
		}
	}
}

// assertQueryResults verifies query results match expected.
func assertQueryResults(t *testing.T, expected, actual []ScheduledRun) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("expected %d results, got %d", len(expected), len(actual))
	}

	for i := range expected {
		if actual[i].JobID != expected[i].JobID || !actual[i].ScheduledAt.Equal(expected[i].ScheduledAt) {
			t.Errorf("result[%d]: expected %+v, got %+v", i, expected[i], actual[i])
		}
	}
}

// assertTimesInRange verifies all runs are within [start, end).
func assertTimesInRange(t *testing.T, runs []ScheduledRun, start, end time.Time) {
	t.Helper()
	for i, run := range runs {
		if run.ScheduledAt.Before(start) {
			t.Errorf("runs[%d].ScheduledAt=%v is before start=%v", i, run.ScheduledAt, start)
		}
		if !run.ScheduledAt.Before(end) {
			t.Errorf("runs[%d].ScheduledAt=%v is not before end=%v", i, run.ScheduledAt, end)
		}
	}
}

// Build Tests

func TestBuild_Empty(t *testing.T) {
	idx := NewScheduledRunIndex([]ScheduledRun{})

	if idx.Len() != 0 {
		t.Errorf("expected Len()=0, got %d", idx.Len())
	}

	results := idx.Query(time.Now(), time.Now().Add(time.Hour))
	if len(results) != 0 {
		t.Errorf("expected empty query results, got %d", len(results))
	}
}

func TestBuild_SingleRun(t *testing.T) {
	now := time.Now()
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: now},
	}

	idx := NewScheduledRunIndex(runs)

	if idx.Len() != 1 {
		t.Errorf("expected Len()=1, got %d", idx.Len())
	}

	results := idx.Query(now, now.Add(time.Second))
	if len(results) != 1 {
		t.Fatalf("expected 1 query result, got %d", len(results))
	}
	if results[0].JobID != "job-001" {
		t.Errorf("expected JobID=job-001, got %s", results[0].JobID)
	}
}

func TestBuild_MultipleRuns_SameTime(t *testing.T) {
	now := time.Now()
	runs := []ScheduledRun{
		{JobID: "job-003", ScheduledAt: now},
		{JobID: "job-001", ScheduledAt: now},
		{JobID: "job-002", ScheduledAt: now},
	}

	idx := NewScheduledRunIndex(runs)

	if idx.Len() != 3 {
		t.Errorf("expected Len()=3, got %d", idx.Len())
	}

	results := idx.Query(now, now.Add(time.Second))
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Should be sorted by JobID when times are equal
	assertSorted(t, results)
	if results[0].JobID != "job-001" || results[1].JobID != "job-002" || results[2].JobID != "job-003" {
		t.Errorf("expected sorted by JobID, got %v, %v, %v", results[0].JobID, results[1].JobID, results[2].JobID)
	}
}

func TestBuild_MultipleRuns_DifferentTimes(t *testing.T) {
	now := time.Now()
	// Intentionally unsorted input
	runs := []ScheduledRun{
		{JobID: "job-003", ScheduledAt: now.Add(30 * time.Second)},
		{JobID: "job-001", ScheduledAt: now},
		{JobID: "job-002", ScheduledAt: now.Add(10 * time.Second)},
	}

	idx := NewScheduledRunIndex(runs)

	if idx.Len() != 3 {
		t.Errorf("expected Len()=3, got %d", idx.Len())
	}

	results := idx.Query(now, now.Add(time.Minute))
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Should be sorted by time
	assertSorted(t, results)
	if results[0].JobID != "job-001" || results[1].JobID != "job-002" || results[2].JobID != "job-003" {
		t.Errorf("expected sorted by time, got %v, %v, %v", results[0].JobID, results[1].JobID, results[2].JobID)
	}
}

func TestBuild_UnsortedData(t *testing.T) {
	now := time.Now()
	// Generate deliberately unsorted runs
	runs := generateUnsortedRuns(100, now, time.Minute)

	idx := NewScheduledRunIndex(runs)

	if idx.Len() != 100 {
		t.Errorf("expected Len()=100, got %d", idx.Len())
	}

	// Query all and verify sorted
	results := idx.Query(now, now.Add(200*time.Minute))
	assertSorted(t, results)
}

func TestBuild_LargeDataset(t *testing.T) {
	now := time.Now()
	runs := generateUnsortedRuns(10000, now, time.Second)

	idx := NewScheduledRunIndex(runs)

	if idx.Len() != 10000 {
		t.Errorf("expected Len()=10000, got %d", idx.Len())
	}

	// Spot check: query first minute
	results := idx.Query(now, now.Add(time.Minute))
	if len(results) == 0 {
		t.Error("expected results in first minute")
	}
	assertSorted(t, results)
}

// Query Tests

func TestQuery_ExactBoundaries(t *testing.T) {
	now := time.Now()
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: now},
		{JobID: "job-002", ScheduledAt: now.Add(time.Second)},
		{JobID: "job-003", ScheduledAt: now.Add(2 * time.Second)},
	}

	idx := NewScheduledRunIndex(runs)

	// Start inclusive
	results := idx.Query(now, now.Add(time.Second))
	if len(results) != 1 {
		t.Errorf("expected 1 result (start inclusive), got %d", len(results))
	}
	if len(results) > 0 && results[0].JobID != "job-001" {
		t.Errorf("expected job-001, got %s", results[0].JobID)
	}

	// End exclusive
	results = idx.Query(now.Add(time.Second), now.Add(2*time.Second))
	if len(results) != 1 {
		t.Errorf("expected 1 result (end exclusive), got %d", len(results))
	}
	if len(results) > 0 && results[0].JobID != "job-002" {
		t.Errorf("expected job-002, got %s", results[0].JobID)
	}
}

func TestQuery_BeforeAllRuns(t *testing.T) {
	now := time.Now()
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: now.Add(time.Hour)},
	}

	idx := NewScheduledRunIndex(runs)

	results := idx.Query(now, now.Add(time.Minute))
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestQuery_AfterAllRuns(t *testing.T) {
	now := time.Now()
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: now},
	}

	idx := NewScheduledRunIndex(runs)

	results := idx.Query(now.Add(time.Hour), now.Add(2*time.Hour))
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestQuery_EmptyWindow(t *testing.T) {
	now := time.Now()
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: now},
	}

	idx := NewScheduledRunIndex(runs)

	results := idx.Query(now, now)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty window, got %d", len(results))
	}
}

func TestQuery_SingleResult(t *testing.T) {
	now := time.Now()
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: now},
		{JobID: "job-002", ScheduledAt: now.Add(time.Hour)},
	}

	idx := NewScheduledRunIndex(runs)

	results := idx.Query(now, now.Add(time.Minute))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].JobID != "job-001" {
		t.Errorf("expected job-001, got %s", results[0].JobID)
	}
}

func TestQuery_MultipleResults(t *testing.T) {
	now := time.Now()
	runs := generateUnsortedRuns(1000, now, time.Second)

	idx := NewScheduledRunIndex(runs)

	// Query 10 seconds worth
	results := idx.Query(now, now.Add(10*time.Second))
	if len(results) == 0 {
		t.Error("expected results")
	}
	assertSorted(t, results)
	assertTimesInRange(t, results, now, now.Add(10*time.Second))
}

func TestQuery_PartialOverlap(t *testing.T) {
	now := time.Now()
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: now},
		{JobID: "job-002", ScheduledAt: now.Add(30 * time.Second)},
		{JobID: "job-003", ScheduledAt: now.Add(time.Minute)},
		{JobID: "job-004", ScheduledAt: now.Add(90 * time.Second)},
	}

	idx := NewScheduledRunIndex(runs)

	// Query middle range
	results := idx.Query(now.Add(20*time.Second), now.Add(70*time.Second))
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].JobID != "job-002" || results[1].JobID != "job-003" {
		t.Errorf("expected job-002 and job-003, got %s and %s", results[0].JobID, results[1].JobID)
	}
}

func TestQuery_AllRuns(t *testing.T) {
	now := time.Now()
	runs := generateUnsortedRuns(100, now, time.Second)

	idx := NewScheduledRunIndex(runs)

	results := idx.Query(now.Add(-time.Hour), now.Add(time.Hour))
	if len(results) != 100 {
		t.Errorf("expected 100 results, got %d", len(results))
	}
	assertSorted(t, results)
}

func TestQuery_NanosecondPrecision(t *testing.T) {
	now := time.Now()
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: now},
		{JobID: "job-002", ScheduledAt: now.Add(1)}, // 1 nanosecond later
	}

	idx := NewScheduledRunIndex(runs)

	// Query should respect nanosecond precision
	results := idx.Query(now, now.Add(1))
	if len(results) != 1 {
		t.Fatalf("expected 1 result (nanosecond boundary), got %d", len(results))
	}
	if results[0].JobID != "job-001" {
		t.Errorf("expected job-001, got %s", results[0].JobID)
	}
}

// Len Tests

func TestLen_EmptyIndex(t *testing.T) {
	idx := NewScheduledRunIndex([]ScheduledRun{})

	if idx.Len() != 0 {
		t.Errorf("expected Len()=0, got %d", idx.Len())
	}
}

func TestLen_AfterBuild(t *testing.T) {
	now := time.Now()
	runs := generateUnsortedRuns(42, now, time.Second)

	idx := NewScheduledRunIndex(runs)

	if idx.Len() != 42 {
		t.Errorf("expected Len()=42, got %d", idx.Len())
	}
}

func TestLen_Consistency(t *testing.T) {
	now := time.Now()
	runs := generateUnsortedRuns(100, now, time.Second)

	idx := NewScheduledRunIndex(runs)

	// Len should match Query(all)
	allResults := idx.Query(now.Add(-time.Hour), now.Add(time.Hour))
	if idx.Len() != len(allResults) {
		t.Errorf("Len()=%d != len(Query(all))=%d", idx.Len(), len(allResults))
	}
}

// Edge Case Tests

func TestEdgeCase_FarFutureTime(t *testing.T) {
	farFuture := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: farFuture},
	}

	idx := NewScheduledRunIndex(runs)

	results := idx.Query(farFuture, farFuture.Add(time.Hour))
	if len(results) != 1 {
		t.Errorf("expected 1 result for far future, got %d", len(results))
	}
}

func TestEdgeCase_PastTime(t *testing.T) {
	past := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: past},
	}

	idx := NewScheduledRunIndex(runs)

	results := idx.Query(past, past.Add(time.Hour))
	if len(results) != 1 {
		t.Errorf("expected 1 result for past time, got %d", len(results))
	}
}

func TestEdgeCase_ZeroTime(t *testing.T) {
	zeroTime := time.Time{}
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: time.Now()},
	}

	idx := NewScheduledRunIndex(runs)

	// Should handle zero time gracefully
	results := idx.Query(zeroTime, time.Now().Add(time.Hour))
	if len(results) != 1 {
		t.Errorf("expected 1 result with zero time start, got %d", len(results))
	}
}

func TestEdgeCase_DuplicateJobIDs_DifferentTimes(t *testing.T) {
	now := time.Now()
	runs := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: now},
		{JobID: "job-001", ScheduledAt: now.Add(time.Minute)},
		{JobID: "job-001", ScheduledAt: now.Add(2 * time.Minute)},
	}

	idx := NewScheduledRunIndex(runs)

	if idx.Len() != 3 {
		t.Errorf("expected Len()=3, got %d", idx.Len())
	}

	results := idx.Query(now, now.Add(3*time.Minute))
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	assertSorted(t, results)
}

func TestEdgeCase_VeryLongJobID(t *testing.T) {
	now := time.Now()
	longID := generateLargeJobID(1, 1000)
	runs := []ScheduledRun{
		{JobID: longID, ScheduledAt: now},
	}

	idx := NewScheduledRunIndex(runs)

	results := idx.Query(now, now.Add(time.Second))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].JobID != longID {
		t.Error("JobID mismatch")
	}
}

func TestEdgeCase_SpecialCharactersInJobID(t *testing.T) {
	now := time.Now()
	runs := []ScheduledRun{
		{JobID: "job-with-émojis-🚀", ScheduledAt: now},
		{JobID: "日本語", ScheduledAt: now.Add(time.Second)},
		{JobID: "job_with!special@chars#", ScheduledAt: now.Add(2 * time.Second)},
	}

	idx := NewScheduledRunIndex(runs)

	if idx.Len() != 3 {
		t.Errorf("expected Len()=3, got %d", idx.Len())
	}

	results := idx.Query(now, now.Add(3*time.Second))
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	assertSorted(t, results)
}

// Concurrency Tests

func TestConcurrency_MultipleReaders(t *testing.T) {
	now := time.Now()
	runs := generateUnsortedRuns(10000, now, time.Second)

	idx := NewScheduledRunIndex(runs)

	// Launch 10 goroutines querying concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				results := idx.Query(now, now.Add(time.Minute))
				if len(results) == 0 {
					t.Error("expected results from concurrent query")
				}
			}
		}()
	}

	wg.Wait()
}

func TestConcurrency_ReadWhileSwap(t *testing.T) {
	now := time.Now()
	runs1 := generateUnsortedRuns(1000, now, time.Second)
	runs2 := generateUnsortedRuns(2000, now, time.Second)

	idx := NewScheduledRunIndex(runs1)

	// Start readers
	done := make(chan bool)
	var wg sync.WaitGroup

	// 5 readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					_ = idx.Query(now, now.Add(time.Minute))
					_ = idx.Len()
				}
			}
		}()
	}

	// Perform multiple swaps while readers are active
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond)
		if i%2 == 0 {
			idx.Swap(runs2)
		} else {
			idx.Swap(runs1)
		}
	}

	// Stop readers
	close(done)
	wg.Wait()
}

func TestConcurrency_MultipleSwaps(t *testing.T) {
	now := time.Now()
	runs1 := generateUnsortedRuns(100, now, time.Second)
	runs2 := generateUnsortedRuns(200, now, time.Second)

	idx := NewScheduledRunIndex(runs1)

	// Multiple goroutines swapping
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if id%2 == 0 {
					idx.Swap(runs1)
				} else {
					idx.Swap(runs2)
				}
			}
		}(i)
	}

	wg.Wait()

	// Index should still be valid
	if idx.Len() != 100 && idx.Len() != 200 {
		t.Errorf("expected Len() to be 100 or 200, got %d", idx.Len())
	}
}

// Atomic Swap Tests

func TestAtomicSwap_ReplacesData(t *testing.T) {
	now := time.Now()
	runs1 := []ScheduledRun{
		{JobID: "job-001", ScheduledAt: now},
	}
	runs2 := []ScheduledRun{
		{JobID: "job-002", ScheduledAt: now},
		{JobID: "job-003", ScheduledAt: now.Add(time.Second)},
	}

	idx := NewScheduledRunIndex(runs1)

	if idx.Len() != 1 {
		t.Fatalf("expected initial Len()=1, got %d", idx.Len())
	}

	idx.Swap(runs2)

	if idx.Len() != 2 {
		t.Fatalf("expected Len()=2 after swap, got %d", idx.Len())
	}

	results := idx.Query(now, now.Add(2*time.Second))
	if len(results) != 2 {
		t.Errorf("expected 2 results after swap, got %d", len(results))
	}
}

func TestAtomicSwap_ToEmpty(t *testing.T) {
	now := time.Now()
	runs := generateUnsortedRuns(100, now, time.Second)

	idx := NewScheduledRunIndex(runs)

	if idx.Len() != 100 {
		t.Fatalf("expected initial Len()=100, got %d", idx.Len())
	}

	idx.Swap([]ScheduledRun{})

	if idx.Len() != 0 {
		t.Errorf("expected Len()=0 after swap to empty, got %d", idx.Len())
	}

	results := idx.Query(now, now.Add(time.Hour))
	if len(results) != 0 {
		t.Errorf("expected 0 results after swap to empty, got %d", len(results))
	}
}

func TestAtomicSwap_FromEmpty(t *testing.T) {
	now := time.Now()
	idx := NewScheduledRunIndex([]ScheduledRun{})

	runs := generateUnsortedRuns(50, now, time.Second)
	idx.Swap(runs)

	if idx.Len() != 50 {
		t.Errorf("expected Len()=50 after swap from empty, got %d", idx.Len())
	}
}
