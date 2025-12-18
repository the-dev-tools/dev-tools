package eventstream

import (
	"context"
	"testing"
	"time"
)

func TestStreamToClientBulk(t *testing.T) {
	type TestTopic string
	type TestPayload string
	type TestResponse struct {
		Values []string
	}

	t.Run("Flush by size and close", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		sent := make([]*TestResponse, 0)
		send := func(r *TestResponse) error {
			sent = append(sent, r)
			return nil
		}

		mockStreamer := &MockStreamer[TestTopic, TestPayload]{
			subscribeFunc: func(ctx context.Context, filter TopicFilter[TestTopic], opts ...SubscribeOption[TestTopic, TestPayload]) (<-chan Event[TestTopic, TestPayload], error) {
				ch := make(chan Event[TestTopic, TestPayload], 5)
				// Send 3 events (BatchSize=2, so expect [1,2] then [3] on close)
				ch <- Event[TestTopic, TestPayload]{Payload: "1"}
				ch <- Event[TestTopic, TestPayload]{Payload: "2"}
				ch <- Event[TestTopic, TestPayload]{Payload: "3"}
				close(ch)
				return ch, nil
			},
		}

		convert := func(payloads []TestPayload) *TestResponse {
			vals := make([]string, len(payloads))
			for i, p := range payloads {
				vals[i] = string(p)
			}
			return &TestResponse{Values: vals}
		}

		opts := &BulkOptions{
			MaxBatchSize:  2,
			FlushInterval: 1 * time.Hour, // Long interval
		}

		// Changed StreamToClientBulk to StreamToClient
		err := StreamToClient(ctx, mockStreamer, nil, nil, convert, send, opts)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(sent) != 2 {
			t.Fatalf("Expected 2 batches, got %d", len(sent))
		}
		if len(sent[0].Values) != 2 || sent[0].Values[0] != "1" || sent[0].Values[1] != "2" {
			t.Errorf("First batch incorrect: %v", sent[0].Values)
		}
		if len(sent[1].Values) != 1 || sent[1].Values[0] != "3" {
			t.Errorf("Second batch incorrect: %v", sent[1].Values)
		}
	})

	t.Run("Flush by interval", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		sent := make([]*TestResponse, 0)
		send := func(r *TestResponse) error {
			sent = append(sent, r)
			if len(sent) == 1 {
				cancel()
			}
			return nil
		}

		mockStreamer := &MockStreamer[TestTopic, TestPayload]{
			subscribeFunc: func(ctx context.Context, filter TopicFilter[TestTopic], opts ...SubscribeOption[TestTopic, TestPayload]) (<-chan Event[TestTopic, TestPayload], error) {
				ch := make(chan Event[TestTopic, TestPayload], 1)
				ch <- Event[TestTopic, TestPayload]{Payload: "1"}
				// Don't close, just wait for interval flush
				return ch, nil
			},
		}

		convert := func(payloads []TestPayload) *TestResponse {
			vals := make([]string, len(payloads))
			for i, p := range payloads {
				vals[i] = string(p)
			}
			return &TestResponse{Values: vals}
		}

		opts := &BulkOptions{
			MaxBatchSize:  10,
			FlushInterval: 10 * time.Millisecond, // Short interval
		}

		// Changed StreamToClientBulk to StreamToClient
		err := StreamToClient(ctx, mockStreamer, nil, nil, convert, send, opts)
		// Context canceled is expected
		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(sent) != 1 {
			t.Fatalf("Expected 1 batch, got %d", len(sent))
		}
		if len(sent[0].Values) != 1 || sent[0].Values[0] != "1" {
			t.Errorf("Batch incorrect: %v", sent[0].Values)
		}
	})
}
