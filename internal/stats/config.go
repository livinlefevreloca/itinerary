package stats

import "time"

// Config defines configuration for the stats collector
type Config struct {
	// Inbox configuration
	InboxBufferSize  int           `toml:"inbox_buffer_size"`
	InboxSendTimeout time.Duration `toml:"inbox_send_timeout"`

	// Flush configuration
	FlushInterval  time.Duration `toml:"flush_interval"`
	FlushThreshold int           `toml:"flush_threshold"`

	// Stats period configuration
	PeriodDuration time.Duration `toml:"period_duration"`
}

// DefaultConfig returns default stats collector configuration
func DefaultConfig() Config {
	return Config{
		InboxBufferSize:  1000,
		InboxSendTimeout: 5 * time.Second,
		FlushInterval:    30 * time.Second,
		FlushThreshold:   100,
		PeriodDuration:   30 * time.Second,
	}
}
