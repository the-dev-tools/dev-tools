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
		Type: EventTypeInsert,
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
	<-errCh  // Wait for goroutine to finish
}

func TestNewLogValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{
			name:    "int slice (original bug case)",
			input:   map[string]any{"iteration_path": []int{1, 2, 3}},
			wantErr: false,
		},
		{
			name:    "string slice",
			input:   map[string]any{"tags": []string{"tag1", "tag2"}},
			wantErr: false,
		},
		{
			name:    "nested structure",
			input:   map[string]any{"nested": map[string]any{"inner": []int{1, 2}}},
			wantErr: false,
		},
		{
			name:    "nil value",
			input:   map[string]any{"value": nil},
			wantErr: false,
		},
		{
			name:    "already compatible types",
			input:   map[string]any{"str": "hello", "num": 42, "bool": true},
			wantErr: false,
		},
		{
			name:    "mixed types",
			input:   map[string]any{"int_slice": []int{1, 2}, "str": "test", "num": 123},
			wantErr: false,
		},
		{
			name:    "empty slice",
			input:   map[string]any{"empty": []int{}},
			wantErr: false,
		},
		{
			name:    "array of ints",
			input:   map[string]any{"arr": [3]int{1, 2, 3}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := NewLogValue(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLogValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && val == nil {
				t.Error("NewLogValue() returned nil value without error")
			}
		})
	}
}

func TestMakeProtoCompatible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		validate func(t *testing.T, result any)
	}{
		{
			name:  "converts []int to []any",
			input: []int{1, 2, 3},
			validate: func(t *testing.T, result any) {
				slice, ok := result.([]any)
				if !ok {
					t.Fatalf("expected []any, got %T", result)
				}
				if len(slice) != 3 {
					t.Fatalf("expected length 3, got %d", len(slice))
				}
				if slice[0] != 1 || slice[1] != 2 || slice[2] != 3 {
					t.Errorf("unexpected values: %v", slice)
				}
			},
		},
		{
			name:  "converts nested []int",
			input: map[string]any{"outer": []int{1, 2}},
			validate: func(t *testing.T, result any) {
				m, ok := result.(map[string]any)
				if !ok {
					t.Fatalf("expected map[string]any, got %T", result)
				}
				inner, ok := m["outer"].([]any)
				if !ok {
					t.Fatalf("expected []any for 'outer', got %T", m["outer"])
				}
				if len(inner) != 2 {
					t.Fatalf("expected length 2, got %d", len(inner))
				}
			},
		},
		{
			name:  "handles nil",
			input: nil,
			validate: func(t *testing.T, result any) {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			},
		},
		{
			name:  "preserves strings",
			input: "hello",
			validate: func(t *testing.T, result any) {
				if result != "hello" {
					t.Errorf("expected 'hello', got %v", result)
				}
			},
		},
		{
			name:  "preserves numbers",
			input: 42,
			validate: func(t *testing.T, result any) {
				if result != 42 {
					t.Errorf("expected 42, got %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := makeProtoCompatible(tt.input)
			tt.validate(t, result)
		})
	}
}

// TestNewLogValueRealWorldCase tests the exact scenario from the bug
func TestNewLogValueRealWorldCase(t *testing.T) {
	t.Parallel()

	// Simulate the exact data structure that was causing the error
	logData := map[string]any{
		"node_id":     "test-node-123",
		"node_name":   "Test Node",
		"state":       "SUCCESS",
		"flow_id":     "test-flow-456",
		"duration_ms": int64(150),
		// This was causing the error: proto: invalid type: []int
		"iteration_path":  []int{1, 2, 3},
		"iteration_index": 2,
	}

	// This should not error anymore
	val, err := NewLogValue(logData)
	if err != nil {
		t.Fatalf("NewLogValue() failed with error: %v", err)
	}

	if val == nil {
		t.Fatal("NewLogValue() returned nil value")
	}

	// Verify it's actually a valid structpb.Value
	if val.GetStructValue() == nil {
		t.Error("expected struct value, got nil")
	}
}
