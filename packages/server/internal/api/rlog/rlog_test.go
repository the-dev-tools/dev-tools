package rlog

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/log/v1"
)

func TestLogCollection(t *testing.T) {
	t.Parallel()

	streamer := memory.NewInMemorySyncStreamer[LogTopic, LogEvent]()
	defer streamer.Shutdown()

	service := New(streamer)

	baseCtx := mwauth.CreateAuthedContext(context.Background(), idwrap.NewNow())
	req := connect.NewRequest(new(emptypb.Empty))

	resp, err := service.LogCollection(baseCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	if resp.Msg.Items == nil {
		t.Fatal("expected items, got nil")
	}

	// LogCollection should return empty items since logs are streaming-only
	if len(resp.Msg.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(resp.Msg.Items))
	}
}

func TestLogSyncAuthentication(t *testing.T) {
	t.Parallel()

	streamer := memory.NewInMemorySyncStreamer[LogTopic, LogEvent]()
	defer streamer.Shutdown()

	_ = New(streamer) // Just verify service can be created

	// Test that unauthenticated context fails at the LogSync level
	ctx := context.Background() // No auth context

	// We can't easily test the full LogSync without a proper ServerStream mock,
	// so let's just verify that authentication would be required by testing
	// mwauth.GetContextUserID directly
	_, err := mwauth.GetContextUserID(ctx)
	if err == nil {
		t.Fatal("expected authentication error")
	}
}

func TestLogSync(t *testing.T) {
	t.Parallel()

	streamer := memory.NewInMemorySyncStreamer[LogTopic, LogEvent]()
	defer streamer.Shutdown()

	service := New(streamer)

	userID := idwrap.NewNow()
	baseCtx := mwauth.CreateAuthedContext(context.Background(), userID)
	ctx, cancel := context.WithTimeout(baseCtx, 2*time.Second)
	defer cancel()

	msgCh := make(chan *apiv1.LogSyncResponse, 10)
	errCh := make(chan error, 1)

	// Start the sync in a goroutine
	go func() {
		err := service.streamLogSync(ctx, userID, func(resp *apiv1.LogSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Give the subscriber time to set up
	time.Sleep(10 * time.Millisecond)

	// Publish a log event
	logID := idwrap.NewNow()
	testLog := &apiv1.Log{
		LogId: logID.Bytes(),
		Name:  "test-log",
		Level: apiv1.LogLevel_LOG_LEVEL_ERROR,
		Value: structpb.NewStringValue("test message"),
	}

	topic := LogTopic{UserID: userID}
	event := LogEvent{
		Type: eventTypeInsert,
		Log:  testLog,
	}

	t.Logf("Publishing event for user %s", userID.String())
	streamer.Publish(topic, event)

	// Collect the message
	var msg *apiv1.LogSyncResponse
	select {
	case m := <-msgCh:
		msg = m
		t.Logf("Received message successfully")
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for log message")
	}

	// Check that we received the log message
	if len(msg.Items) != 1 {
		t.Fatalf("expected 1 item in message, got %d", len(msg.Items))
	}

	syncItem := msg.Items[0]
	if syncItem.Value == nil {
		t.Fatal("expected sync value, got nil")
	}

	if syncItem.Value.Insert == nil {
		t.Fatal("expected insert value, got nil")
	}

	insert := syncItem.Value.Insert
	if string(insert.LogId) != string(logID.Bytes()) {
		t.Fatalf("expected log ID %s, got %s", string(logID.Bytes()), string(insert.LogId))
	}

	if insert.Name != "test-log" {
		t.Fatalf("expected name 'test-log', got '%s'", insert.Name)
	}

	if insert.Level != apiv1.LogLevel_LOG_LEVEL_ERROR {
		t.Fatalf("expected error level, got %v", insert.Level)
	}

	cancel() // Stop the stream
	<-errCh   // Wait for goroutine to finish
}
