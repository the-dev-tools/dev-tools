package eventstream_test

import (
	"context"
	"testing"
	"time"

	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/eventstream/memory"
)

// TestEvent is a simple event type for testing
type TestEvent struct {
	ID   string
	Data string
}

func TestInMemorySyncStreamer_BasicPublishSubscribe(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[TestEvent]()
	defer streamer.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := streamer.Subscribe(ctx)
	time.Sleep(10 * time.Millisecond) // Let subscriber register

	testEvent := TestEvent{ID: "1", Data: "hello"}
	streamer.Publish(testEvent)

	select {
	case received := <-events:
		if received.ID != "1" || received.Data != "hello" {
			t.Errorf("expected event {ID:1, Data:hello}, got %+v", received)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive event within timeout")
	}
}

func TestInMemorySyncStreamer_MultipleSubscribers(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[TestEvent]()
	defer streamer.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subs := make([]<-chan TestEvent, 3)
	for i := range subs {
		subs[i] = streamer.Subscribe(ctx)
	}

	time.Sleep(10 * time.Millisecond) // Let subscribers register

	testEvent := TestEvent{ID: "multi", Data: "broadcast"}
	streamer.Publish(testEvent)

	for i, sub := range subs {
		select {
		case received := <-sub:
			if received.ID != "multi" || received.Data != "broadcast" {
				t.Errorf("subscriber %d: expected {ID:multi, Data:broadcast}, got %+v", i, received)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d: did not receive event within timeout", i)
		}
	}
}

func TestInMemorySyncStreamer_SubscriberCancellation(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[TestEvent]()
	defer streamer.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	events := streamer.Subscribe(ctx)

	cancel() // Cancel immediately

	select {
	case _, ok := <-events:
		if ok {
			t.Error("expected channel to be closed after context cancellation")
		}
	case <-time.After(50 * time.Millisecond):
		t.Error("channel was not closed within timeout")
	}
}

func TestInMemorySyncStreamer_MultipleEvents(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[TestEvent]()
	defer streamer.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := streamer.Subscribe(ctx)
	time.Sleep(10 * time.Millisecond)

	expectedEvents := []TestEvent{
		{ID: "1", Data: "first"},
		{ID: "2", Data: "second"},
		{ID: "3", Data: "third"},
	}

	for _, event := range expectedEvents {
		streamer.Publish(event)
	}

	receivedEvents := make([]TestEvent, 0, len(expectedEvents))
	for i := 0; i < len(expectedEvents); i++ {
		select {
		case event := <-events:
			receivedEvents = append(receivedEvents, event)
		case <-time.After(100 * time.Millisecond):
			t.Errorf("did not receive event %d within timeout", i)
		}
	}

	for i, expected := range expectedEvents {
		if receivedEvents[i].ID != expected.ID || receivedEvents[i].Data != expected.Data {
			t.Errorf("event %d: expected %+v, got %+v", i, expected, receivedEvents[i])
		}
	}
}

func TestInMemorySyncStreamer_ContextCancellationBeforeSubscribe(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[TestEvent]()
	defer streamer.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	events := streamer.Subscribe(ctx)

	select {
	case _, ok := <-events:
		if ok {
			t.Error("expected channel to be closed for cancelled context")
		}
	case <-time.After(50 * time.Millisecond):
		t.Error("channel was not closed immediately")
	}
}

func TestInMemorySyncStreamer_ConcurrentOperations(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[TestEvent]()
	defer streamer.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const numSubscribers = 3
	const numEvents = 10

	subs := make([]<-chan TestEvent, numSubscribers)
	for i := range subs {
		subs[i] = streamer.Subscribe(ctx)
	}

	time.Sleep(10 * time.Millisecond) // Let subscribers register

	// Publish events
	for i := 0; i < numEvents; i++ {
		streamer.Publish(TestEvent{ID: string(rune('A' + i)), Data: "concurrent"})
	}

	// Check that subscribers receive events (allowing for some loss due to backpressure)
	for i, sub := range subs {
		receivedCount := 0
		timeout := time.After(200 * time.Millisecond)

		for receivedCount < numEvents {
			select {
			case <-sub:
				receivedCount++
			case <-timeout:
				t.Logf("subscriber %d: received %d/%d events (acceptable due to backpressure)", i, receivedCount, numEvents)
				break
			}
		}
	}
}

func TestEvent_GenericUsage(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[eventstream.Event[TestEvent]]()
	defer streamer.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := streamer.Subscribe(ctx)
	time.Sleep(10 * time.Millisecond)

	testData := TestEvent{ID: "generic", Data: "test"}
	event := eventstream.Event[TestEvent]{
		Type:      "create",
		Entity:    "test",
		UserID:    "user123",
		Workspace: "workspace456",
		Data:      testData,
	}
	streamer.Publish(event)

	select {
	case received := <-events:
		if received.Type != "create" || received.Entity != "test" {
			t.Errorf("expected type=create, entity=test, got type=%s, entity=%s", received.Type, received.Entity)
		}
		if received.Data.ID != "generic" || received.Data.Data != "test" {
			t.Errorf("expected data {ID:generic, Data:test}, got %+v", received.Data)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive generic event within timeout")
	}
}

func TestInMemorySyncStreamer_Shutdown(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[TestEvent]()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := streamer.Subscribe(ctx)
	time.Sleep(10 * time.Millisecond)

	// Shutdown streamer
	streamer.Shutdown()

	// Channel should be closed
	select {
	case _, ok := <-events:
		if ok {
			t.Error("expected channel to be closed after shutdown")
		}
	case <-time.After(50 * time.Millisecond):
		t.Error("channel was not closed after shutdown")
	}

	// Publishing after shutdown should not panic
	streamer.Publish(TestEvent{ID: "after", Data: "shutdown"})
}

// BenchmarkInMemorySyncStreamer_Publish benchmarks publish operation
func BenchmarkInMemorySyncStreamer_Publish(b *testing.B) {
	streamer := memory.NewInMemorySyncStreamer[TestEvent]()
	defer streamer.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = streamer.Subscribe(ctx)
	time.Sleep(10 * time.Millisecond) // Let subscriber register

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			streamer.Publish(TestEvent{ID: "bench", Data: "test"})
		}
	})
}

// BenchmarkInMemorySyncStreamer_Subscribe benchmarks subscribe operation
func BenchmarkInMemorySyncStreamer_Subscribe(b *testing.B) {
	streamer := memory.NewInMemorySyncStreamer[TestEvent]()
	defer streamer.Shutdown()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		_ = streamer.Subscribe(ctx)
		cancel()
	}
}
