package inbox

import (
	"log/slog"
	"sync/atomic"
	"time"
)

// Inbox provides a generic typed interface for message channels with timeout support
// T is the message type that will be sent through the inbox
type Inbox[T any] struct {
	ch      chan T
	timeout time.Duration
	logger  *slog.Logger
	stats   *Stats
}

// Stats tracks inbox usage and performance metrics
type Stats struct {
	TotalSent     int64
	TotalReceived int64
	TimeoutCount  int64
	CurrentDepth  int
	MaxDepthSeen  int
}

// New creates a new inbox with the specified buffer size and timeout
func New[T any](bufferSize int, timeout time.Duration, logger *slog.Logger) *Inbox[T] {
	return &Inbox[T]{
		ch:      make(chan T, bufferSize),
		timeout: timeout,
		logger:  logger,
		stats:   &Stats{},
	}
}

// Send sends a message to the inbox with timeout
// Returns true if message was sent successfully, false if timeout occurred
func (ib *Inbox[T]) Send(msg T) bool {
	select {
	case ib.ch <- msg:
		atomic.AddInt64(&ib.stats.TotalSent, 1)
		return true
	case <-time.After(ib.timeout):
		atomic.AddInt64(&ib.stats.TimeoutCount, 1)
		ib.logger.Warn("inbox send timeout",
			"timeout", ib.timeout,
			"current_depth", len(ib.ch))
		return false
	}
}

// TryReceive attempts to receive a message without blocking
// Returns the message and true if available, zero value and false otherwise
func (ib *Inbox[T]) TryReceive() (T, bool) {
	select {
	case msg := <-ib.ch:
		atomic.AddInt64(&ib.stats.TotalReceived, 1)
		return msg, true
	default:
		var zero T
		return zero, false
	}
}

// Receive blocks until a message is available
func (ib *Inbox[T]) Receive() T {
	msg := <-ib.ch
	atomic.AddInt64(&ib.stats.TotalReceived, 1)
	return msg
}

// UpdateDepthStats updates the current and maximum depth statistics
func (ib *Inbox[T]) UpdateDepthStats() {
	depth := len(ib.ch)
	ib.stats.CurrentDepth = depth
	if depth > ib.stats.MaxDepthSeen {
		ib.stats.MaxDepthSeen = depth
	}
}

// GetStats returns a copy of the current inbox statistics
func (ib *Inbox[T]) GetStats() Stats {
	return Stats{
		TotalSent:     atomic.LoadInt64(&ib.stats.TotalSent),
		TotalReceived: atomic.LoadInt64(&ib.stats.TotalReceived),
		TimeoutCount:  atomic.LoadInt64(&ib.stats.TimeoutCount),
		CurrentDepth:  ib.stats.CurrentDepth,
		MaxDepthSeen:  ib.stats.MaxDepthSeen,
	}
}

// Len returns the current number of messages in the inbox
func (ib *Inbox[T]) Len() int {
	return len(ib.ch)
}

// Close closes the inbox channel
func (ib *Inbox[T]) Close() {
	close(ib.ch)
}
