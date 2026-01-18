package testutil

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// MockDB provides a mock database for testing
type MockDB struct {
	mu             sync.Mutex
	jobs           []*Job
	writtenUpdates []interface{} // Store as interface{} to avoid import cycle
	writtenStats   []interface{} // Store as interface{} to avoid import cycle
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

func NewMockDB() *MockDB {
	return &MockDB{
		jobs:           make([]*Job, 0),
		writtenUpdates: make([]interface{}, 0),
		writtenStats:   make([]interface{}, 0),
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

func (m *MockDB) WriteJobRunUpdate(update interface{}) error {
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

func (m *MockDB) WriteStatsUpdate(update interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.writeError != nil {
		return m.writeError
	}

	m.writtenStats = append(m.writtenStats, update)
	return nil
}

func (m *MockDB) GetWrittenUpdates() []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]interface{}, len(m.writtenUpdates))
	copy(result, m.writtenUpdates)
	return result
}

func (m *MockDB) GetWrittenStats() []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]interface{}, len(m.writtenStats))
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
	m.writtenUpdates = make([]interface{}, 0)
	m.writtenStats = make([]interface{}, 0)
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

func (l *TestLogger) HasDebug() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.entries {
		if entry.Level == "DEBUG" {
			return true
		}
	}
	return false
}

// Logger returns a *slog.Logger that writes to this TestLogger
func (l *TestLogger) Logger() *slog.Logger {
	return slog.New(&testLogHandler{logger: l})
}

// testLogHandler implements slog.Handler for TestLogger
type testLogHandler struct {
	logger *TestLogger
	attrs  []slog.Attr
	groups []string
}

func (h *testLogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	level := r.Level.String()
	msg := r.Message

	// Collect all attributes
	fields := make([]interface{}, 0, r.NumAttrs()*2)
	r.Attrs(func(a slog.Attr) bool {
		fields = append(fields, a.Key, a.Value.Any())
		return true
	})

	// Add handler-level attributes
	for _, attr := range h.attrs {
		fields = append(fields, attr.Key, attr.Value.Any())
	}

	h.logger.log(level, msg, fields...)
	return nil
}

func (h *testLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &testLogHandler{
		logger: h.logger,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

func (h *testLogHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name
	return &testLogHandler{
		logger: h.logger,
		attrs:  h.attrs,
		groups: newGroups,
	}
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
