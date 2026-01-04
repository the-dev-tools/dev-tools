package eventstream

import (
	"context"
	"errors"
	"testing"
)

// MockStreamer implements SyncStreamer for testing
type MockStreamer[Topic any, Payload any] struct {
	subscribeFunc func(ctx context.Context, filter TopicFilter[Topic]) (<-chan Event[Topic, Payload], error)
}

func (m *MockStreamer[Topic, Payload]) Publish(topic Topic, payloads ...Payload) {}
func (m *MockStreamer[Topic, Payload]) Shutdown()                                {}
func (m *MockStreamer[Topic, Payload]) Subscribe(ctx context.Context, filter TopicFilter[Topic]) (<-chan Event[Topic, Payload], error) {
	return m.subscribeFunc(ctx, filter)
}

func TestStreamToClient(t *testing.T) {
	type TestTopic string
	type TestPayload string
	type TestResponse struct {
		Value string
	}

	t.Run("Success flow with events", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Mock sender
		sent := make([]*TestResponse, 0)
		send := func(r *TestResponse) error {
			sent = append(sent, r)
			if len(sent) == 1 {
				cancel() // Stop after receiving 1 event
			}
			return nil
		}

		// Mock streamer
		mockStreamer := &MockStreamer[TestTopic, TestPayload]{
			subscribeFunc: func(ctx context.Context, filter TopicFilter[TestTopic]) (<-chan Event[TestTopic, TestPayload], error) {
				ch := make(chan Event[TestTopic, TestPayload], 1)

				// Simulate async event
				go func() {
					ch <- Event[TestTopic, TestPayload]{Payload: "event1"}
				}()

				return ch, nil
			},
		}

		// Updated convert to handle bulk slice
		convert := func(payloads []TestPayload) *TestResponse {
			if len(payloads) > 0 {
				return &TestResponse{Value: string(payloads[0])}
			}
			return nil
		}

		// Set max batch size to 1 to force immediate flush for each item
		opts := &BulkOptions{MaxBatchSize: 1}

		err := StreamToClient(ctx, mockStreamer, nil, convert, send, opts)

		// Expect context cancelled error or nil depending on race
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(sent) != 1 {
			t.Errorf("Expected 1 message, got %d", len(sent))
		}
		if len(sent) > 0 && sent[0].Value != "event1" {
			t.Errorf("Expected event1, got %s", sent[0].Value)
		}
	})

	t.Run("Subscribe error", func(t *testing.T) {
		expectedErr := errors.New("subscribe failed")
		mockStreamer := &MockStreamer[TestTopic, TestPayload]{
			subscribeFunc: func(ctx context.Context, filter TopicFilter[TestTopic]) (<-chan Event[TestTopic, TestPayload], error) {
				return nil, expectedErr
			},
		}

		var convert func([]TestPayload) *TestResponse = nil
		var send func(*TestResponse) error = nil

		err := StreamToClient(context.Background(), mockStreamer, nil, convert, send, nil)
		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("Send error stops loop", func(t *testing.T) {
		mockStreamer := &MockStreamer[TestTopic, TestPayload]{
			subscribeFunc: func(ctx context.Context, filter TopicFilter[TestTopic]) (<-chan Event[TestTopic, TestPayload], error) {
				ch := make(chan Event[TestTopic, TestPayload], 1)
				ch <- Event[TestTopic, TestPayload]{Payload: "event1"}
				return ch, nil
			},
		}

		sendErr := errors.New("send failed")
		send := func(r *TestResponse) error {
			return sendErr
		}

		convert := func(p []TestPayload) *TestResponse { return &TestResponse{} }

		// Use batch size 1 to force flush
		opts := &BulkOptions{MaxBatchSize: 1}

		err := StreamToClient(context.Background(), mockStreamer, nil, convert, send, opts)
		if err != sendErr {
			t.Errorf("Expected error %v, got %v", sendErr, err)
		}
	})
}
