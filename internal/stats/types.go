package stats

import "time"

// StatsSource identifies which component sent the stats
type StatsSource int

const (
	StatsSourceScheduler StatsSource = iota
	StatsSourceOrchestrator
	StatsSourceSyncer
	StatsSourceWebhookHandler
)

// StatsMessage is the container for all stats messages
type StatsMessage struct {
	Source    StatsSource
	Timestamp time.Time
	Data      interface{} // Actual type depends on Source
}

// SchedulerStatsData contains statistics from the scheduler
type SchedulerStatsData struct {
	Iterations      int
	JobsRun         int
	LateJobs        int
	MissedJobs      int
	JobsCancelled   int
	InboxLength     int
	InboxEmptyTime  time.Duration
	MessageWaitTime time.Duration // Time message spent in inbox
}

// OrchestratorStatsData contains statistics from an orchestrator
type OrchestratorStatsData struct {
	RunID              string
	Runtime            time.Duration
	ConstraintsChecked int
	ActionsTaken       int
}

// SyncerStatsData contains statistics from the syncer
type SyncerStatsData struct {
	TotalWrites     int
	WritesSucceeded int
	WritesFailed    int
	WritesInFlight  int
	QueuedWrites    int
	InboxLength     int
	TimeInQueue     time.Duration
	TimeInInbox     time.Duration
}

// WebhookStatsData contains statistics from the webhook handler (future)
type WebhookStatsData struct {
	WebhooksSent      int
	WebhooksSucceeded int
	WebhooksFailed    int
	TotalRetries      int
	DeliveryTime      time.Duration
	InboxLength       int
}

// SchedulerStatsAccumulator accumulates scheduler statistics for a period
type SchedulerStatsAccumulator struct {
	Iterations     int
	JobsRun        int
	LateJobs       int
	MissedJobs     int
	JobsCancelled  int

	// Samples for min/max/avg calculations
	InboxLengthSamples []int
	EmptyInboxTime     time.Duration
	MessageWaitTimes   []time.Duration
}

// Add adds scheduler stats data to the accumulator
func (acc *SchedulerStatsAccumulator) Add(data *SchedulerStatsData) {
	acc.Iterations += data.Iterations
	acc.JobsRun += data.JobsRun
	acc.LateJobs += data.LateJobs
	acc.MissedJobs += data.MissedJobs
	acc.JobsCancelled += data.JobsCancelled
	acc.InboxLengthSamples = append(acc.InboxLengthSamples, data.InboxLength)
	acc.EmptyInboxTime += data.InboxEmptyTime
	acc.MessageWaitTimes = append(acc.MessageWaitTimes, data.MessageWaitTime)
}

// Reset clears the accumulator for a new period
func (acc *SchedulerStatsAccumulator) Reset() {
	acc.Iterations = 0
	acc.JobsRun = 0
	acc.LateJobs = 0
	acc.MissedJobs = 0
	acc.JobsCancelled = 0
	acc.InboxLengthSamples = make([]int, 0)
	acc.EmptyInboxTime = 0
	acc.MessageWaitTimes = make([]time.Duration, 0)
}

// SyncerStatsAccumulator accumulates syncer statistics for a period
type SyncerStatsAccumulator struct {
	TotalWrites     int
	WritesSucceeded int
	WritesFailed    int

	// Samples for min/max/avg calculations
	WritesInFlightSamples []int
	QueuedWritesSamples   []int
	InboxLengthSamples    []int
	TimeInQueueSamples    []time.Duration
	TimeInInboxSamples    []time.Duration
}

// Add adds syncer stats data to the accumulator
func (acc *SyncerStatsAccumulator) Add(data *SyncerStatsData) {
	acc.TotalWrites += data.TotalWrites
	acc.WritesSucceeded += data.WritesSucceeded
	acc.WritesFailed += data.WritesFailed
	acc.WritesInFlightSamples = append(acc.WritesInFlightSamples, data.WritesInFlight)
	acc.QueuedWritesSamples = append(acc.QueuedWritesSamples, data.QueuedWrites)
	acc.InboxLengthSamples = append(acc.InboxLengthSamples, data.InboxLength)
	acc.TimeInQueueSamples = append(acc.TimeInQueueSamples, data.TimeInQueue)
	acc.TimeInInboxSamples = append(acc.TimeInInboxSamples, data.TimeInInbox)
}

// Reset clears the accumulator for a new period
func (acc *SyncerStatsAccumulator) Reset() {
	acc.TotalWrites = 0
	acc.WritesSucceeded = 0
	acc.WritesFailed = 0
	acc.WritesInFlightSamples = make([]int, 0)
	acc.QueuedWritesSamples = make([]int, 0)
	acc.InboxLengthSamples = make([]int, 0)
	acc.TimeInQueueSamples = make([]time.Duration, 0)
	acc.TimeInInboxSamples = make([]time.Duration, 0)
}

// WebhookStatsAccumulator accumulates webhook stats for a period (future)
type WebhookStatsAccumulator struct {
	WebhooksSent      int
	WebhooksSucceeded int
	WebhooksFailed    int
	TotalRetries      int

	// Samples for min/max/avg calculations
	DeliveryTimeSamples []time.Duration
	InboxLengthSamples  []int
}
