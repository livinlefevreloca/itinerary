package testutil

import (
	"fmt"
	"sync"
	"time"
)

// MockDB provides a mock database for testing
type MockDB struct {
	mu             sync.Mutex
	jobs           []*Job
	writtenUpdates []JobRunUpdate
	writtenStats   []StatsUpdate
	queryError     error
	writeError     error
	writeDelay     time.Duration
}

// Job represents a job definition for testing
type Job struct {
	ID       string
	Name     string
	Schedule string
	PodSpec  string
}

// JobRunUpdate represents a job run update for testing
type JobRunUpdate struct {
	UpdateID    string
	RunID       string
	JobID       string
	ScheduledAt time.Time
	CompletedAt time.Time
	Status      string
	Success     bool
	Error       error
}

// StatsUpdate represents a stats update for testing
type StatsUpdate struct {
	Stats []SchedulerIterationStats
}

// SchedulerIterationStats represents iteration statistics for testing
type SchedulerIterationStats struct {
	Timestamp               time.Time
	IterationDuration       time.Duration
	ActiveOrchestratorCount int
	IndexSize               int
	InboxDepth              int
	MessagesProcessed       int
}

func NewMockDB() *MockDB {
	return &MockDB{
		jobs:           make([]*Job, 0),
		writtenUpdates: make([]JobRunUpdate, 0),
		writtenStats:   make([]StatsUpdate, 0),
	}
}

func (m *MockDB) SetJobs(jobs []*Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = jobs
}

func (m *MockDB) SetQueryError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryError = err
}

func (m *MockDB) SetWriteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeError = err
}

func (m *MockDB) SetWriteDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeDelay = delay
}

func (m *MockDB) QueryJobDefinitions() ([]*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.queryError != nil {
		return nil, m.queryError
	}

	return m.jobs, nil
}

func (m *MockDB) WriteJobRunUpdate(update JobRunUpdate) error {
	m.mu.Lock()
	delay := m.writeDelay
	err := m.writeError
	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil {
		return err
	}

	m.writtenUpdates = append(m.writtenUpdates, update)
	return nil
}

func (m *MockDB) WriteStatsUpdate(update StatsUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.writeError != nil {
		return m.writeError
	}

	m.writtenStats = append(m.writtenStats, update)
	return nil
}

func (m *MockDB) GetWrittenUpdates() []JobRunUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]JobRunUpdate, len(m.writtenUpdates))
	copy(result, m.writtenUpdates)
	return result
}

func (m *MockDB) GetWrittenStats() []StatsUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]StatsUpdate, len(m.writtenStats))
	copy(result, m.writtenStats)
	return result
}

func (m *MockDB) CountWrittenUpdates() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.writtenUpdates)
}

func (m *MockDB) CountWrittenStats() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.writtenStats)
}

func (m *MockDB) ClearWritten() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writtenUpdates = make([]JobRunUpdate, 0)
	m.writtenStats = make([]StatsUpdate, 0)
}

// Satisfy sql.DB interface (minimally)
func (m *MockDB) Close() error {
	return nil
}

// MockClock provides controllable time for testing
type MockClock struct {
	mu      sync.Mutex
	current time.Time
}

func NewMockClock(start time.Time) *MockClock {
	return &MockClock{
		current: start,
	}
}

func (m *MockClock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

func (m *MockClock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = m.current.Add(d)
}

func (m *MockClock) Set(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = t
}

// TestLogger provides a logger that captures logs for testing
type TestLogger struct {
	mu      sync.Mutex
	entries []LogEntry
}

type LogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

func NewTestLogger() *TestLogger {
	return &TestLogger{
		entries: make([]LogEntry, 0),
	}
}

func (l *TestLogger) Debug(msg string, fields ...interface{}) {
	l.log("DEBUG", msg, fields...)
}

func (l *TestLogger) Info(msg string, fields ...interface{}) {
	l.log("INFO", msg, fields...)
}

func (l *TestLogger) Warn(msg string, fields ...interface{}) {
	l.log("WARN", msg, fields...)
}

func (l *TestLogger) Error(msg string, fields ...interface{}) {
	l.log("ERROR", msg, fields...)
}

func (l *TestLogger) log(level, msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Level:   level,
		Message: msg,
		Fields:  make(map[string]interface{}),
	}

	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprintf("%v", fields[i])
			entry.Fields[key] = fields[i+1]
		}
	}

	l.entries = append(l.entries, entry)
}

func (l *TestLogger) GetEntries() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	result := make([]LogEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

func (l *TestLogger) GetEntriesByLevel(level string) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	result := make([]LogEntry, 0)
	for _, entry := range l.entries {
		if entry.Level == level {
			result = append(result, entry)
		}
	}
	return result
}

func (l *TestLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]LogEntry, 0)
}

func (l *TestLogger) HasError() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.entries {
		if entry.Level == "ERROR" {
			return true
		}
	}
	return false
}

func (l *TestLogger) HasWarning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.entries {
		if entry.Level == "WARN" {
			return true
		}
	}
	return false
}

// WaitFor waits for a condition to be true with timeout
func WaitFor(t TestingT, condition func() bool, timeout time.Duration, msgAndArgs ...interface{}) bool {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return true
		}

		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				t.Errorf("timeout waiting for condition: %v", msgAndArgs)
				return false
			}
		}
	}
}

// TestingT is a minimal interface for testing
type TestingT interface {
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}
