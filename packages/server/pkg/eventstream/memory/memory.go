package memory

import (
	"context"
	"the-dev-tools/server/pkg/eventstream"
)

// NewInMemorySyncStreamer creates a new lockless in-memory sync streamer.
// It uses a single goroutine dispatcher and channels for lockless operation.
func NewInMemorySyncStreamer[T any]() eventstream.SyncStreamer[T] {
	s := &inMemorySyncStreamer[T]{
		subscribers: make(chan chan T),
		broadcast:   make(chan T, 100),
		done:        make(chan struct{}),
	}

	go s.dispatch()
	return s
}

// inMemorySyncStreamer is a lockless implementation using channels and a single dispatcher goroutine.
type inMemorySyncStreamer[T any] struct {
	subscribers chan chan T   // Channel for adding new subscribers
	broadcast   chan T        // Single broadcast channel
	done        chan struct{} // Shutdown signal
}

func (s *inMemorySyncStreamer[T]) Subscribe(ctx context.Context) <-chan T {
	events := make(chan T, 10)

	// Register subscriber and handle cleanup
	go func() {
		defer close(events) // Always close the channel

		// Try to register subscriber
		select {
		case s.subscribers <- events:
			// Successfully registered, now wait for cleanup
			select {
			case <-ctx.Done():
				// Context cancelled - channel closed by defer
			case <-s.done:
				// Streamer shutting down - channel closed by defer
			}
		case <-s.done:
			// Streamer shutting down - channel closed by defer
		case <-ctx.Done():
			// Context cancelled - channel closed by defer
		}
	}()

	return events
}

func (s *inMemorySyncStreamer[T]) Publish(event T) {
	select {
	case s.broadcast <- event:
		// Event queued for broadcast
	case <-s.done:
		// Streamer shutting down, drop event
	}
}

func (s *inMemorySyncStreamer[T]) Shutdown() {
	close(s.done)
}

// dispatch runs in a single goroutine and handles all subscriber management and event broadcasting.
// This lockless design uses channel operations instead of mutexes.
func (s *inMemorySyncStreamer[T]) dispatch() {
	var subscribers []chan T

	for {
		select {
		case newSub, ok := <-s.subscribers:
			if !ok {
				// subscribers channel closed, shutdown
				return
			}
			// Add new subscriber
			subscribers = append(subscribers, newSub)

		case event, ok := <-s.broadcast:
			if !ok {
				// broadcast channel closed, shutdown
				return
			}

			// Broadcast event to all subscribers
			activeSubscribers := subscribers[:0]
			for _, sub := range subscribers {
				select {
				case sub <- event:
					// Event sent successfully, keep subscriber
					activeSubscribers = append(activeSubscribers, sub)
				default:
					// Slow subscriber, remove them by not adding to active list
					// Don't close here - let's subscriber goroutine close on context cancellation
				}
			}
			subscribers = activeSubscribers

		case <-s.done:
			// Shutdown signal received
			return
		}
	}
}
