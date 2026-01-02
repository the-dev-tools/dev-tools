package eventsync

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"the-dev-tools/server/pkg/eventstream"
)

// PublishFunc is a function that publishes an event.
type PublishFunc func()

// EventEntry represents a single event to be published.
type EventEntry struct {
	Kind     EventKind
	SubOrder int // For ordering within the same kind (e.g., node graph level)
	Publish  PublishFunc
}

// EventBatch collects events and publishes them in dependency order.
// Events can be added in any order - the batch sorts them before publishing.
//
// Usage:
//
//	batch := eventsync.NewEventBatch()
//	batch.Add(eventsync.KindFlow, 0, func() { publishFlow(...) })
//	batch.Add(eventsync.KindNode, nodeLevel, func() { publishNode(...) })
//	batch.Add(eventsync.KindFlowFile, 0, func() { publishFlowFile(...) })
//	batch.Publish() // Publishes in correct order: Flow, FlowFile, Node, ...
type EventBatch struct {
	mu      sync.Mutex
	entries []EventEntry
}

// NewEventBatch creates a new empty event batch.
func NewEventBatch() *EventBatch {
	return &EventBatch{
		entries: make([]EventEntry, 0, 32), // Pre-allocate for typical batch size
	}
}

// Add queues one or more events for later publishing.
// subOrder is used for ordering within the same event kind (e.g., node graph level).
// Lower subOrder values are published first.
func (b *EventBatch) Add(kind EventKind, subOrder int, publish ...PublishFunc) {
	if len(publish) == 0 {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, f := range publish {
		b.entries = append(b.entries, EventEntry{
			Kind:     kind,
			SubOrder: subOrder,
			Publish:  f,
		})
	}
}

// AddSimple queues one or more events with default subOrder (0).
func (b *EventBatch) AddSimple(kind EventKind, publish ...PublishFunc) {
	b.Add(kind, 0, publish...)
}

// Len returns the number of queued events.
func (b *EventBatch) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.entries)
}

// Publish executes all events in dependency-sorted order.
// After publishing, the batch is cleared.
//
// The order is determined by:
// 1. EventKind priority (from topological sort of Dependencies)
// 2. SubOrder (lower values first, for ordering within same kind)
func (b *EventBatch) Publish(ctx context.Context) error {
	b.mu.Lock()
	entries := b.entries
	b.entries = nil // Let GC handle old slice, reset for reuse
	b.mu.Unlock()

	if len(entries) == 0 {
		return nil
	}

	// Sort entries by kind priority, then by subOrder
	slices.SortStableFunc(entries, func(a, b EventEntry) int {
		priA := GetEventPriority(a.Kind)
		priB := GetEventPriority(b.Kind)
		if priA != priB {
			return cmp.Compare(priA, priB)
		}
		return cmp.Compare(a.SubOrder, b.SubOrder)
	})

	// Execute in sorted order, checking context for cancellation
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry.Publish()
	}

	return nil
}

// Clear removes all queued events without publishing.
func (b *EventBatch) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = b.entries[:0]
}

// GetOrderedKinds returns the event kinds in the batch in sorted order.
// Useful for debugging and testing.
func (b *EventBatch) GetOrderedKinds() []EventKind {
	b.mu.Lock()
	entries := slices.Clone(b.entries)
	b.mu.Unlock()

	// Sort
	slices.SortStableFunc(entries, func(a, b EventEntry) int {
		priA := GetEventPriority(a.Kind)
		priB := GetEventPriority(b.Kind)
		if priA != priB {
			return cmp.Compare(priA, priB)
		}
		return cmp.Compare(a.SubOrder, b.SubOrder)
	})

	// Extract kinds (with dedup for debugging clarity)
	result := make([]EventKind, 0, len(entries))
	seen := make(map[EventKind]bool)
	for _, e := range entries {
		if !seen[e.Kind] {
			result = append(result, e.Kind)
			seen[e.Kind] = true
		}
	}
	return result
}

// AddSync adds multiple payloads to the batch for a specific streamer and topic.
// This handles the closure wrapping and capture logic automatically.
func AddSync[Topic any, Payload any](
	batch *EventBatch,
	kind EventKind,
	subOrder int,
	streamer eventstream.SyncStreamer[Topic, Payload],
	topic Topic,
	payloads ...Payload,
) {
	for _, p := range payloads {
		batch.Add(kind, subOrder, func() {
			streamer.Publish(topic, p)
		})
	}
}

// AddSyncSimple is AddSync with default subOrder (0).
func AddSyncSimple[Topic any, Payload any](
	batch *EventBatch,
	kind EventKind,
	streamer eventstream.SyncStreamer[Topic, Payload],
	topic Topic,
	payloads ...Payload,
) {
	AddSync(batch, kind, 0, streamer, topic, payloads...)
}

// AddSyncTransform adds multiple items to the batch after transforming them into payloads.
// This is ideal for converting internal models to API events during bulk operations.
func AddSyncTransform[T any, Topic any, Payload any](
	batch *EventBatch,
	kind EventKind,
	subOrder int,
	streamer eventstream.SyncStreamer[Topic, Payload],
	topic Topic,
	items []T,
	transform func(T) Payload,
) {
	for _, item := range items {
		batch.Add(kind, subOrder, func() {
			batch.mu.Lock()
			payload := transform(item)
			batch.mu.Unlock()
			streamer.Publish(topic, payload)
		})
	}
}

// AddSyncTransformSimple is AddSyncTransform with default subOrder (0).
func AddSyncTransformSimple[T any, Topic any, Payload any](
	batch *EventBatch,
	kind EventKind,
	streamer eventstream.SyncStreamer[Topic, Payload],
	topic Topic,
	items []T,
	transform func(T) Payload,
) {
	AddSyncTransform(batch, kind, 0, streamer, topic, items, transform)
}
