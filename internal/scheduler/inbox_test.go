package scheduler

import (
	"testing"
	"time"

	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

func TestInbox_Send_Success(t *testing.T) {
	logger := testutil.NewTestLogger()
	inbox := NewInbox(10, 100*time.Millisecond, logger)

	// Send 5 messages
	for i := 0; i < 5; i++ {
		msg := InboxMessage{
			Type: MsgOrchestratorStateChange,
			Data: OrchestratorStateChangeMsg{
				RunID:     "test-run",
				NewStatus: OrchestratorPending,
				Timestamp: time.Now(),
			},
		}
		success := inbox.Send(msg)
		if !success {
			t.Errorf("expected send %d to succeed", i)
		}
	}

	stats := inbox.Stats()
	if stats.TotalSent != 5 {
		t.Errorf("expected TotalSent to be 5, got %d", stats.TotalSent)
	}

	if stats.TimeoutCount != 0 {
		t.Errorf("expected TimeoutCount to be 0, got %d", stats.TimeoutCount)
	}
}

func TestInbox_Send_Timeout(t *testing.T) {
	logger := testutil.NewTestLogger()
	inbox := NewInbox(2, 10*time.Millisecond, logger)

	// Fill buffer with 2 messages
	for i := 0; i < 2; i++ {
		msg := InboxMessage{
			Type: MsgShutdown,
		}
		success := inbox.Send(msg)
		if !success {
			t.Errorf("expected send %d to succeed", i)
		}
	}

	// Third send should timeout
	msg := InboxMessage{
		Type: MsgShutdown,
	}
	success := inbox.Send(msg)
	if success {
		t.Error("expected third send to timeout")
	}

	stats := inbox.Stats()
	if stats.TimeoutCount != 1 {
		t.Errorf("expected TimeoutCount to be 1, got %d", stats.TimeoutCount)
	}
}

func TestInbox_TryReceive_Success(t *testing.T) {
	logger := testutil.NewTestLogger()
	inbox := NewInbox(10, 100*time.Millisecond, logger)

	// Send 3 messages
	for i := 0; i < 3; i++ {
		msg := InboxMessage{
			Type: MsgShutdown,
		}
		inbox.Send(msg)
	}

	// Try to receive 3 messages
	for i := 0; i < 3; i++ {
		msg, ok := inbox.TryReceive()
		if !ok {
			t.Errorf("expected TryReceive %d to succeed", i)
		}
		if msg.Type != MsgShutdown {
			t.Errorf("expected message type MsgShutdown, got %v", msg.Type)
		}
	}

	stats := inbox.Stats()
	if stats.TotalReceived != 3 {
		t.Errorf("expected TotalReceived to be 3, got %d", stats.TotalReceived)
	}
}

func TestInbox_TryReceive_Empty(t *testing.T) {
	logger := testutil.NewTestLogger()
	inbox := NewInbox(10, 100*time.Millisecond, logger)

	msg, ok := inbox.TryReceive()
	if ok {
		t.Error("expected TryReceive to return false for empty inbox")
	}

	if msg.Type != 0 {
		t.Error("expected zero message value")
	}
}

func TestInbox_Receive_Blocking(t *testing.T) {
	logger := testutil.NewTestLogger()
	inbox := NewInbox(10, 100*time.Millisecond, logger)

	// Start goroutine that will receive (blocks)
	received := make(chan InboxMessage, 1)
	go func() {
		msg := inbox.Receive()
		received <- msg
	}()

	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Send message after 50ms
	time.Sleep(50 * time.Millisecond)
	sentMsg := InboxMessage{
		Type: MsgShutdown,
	}
	inbox.Send(sentMsg)

	// Goroutine should unblock and receive message
	select {
	case msg := <-received:
		if msg.Type != MsgShutdown {
			t.Errorf("expected message type MsgShutdown, got %v", msg.Type)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for receive")
	}
}

func TestInbox_UpdateDepthStats(t *testing.T) {
	logger := testutil.NewTestLogger()
	inbox := NewInbox(10, 100*time.Millisecond, logger)

	// Send 5 messages
	for i := 0; i < 5; i++ {
		inbox.Send(InboxMessage{Type: MsgShutdown})
	}

	inbox.UpdateDepthStats()
	stats := inbox.Stats()
	if stats.CurrentDepth != 5 {
		t.Errorf("expected CurrentDepth to be 5, got %d", stats.CurrentDepth)
	}
	if stats.MaxDepthSeen != 5 {
		t.Errorf("expected MaxDepthSeen to be 5, got %d", stats.MaxDepthSeen)
	}

	// Send 3 more (total 8)
	for i := 0; i < 3; i++ {
		inbox.Send(InboxMessage{Type: MsgShutdown})
	}

	inbox.UpdateDepthStats()
	stats = inbox.Stats()
	if stats.CurrentDepth != 8 {
		t.Errorf("expected CurrentDepth to be 8, got %d", stats.CurrentDepth)
	}
	if stats.MaxDepthSeen != 8 {
		t.Errorf("expected MaxDepthSeen to be 8, got %d", stats.MaxDepthSeen)
	}

	// Receive 4 messages (leaving 4)
	for i := 0; i < 4; i++ {
		inbox.TryReceive()
	}

	inbox.UpdateDepthStats()
	stats = inbox.Stats()
	if stats.CurrentDepth != 4 {
		t.Errorf("expected CurrentDepth to be 4, got %d", stats.CurrentDepth)
	}
	// MaxDepthSeen should still be 8
	if stats.MaxDepthSeen != 8 {
		t.Errorf("expected MaxDepthSeen to still be 8, got %d", stats.MaxDepthSeen)
	}
}

func TestInbox_ConcurrentSendReceive(t *testing.T) {
	logger := testutil.NewTestLogger()
	inbox := NewInbox(100, 100*time.Millisecond, logger)

	const numSenders = 5
	const numMessages = 20
	done := make(chan bool)

	// Start senders
	for i := 0; i < numSenders; i++ {
		go func(id int) {
			for j := 0; j < numMessages; j++ {
				msg := InboxMessage{
					Type: MsgShutdown,
				}
				inbox.Send(msg)
			}
			done <- true
		}(i)
	}

	// Wait for all senders to finish
	for i := 0; i < numSenders; i++ {
		<-done
	}

	// All messages should be received
	expectedMessages := numSenders * numMessages
	receivedCount := 0
	for {
		_, ok := inbox.TryReceive()
		if !ok {
			break
		}
		receivedCount++
	}

	if receivedCount != expectedMessages {
		t.Errorf("expected to receive %d messages, got %d", expectedMessages, receivedCount)
	}
}
