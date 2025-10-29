package eventstream_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/eventstream/memory"
)

type testTopic struct {
	workspace string
}

type testPayload struct {
	ID   string
	Data string
}

func allowAll(_ testTopic) bool { return true }

func TestInMemorySyncStreamer_PublishSubscribe(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[testTopic, testPayload]()
	t.Cleanup(streamer.Shutdown)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	ch, err := streamer.Subscribe(ctx, allowAll)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	streamer.Publish(testTopic{workspace: "w"}, testPayload{ID: "1", Data: "hello"})

	select {
	case evt := <-ch:
		if evt.Payload.ID != "1" || evt.Payload.Data != "hello" {
			t.Fatalf("unexpected payload: %+v", evt.Payload)
		}
		if evt.Topic.workspace != "w" {
			t.Fatalf("unexpected topic: %+v", evt.Topic)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestInMemorySyncStreamer_RespectsFilter(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[testTopic, testPayload]()
	t.Cleanup(streamer.Shutdown)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	filterA := func(topic testTopic) bool { return topic.workspace == "A" }
	filterB := func(topic testTopic) bool { return topic.workspace == "B" }

	subA, err := streamer.Subscribe(ctx, filterA)
	if err != nil {
		t.Fatalf("subscribe A: %v", err)
	}
	subB, err := streamer.Subscribe(ctx, filterB)
	if err != nil {
		t.Fatalf("subscribe B: %v", err)
	}

	streamer.Publish(testTopic{workspace: "A"}, testPayload{ID: "1"})
	streamer.Publish(testTopic{workspace: "B"}, testPayload{ID: "2"})

	select {
	case evt := <-subA:
		if evt.Payload.ID != "1" {
			t.Fatalf("subscriber A got wrong event: %+v", evt)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber A did not receive event")
	}

	select {
	case evt := <-subB:
		if evt.Payload.ID != "2" {
			t.Fatalf("subscriber B got wrong event: %+v", evt)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber B did not receive event")
	}
}

func TestInMemorySyncStreamer_ContextCancellation(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[testTopic, testPayload]()
	t.Cleanup(streamer.Shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := streamer.Subscribe(ctx, allowAll)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to close after cancellation")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("channel did not close after cancellation")
	}
}

func TestInMemorySyncStreamer_Snapshot(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[testTopic, testPayload]()
	t.Cleanup(streamer.Shutdown)

	snapshot := func(context.Context) ([]eventstream.Event[testTopic, testPayload], error) {
		return []eventstream.Event[testTopic, testPayload]{
			{Topic: testTopic{workspace: "A"}, Payload: testPayload{ID: "snap"}},
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	ch, err := streamer.Subscribe(ctx, allowAll, eventstream.WithSnapshot(snapshot))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	select {
	case evt := <-ch:
		if evt.Payload.ID != "snap" {
			t.Fatalf("expected snapshot event, got %+v", evt)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive snapshot event")
	}
}

func TestInMemorySyncStreamer_SnapshotErrorIgnored(t *testing.T) {
	streamer := memory.NewInMemorySyncStreamer[testTopic, testPayload]()
	t.Cleanup(streamer.Shutdown)

	snapshotErr := errors.New("boom")
	snapshot := func(context.Context) ([]eventstream.Event[testTopic, testPayload], error) {
		return nil, snapshotErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	ch, err := streamer.Subscribe(ctx, allowAll, eventstream.WithSnapshot(snapshot))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	streamer.Publish(testTopic{workspace: "A"}, testPayload{ID: "live"})

	select {
	case evt := <-ch:
		if evt.Payload.ID != "live" {
			t.Fatalf("expected live event, got %+v", evt)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive live event")
	}
}
