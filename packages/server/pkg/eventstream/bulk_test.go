package eventstream_test

import (
	"context"
	"testing"
	"time"

	"the-dev-tools/server/pkg/eventstream/memory"
)

// TestInMemorySyncStreamer_BulkPublish verifies that the buffer is large enough
// to handle a burst of events (e.g. 100) which is greater than the old default of 32.
func TestInMemorySyncStreamer_BulkPublish(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[testTopic, testPayload]()
	t.Cleanup(streamer.Shutdown)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	// Subscribe
	ch, err := streamer.Subscribe(ctx, allowAll)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Publish 100 events in a tight loop (burst)
	// If buffer was still 32, many of these would be dropped because
	// we are not reading from 'ch' yet.
	const count = 100
	for i := 0; i < count; i++ {
		streamer.Publish(testTopic{workspace: "A"}, testPayload{ID: "test"})
	}

	// Now read all events
	received := 0
	timeout := time.After(2 * time.Second)

Loop:
	for i := 0; i < count; i++ {
		select {
		case <-ch:
			received++
		case <-timeout:
			t.Errorf("timeout waiting for event %d", i+1)
			break Loop
		}
	}

	if received != count {
		t.Errorf("Expected %d events, got %d. Some events were dropped due to small buffer.", count, received)
	}
}
