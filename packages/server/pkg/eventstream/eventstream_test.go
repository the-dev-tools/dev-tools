package eventstream_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
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
	require.NoError(t, err, "subscribe")

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
	require.NoError(t, err, "subscribe A")
	subB, err := streamer.Subscribe(ctx, filterB)
	require.NoError(t, err, "subscribe B")

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
	require.NoError(t, err, "subscribe")

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
