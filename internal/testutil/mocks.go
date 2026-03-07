package testutil

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/db"
)

// MockDB provides a mock database for testing
type MockDB struct {
	mu             sync.Mutex
	jobs           []*db.Job
	writtenUpdates []interface{}
	writtenStats   []interface{}
	queryError     error
	writeError     error
	writeDelay     time.Duration
}

func NewMockDB() *MockDB {
	return &MockDB{
		jobs:           make([]*db.Job, 0),
		writtenUpdates: make([]interface{}, 0),
		writtenStats:   make([]interface{}, 0),
	}
}

func (m *MockDB) SetJobs(jobs []*db.Job) {
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

func (m *MockDB) QueryJobDefinitions() (interface{}, error) {
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

// MockSchedulerInbox for testing message sending to scheduler
type MockSchedulerInbox struct {
	messages      []interface{}
	mu            sync.Mutex
	responseFunc  func(msg interface{})
	shouldError   bool
	errorToReturn error
}

func NewMockSchedulerInbox() *MockSchedulerInbox {
	return &MockSchedulerInbox{
		messages: []interface{}{},
	}
}

func (m *MockSchedulerInbox) Send(msg interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldError {
		return m.errorToReturn
	}

	m.messages = append(m.messages, msg)

	// Auto-respond if response function is set
	if m.responseFunc != nil {
		m.responseFunc(msg)
	}

	return nil
}

func (m *MockSchedulerInbox) GetMessages() []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]interface{}{}, m.messages...)
}

func (m *MockSchedulerInbox) SetResponseFunc(f func(msg interface{})) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseFunc = f
}

func (m *MockSchedulerInbox) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldError = true
	m.errorToReturn = err
}

func (m *MockSchedulerInbox) ClearMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = []interface{}{}
}

// CreateTestSlogLogger creates a simple slog logger for testing that writes to stderr
func CreateTestSlogLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// CreateTestHTTPClient creates a standard HTTP client for testing
func CreateTestHTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}

// CreateTestHTTPServer creates an HTTP test server with configurable response
func CreateTestHTTPServer(statusCode int, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		w.WriteHeader(statusCode)
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
}

// Action-specific mocks

// MockWebhookHandler implements WebhookSender for testing
type MockWebhookHandler struct {
	mu             sync.Mutex
	calls          []WebhookCall
	returnError    error
	shouldFail     bool
	failureMessage string
}

type WebhookCall struct {
	URL     string
	Payload interface{}
}

func NewMockWebhookHandler() *MockWebhookHandler {
	return &MockWebhookHandler{
		calls: make([]WebhookCall, 0),
	}
}

func (m *MockWebhookHandler) SendWebhook(url string, payload interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, WebhookCall{
		URL:     url,
		Payload: payload,
	})

	if m.shouldFail {
		return m.returnError
	}

	return nil
}

func (m *MockWebhookHandler) GetCalls() []WebhookCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]WebhookCall{}, m.calls...)
}

func (m *MockWebhookHandler) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.returnError = err
	m.shouldFail = true
}

func (m *MockWebhookHandler) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]WebhookCall, 0)
	m.shouldFail = false
	m.returnError = nil
}

// MockJobController implements JobController for testing
type MockJobController struct {
	mu              sync.Mutex
	retryCalls      []string
	triggerCalls    []TriggerCall
	killAllCalls    []string
	killLatestCalls []string
	skipNextCalls   []string
	returnError     error
	shouldFail      bool
}

type TriggerCall struct {
	JobID string
	Args  map[string]interface{}
}

func NewMockJobController() *MockJobController {
	return &MockJobController{
		retryCalls:      make([]string, 0),
		triggerCalls:    make([]TriggerCall, 0),
		killAllCalls:    make([]string, 0),
		killLatestCalls: make([]string, 0),
		skipNextCalls:   make([]string, 0),
	}
}

func (m *MockJobController) RetryJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryCalls = append(m.retryCalls, jobID)
	if m.shouldFail {
		return m.returnError
	}
	return nil
}

func (m *MockJobController) TriggerJob(jobID string, args map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.triggerCalls = append(m.triggerCalls, TriggerCall{JobID: jobID, Args: args})
	if m.shouldFail {
		return m.returnError
	}
	return nil
}

func (m *MockJobController) KillAllInstances(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.killAllCalls = append(m.killAllCalls, jobID)
	if m.shouldFail {
		return m.returnError
	}
	return nil
}

func (m *MockJobController) KillLatestInstance(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.killLatestCalls = append(m.killLatestCalls, jobID)
	if m.shouldFail {
		return m.returnError
	}
	return nil
}

func (m *MockJobController) SkipNextInstance(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skipNextCalls = append(m.skipNextCalls, jobID)
	if m.shouldFail {
		return m.returnError
	}
	return nil
}

func (m *MockJobController) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.returnError = err
	m.shouldFail = true
}

func (m *MockJobController) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryCalls = make([]string, 0)
	m.triggerCalls = make([]TriggerCall, 0)
	m.killAllCalls = make([]string, 0)
	m.killLatestCalls = make([]string, 0)
	m.skipNextCalls = make([]string, 0)
	m.shouldFail = false
	m.returnError = nil
}

func (m *MockJobController) GetRetryCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.retryCalls...)
}

func (m *MockJobController) GetTriggerCalls() []TriggerCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]TriggerCall{}, m.triggerCalls...)
}

func (m *MockJobController) GetKillAllCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.killAllCalls...)
}

func (m *MockJobController) GetKillLatestCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.killLatestCalls...)
}

func (m *MockJobController) GetSkipNextCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.skipNextCalls...)
}

// MockMetadataUpdater implements MetadataUpdater for testing
type MockMetadataUpdater struct {
	mu          sync.Mutex
	calls       []MetadataUpdateCall
	returnError error
	shouldFail  bool
}

type MetadataUpdateCall struct {
	JobID    string
	Metadata map[string]interface{}
}

func NewMockMetadataUpdater() *MockMetadataUpdater {
	return &MockMetadataUpdater{
		calls: make([]MetadataUpdateCall, 0),
	}
}

func (m *MockMetadataUpdater) UpdateMetadata(jobID string, metadata map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, MetadataUpdateCall{JobID: jobID, Metadata: metadata})
	if m.shouldFail {
		return m.returnError
	}
	return nil
}

func (m *MockMetadataUpdater) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.returnError = err
	m.shouldFail = true
}

func (m *MockMetadataUpdater) GetCalls() []MetadataUpdateCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MetadataUpdateCall{}, m.calls...)
}

func (m *MockMetadataUpdater) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]MetadataUpdateCall, 0)
	m.shouldFail = false
	m.returnError = nil
}

// MockMetricRecorder implements MetricRecorder for testing
type MockMetricRecorder struct {
	mu          sync.Mutex
	calls       []MetricRecordCall
	returnError error
	shouldFail  bool
}

type MetricRecordCall struct {
	Name  string
	Value float64
	Tags  map[string]string
}

func NewMockMetricRecorder() *MockMetricRecorder {
	return &MockMetricRecorder{
		calls: make([]MetricRecordCall, 0),
	}
}

func (m *MockMetricRecorder) RecordMetric(name string, value float64, tags map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, MetricRecordCall{Name: name, Value: value, Tags: tags})
	if m.shouldFail {
		return m.returnError
	}
	return nil
}

func (m *MockMetricRecorder) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.returnError = err
	m.shouldFail = true
}

func (m *MockMetricRecorder) GetCalls() []MetricRecordCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MetricRecordCall{}, m.calls...)
}

func (m *MockMetricRecorder) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]MetricRecordCall, 0)
	m.shouldFail = false
	m.returnError = nil
}

