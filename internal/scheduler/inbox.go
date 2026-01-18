package scheduler

import (
	"log/slog"
	"sync/atomic"
	"time"
)

// Inbox provides a typed interface for the scheduler's message channel with timeout support
type Inbox struct {
	ch      chan InboxMessage
	timeout time.Duration
	logger  *slog.Logger
	stats   *InboxStats
}

// InboxStats tracks inbox usage and performance metrics
type InboxStats struct {
	TotalSent     int64
	TotalReceived int64
	TimeoutCount  int64
	CurrentDepth  int
	MaxDepthSeen  int
}

// NewInbox creates a new inbox with the specified buffer size and timeout
func NewInbox(bufferSize int, timeout time.Duration, logger *slog.Logger) *Inbox {
	return &Inbox{
		ch:      make(chan InboxMessage, bufferSize),
		timeout: timeout,
		logger:  logger,
		stats:   &InboxStats{},
	}
}

// Send sends a message to the inbox with timeout
// Returns true if message was sent successfully, false if timeout occurred
func (ib *Inbox) Send(msg InboxMessage) bool {
	select {
	case ib.ch <- msg:
		atomic.AddInt64(&ib.stats.TotalSent, 1)
		return true
	case <-time.After(ib.timeout):
		atomic.AddInt64(&ib.stats.TimeoutCount, 1)
		ib.logger.Warn("inbox send timeout",
			"msg_type", msg.Type,
			"timeout", ib.timeout,
			"current_depth", len(ib.ch))
		return false
	}
}

// TryReceive attempts to receive a message without blocking
// Returns the message and true if available, zero value and false otherwise
func (ib *Inbox) TryReceive() (InboxMessage, bool) {
	select {
	case msg := <-ib.ch:
		atomic.AddInt64(&ib.stats.TotalReceived, 1)
		return msg, true
	default:
		return InboxMessage{}, false
	}
}

// Receive blocks until a message is available
func (ib *Inbox) Receive() InboxMessage {
	msg := <-ib.ch
	atomic.AddInt64(&ib.stats.TotalReceived, 1)
	return msg
}

// UpdateDepthStats updates the current and maximum depth statistics
func (ib *Inbox) UpdateDepthStats() {
	depth := len(ib.ch)
	ib.stats.CurrentDepth = depth
	if depth > ib.stats.MaxDepthSeen {
		ib.stats.MaxDepthSeen = depth
	}
}

// Stats returns a copy of the current inbox statistics
func (ib *Inbox) Stats() InboxStats {
	return InboxStats{
		TotalSent:     atomic.LoadInt64(&ib.stats.TotalSent),
		TotalReceived: atomic.LoadInt64(&ib.stats.TotalReceived),
		TimeoutCount:  atomic.LoadInt64(&ib.stats.TimeoutCount),
		CurrentDepth:  ib.stats.CurrentDepth,
		MaxDepthSeen:  ib.stats.MaxDepthSeen,
	}
}
