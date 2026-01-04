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

// SyncStreamer defines a generic interface for real-time streaming of
// domain events. Publishers provide a Topic and Payload when emitting
// changes. Subscribers specify a filter that decides which topics they
// are interested in.
type SyncStreamer[Topic any, Payload any] interface {
	Publish(topic Topic, payloads ...Payload)

	Subscribe(ctx context.Context, filter TopicFilter[Topic]) (<-chan Event[Topic, Payload], error)

	Shutdown()
}

