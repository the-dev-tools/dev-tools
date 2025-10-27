package eventstream

import "context"

// SyncStreamer defines a generic, lockless interface for real-time event streaming.
// It uses Go generics for compile-time type safety and channels for lockless operation.
//
// Example usage:
//
//	type MyEvent struct { ID string; Data string }
//	streamer := memory.NewInMemorySyncStreamer[MyEvent]()
//	events := streamer.Subscribe(ctx)
//	streamer.Publish(MyEvent{ID: "1", Data: "hello"})
//	defer streamer.Shutdown() // Clean up resources
type SyncStreamer[T any] interface {
	// Subscribe returns a read-only channel for receiving events.
	// The channel is closed when the context is cancelled or streamer shuts down.
	Subscribe(ctx context.Context) <-chan T

	// Publish sends an event to all active subscribers.
	// Non-blocking - events are dropped if broadcast buffer is full.
	Publish(event T)

	// Shutdown gracefully shuts down the streamer and closes all subscriber channels.
	Shutdown()
}

// Event represents a generic sync event with metadata.
// Use this as a base type or create your own event types.
type Event[T any] struct {
	Type      string // "create", "update", "delete"
	Entity    string // "environment", "flow", "user", etc.
	UserID    string // User who triggered the event
	Workspace string // Workspace context for filtering
	Data      T      // Generic event data
}
