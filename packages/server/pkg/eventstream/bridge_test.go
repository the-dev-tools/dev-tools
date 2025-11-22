package eventstream

import (
	"context"
	"errors"
	"testing"
)

// MockStreamer implements SyncStreamer for testing
type MockStreamer[Topic any, Payload any] struct {
	subscribeFunc func(ctx context.Context, filter TopicFilter[Topic], opts ...SubscribeOption[Topic, Payload]) (<-chan Event[Topic, Payload], error)
}

func (m *MockStreamer[Topic, Payload]) Publish(topic Topic, payload Payload) {}
func (m *MockStreamer[Topic, Payload]) Shutdown()                            {}
func (m *MockStreamer[Topic, Payload]) Subscribe(ctx context.Context, filter TopicFilter[Topic], opts ...SubscribeOption[Topic, Payload]) (<-chan Event[Topic, Payload], error) {
	return m.subscribeFunc(ctx, filter, opts...)
}

func TestStreamToClient(t *testing.T) {
	type TestTopic string
	type TestPayload string
	type TestResponse struct {
		Value string
	}

	t.Run("Success flow with snapshot and events", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Mock sender
		sent := make([]*TestResponse, 0)
		send := func(r *TestResponse) error {
			sent = append(sent, r)
			if len(sent) == 2 {
				cancel() // Stop after receiving snapshot + 1 event
			}
			return nil
		}

		// Mock streamer
		mockStreamer := &MockStreamer[TestTopic, TestPayload]{
			subscribeFunc: func(ctx context.Context, filter TopicFilter[TestTopic], opts ...SubscribeOption[TestTopic, TestPayload]) (<-chan Event[TestTopic, TestPayload], error) {
				ch := make(chan Event[TestTopic, TestPayload], 1)
				
				// Execute snapshot logic if provided
				options := &SubscribeOptions[TestTopic, TestPayload]{}
				for _, opt := range opts {
					opt(options)
				}
				if options.Snapshot != nil {
					events, _ := options.Snapshot(ctx)
					for _, evt := range events {
						// Send snapshot via sender manually for this test setup or 
						// typically snapshot logic is internal to Subscribe implementation.
						// Here we simulate that Subscribe returns a channel that first yields snapshot items (if any) 
						// effectively, or that the caller processes snapshot.
						// Actually StreamToClient relies on Subscribe returning a channel that delivers events.
						// The real memory implementation delivers snapshot items first.
						// So we simulate that:
						ch <- evt
					}
				}

				// Simulate async event after snapshot
				go func() {
					ch <- Event[TestTopic, TestPayload]{Payload: "event1"}
				}()
				
				return ch, nil
			},
		}

		snapshot := func(ctx context.Context) ([]Event[TestTopic, TestPayload], error) {
			return []Event[TestTopic, TestPayload]{{Payload: "snapshot1"}}, nil
		}

		convert := func(p TestPayload) *TestResponse {
			return &TestResponse{Value: string(p)}
		}

		err := StreamToClient(ctx, mockStreamer, snapshot, nil, convert, send)
		
		// Expect context cancelled error or nil depending on race
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(sent) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(sent))
		}
		if sent[0].Value != "snapshot1" {
			t.Errorf("Expected snapshot1, got %s", sent[0].Value)
		}
		if sent[1].Value != "event1" {
			t.Errorf("Expected event1, got %s", sent[1].Value)
		}
	})

	t.Run("Subscribe error", func(t *testing.T) {
		expectedErr := errors.New("subscribe failed")
		mockStreamer := &MockStreamer[TestTopic, TestPayload]{
			subscribeFunc: func(ctx context.Context, filter TopicFilter[TestTopic], opts ...SubscribeOption[TestTopic, TestPayload]) (<-chan Event[TestTopic, TestPayload], error) {
				return nil, expectedErr
			},
		}

		var snapshot SnapshotProvider[TestTopic, TestPayload] = nil
		var convert func(TestPayload) *TestResponse = nil
		var send func(*TestResponse) error = nil

		err := StreamToClient(context.Background(), mockStreamer, snapshot, nil, convert, send)
		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("Send error stops loop", func(t *testing.T) {
		mockStreamer := &MockStreamer[TestTopic, TestPayload]{
			subscribeFunc: func(ctx context.Context, filter TopicFilter[TestTopic], opts ...SubscribeOption[TestTopic, TestPayload]) (<-chan Event[TestTopic, TestPayload], error) {
				ch := make(chan Event[TestTopic, TestPayload], 1)
				ch <- Event[TestTopic, TestPayload]{Payload: "event1"}
				return ch, nil
			},
		}

		sendErr := errors.New("send failed")
		send := func(r *TestResponse) error {
			return sendErr
		}
		
		convert := func(p TestPayload) *TestResponse { return &TestResponse{} }
		var snapshot SnapshotProvider[TestTopic, TestPayload] = nil

		err := StreamToClient(context.Background(), mockStreamer, snapshot, nil, convert, send)
		if err != sendErr {
			t.Errorf("Expected error %v, got %v", sendErr, err)
		}
	})
}
