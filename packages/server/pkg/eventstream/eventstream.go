package eventstream

import "context"

// TopicFilter is used to decide whether a subscriber should receive
// an event published on the given topic. Returning true delivers the
// event to the subscriber.
type TopicFilter[Topic any] func(topic Topic) bool

// Event is the message delivered to subscribers. It contains the
// topic metadata alongside the payload so handlers can reason about
// ordering and provenance.
type Event[Topic any, Payload any] struct {
	Topic   Topic
	Payload Payload
}

// SnapshotProvider can be supplied when subscribing so that callers
// receive an initial view of the data before live updates arrive. The
// returned events should already be filtered to those the subscriber
// is allowed to see.
type SnapshotProvider[Topic any, Payload any] func(ctx context.Context) ([]Event[Topic, Payload], error)

// SubscribeOptions controls optional behaviour when subscribing to a
// stream.
type SubscribeOptions[Topic any, Payload any] struct {
	Snapshot SnapshotProvider[Topic, Payload]
}

// SubscribeOption mutates SubscribeOptions.
type SubscribeOption[Topic any, Payload any] func(*SubscribeOptions[Topic, Payload])

// WithSnapshot registers a snapshot provider that will be invoked once
// the subscription is established. The resulting events are delivered
// to the subscriber before live updates.
func WithSnapshot[Topic any, Payload any](provider SnapshotProvider[Topic, Payload]) SubscribeOption[Topic, Payload] {
	return func(cfg *SubscribeOptions[Topic, Payload]) {
		cfg.Snapshot = provider
	}
}

// SyncStreamer defines a generic interface for real-time streaming of
// domain events. Publishers provide a Topic and Payload when emitting
// changes. Subscribers specify a filter that decides which topics they
// are interested in and optionally provide a snapshot to seed their
// initial state.
type SyncStreamer[Topic any, Payload any] interface {
	Publish(topic Topic, payload Payload)

	Subscribe(ctx context.Context, filter TopicFilter[Topic], opts ...SubscribeOption[Topic, Payload]) (<-chan Event[Topic, Payload], error)

	Shutdown()
}
